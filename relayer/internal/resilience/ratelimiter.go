package resilience

import (
	"errors"
	"sync"
	"time"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// TokenBucket implements the token bucket rate limiting algorithm.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func NewTokenBucket(maxTokens, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}

func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// SlidingWindow implements sliding window rate limiting.
type SlidingWindow struct {
	mu         sync.Mutex
	windows    []time.Time
	maxCount   int
	windowSize time.Duration
}

func NewSlidingWindow(maxCount int, windowSize time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windows:    make([]time.Time, 0, maxCount),
		maxCount:   maxCount,
		windowSize: windowSize,
	}
}

func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-sw.windowSize)

	idx := 0
	for idx < len(sw.windows) && sw.windows[idx].Before(cutoff) {
		idx++
	}
	sw.windows = sw.windows[idx:]

	if len(sw.windows) < sw.maxCount {
		sw.windows = append(sw.windows, now)
		return true
	}
	return false
}

func (sw *SlidingWindow) Count() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-sw.windowSize)
	count := 0
	for _, t := range sw.windows {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count
}

// FixedWindow implements fixed window rate limiting.
type FixedWindow struct {
	mu         sync.Mutex
	windowStart time.Time
	count      int
	maxCount   int
	windowSize time.Duration
}

func NewFixedWindow(maxCount int, windowSize time.Duration) *FixedWindow {
	return &FixedWindow{
		windowStart: time.Now(),
		maxCount:    maxCount,
		windowSize:  windowSize,
	}
}

func (fw *FixedWindow) Allow() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	now := time.Now()
	if now.Sub(fw.windowStart) > fw.windowSize {
		fw.windowStart = now
		fw.count = 0
	}
	if fw.count < fw.maxCount {
		fw.count++
		return true
	}
	return false
}

// PerKeyRateLimiter manages rate limiters per key.
type PerKeyRateLimiter struct {
	limiters map[string]*TokenBucket
	mu       sync.RWMutex
	rate     float64
	burst    float64
}

func NewPerKeyRateLimiter(rate, burst float64) *PerKeyRateLimiter {
	return &PerKeyRateLimiter{
		limiters: make(map[string]*TokenBucket),
		rate:     rate,
		burst:    burst,
	}
}

func (rl *PerKeyRateLimiter) Allow(key string) bool {
	return rl.AllowN(key, 1)
}

func (rl *PerKeyRateLimiter) AllowN(key string, n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	bucket, ok := rl.limiters[key]
	if !ok {
		bucket = NewTokenBucket(rl.burst, rl.rate)
		rl.limiters[key] = bucket
	}
	return bucket.AllowN(n)
}

func (rl *PerKeyRateLimiter) Remove(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.limiters, key)
}

func (rl *PerKeyRateLimiter) Count() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.limiters)
}
