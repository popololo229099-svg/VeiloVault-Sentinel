package concurrency

import (
	"context"
	"sync"
	"time"
)

type WorkerPoolAdvanced struct {
	config     WorkerPoolConfig
	jobCh      chan WorkerJob
	resultCh   chan WorkerResult
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	workers    []*Worker
	metrics    *WorkerPoolMetrics
	semaphore  *Semaphore
}

type Worker struct {
	ID       int
	active   bool
	jobsDone int64
	mu       sync.Mutex
}

type WorkerPoolMetrics struct {
	TotalJobs     int64
	CompletedJobs int64
	FailedJobs    int64
	ActiveWorkers int
	QueueLength   int
	AvgJobTime    time.Duration
	mu            sync.Mutex
}

func NewWorkerPoolAdvanced(cfg WorkerPoolConfig) *WorkerPoolAdvanced {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPoolAdvanced{
		config:    cfg,
		jobCh:     make(chan WorkerJob, cfg.QueueSize),
		resultCh:  make(chan WorkerResult, cfg.QueueSize),
		ctx:       ctx,
		cancel:    cancel,
		workers:   make([]*Worker, cfg.Workers),
		metrics:   &WorkerPoolMetrics{},
		semaphore: NewSemaphore(cfg.Workers),
	}
}

func (p *WorkerPoolAdvanced) Start() {
	for i := 0; i < p.config.Workers; i++ {
		p.workers[i] = &Worker{ID: i}
		p.wg.Add(1)
		go p.workerLoop(p.workers[i])
	}
}

func (p *WorkerPoolAdvanced) workerLoop(w *Worker) {
	defer p.wg.Done()
	w.mu.Lock()
	w.active = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.active = false
		w.mu.Unlock()
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobCh:
			if !ok {
				return
			}
			p.semaphore.Acquire()
			start := time.Now()
			result := p.executeJob(job)
			duration := time.Since(start)
			p.semaphore.Release()

			w.mu.Lock()
			w.jobsDone++
			w.mu.Unlock()

			p.metrics.mu.Lock()
			p.metrics.TotalJobs++
			if result.Error != nil {
				p.metrics.FailedJobs++
			} else {
				p.metrics.CompletedJobs++
			}
			p.metrics.AvgJobTime = (p.metrics.AvgJobTime * time.Duration(p.metrics.TotalJobs-1) + duration) / time.Duration(p.metrics.TotalJobs)
			p.metrics.mu.Unlock()

			select {
			case p.resultCh <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

func (p *WorkerPoolAdvanced) executeJob(job WorkerJob) WorkerResult {
	if p.config.MaxRetries > 0 {
		var lastErr error
		for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
			result, err := job.Fn(p.ctx, job.Payload)
			if err == nil {
				return WorkerResult{JobID: job.ID, Result: result}
			}
			lastErr = err
			if attempt < p.config.MaxRetries {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
		}
		return WorkerResult{JobID: job.ID, Error: lastErr}
	}
	result, err := job.Fn(p.ctx, job.Payload)
	return WorkerResult{JobID: job.ID, Result: result, Error: err}
}

func (p *WorkerPoolAdvanced) Submit(job WorkerJob) error {
	select {
	case p.jobCh <- job:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *WorkerPoolAdvanced) Results() <-chan WorkerResult {
	return p.resultCh
}

func (p *WorkerPoolAdvanced) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.jobCh)
	close(p.resultCh)
}

func (p *WorkerPoolAdvanced) Stats() WorkerPoolMetrics {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	active := 0
	for _, w := range p.workers {
		if w != nil {
			w.mu.Lock()
			if w.active {
				active++
			}
			w.mu.Unlock()
		}
	}
	return WorkerPoolMetrics{
		TotalJobs:     p.metrics.TotalJobs,
		CompletedJobs: p.metrics.CompletedJobs,
		FailedJobs:    p.metrics.FailedJobs,
		ActiveWorkers: active,
		QueueLength:   len(p.jobCh),
		AvgJobTime:    p.metrics.AvgJobTime,
	}
}

type RateLimiterAdvanced struct {
	rate       int
	burst      int
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

func NewRateLimiterAdvanced(rate, burst int) *RateLimiterAdvanced {
	return &RateLimiterAdvanced{
		rate:       rate,
		burst:      burst,
		tokens:     burst,
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiterAdvanced) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *RateLimiterAdvanced) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (rl *RateLimiterAdvanced) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	newTokens := int(float64(rl.rate) * elapsed.Seconds())
	if newTokens > 0 {
		rl.tokens += newTokens
		if rl.tokens > rl.burst {
			rl.tokens = rl.burst
		}
		rl.lastRefill = now
	}
}

func (rl *RateLimiterAdvanced) Available() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}

type SemaphoreAdvanced struct {
	tickets chan struct{}
	max     int
	active  int
	mu      sync.Mutex
}

func NewSemaphoreAdvanced(max int) *SemaphoreAdvanced {
	return &SemaphoreAdvanced{
		tickets: make(chan struct{}, max),
		max:     max,
	}
}

func (s *SemaphoreAdvanced) Acquire() {
	s.tickets <- struct{}{}
	s.mu.Lock()
	s.active++
	s.mu.Unlock()
}

func (s *SemaphoreAdvanced) Release() {
	<-s.tickets
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
}

func (s *SemaphoreAdvanced) TryAcquire() bool {
	select {
	case s.tickets <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.mu.Unlock()
		return true
	default:
		return false
	}
}

func (s *SemaphoreAdvanced) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *SemaphoreAdvanced) Capacity() int {
	return s.max
}
