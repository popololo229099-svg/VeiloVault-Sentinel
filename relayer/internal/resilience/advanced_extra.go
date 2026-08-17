package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type CircuitState int

const (
	CircuitClosed   CircuitState = 0
	CircuitOpen     CircuitState = 1
	CircuitHalfOpen CircuitState = 2
)

type BulkheadConfigAdvanced struct {
	MaxConcurrent int
	MaxQueue      int
	Timeout       time.Duration
	OnReject      func()
	OnAcquire     func()
	OnRelease     func()
}

type AdvancedBulkhead struct {
	config    BulkheadConfigAdvanced
	sem       chan struct{}
	queue     chan struct{}
	active    int
	waiting   int
	rejected  int64
	totalWait time.Duration
	mu        sync.Mutex
}

func NewAdvancedBulkhead(cfg BulkheadConfigAdvanced) *AdvancedBulkhead {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 10
	}
	if cfg.MaxQueue <= 0 {
		cfg.MaxQueue = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &AdvancedBulkhead{
		config: cfg,
		sem:    make(chan struct{}, cfg.MaxConcurrent),
		queue:  make(chan struct{}, cfg.MaxQueue),
	}
}

func (b *AdvancedBulkhead) Execute(ctx context.Context, fn func() error) error {
	select {
	case b.sem <- struct{}{}:
		b.mu.Lock()
		b.active++
		b.mu.Unlock()
		if b.config.OnAcquire != nil {
			b.config.OnAcquire()
		}
		defer func() {
			<-b.sem
			b.mu.Lock()
			b.active--
			b.mu.Unlock()
			if b.config.OnRelease != nil {
				b.config.OnRelease()
			}
		}()
		return fn()
	default:
	}

	select {
	case b.queue <- struct{}{}:
		b.mu.Lock()
		b.waiting++
		b.mu.Unlock()
		defer func() {
			<-b.queue
			b.mu.Lock()
			b.waiting--
			b.mu.Unlock()
		}()
	case <-ctx.Done():
		b.mu.Lock()
		b.rejected++
		b.mu.Unlock()
		if b.config.OnReject != nil {
			b.config.OnReject()
		}
		return ctx.Err()
	case <-time.After(b.config.Timeout):
		b.mu.Lock()
		b.rejected++
		b.mu.Unlock()
		if b.config.OnReject != nil {
			b.config.OnReject()
		}
		return fmt.Errorf("bulkhead: timeout waiting for slot")
	}

	select {
	case b.sem <- struct{}{}:
		b.mu.Lock()
		b.active++
		waitStart := time.Now()
		b.totalWait += time.Since(waitStart)
		b.mu.Unlock()
		defer func() {
			<-b.sem
			b.mu.Lock()
			b.active--
			b.mu.Unlock()
		}()
		return fn()
	case <-ctx.Done():
		b.mu.Lock()
		b.rejected++
		b.mu.Unlock()
		if b.config.OnReject != nil {
			b.config.OnReject()
		}
		return ctx.Err()
	}
}

func (b *AdvancedBulkhead) Stats() (active, waiting, rejected int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active, b.waiting, int(b.rejected)
}

func (b *AdvancedBulkhead) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.active = 0
	b.waiting = 0
	b.rejected = 0
	b.totalWait = 0
}

type CircuitBreakerAdvanced struct {
	state             CircuitState
	failureCount      int64
	successCount      int64
	failureThreshold  int64
	successThreshold  int64
	timeout           time.Duration
	halfOpenMax       int
	halfOpenCount     int
	lastStateChange   time.Time
	onStateChange     func(from, to CircuitState)
	mu                sync.Mutex
}

func NewCircuitBreakerAdvanced(failureThreshold, successThreshold int64, timeout time.Duration) *CircuitBreakerAdvanced {
	return &CircuitBreakerAdvanced{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		halfOpenMax:      3,
		lastStateChange:  time.Now(),
	}
}

func (cb *CircuitBreakerAdvanced) OnStateChange(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

func (cb *CircuitBreakerAdvanced) Execute(ctx context.Context, fn func() error) error {
	if !cb.Allow() {
		return fmt.Errorf("circuit breaker is open")
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	cb.RecordSuccess()
	return nil
}

func (cb *CircuitBreakerAdvanced) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastStateChange) > cb.timeout {
			cb.transitionTo(CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.halfOpenCount < int(cb.halfOpenMax)
	}
	return false
}

func (cb *CircuitBreakerAdvanced) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == CircuitHalfOpen {
		cb.successCount++
		cb.halfOpenCount++
		if cb.successCount >= cb.successThreshold {
			cb.transitionTo(CircuitClosed)
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreakerAdvanced) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.state == CircuitHalfOpen {
		cb.transitionTo(CircuitOpen)
	} else if cb.failureCount >= cb.failureThreshold {
		cb.transitionTo(CircuitOpen)
	}
}

func (cb *CircuitBreakerAdvanced) transitionTo(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	if newState == CircuitClosed {
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenCount = 0
	} else if newState == CircuitOpen {
		cb.successCount = 0
		cb.halfOpenCount = 0
	} else if newState == CircuitHalfOpen {
		cb.halfOpenCount = 0
		cb.successCount = 0
	}
	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

func (cb *CircuitBreakerAdvanced) StateInfo() (CircuitState, int64, int64, time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state, cb.failureCount, cb.successCount, cb.lastStateChange
}

type TimeoutAdvanced struct {
	defaultTimeout time.Duration
	minTimeout     time.Duration
	maxTimeout     time.Duration
	adaptiveWindow time.Duration
	metrics        *TimeoutMetrics
	mu             sync.RWMutex
}

type TimeoutMetrics struct {
	TotalRequests int64
	Timeouts      int64
	AvgLatency    time.Duration
	P99Latency    time.Duration
}

func NewTimeoutAdvanced(defaultTimeout, minTimeout, maxTimeout time.Duration) *TimeoutAdvanced {
	return &TimeoutAdvanced{
		defaultTimeout: defaultTimeout,
		minTimeout:     minTimeout,
		maxTimeout:     maxTimeout,
		adaptiveWindow: 1 * time.Minute,
		metrics:        &TimeoutMetrics{},
	}
}

func (t *TimeoutAdvanced) Execute(ctx context.Context, fn func() error) error {
	deadline := time.After(t.defaultTimeout)
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	select {
	case <-deadline:
		t.mu.Lock()
		t.metrics.Timeouts++
		t.mu.Unlock()
		return fmt.Errorf("operation timed out after %v", t.defaultTimeout)
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *TimeoutAdvanced) RecordLatency(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics.TotalRequests++
	t.metrics.AvgLatency = (t.metrics.AvgLatency * time.Duration(t.metrics.TotalRequests-1) + d) / time.Duration(t.metrics.TotalRequests)
}

func (t *TimeoutAdvanced) GetMetrics() TimeoutMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return *t.metrics
}
