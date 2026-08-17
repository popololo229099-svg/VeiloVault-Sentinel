package concurrency

import (
	"context"
	"sync"
	"time"
)

type WorkerPoolConfig struct {
	Workers     int
	QueueSize   int
	MaxRetries  int
	IdleTimeout time.Duration
}

type ManagedWorkerPool struct {
	config    WorkerPoolConfig
	jobs      chan WorkerJob
	results   chan WorkerResult
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	semaphore *Semaphore
	metrics   *PoolMetrics
}

type WorkerJob struct {
	ID      string
	Payload interface{}
	Fn      func(ctx context.Context, payload interface{}) (interface{}, error)
}

type WorkerResult struct {
	JobID  string
	Result interface{}
	Error  error
}

type PoolMetrics struct {
	TotalJobs    int64
	CompletedJobs int64
	FailedJobs    int64
	ActiveWorkers int64
	QueueLength   int64
}

func NewManagedWorkerPool(cfg WorkerPoolConfig) *ManagedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &ManagedWorkerPool{
		config:    cfg,
		jobs:      make(chan WorkerJob, cfg.QueueSize),
		results:   make(chan WorkerResult, cfg.QueueSize),
		ctx:       ctx,
		cancel:    cancel,
		semaphore: NewSemaphore(cfg.Workers),
		metrics:   &PoolMetrics{},
	}
}

func (p *ManagedWorkerPool) Start() {
	for i := 0; i < p.config.Workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *ManagedWorkerPool) worker(id int) {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.semaphore.Acquire()
			result := p.executeJob(job)
			p.semaphore.Release()
			select {
			case p.results <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

func (p *ManagedWorkerPool) executeJob(job WorkerJob) WorkerResult {
	deadline := time.After(p.config.IdleTimeout)
	resultCh := make(chan WorkerResult, 1)
	go func() {
		result, err := job.Fn(p.ctx, job.Payload)
		resultCh <- WorkerResult{JobID: job.ID, Result: result, Error: err}
	}()
	select {
	case <-deadline:
		return WorkerResult{JobID: job.ID, Error: context.DeadlineExceeded}
	case r := <-resultCh:
		return r
	}
}

func (p *ManagedWorkerPool) Submit(job WorkerJob) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *ManagedWorkerPool) Results() <-chan WorkerResult {
	return p.results
}

func (p *ManagedWorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.jobs)
	close(p.results)
}

func (p *ManagedWorkerPool) Stats() PoolMetrics {
	return PoolMetrics{
		TotalJobs:    p.metrics.TotalJobs,
		CompletedJobs: p.metrics.CompletedJobs,
		FailedJobs:    p.metrics.FailedJobs,
		ActiveWorkers: int64(p.semaphore.Active()),
		QueueLength:   int64(len(p.jobs)),
	}
}

type WorkStealingPool struct {
	workers   []*StealingWorker
	mu        sync.RWMutex
}

type StealingWorker struct {
	ID      int
	queue   chan func()
	balance int64
}

func NewWorkStealingPool(workers int, queueSize int) *WorkStealingPool {
	pool := &WorkStealingPool{
		workers: make([]*StealingWorker, workers),
	}
	for i := 0; i < workers; i++ {
		pool.workers[i] = &StealingWorker{
			ID:    i,
			queue: make(chan func(), queueSize),
		}
		go pool.workers[i].run()
	}
	return pool
}

func (p *WorkStealingPool) Submit(fn func()) {
	minQueue := p.workers[0]
	for _, w := range p.workers[1:] {
		if len(w.queue) < len(minQueue.queue) {
			minQueue = w
		}
	}
	minQueue.queue <- fn
}

func (p *WorkStealingPool) Stop() {
	for _, w := range p.workers {
		close(w.queue)
	}
}

func (w *StealingWorker) run() {
	for fn := range w.queue {
		fn()
	}
}

type ParallelFor struct {
	goroutines int
}

func NewParallelFor(goroutines int) *ParallelFor {
	if goroutines <= 0 {
		goroutines = 4
	}
	return &ParallelFor{goroutines: goroutines}
}

func (pf *ParallelFor) Execute(n int, fn func(i int)) {
	chunkSize := (n + pf.goroutines - 1) / pf.goroutines
	var wg sync.WaitGroup
	for i := 0; i < pf.goroutines; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= n {
			break
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for j := s; j < e; j++ {
				fn(j)
			}
		}(start, end)
	}
	wg.Wait()
}

type Memoize struct {
	cache sync.Map
}

func NewMemoize() *Memoize {
	return &Memoize{}
}

func (m *Memoize) GetOrCompute(key string, compute func() (interface{}, error)) (interface{}, error) {
	if val, ok := m.cache.Load(key); ok {
		return val, nil
	}
	val, err := compute()
	if err != nil {
		return nil, err
	}
	m.cache.Store(key, val)
	return val, nil
}

func (m *Memoize) Invalidate(key string) {
	m.cache.Delete(key)
}

func (m *Memoize) Clear() {
	m.cache.Range(func(key, value interface{}) bool {
		m.cache.Delete(key)
		return true
	})
}

type RequestDeduplicator struct {
	inflight sync.Map
	mu       sync.Mutex
}

func NewRequestDeduplicator() *RequestDeduplicator {
	return &RequestDeduplicator{}
}

func (rd *RequestDeduplicator) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	ch := make(chan struct{}, 1)
	actual, loaded := rd.inflight.LoadOrStore(key, ch)
	if loaded {
		existing := actual.(chan struct{})
		<-existing
		return nil, nil
	}
	defer func() {
		rd.inflight.Delete(key)
		close(ch)
	}()
	return fn()
}

func (rd *RequestDeduplicator) InflightCount() int {
	count := 0
	rd.inflight.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
