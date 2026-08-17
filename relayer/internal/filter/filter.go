package filter

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type Filter[T any] interface {
	Match(item T) bool
	Name() string
}

type FilterChain[T any] struct {
	filters []Filter[T]
	mu      sync.RWMutex
}

func NewFilterChain[T any](filters ...Filter[T]) *FilterChain[T] {
	return &FilterChain[T]{filters: filters}
}

func (fc *FilterChain[T]) Add(f Filter[T]) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.filters = append(fc.filters, f)
}

func (fc *FilterChain[T]) MatchAll(item T) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	for _, f := range fc.filters {
		if !f.Match(item) {
			return false
		}
	}
	return true
}

func (fc *FilterChain[T]) MatchAny(item T) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	for _, f := range fc.filters {
		if f.Match(item) {
			return true
		}
	}
	return false
}

func (fc *FilterChain[T]) Filter(items []T) []T {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	result := make([]T, 0)
	for _, item := range items {
		matches := true
		for _, f := range fc.filters {
			if !f.Match(item) {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, item)
		}
	}
	return result
}

func (fc *FilterChain[T]) FilterAny(items []T) []T {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	result := make([]T, 0)
	for _, item := range items {
		for _, f := range fc.filters {
			if f.Match(item) {
				result = append(result, item)
				break
			}
		}
	}
	return result
}

type StringFilter struct {
	pattern string
	exact   bool
	mu      sync.RWMutex
}

func NewStringFilter(pattern string, exact ...bool) *StringFilter {
	e := false
	if len(exact) > 0 {
		e = exact[0]
	}
	return &StringFilter{pattern: pattern, exact: e}
}

func (sf *StringFilter) Match(item string) bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if sf.exact {
		return item == sf.pattern
	}
	return strings.Contains(item, sf.pattern)
}

func (sf *StringFilter) Name() string { return "string" }

type RegexFilter struct {
	regex   *regexp.Regexp
	mu      sync.RWMutex
}

func NewRegexFilter(pattern string) (*RegexFilter, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexFilter{regex: regex}, nil
}

func (rf *RegexFilter) Match(item string) bool {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.regex.MatchString(item)
}

func (rf *RegexFilter) Name() string { return "regex" }

type PrefixFilter struct {
	prefix string
	mu     sync.RWMutex
}

func NewPrefixFilter(prefix string) *PrefixFilter {
	return &PrefixFilter{prefix: prefix}
}

func (pf *PrefixFilter) Match(item string) bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return strings.HasPrefix(item, pf.prefix)
}

func (pf *PrefixFilter) Name() string { return "prefix" }

type SuffixFilter struct {
	suffix string
	mu     sync.RWMutex
}

func NewSuffixFilter(suffix string) *SuffixFilter {
	return &SuffixFilter{suffix: suffix}
}

func (sf *SuffixFilter) Match(item string) bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return strings.HasSuffix(item, sf.suffix)
}

func (sf *SuffixFilter) Name() string { return "suffix" }

type LengthFilter struct {
	minLen int
	maxLen int
	mu     sync.RWMutex
}

func NewLengthFilter(minLen, maxLen int) *LengthFilter {
	if minLen < 0 {
		minLen = 0
	}
	if maxLen < 0 {
		maxLen = int(^uint(0) >> 1)
	}
	return &LengthFilter{minLen: minLen, maxLen: maxLen}
}

func (lf *LengthFilter) Match(item string) bool {
	lf.mu.RLock()
	defer lf.mu.RUnlock()
	return len(item) >= lf.minLen && len(item) <= lf.maxLen
}

func (lf *LengthFilter) Name() string { return "length" }

type RangeFilter struct {
	minVal float64
	maxVal float64
	mu     sync.RWMutex
}

func NewRangeFilter(minVal, maxVal float64) *RangeFilter {
	return &RangeFilter{minVal: minVal, maxVal: maxVal}
}

