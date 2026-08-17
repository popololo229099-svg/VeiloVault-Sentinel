package resilience

import (
	"context"
	"sync"
	"time"
)

type RateLimitConfig struct {
	RequestsPerSecond int
	BurstSize         int
	CleanupInterval   time.Duration
}

type RateLimiterManager struct {
	limiters map[string]*SlidingWindow
	cfg      RateLimitConfig
	mu       sync.RWMutex
}

func NewRateLimiterManager(cfg RateLimitConfig) *RateLimiterManager {
	return &RateLimiterManager{
		limiters: make(map[string]*SlidingWindow),
		cfg:      cfg,
	}
}

func (m *RateLimiterManager) Allow(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	limiter, ok := m.limiters[key]
	if !ok {
		limiter = NewSlidingWindow(m.cfg.RequestsPerSecond, time.Second)
		m.limiters[key] = limiter
	}
	return limiter.Allow()
}

func (m *RateLimiterManager) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.limiters, key)
}

func (m *RateLimiterManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.limiters)
}

type RetryBudget struct {
	maxRetries  int
	windowSize  time.Duration
	retryCounts map[string]int
	windowStart map[string]time.Time
	mu          sync.Mutex
}

func NewRetryBudget(maxRetries int, windowSize time.Duration) *RetryBudget {
	return &RetryBudget{
		maxRetries:  maxRetries,
		windowSize:  windowSize,
		retryCounts: make(map[string]int),
		windowStart: make(map[string]time.Time),
	}
}

func (rb *RetryBudget) Allow(key string) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	now := time.Now()
	if start, ok := rb.windowStart[key]; ok {
		if now.Sub(start) > rb.windowSize {
			rb.retryCounts[key] = 0
			rb.windowStart[key] = now
		}
	} else {
		rb.windowStart[key] = now
	}
	if rb.retryCounts[key] >= rb.maxRetries {
		return false
	}
	rb.retryCounts[key]++
	return true
}

func (rb *RetryBudget) Reset(key string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	delete(rb.retryCounts, key)
	delete(rb.windowStart, key)
}

type FallbackChain struct {
	fallbacks []FallbackFunc
	mu        sync.RWMutex
}

type FallbackFunc func() (interface{}, error)

func NewFallbackChain() *FallbackChain {
	return &FallbackChain{fallbacks: make([]FallbackFunc, 0)}
}

func (fc *FallbackChain) Add(fallback FallbackFunc) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.fallbacks = append(fc.fallbacks, fallback)
}

func (fc *FallbackChain) Execute() (interface{}, error) {
	fc.mu.RLock()
	fallbacks := make([]FallbackFunc, len(fc.fallbacks))
	copy(fallbacks, fc.fallbacks)
	fc.mu.RUnlock()

	for _, fb := range fallbacks {
		result, err := fb()
		if err == nil {
			return result, nil
		}
	}
	return nil, ErrAllFallbacksFailed
}

var ErrAllFallbacksFailed = &AdvancedResilienceError{"all fallbacks failed"}

type AdvancedResilienceError struct{ msg string }

func (e *AdvancedResilienceError) Error() string { return e.msg }

type NamedBulkheadManager struct {
	bulkheads map[string]*Bulkhead
	maxConc   int
	maxQueue  int
	timeout   time.Duration
	mu        sync.RWMutex
}

func NewNamedBulkheadManager(maxConc, maxQueue int, timeout time.Duration) *NamedBulkheadManager {
	return &NamedBulkheadManager{
		bulkheads: make(map[string]*Bulkhead),
		maxConc:   maxConc,
		maxQueue:  maxQueue,
		timeout:   timeout,
	}
}

func (m *NamedBulkheadManager) Get(name string) *Bulkhead {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.bulkheads[name]; ok {
		return b
	}
	b := NewBulkhead(BulkheadConfig{
		MaxConcurrency: m.maxConc,
		MaxQueue:       m.maxQueue,
		Timeout:        m.timeout,
	})
	m.bulkheads[name] = b
	return b
}

func (m *NamedBulkheadManager) Execute(name string, fn func() error) error {
	return m.Get(name).Execute(fn)
}

func (m *NamedBulkheadManager) Stats() map[string]BulkheadStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := make(map[string]BulkheadStats)
	for k, b := range m.bulkheads {
		active, waiting, capacity := b.Stats()
		stats[k] = BulkheadStats{
			Active:   active,
			Waiting:  waiting,
			Capacity: capacity,
		}
	}
	return stats
}

type BulkheadStats struct {
	Active   int
	Waiting  int
	Capacity int
}

type RetryWithFallback struct {
	retry    *RetryEngine
	fallback *FallbackChain
}

func NewRetryWithFallback(retryCfg RetryConfig) *RetryWithFallback {
	return &RetryWithFallback{
		retry:    NewRetryEngine(retryCfg),
		fallback: NewFallbackChain(),
	}
}

func (rwf *RetryWithFallback) AddFallback(fb FallbackFunc) {
	rwf.fallback.Add(fb)
}

func (rwf *RetryWithFallback) Execute(ctx context.Context, primary func() error) error {
	err := rwf.retry.Execute(ctx, primary)
	if err == nil {
		return nil
	}
	_, fbErr := rwf.fallback.Execute()
	if fbErr != nil {
		return err
	}
	return nil
}
