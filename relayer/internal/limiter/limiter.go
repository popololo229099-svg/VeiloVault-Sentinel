package limiter

import (
	"sync"
	"time"
)

type RateLimiter interface {
	Allow() bool
	AllowN(n int) bool
	Wait() bool
	WaitN(n int) bool
	Reset()
	Stats() LimiterStats
}

type LimiterStats struct {
	TotalRequests  int64
	AllowedRequests int64
	DeniedRequests  int64
	CurrentTokens  float64
	MaxTokens      float64
}

type TokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
	stats      limiterStats
}

type limiterStats struct {
	total    int64
	allowed  int64
	denied   int64
}

func NewTokenBucket(maxTokens int, refillRatePerSecond float64) *TokenBucket {
	return &TokenBucket{
		tokens:     float64(maxTokens),
		maxTokens:  float64(maxTokens),
		refillRate: refillRatePerSecond,
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
	tb.stats.total++

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		tb.stats.allowed++
		return true
	}

	tb.stats.denied++
	return false
}

func (tb *TokenBucket) Wait() bool {
	return tb.WaitN(1)
}

func (tb *TokenBucket) WaitN(n int) bool {
	for {
		if tb.AllowN(n) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
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

func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = tb.maxTokens
	tb.lastRefill = time.Now()
}

func (tb *TokenBucket) Stats() LimiterStats {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return LimiterStats{
		TotalRequests:   tb.stats.total,
		AllowedRequests: tb.stats.allowed,
		DeniedRequests:  tb.stats.denied,
		CurrentTokens:   tb.tokens,
		MaxTokens:       tb.maxTokens,
	}
}

type SlidingWindow struct {
	windows    []windowEntry
	windowSize time.Duration
	maxCount   int
	mu         sync.Mutex
	stats      limiterStats
}

type windowEntry struct {
	timestamp time.Time
	count     int
}

func NewSlidingWindow(windowSize time.Duration, maxCount int) *SlidingWindow {
	return &SlidingWindow{
		windows:    make([]windowEntry, 0),
		windowSize: windowSize,
		maxCount:   maxCount,
	}
}

func (sw *SlidingWindow) Allow() bool {
	return sw.AllowN(1)
}

func (sw *SlidingWindow) AllowN(n int) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.stats.total++
	sw.clean()

	total := 0
	for _, w := range sw.windows {
		total += w.count
	}

	if total+n <= sw.maxCount {
		sw.windows = append(sw.windows, windowEntry{
			timestamp: time.Now(),
			count:     n,
		})
		sw.stats.allowed++
		return true
	}

	sw.stats.denied++
	return false
}

