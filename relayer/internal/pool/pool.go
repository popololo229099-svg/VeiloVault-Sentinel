package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

type Pool[T any] struct {
	items     chan T
	factory   func() (T, error)
	validator func(T) bool
	destroy   func(T)
	maxSize   int
	minSize   int
	stats     PoolStats
	mu        sync.RWMutex
	closed    bool
	stopCh    chan struct{}
}

type PoolConfig[T any] struct {
	Factory   func() (T, error)
	Validator func(T) bool
	Destroy   func(T)
	MaxSize   int
	MinSize   int
}

func NewPool[T any](config PoolConfig[T]) *Pool[T] {
	if config.MaxSize <= 0 {
		config.MaxSize = 100
	}
	if config.MinSize <= 0 {
		config.MinSize = 5
	}

	p := &Pool[T]{
		items:     make(chan T, config.MaxSize),
		factory:   config.Factory,
		validator: config.Validator,
		destroy:   config.Destroy,
		maxSize:   config.MaxSize,
		minSize:   config.MinSize,
		stopCh:    make(chan struct{}),
	}

	for i := 0; i < config.MinSize; i++ {
		item, err := config.Factory()
		if err == nil {
			p.items <- item
			atomic.AddInt64(&p.stats.TotalCreated, 1)
		}
	}

	return p
}

func (p *Pool[T]) Get() (T, error) {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		var zero T
		return zero, ErrPoolClosed
	}

	select {
	case item := <-p.items:
		if p.validator != nil && !p.validator(item) {
			atomic.AddInt64(&p.stats.TotalDestroyed, 1)
			if p.destroy != nil {
				p.destroy(item)
			}
			return p.create()
		}
		atomic.AddInt64(&p.stats.TotalAcquired, 1)
		return item, nil
	default:
		return p.create()
	}
}

func (p *Pool[T]) create() (T, error) {
	item, err := p.factory()
	if err != nil {
		var zero T
		return zero, err
	}
	atomic.AddInt64(&p.stats.TotalCreated, 1)
	atomic.AddInt64(&p.stats.TotalAcquired, 1)
	return item, nil
}

func (p *Pool[T]) Put(item T) {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		if p.destroy != nil {
			p.destroy(item)
		}
		return
	}

	if p.validator != nil && !p.validator(item) {
		atomic.AddInt64(&p.stats.TotalDestroyed, 1)
		if p.destroy != nil {
			p.destroy(item)
		}
		return
	}

	select {
	case p.items <- item:
		atomic.AddInt64(&p.stats.TotalReleased, 1)
	default:
		atomic.AddInt64(&p.stats.TotalDestroyed, 1)
		if p.destroy != nil {
			p.destroy(item)
		}
	}
}

func (p *Pool[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true
	close(p.stopCh)

	for {
		select {
		case item := <-p.items:
			if p.destroy != nil {
				p.destroy(item)
			}
			atomic.AddInt64(&p.stats.TotalDestroyed, 1)
		default:
			return
		}
	}
}

func (p *Pool[T]) Stats() PoolStats {
	return PoolStats{
		MaxSize:        p.maxSize,
		Available:      int64(len(p.items)),
		TotalCreated:   atomic.LoadInt64(&p.stats.TotalCreated),
		TotalAcquired:  atomic.LoadInt64(&p.stats.TotalAcquired),
		TotalReleased:  atomic.LoadInt64(&p.stats.TotalReleased),
		TotalDestroyed: atomic.LoadInt64(&p.stats.TotalDestroyed),
	}
}

func (p *Pool[T]) Len() int {
	return len(p.items)
}

type PoolStats struct {
	MaxSize        int
	Available      int64
	TotalCreated   int64
	TotalAcquired  int64
	TotalReleased  int64
	TotalDestroyed int64
}

type ObjectPool[T comparable] struct {
	items  []T
	inUse  map[int]bool
	factory func() T
	mu     sync.Mutex
	maxSize int
}

func NewObjectPool[T comparable](maxSize int, factory func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		items:   make([]T, 0, maxSize),
		inUse:   make(map[int]bool),
		factory: factory,
		maxSize: maxSize,
	}
}

