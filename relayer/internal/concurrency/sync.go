package concurrency

import (
	"sync"
	"time"
)

type Semaphore struct {
	sem    chan struct{}
	max    int
	active int
	mu     sync.Mutex
}

func NewSemaphore(max int) *Semaphore {
	return &Semaphore{sem: make(chan struct{}, max), max: max}
}

func (s *Semaphore) Acquire() {
	s.sem <- struct{}{}
	s.mu.Lock()
	s.active++
	s.mu.Unlock()
}

func (s *Semaphore) Release() {
	<-s.sem
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
}

func (s *Semaphore) TryAcquire() bool {
	select {
	case s.sem <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.mu.Unlock()
		return true
	default:
		return false
	}
}

func (s *Semaphore) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

type RWMutex struct {
	sync.RWMutex
}

func (m *RWMutex) WithReadLock(fn func() error) error {
	m.RLock()
	defer m.RUnlock()
	return fn()
}

func (m *RWMutex) WithWriteLock(fn func() error) error {
	m.Lock()
	defer m.Unlock()
	return fn()
}

type Debouncer struct {
	mu      sync.Mutex
	timer   *time.Timer
	delay   time.Duration
	pending bool
}

func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{delay: delay}
}

func (d *Debouncer) Call(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.pending = true
	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		d.pending = false
		d.mu.Unlock()
		fn()
	})
}

func (d *Debouncer) Pending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pending
}

type Throttler struct {
	mu       sync.Mutex
	interval time.Duration
	lastCall time.Time
}

func NewThrottler(interval time.Duration) *Throttler {
	return &Throttler{interval: interval}
}

func (t *Throttler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if now.Sub(t.lastCall) >= t.interval {
		t.lastCall = now
		return true
	}
	return false
}

func (t *Throttler) Throttle(fn func()) {
	t.mu.Lock()
	now := time.Now()
	wait := t.interval - now.Sub(t.lastCall)
	t.mu.Unlock()
	if wait > 0 {
		time.Sleep(wait)
	}
	t.mu.Lock()
	t.lastCall = time.Now()
	t.mu.Unlock()
	fn()
}

type Barrier struct {
	n       int
	count   int
	mu      sync.Mutex
	barrier chan struct{}
}

func NewBarrier(n int) *Barrier {
	return &Barrier{n: n, barrier: make(chan struct{})}
}

func (b *Barrier) Wait() {
	b.mu.Lock()
	b.count++
	if b.count == b.n {
		close(b.barrier)
	}
	b.mu.Unlock()
	<-b.barrier
}

type OncePool struct {
	sync.Map
}

func (op *OncePool) Do(key string, fn func()) {
	val, _ := op.LoadOrStore(key, &sync.Once{})
	once := val.(*sync.Once)
	once.Do(fn)
}

type GenericPool[T any] struct {
	pool  chan *T
	newFn func() *T
}

func NewGenericPool[T any](size int, newFn func() *T) *GenericPool[T] {
	p := &GenericPool[T]{
		pool:  make(chan *T, size),
		newFn: newFn,
	}
	for i := 0; i < size; i++ {
		p.pool <- newFn()
	}
	return p
}

func (p *GenericPool[T]) Get() *T {
	select {
	case item := <-p.pool:
		return item
	default:
		return p.newFn()
	}
}

func (p *GenericPool[T]) Put(item *T) {
	select {
	case p.pool <- item:
	default:
	}
}
