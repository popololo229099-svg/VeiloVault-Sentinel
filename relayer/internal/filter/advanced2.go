package filter

import (
	"sync"
	"time"
)

type ExpiryFilter[T any] struct {
	entries map[string]time.Time
	ttl     time.Duration
	mu      sync.RWMutex
}

func NewExpiryFilter[T any](ttl time.Duration) *ExpiryFilter[T] {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &ExpiryFilter[T]{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

func (ef *ExpiryFilter[T]) Add(key string) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.entries[key] = time.Now()
}

func (ef *ExpiryFilter[T]) IsExpired(key string) bool {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	t, ok := ef.entries[key]
	if !ok {
		return true
	}
	return time.Since(t) > ef.ttl
}

func (ef *ExpiryFilter[T]) Match(key string) bool {
	return !ef.IsExpired(key)
}

func (ef *ExpiryFilter[T]) Cleanup() {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	now := time.Now()
	for key, t := range ef.entries {
		if now.Sub(t) > ef.ttl {
			delete(ef.entries, key)
		}
	}
}

func (ef *ExpiryFilter[T]) Name() string { return "expiry" }

type CircularFilter[T any] struct {
	data     []T
	position int
	size     int
	mu       sync.RWMutex
}

func NewCircularFilter[T any](size int) *CircularFilter[T] {
	if size <= 0 {
		size = 100
	}
	return &CircularFilter[T]{
		data: make([]T, size),
		size: size,
	}
}

func (cf *CircularFilter[T]) Add(item T) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.data[cf.position] = item
	cf.position = (cf.position + 1) % cf.size
}

func (cf *CircularFilter[T]) Recent(count int) []T {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	if count > cf.size {
		count = cf.size
	}

	result := make([]T, 0, count)
	start := (cf.position - count + cf.size) % cf.size
	for i := 0; i < count; i++ {
		idx := (start + i) % cf.size
		result = append(result, cf.data[idx])
	}
	return result
}

func (cf *CircularFilter[T]) Match(item T) bool { return true }
func (cf *CircularFilter[T]) Name() string       { return "circular" }

type ThresholdFilter[T any] struct {
	threshold float64
	value     float64
	count     int64
	mu        sync.RWMutex
}

func NewThresholdFilter[T any](threshold float64) *ThresholdFilter[T] {
	return &ThresholdFilter[T]{threshold: threshold}
}

func (tf *ThresholdFilter[T]) AddValue(value float64) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.value += value
	tf.count++
}

func (tf *ThresholdFilter[T]) Exceeded() bool {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.value >= tf.threshold
}

func (tf *ThresholdFilter[T]) Reset() {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.value = 0
	tf.count = 0
}

func (tf *ThresholdFilter[T]) Match(item T) bool { return true }
func (tf *ThresholdFilter[T]) Name() string       { return "threshold" }

type PatternFilter struct {
	patterns map[string]bool
	mu       sync.RWMutex
}

func NewPatternFilter() *PatternFilter {
	return &PatternFilter{patterns: make(map[string]bool)}
}

func (pf *PatternFilter) AddPattern(pattern string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.patterns[pattern] = true
}

func (pf *PatternFilter) Match(item string) bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.patterns[item]
}

func (pf *PatternFilter) Name() string { return "pattern" }

type SlidingWindowFilter struct {
	window  time.Duration
	events  []time.Time
	maxRate int
	mu      sync.Mutex
}

func NewSlidingWindowFilter(window time.Duration, maxRate int) *SlidingWindowFilter {
	if window <= 0 {
		window = time.Minute
	}
	if maxRate <= 0 {
		maxRate = 100
	}
	return &SlidingWindowFilter{
		window:  window,
		events:  make([]time.Time, 0),
		maxRate: maxRate,
	}
}

func (swf *SlidingWindowFilter) Allow() bool {
	swf.mu.Lock()
	defer swf.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-swf.window)

	valid := make([]time.Time, 0)
	for _, t := range swf.events {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	swf.events = valid

	if len(swf.events) >= swf.maxRate {
		return false
	}

	swf.events = append(swf.events, now)
	return true
}

