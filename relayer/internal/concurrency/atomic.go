package concurrency

import (
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiter struct {
	rate     int64
	burst    int64
	current  int64
	lastTime time.Time
	mu       sync.Mutex
}

func NewRateLimiter(ratePerSecond int) *RateLimiter {
	return &RateLimiter{
		rate:     int64(ratePerSecond),
		burst:    int64(ratePerSecond),
		current:  int64(ratePerSecond),
		lastTime: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.current += int64(elapsed * float64(rl.rate))
	if rl.current > rl.burst {
		rl.current = rl.burst
	}
	rl.lastTime = now
	if rl.current > 0 {
		rl.current--
		return true
	}
	return false
}

type AtomicCounter struct {
	value int64
}

func NewAtomicCounter() *AtomicCounter {
	return &AtomicCounter{}
}

func (c *AtomicCounter) Inc() int64     { return atomic.AddInt64(&c.value, 1) }
func (c *AtomicCounter) Dec() int64     { return atomic.AddInt64(&c.value, -1) }
func (c *AtomicCounter) Get() int64     { return atomic.LoadInt64(&c.value) }
func (c *AtomicCounter) Set(v int64)    { atomic.StoreInt64(&c.value, v) }
func (c *AtomicCounter) Add(v int64) int64 { return atomic.AddInt64(&c.value, v) }
func (c *AtomicCounter) Reset()         { atomic.StoreInt64(&c.value, 0) }

type AtomicBool struct {
	value int32
}

func NewAtomicBool(initial bool) *AtomicBool {
	var v int32
	if initial {
		v = 1
	}
	return &AtomicBool{value: v}
}

func (b *AtomicBool) Get() bool          { return atomic.LoadInt32(&b.value) == 1 }
func (b *AtomicBool) Set(v bool)         { var x int32; if v { x = 1 }; atomic.StoreInt32(&b.value, x) }
func (b *AtomicBool) Toggle() bool       { return atomic.AddInt32(&b.value, 1) == 1 }
func (b *AtomicBool) CompareAndSwap(old, new bool) bool {
	var o, n int32
	if old { o = 1 }
	if new { n = 1 }
	return atomic.CompareAndSwapInt32(&b.value, o, n)
}

type LoadBalancer struct {
	endpoints []string
	current   uint64
}

func NewLoadBalancer(endpoints []string) *LoadBalancer {
	return &LoadBalancer{endpoints: endpoints}
}

func (lb *LoadBalancer) Next() string {
	if len(lb.endpoints) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&lb.current, 1) % uint64(len(lb.endpoints))
	return lb.endpoints[idx]
}

func (lb *LoadBalancer) Count() int { return len(lb.endpoints) }

type BatchProcessor[T any] struct {
	batchSize int
	flushWait time.Duration
	batch     []T
	mu        sync.Mutex
	handler   func([]T)
	timer     *time.Timer
}

func NewBatchProcessor[T any](batchSize int, flushWait time.Duration, handler func([]T)) *BatchProcessor[T] {
	return &BatchProcessor[T]{
		batchSize: batchSize,
		flushWait: flushWait,
		handler:   handler,
		batch:     make([]T, 0, batchSize),
	}
}

func (bp *BatchProcessor[T]) Add(item T) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.batch = append(bp.batch, item)
	if len(bp.batch) >= bp.batchSize {
		bp.flush()
		return
	}
	if bp.timer == nil {
		bp.timer = time.AfterFunc(bp.flushWait, func() {
			bp.mu.Lock()
			defer bp.mu.Unlock()
			if len(bp.batch) > 0 {
				bp.flush()
			}
		})
	}
}

func (bp *BatchProcessor[T]) flush() {
	batch := bp.batch
	bp.batch = make([]T, 0, bp.batchSize)
	bp.timer = nil
	go bp.handler(batch)
}

func (bp *BatchProcessor[T]) Flush() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if len(bp.batch) > 0 {
		bp.flush()
	}
}