func (rf *RangeFilter) Match(item float64) bool {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return item >= rf.minVal && item <= rf.maxVal
}

func (rf *RangeFilter) Name() string { return "range" }

type CompositeFilter[T any] struct {
	filters []Filter[T]
	mode    string
	mu      sync.RWMutex
}

func NewCompositeFilter[T any](mode string, filters ...Filter[T]) *CompositeFilter[T] {
	return &CompositeFilter[T]{filters: filters, mode: mode}
}

func (cf *CompositeFilter[T]) Match(item T) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	switch cf.mode {
	case "and":
		for _, f := range cf.filters {
			if !f.Match(item) {
				return false
			}
		}
		return true
	case "or":
		for _, f := range cf.filters {
			if f.Match(item) {
				return true
			}
		}
		return false
	case "not":
		for _, f := range cf.filters {
			if f.Match(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (cf *CompositeFilter[T]) Name() string { return "composite" }

type MapFilter[K comparable, V any] struct {
	key   K
	value V
	mu    sync.RWMutex
}

func NewMapFilter[K comparable, V any](key K, value V) *MapFilter[K, V] {
	return &MapFilter[K, V]{key: key, value: value}
}

func (mf *MapFilter[K, V]) Match(item map[K]V) bool {
	mf.mu.RLock()
	defer mf.mu.RUnlock()
	val, ok := item[mf.key]
	if !ok {
		return false
	}
	if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", mf.value) {
		return true
	}
	return false
}

func (mf *MapFilter[K, V]) Name() string { return "map" }

type FilterRegistry struct {
	filters map[string]interface{}
	mu      sync.RWMutex
}

func NewFilterRegistry() *FilterRegistry {
	return &FilterRegistry{
		filters: make(map[string]interface{}),
	}
}

func (fr *FilterRegistry) Register(name string, filter interface{}) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.filters[name] = filter
}

func (fr *FilterRegistry) Get(name string) interface{} {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	return fr.filters[name]
}

func (fr *FilterRegistry) List() []string {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	names := make([]string, 0, len(fr.filters))
	for name := range fr.filters {
		names = append(names, name)
	}
	return names
}

type NegateFilter[T any] struct {
	inner Filter[T]
	mu    sync.RWMutex
}

func NewNegateFilter[T any](inner Filter[T]) *NegateFilter[T] {
	return &NegateFilter[T]{inner: inner}
}

func (nf *NegateFilter[T]) Match(item T) bool {
	nf.mu.RLock()
	defer nf.mu.RUnlock()
	return !nf.inner.Match(item)
}

func (nf *NegateFilter[T]) Name() string { return "negate" }

type DeduplicateFilter[T comparable] struct {
	seen   map[T]bool
	mu     sync.RWMutex
}

func NewDeduplicateFilter[T comparable]() *DeduplicateFilter[T] {
	return &DeduplicateFilter[T]{
		seen: make(map[T]bool),
	}
}

func (df *DeduplicateFilter[T]) Match(item T) bool {
	df.mu.Lock()
	defer df.mu.Unlock()
	if df.seen[item] {
		return false
	}
	df.seen[item] = true
	return true
}

func (df *DeduplicateFilter[T]) Reset() {
	df.mu.Lock()
	defer df.mu.Unlock()
	df.seen = make(map[T]bool)
}

func (df *DeduplicateFilter[T]) Name() string { return "deduplicate" }

type SamplingFilter[T any] struct {
	rate  float64
	count int64
	mu    sync.RWMutex
}

func NewSamplingFilter[T any](rate float64) *SamplingFilter[T] {
	if rate <= 0 || rate > 1 {
		rate = 0.1
	}
	return &SamplingFilter[T]{rate: rate}
}

func (sf *SamplingFilter[T]) Match(item T) bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.count++
	return float64(sf.count%1000)/1000.0 < sf.rate
}

func (sf *SamplingFilter[T]) Name() string { return "sampling" }