func (swf *SlidingWindowFilter) Count() int {
	swf.mu.Lock()
	defer swf.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-swf.window)

	count := 0
	for _, t := range swf.events {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}

type WindowFilter[T any] struct {
	windows map[string][]T
	size    int
	mu      sync.RWMutex
}

func NewWindowFilter[T any](size int) *WindowFilter[T] {
	if size <= 0 {
		size = 10
	}
	return &WindowFilter[T]{
		windows: make(map[string][]T),
		size:    size,
	}
}

func (wf *WindowFilter[T]) Add(key string, item T) {
	wf.mu.Lock()
	defer wf.mu.Unlock()
	wf.windows[key] = append(wf.windows[key], item)
	if len(wf.windows[key]) > wf.size {
		wf.windows[key] = wf.windows[key][1:]
	}
}

func (wf *WindowFilter[T]) Get(key string) []T {
	wf.mu.RLock()
	defer wf.mu.RUnlock()
	items := wf.windows[key]
	result := make([]T, len(items))
	copy(result, items)
	return result
}

func (wf *WindowFilter[T]) Match(item T) bool { return true }
func (wf *WindowFilter[T]) Name() string       { return "window" }

type RankFilter[T any] struct {
	rankFunc func(T) float64
	topK     int
	mu       sync.RWMutex
}

func NewRankFilter[T any](rankFunc func(T) float64, topK int) *RankFilter[T] {
	if topK <= 0 {
		topK = 10
	}
	return &RankFilter[T]{rankFunc: rankFunc, topK: topK}
}

func (rf *RankFilter[T]) Top(items []T) []T {
	rf.mu.RLock()
	defer rf.mu.RUnlock()

	type ranked struct {
		item T
		score float64
	}

	rankedItems := make([]ranked, len(items))
	for i, item := range items {
		rankedItems[i] = ranked{item: item, score: rf.rankFunc(item)}
	}

	for i := 1; i < len(rankedItems); i++ {
		for j := i; j > 0 && rankedItems[j].score > rankedItems[j-1].score; j-- {
			rankedItems[j], rankedItems[j-1] = rankedItems[j-1], rankedItems[j]
		}
	}

	if rf.topK > len(rankedItems) {
		rf.topK = len(rankedItems)
	}

	result := make([]T, rf.topK)
	for i := 0; i < rf.topK; i++ {
		result[i] = rankedItems[i].item
	}
	return result
}

func (rf *RankFilter[T]) Match(item T) bool { return true }
func (rf *RankFilter[T]) Name() string       { return "rank" }

type SamplingRateFilter[T any] struct {
	rate  float64
	count int64
	mu    sync.RWMutex
}

func NewSamplingRateFilter[T any](rate float64) *SamplingRateFilter[T] {
	if rate <= 0 || rate > 1 {
		rate = 0.1
	}
	return &SamplingRateFilter[T]{rate: rate}
}

func (srf *SamplingRateFilter[T]) ShouldInclude() bool {
	srf.mu.Lock()
	defer srf.mu.Unlock()
	srf.count++
	return float64(srf.count%10000)/10000.0 < srf.rate
}

func (srf *SamplingRateFilter[T]) Match(item T) bool {
	return srf.ShouldInclude()
}

func (srf *SamplingRateFilter[T]) Name() string { return "sampling_rate" }

type FrequencyFilter[T comparable] struct {
	frequencies map[T]int
	threshold   int
	mu          sync.RWMutex
}

func NewFrequencyFilter[T comparable](threshold int) *FrequencyFilter[T] {
	if threshold <= 0 {
		threshold = 2
	}
	return &FrequencyFilter[T]{
		frequencies: make(map[T]int),
		threshold:   threshold,
	}
}

func (ff *FrequencyFilter[T]) Record(item T) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.frequencies[item]++
}

func (ff *FrequencyFilter[T]) IsFrequent(item T) bool {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	return ff.frequencies[item] >= ff.threshold
}

func (ff *FrequencyFilter[T]) Match(item T) bool {
	return !ff.IsFrequent(item)
}

func (ff *FrequencyFilter[T]) Name() string { return "frequency" }

func (ff *FrequencyFilter[T]) Reset() {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.frequencies = make(map[T]int)
}
