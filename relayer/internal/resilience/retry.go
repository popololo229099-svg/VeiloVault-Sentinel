package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
	RetryableFn func(error) bool
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
		RetryableFn: func(err error) bool {
			return err != nil
		},
	}
}

type RetryEngine struct {
	config RetryConfig
}

func NewRetryEngine(config RetryConfig) *RetryEngine {
	return &RetryEngine{config: config}
}

func (r *RetryEngine) Execute(ctx context.Context, fn func() error) error {
	return r.ExecuteWithResult(ctx, func() error { return fn() })
}

func (r *RetryEngine) ExecuteWithResult(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if r.config.RetryableFn != nil && !r.config.RetryableFn(lastErr) {
			return lastErr
		}
	}
	return ErrMaxRetriesExceeded
}

func (r *RetryEngine) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	if r.config.Jitter {
		delay = delay * (0.5 + rand.Float64()*0.5)
	}
	return time.Duration(delay)
}

// RetryWithCallback provides attempt information to the callback.
type RetryWithCallback struct {
	config RetryConfig
}

func NewRetryWithCallback(config RetryConfig) *RetryWithCallback {
	return &RetryWithCallback{config: config}
}

func (r *RetryWithCallback) Execute(ctx context.Context, fn func(attempt int) error) error {
	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (r *RetryWithCallback) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	if r.config.Jitter {
		delay = delay * (0.5 + rand.Float64()*0.5)
	}
	return time.Duration(delay)
}

// RetryMetrics tracks retry statistics.
type RetryMetrics struct {
	TotalAttempts int64
	TotalRetries  int64
	TotalSuccess  int64
	TotalFailures int64
	mu            sync.Mutex
}

func NewRetryMetrics() *RetryMetrics {
	return &RetryMetrics{}
}

func (m *RetryMetrics) Record(success bool, attempts int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalAttempts += int64(attempts)
	m.TotalRetries += int64(attempts - 1)
	if success {
		m.TotalSuccess++
	} else {
		m.TotalFailures++
	}
}

func (m *RetryMetrics) Snapshot() (attempts, retries, success, failures int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.TotalAttempts, m.TotalRetries, m.TotalSuccess, m.TotalFailures
}