func (op *ObjectPool[T]) Acquire() T {
	op.mu.Lock()
	defer op.mu.Unlock()

	for i := range op.items {
		if !op.inUse[i] {
			op.inUse[i] = true
			return op.items[i]
		}
	}

	if len(op.items) < op.maxSize {
		item := op.factory()
		idx := len(op.items)
		op.items = append(op.items, item)
		op.inUse[idx] = true
		return item
	}

	return op.factory()
}

func (op *ObjectPool[T]) Release(item T) {
	op.mu.Lock()
	defer op.mu.Unlock()

	for i, existing := range op.items {
		if existing == item {
			op.inUse[i] = false
			return
		}
	}
}

func (op *ObjectPool[T]) Size() int {
	op.mu.Lock()
	defer op.mu.Unlock()
	return len(op.items)
}

func (op *ObjectPool[T]) InUse() int {
	op.mu.Lock()
	defer op.mu.Unlock()
	count := 0
	for _, used := range op.inUse {
		if used {
			count++
		}
	}
	return count
}

type BufferPool struct {
	buffers   chan []byte
	size      int
	stats     PoolStats
	mu        sync.RWMutex
}

func NewBufferPool(size, maxBuffers int) *BufferPool {
	return &BufferPool{
		buffers: make(chan []byte, maxBuffers),
		size:    size,
	}
}

func (bp *BufferPool) Get() []byte {
	select {
	case buf := <-bp.buffers:
		atomic.AddInt64(&bp.stats.TotalAcquired, 1)
		return buf[:bp.size]
	default:
		atomic.AddInt64(&bp.stats.TotalCreated, 1)
		atomic.AddInt64(&bp.stats.TotalAcquired, 1)
		return make([]byte, bp.size)
	}
}

func (bp *BufferPool) Put(buf []byte) {
	if cap(buf) < bp.size {
		return
	}

	select {
	case bp.buffers <- buf[:bp.size]:
		atomic.AddInt64(&bp.stats.TotalReleased, 1)
	default:
		atomic.AddInt64(&bp.stats.TotalDestroyed, 1)
	}
}

func (bp *BufferPool) Stats() PoolStats {
	return PoolStats{
		Available:      int64(len(bp.buffers)),
		TotalCreated:   atomic.LoadInt64(&bp.stats.TotalCreated),
		TotalAcquired:  atomic.LoadInt64(&bp.stats.TotalAcquired),
		TotalReleased:  atomic.LoadInt64(&bp.stats.TotalReleased),
		TotalDestroyed: atomic.LoadInt64(&bp.stats.TotalDestroyed),
	}
}