func (sw *SlidingWindow) Wait() bool {
	for {
		if sw.Allow() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (sw *SlidingWindow) WaitN(n int) bool {
	for {
		if sw.AllowN(n) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (sw *SlidingWindow) clean() {
	cutoff := time.Now().Add(-sw.windowSize)
	idx := 0
	for i, w := range sw.windows {
		if w.timestamp.After(cutoff) {
			idx = i
			break
		}
		if i == len(sw.windows)-1 {
			idx = len(sw.windows)
		}
	}
	sw.windows = sw.windows[idx:]
}

func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.windows = sw.windows[:0]
}

func (sw *SlidingWindow) Stats() LimiterStats {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.clean()

	total := 0
	for _, w := range sw.windows {
		total += w.count
	}

	return LimiterStats{
		TotalRequests:   sw.stats.total,
		AllowedRequests: sw.stats.allowed,
		DeniedRequests:  sw.stats.denied,
		CurrentTokens:   float64(sw.maxCount - total),
		MaxTokens:       float64(sw.maxCount),
	}
}

type FixedWindow struct {
	windowStart time.Time
	windowSize  time.Duration
	maxCount    int
	current     int
	mu          sync.Mutex
	stats       limiterStats
}

func NewFixedWindow(windowSize time.Duration, maxCount int) *FixedWindow {
	return &FixedWindow{
		windowStart: time.Now(),
		windowSize:  windowSize,
		maxCount:    maxCount,
	}
}

func (fw *FixedWindow) Allow() bool {
	return fw.AllowN(1)
}

func (fw *FixedWindow) AllowN(n int) bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.stats.total++
	fw.checkWindow()

	if fw.current+n <= fw.maxCount {
		fw.current += n
		fw.stats.allowed++
		return true
	}

	fw.stats.denied++
	return false
}

func (fw *FixedWindow) checkWindow() {
	if time.Since(fw.windowStart) >= fw.windowSize {
		fw.windowStart = time.Now()
		fw.current = 0
	}
}

func (fw *FixedWindow) Wait() bool {
	for {
		if fw.Allow() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (fw *FixedWindow) WaitN(n int) bool {
	for {
		if fw.AllowN(n) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (fw *FixedWindow) Reset() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.windowStart = time.Now()
	fw.current = 0
}

func (fw *FixedWindow) Stats() LimiterStats {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return LimiterStats{
		TotalRequests:   fw.stats.total,
		AllowedRequests: fw.stats.allowed,
		DeniedRequests:  fw.stats.denied,
		CurrentTokens:   float64(fw.maxCount - fw.current),
		MaxTokens:       float64(fw.maxCount),
	}
}

type LeakyBucket struct {
	capacity   int
	tokens     int
	leakRate   time.Duration
	lastLeak   time.Time
	queue      []time.Time
	mu         sync.Mutex
	stats      limiterStats
}

func NewLeakyBucket(capacity int, leakRate time.Duration) *LeakyBucket {
	return &LeakyBucket{
		capacity: capacity,
		tokens:   capacity,
		leakRate: leakRate,
		lastLeak: time.Now(),
		queue:    make([]time.Time, 0),
	}
}

func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.stats.total++
	lb.leak()

	if lb.tokens > 0 {
		lb.tokens--
		lb.stats.allowed++
		return true
	}

	lb.stats.denied++
	return false
}

func (lb *LeakyBucket) AllowN(n int) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.stats.total++
	lb.leak()

	if lb.tokens >= n {
		lb.tokens -= n
		lb.stats.allowed++
		return true
	}

	lb.stats.denied++
	return false
}

func (lb *LeakyBucket) Wait() bool {
	for {
		if lb.Allow() {
			return true
		}
		time.Sleep(lb.leakRate)
	}
}

func (lb *LeakyBucket) leak() {
	now := time.Now()
	elapsed := now.Sub(lb.lastLeak)
	leaked := int(elapsed / lb.leakRate)
	if leaked > 0 {
		lb.tokens += leaked
		if lb.tokens > lb.capacity {
			lb.tokens = lb.capacity
		}
		lb.lastLeak = now
	}
}

func (lb *LeakyBucket) Reset() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.tokens = lb.capacity
	lb.lastLeak = time.Now()
}

func (lb *LeakyBucket) Stats() LimiterStats {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return LimiterStats{
		TotalRequests:   lb.stats.total,
		AllowedRequests: lb.stats.allowed,
		DeniedRequests:  lb.stats.denied,
		CurrentTokens:   float64(lb.tokens),
		MaxTokens:       float64(lb.capacity),
	}
}

type KeyLimiter struct {
	limiters map[string]RateLimiter
	factory  func() RateLimiter
	mu       sync.RWMutex
}

func NewKeyLimiter(factory func() RateLimiter) *KeyLimiter {
	return &KeyLimiter{
		limiters: make(map[string]RateLimiter),
		factory:  factory,
	}
}

func (kl *KeyLimiter) Allow(key string) bool {
	return kl.AllowN(key, 1)
}

func (kl *KeyLimiter) AllowN(key string, n int) bool {
	kl.mu.RLock()
	limiter, exists := kl.limiters[key]
	kl.mu.RUnlock()

	if !exists {
		kl.mu.Lock()
		limiter, exists = kl.limiters[key]
		if !exists {
			limiter = kl.factory()
			kl.limiters[key] = limiter
		}
		kl.mu.Unlock()
	}

	return limiter.AllowN(n)
}

func (kl *KeyLimiter) GetLimiter(key string) RateLimiter {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return kl.limiters[key]
}

func (kl *KeyLimiter) Remove(key string) {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	delete(kl.limiters, key)
}

func (kl *KeyLimiter) KeyCount() int {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return len(kl.limiters)
}

func (kl *KeyLimiter) Cleanup(maxAge time.Duration) {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	for key := range kl.limiters {
		delete(kl.limiters, key)
	}
}

type AdaptiveLimiter struct {
	base       RateLimiter
	threshold  float64
	currentRate float64
	maxRate    float64
	minRate    float64
	adjustment time.Duration
	mu         sync.RWMutex
	stopCh     chan struct{}
}

func NewAdaptiveLimiter(base RateLimiter, maxRate float64) *AdaptiveLimiter {
	return &AdaptiveLimiter{
		base:       base,
		threshold:  0.8,
		currentRate: maxRate,
		maxRate:    maxRate,
		minRate:    1,
		adjustment: time.Second,
		stopCh:     make(chan struct{}),
	}
}

func (al *AdaptiveLimiter) Allow() bool {
	return al.base.Allow()
}

func (al *AdaptiveLimiter) AllowN(n int) bool {
	return al.base.AllowN(n)
}

func (al *AdaptiveLimiter) Wait() bool {
	return al.base.Wait()
}

func (al *AdaptiveLimiter) WaitN(n int) bool {
	return al.base.WaitN(n)
}

func (al *AdaptiveLimiter) Reset() {
	al.base.Reset()
	al.mu.Lock()
	defer al.mu.Unlock()
	al.currentRate = al.maxRate
}

func (al *AdaptiveLimiter) Stats() LimiterStats {
	return al.base.Stats()
}

func (al *AdaptiveLimiter) SetThreshold(threshold float64) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.threshold = threshold
}

func (al *AdaptiveLimiter) CurrentRate() float64 {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.currentRate
}

type BurstLimiter struct {
	normal   RateLimiter
	burst    RateLimiter
	bursting bool
	burstMax int
	burstCount int
	mu       sync.Mutex
}

func NewBurstLimiter(normal RateLimiter, burst RateLimiter, burstMax int) *BurstLimiter {
	return &BurstLimiter{
		normal:   normal,
		burst:    burst,
		burstMax: burstMax,
	}
}

func (bl *BurstLimiter) Allow() bool {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	if bl.bursting {
		if bl.burstCount < bl.burstMax && bl.burst.Allow() {
			bl.burstCount++
			return true
		}
		bl.bursting = false
		bl.burstCount = 0
	}

	if bl.normal.Allow() {
		return true
	}

	bl.bursting = true
	bl.burstCount = 1
	return bl.burst.Allow()
}

func (bl *BurstLimiter) AllowN(n int) bool {
	for i := 0; i < n; i++ {
		if !bl.Allow() {
			return false
		}
	}
	return true
}

func (bl *BurstLimiter) Wait() bool {
	for {
		if bl.Allow() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (bl *BurstLimiter) WaitN(n int) bool {
	for {
		if bl.AllowN(n) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (bl *BurstLimiter) Reset() {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.normal.Reset()
	bl.burst.Reset()
	bl.bursting = false
	bl.burstCount = 0
}

func (bl *BurstLimiter) Stats() LimiterStats {
	return bl.normal.Stats()
}

type BackpressureLimiter struct {
	inner     RateLimiter
	maxQueue  int
	queue     chan struct{}
	mu        sync.RWMutex
}

func NewBackpressureLimiter(inner RateLimiter, maxQueue int) *BackpressureLimiter {
	return &BackpressureLimiter{
		inner:    inner,
		maxQueue: maxQueue,
		queue:    make(chan struct{}, maxQueue),
	}
}

func (bpl *BackpressureLimiter) Allow() bool {
	return bpl.inner.Allow()
}

func (bpl *BackpressureLimiter) AllowN(n int) bool {
	return bpl.inner.AllowN(n)
}

func (bpl *BackpressureLimiter) Wait() bool {
	select {
	case bpl.queue <- struct{}{}:
		defer func() { <-bpl.queue }()
		return bpl.inner.Wait()
	default:
		return false
	}
}

func (bpl *BackpressureLimiter) WaitN(n int) bool {
	select {
	case bpl.queue <- struct{}{}:
		defer func() { <-bpl.queue }()
		return bpl.inner.WaitN(n)
	default:
		return false
	}
}

func (bpl *BackpressureLimiter) Reset() {
	bpl.inner.Reset()
}

func (bpl *BackpressureLimiter) Stats() LimiterStats {
	return bpl.inner.Stats()
}

func (bpl *BackpressureLimiter) QueueLength() int {
	return len(bpl.queue)
}

func (bpl *BackpressureLimiter) QueueCapacity() int {
	return bpl.maxQueue
}

type CompositeLimiter struct {
	limiters []RateLimiter
	mu       sync.RWMutex
}

func NewCompositeLimiter(limiters ...RateLimiter) *CompositeLimiter {
	return &CompositeLimiter{limiters: limiters}
}

func (cl *CompositeLimiter) Allow() bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	for _, l := range cl.limiters {
		if !l.Allow() {
			return false
		}
	}
	return true
}

func (cl *CompositeLimiter) AllowN(n int) bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	for _, l := range cl.limiters {
		if !l.AllowN(n) {
			return false
		}
	}
	return true
}

func (cl *CompositeLimiter) Wait() bool {
	for {
		if cl.Allow() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (cl *CompositeLimiter) WaitN(n int) bool {
	for {
		if cl.AllowN(n) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

func (cl *CompositeLimiter) Reset() {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	for _, l := range cl.limiters {
		l.Reset()
	}
}

func (cl *CompositeLimiter) Stats() LimiterStats {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	total := LimiterStats{MaxTokens: 1e18}
	for _, l := range cl.limiters {
		s := l.Stats()
		total.TotalRequests += s.TotalRequests
		total.AllowedRequests += s.AllowedRequests
		total.DeniedRequests += s.DeniedRequests
		if s.CurrentTokens < total.CurrentTokens {
			total.CurrentTokens = s.CurrentTokens
		}
	}
	return total
}

func (cl *CompositeLimiter) Add(limiter RateLimiter) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.limiters = append(cl.limiters, limiter)
}

func (cl *CompositeLimiter) Count() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return len(cl.limiters)
}

type LimiterMiddleware struct {
	limiter   RateLimiter
	keyFunc   func(string) string
	onLimited func(string)
	mu        sync.RWMutex
}

func NewLimiterMiddleware(limiter RateLimiter) *LimiterMiddleware {
	return &LimiterMiddleware{
		limiter: limiter,
		keyFunc: func(s string) string { return s },
	}
}

func (lm *LimiterMiddleware) SetKeyFunc(fn func(string) string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.keyFunc = fn
}

func (lm *LimiterMiddleware) SetOnLimited(fn func(string)) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.onLimited = fn
}

func (lm *LimiterMiddleware) Allow(key string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.limiter.Allow()
}

func (lm *LimiterMiddleware) AllowByKey(key string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.limiter.Allow()
}
