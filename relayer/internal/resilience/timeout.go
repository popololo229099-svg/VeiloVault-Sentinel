package resilience

import (
	"context"
	"sync"
	"time"
)

type TimeoutConfig struct {
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
}

func NewTimeoutConfig(defaultTimeout, maxTimeout time.Duration) TimeoutConfig {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Second
	}
	if maxTimeout <= 0 {
		maxTimeout = 5 * time.Minute
	}
	return TimeoutConfig{DefaultTimeout: defaultTimeout, MaxTimeout: maxTimeout}
}

func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}

func WithDeadline(ctx context.Context, deadline time.Time, fn func(context.Context) error) error {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	return fn(ctx)
}

func WithCancellation(ctx context.Context, fn func(context.Context) error) (context.Context, error) {
	ctx, _ = context.WithCancel(ctx)
	err := fn(ctx)
	return ctx, err
}

// AdaptiveTimeout adjusts timeout based on historical latency.
type AdaptiveTimeout struct {
	mu          sync.Mutex
	history     []time.Duration
	windowSize  int
	multiplier  float64
	minTimeout  time.Duration
	maxTimeout  time.Duration
}

func NewAdaptiveTimeout(windowSize int, multiplier float64) *AdaptiveTimeout {
	return &AdaptiveTimeout{
		history:    make([]time.Duration, 0, windowSize),
		windowSize: windowSize,
		multiplier: multiplier,
		minTimeout: 100 * time.Millisecond,
		maxTimeout: 60 * time.Second,
	}
}

func (at *AdaptiveTimeout) Record(d time.Duration) {
	at.mu.Lock()
	defer at.mu.Unlock()
	if len(at.history) >= at.windowSize {
		at.history = at.history[1:]
	}
	at.history = append(at.history, d)
}

func (at *AdaptiveTimeout) Calculate() time.Duration {
	at.mu.Lock()
	defer at.mu.Unlock()
	if len(at.history) == 0 {
		return at.minTimeout * time.Duration(at.multiplier)
	}

	var total time.Duration
	var max time.Duration
	for _, d := range at.history {
		total += d
		if d > max {
			max = d
		}
	}
	avg := total / time.Duration(len(at.history))
	result := time.Duration(float64(avg) * at.multiplier)

	if result < at.minTimeout {
		result = at.minTimeout
	}
	if result > at.maxTimeout {
		result = at.maxTimeout
	}
	return result
}

func (at *AdaptiveTimeout) P99() time.Duration {
	at.mu.Lock()
	defer at.mu.Unlock()
	if len(at.history) == 0 {
		return at.maxTimeout
	}
	sorted := make([]time.Duration, len(at.history))
	copy(sorted, at.history)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