type GoroutinePool struct {
	tasks    chan func()
	workers  int
	running  bool
	stats    PoolStats
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type GoroutinePoolConfig struct {
	Workers    int
	QueueSize  int
}

func NewGoroutinePool(config GoroutinePoolConfig) *GoroutinePool {
	if config.Workers <= 0 {
		config.Workers = 10
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}

	return &GoroutinePool{
		tasks:   make(chan func(), config.QueueSize),
		workers: config.Workers,
		stopCh:  make(chan struct{}),
	}
}

func (gp *GoroutinePool) Start() {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if gp.running {
		return
	}
	gp.running = true

	for i := 0; i < gp.workers; i++ {
		gp.wg.Add(1)
		go gp.worker()
	}
}

func (gp *GoroutinePool) worker() {
	defer gp.wg.Done()
	for {
		select {
		case task, ok := <-gp.tasks:
			if !ok {
				return
			}
			task()
			atomic.AddInt64(&gp.stats.TotalReleased, 1)
		case <-gp.stopCh:
			return
		}
	}
}

func (gp *GoroutinePool) Submit(task func()) error {
	gp.mu.RLock()
	if !gp.running {
		gp.mu.RUnlock()
		return ErrPoolClosed
	}
	gp.mu.RUnlock()

	select {
	case gp.tasks <- task:
		atomic.AddInt64(&gp.stats.TotalAcquired, 1)
		return nil
	default:
		return ErrPoolFull
	}
}

func (gp *GoroutinePool) Stop() {
	gp.mu.Lock()
	if !gp.running {
		gp.mu.Unlock()
		return
	}
	gp.running = false
	gp.mu.Unlock()

	close(gp.stopCh)
	gp.wg.Wait()
}

func (gp *GoroutinePool) Wait() {
	gp.wg.Wait()
}

func (gp *GoroutinePool) Stats() PoolStats {
	return PoolStats{
		MaxSize:        gp.workers,
		Available:      int64(len(gp.tasks)),
		TotalAcquired:  atomic.LoadInt64(&gp.stats.TotalAcquired),
		TotalReleased:  atomic.LoadInt64(&gp.stats.TotalReleased),
	}
}

func (gp *GoroutinePool) Running() bool {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	return gp.running
}

type PoolResizer struct {
	pool     *Pool[interface{}]
	interval time.Duration
	minSize  int
	maxSize  int
	mu       sync.RWMutex
	stopCh   chan struct{}
}

func NewPoolResizer(pool *Pool[interface{}], interval time.Duration) *PoolResizer {
	return &PoolResizer{
		pool:     pool,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (pr *PoolResizer) Start() {
	go func() {
		ticker := time.NewTicker(pr.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pr.resize()
			case <-pr.stopCh:
				return
			}
		}
	}()
}

func (pr *PoolResizer) resize() {
	stats := pr.pool.Stats()
	if float64(stats.Available) > float64(stats.MaxSize)*0.8 {
		for i := 0; i < 5; i++ {
			item, err := pr.pool.Get()
			if err == nil {
				pr.pool.Put(item)
			}
		}
	}
}

func (pr *PoolResizer) Stop() {
	close(pr.stopCh)
}

type PoolMonitor[T any] struct {
	pool     *Pool[T]
	interval time.Duration
	callback func(PoolStats)
	stopCh   chan struct{}
}

func NewPoolMonitor[T any](pool *Pool[T], interval time.Duration, callback func(PoolStats)) *PoolMonitor[T] {
	return &PoolMonitor[T]{
		pool:     pool,
		interval: interval,
		callback: callback,
		stopCh:   make(chan struct{}),
	}
}

func (pm *PoolMonitor[T]) Start() {
	go func() {
		ticker := time.NewTicker(pm.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := pm.pool.Stats()
				pm.callback(stats)
			case <-pm.stopCh:
				return
			}
		}
	}()
}

func (pm *PoolMonitor[T]) Stop() {
	close(pm.stopCh)
}

type PoolHealth[T any] struct {
	pool      *Pool[T]
	healthy   bool
	lastCheck time.Time
	mu        sync.RWMutex
}

func NewPoolHealth[T any](pool *Pool[T]) *PoolHealth[T] {
	return &PoolHealth[T]{
		pool:    pool,
		healthy: true,
	}
}

func (ph *PoolHealth[T]) Check() bool {
	stats := ph.pool.Stats()
	ph.mu.Lock()
	defer ph.mu.Unlock()

	ph.lastCheck = time.Now()
	ph.healthy = stats.Available > 0 || stats.TotalCreated < int64(stats.MaxSize)
	return ph.healthy
}

func (ph *PoolHealth[T]) IsHealthy() bool {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	return ph.healthy
}

type ConnectionPool[T any] struct {
	inner     *Pool[T]
	maxIdle   int
	maxOpen   int
	idleTimeout time.Duration
	mu        sync.RWMutex
}

func NewConnectionPool[T any](config PoolConfig[T]) *ConnectionPool[T] {
	return &ConnectionPool[T]{
		inner:       NewPool(config),
		maxIdle:     config.MaxSize,
		maxOpen:     config.MaxSize * 2,
		idleTimeout: 5 * time.Minute,
	}
}

func (cp *ConnectionPool[T]) Get() (T, error) {
	return cp.inner.Get()
}

func (cp *ConnectionPool[T]) Put(item T) {
	cp.inner.Put(item)
}

func (cp *ConnectionPool[T]) Close() {
	cp.inner.Close()
}

func (cp *ConnectionPool[T]) Stats() PoolStats {
	return cp.inner.Stats()
}

var (
	ErrPoolClosed = &PoolError{Message: "pool is closed"}
	ErrPoolFull   = &PoolError{Message: "pool is full"}
)

type PoolError struct {
	Message string
}

func (pe *PoolError) Error() string {
	return pe.Message
}
