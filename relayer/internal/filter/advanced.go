package filter

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type TimeRangeFilter struct {
	start time.Time
	end   time.Time
	mu    sync.RWMutex
}

func NewTimeRangeFilter(start, end time.Time) *TimeRangeFilter {
	return &TimeRangeFilter{start: start, end: end}
}

func (trf *TimeRangeFilter) Match(item time.Time) bool {
	trf.mu.RLock()
	defer trf.mu.RUnlock()
	return !item.Before(trf.start) && !item.After(trf.end)
}

func (trf *TimeRangeFilter) Name() string { return "time_range" }

type ContainsFilter struct {
	substring string
	caseSense bool
	mu        sync.RWMutex
}

func NewContainsFilter(substring string, caseSense ...bool) *ContainsFilter {
	cs := true
	if len(caseSense) > 0 {
		cs = caseSense[0]
	}
	return &ContainsFilter{substring: substring, caseSense: cs}
}

func (cf *ContainsFilter) Match(item string) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	if cf.caseSense {
		return strings.Contains(item, cf.substring)
	}
	return strings.Contains(strings.ToLower(item), strings.ToLower(cf.substring))
}

func (cf *ContainsFilter) Name() string { return "contains" }

type AllOfFilter[T any] struct {
	filters []Filter[T]
	mu      sync.RWMutex
}

func NewAllOfFilter[T any](filters ...Filter[T]) *AllOfFilter[T] {
	return &AllOfFilter[T]{filters: filters}
}

func (af *AllOfFilter[T]) Match(item T) bool {
	af.mu.RLock()
	defer af.mu.RUnlock()
	for _, f := range af.filters {
		if !f.Match(item) {
			return false
		}
	}
	return true
}

func (af *AllOfFilter[T]) Name() string { return "all_of" }

type AnyOfFilter[T any] struct {
	filters []Filter[T]
	mu      sync.RWMutex
}

func NewAnyOfFilter[T any](filters ...Filter[T]) *AnyOfFilter[T] {
	return &AnyOfFilter[T]{filters: filters}
}

func (af *AnyOfFilter[T]) Match(item T) bool {
	af.mu.RLock()
	defer af.mu.RUnlock()
	for _, f := range af.filters {
		if f.Match(item) {
			return true
		}
	}
	return false
}

func (af *AnyOfFilter[T]) Name() string { return "any_of" }

type NoneOfFilter[T any] struct {
	filters []Filter[T]
	mu      sync.RWMutex
}

func NewNoneOfFilter[T any](filters ...Filter[T]) *NoneOfFilter[T] {
	return &NoneOfFilter[T]{filters: filters}
}

func (nf *NoneOfFilter[T]) Match(item T) bool {
	nf.mu.RLock()
	defer nf.mu.RUnlock()
	for _, f := range nf.filters {
		if f.Match(item) {
			return false
		}
	}
	return true
}

func (nf *NoneOfFilter[T]) Name() string { return "none_of" }

type SetFilter[T comparable] struct {
	allowed map[T]bool
	mu      sync.RWMutex
}

func NewSetFilter[T comparable](allowed ...T) *SetFilter[T] {
	set := make(map[T]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	return &SetFilter[T]{allowed: set}
}

func (sf *SetFilter[T]) Match(item T) bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.allowed[item]
}

func (sf *SetFilter[T]) Name() string { return "set" }

type ExcludeSetFilter[T comparable] struct {
	excluded map[T]bool
	mu       sync.RWMutex
}

func NewExcludeSetFilter[T comparable](excluded ...T) *ExcludeSetFilter[T] {
	set := make(map[T]bool, len(excluded))
	for _, e := range excluded {
		set[e] = true
	}
	return &ExcludeSetFilter[T]{excluded: set}
}

func (esf *ExcludeSetFilter[T]) Match(item T) bool {
	esf.mu.RLock()
	defer esf.mu.RUnlock()
	return !esf.excluded[item]
}

func (esf *ExcludeSetFilter[T]) Name() string { return "exclude_set" }

type SliceFilter[T any] struct {
	index int
	inner Filter[T]
	mu    sync.RWMutex
}

func NewSliceFilter[T any](index int, inner Filter[T]) *SliceFilter[T] {
	return &SliceFilter[T]{index: index, inner: inner}
}

func (sf *SliceFilter[T]) Match(item T) bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.inner.Match(item)
}

func (sf *SliceFilter[T]) FilterSlice(items []T) []T {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	result := make([]T, 0)
	for _, item := range items {
		if sf.inner.Match(item) {
			result = append(result, item)
		}
	}
	return result
}

func (sf *SliceFilter[T]) Name() string { return "slice" }

type SortFilter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewSortFilter[T any](less func(a, b T) bool) *SortFilter[T] {
	return &SortFilter[T]{less: less}
}

func (srtf *SortFilter[T]) Match(item T) bool {
	return true
}

func (srtf *SortFilter[T]) Sort(items []T) []T {
	srtf.mu.RLock()
	less := srtf.less
	srtf.mu.RUnlock()

	result := make([]T, len(items))
	copy(result, items)
	sort.SliceStable(result, func(i, j int) bool {
		return less(result[i], result[j])
	})
	return result
}

func (srtf *SortFilter[T]) Name() string { return "sort" }

type UniqueFilter[T comparable] struct {
	mu sync.RWMutex
}

func NewUniqueFilter[T comparable]() *UniqueFilter[T] {
	return &UniqueFilter[T]{}
}

func (uf *UniqueFilter[T]) Filter(items []T) []T {
	uf.mu.RLock()
	defer uf.mu.RUnlock()

	seen := make(map[T]bool)
	result := make([]T, 0)
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func (uf *UniqueFilter[T]) Match(item T) bool {
	return true
}

func (uf *UniqueFilter[T]) Name() string { return "unique" }

type LimitFilter[T any] struct {
	count int
	mu    sync.RWMutex
}

func NewLimitFilter[T any](count int) *LimitFilter[T] {
	if count <= 0 {
		count = 10
	}
	return &LimitFilter[T]{count: count}
}

func (lf *LimitFilter[T]) Limit(items []T) []T {
	lf.mu.RLock()
	defer lf.mu.RUnlock()
	if len(items) > lf.count {
		return items[:lf.count]
	}
	return items
}

func (lf *LimitFilter[T]) Match(item T) bool {
	return true
}

func (lf *LimitFilter[T]) Name() string { return "limit" }

type SkipFilter[T any] struct {
	count int
	mu    sync.RWMutex
}

func NewSkipFilter[T any](count int) *SkipFilter[T] {
	if count < 0 {
		count = 0
	}
	return &SkipFilter[T]{count: count}
}

func (sf *SkipFilter[T]) Skip(items []T) []T {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if sf.count >= len(items) {
		return nil
	}
	return items[sf.count:]
}

func (sf *SkipFilter[T]) Match(item T) bool {
	return true
}

func (sf *SkipFilter[T]) Name() string { return "skip" }

type GroupByFilter[T any, K comparable] struct {
	keyFunc func(T) K
	mu      sync.RWMutex
}

func NewGroupByFilter[T any, K comparable](keyFunc func(T) K) *GroupByFilter[T, K] {
	return &GroupByFilter[T, K]{keyFunc: keyFunc}
}

func (gbf *GroupByFilter[T, K]) Group(items []T) map[K][]T {
	gbf.mu.RLock()
	defer gbf.mu.RUnlock()

	groups := make(map[K][]T)
	for _, item := range items {
		key := gbf.keyFunc(item)
		groups[key] = append(groups[key], item)
	}
	return groups
}

func (gbf *GroupByFilter[T, K]) Match(item T) bool {
	return true
}

func (gbf *GroupByFilter[T, K]) Name() string { return "group_by" }

type MapTransformFilter[T any, R any] struct {
	transform func(T) R
	mu        sync.RWMutex
}

func NewMapTransformFilter[T any, R any](transform func(T) R) *MapTransformFilter[T, R] {
	return &MapTransformFilter[T, R]{transform: transform}
}

func (mtf *MapTransformFilter[T, R]) Transform(items []T) []R {
	mtf.mu.RLock()
	defer mtf.mu.RUnlock()
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = mtf.transform(item)
	}
	return result
}

func (mtf *MapTransformFilter[T, R]) Match(item T) bool {
	return true
}

func (mtf *MapTransformFilter[T, R]) Name() string { return "map" }

type FlatMapFilter[T any, R any] struct {
	transform func(T) []R
	mu        sync.RWMutex
}

func NewFlatMapFilter[T any, R any](transform func(T) []R) *FlatMapFilter[T, R] {
	return &FlatMapFilter[T, R]{transform: transform}
}

func (fmf *FlatMapFilter[T, R]) Transform(items []T) []R {
	fmf.mu.RLock()
	defer fmf.mu.RUnlock()
	result := make([]R, 0)
	for _, item := range items {
		result = append(result, fmf.transform(item)...)
	}
	return result
}

func (fmf *FlatMapFilter[T, R]) Match(item T) bool {
	return true
}

func (fmf *FlatMapFilter[T, R]) Name() string { return "flat_map" }

type ReduceFilter[T any] struct {
	reduce func(T, T) T
	mu     sync.RWMutex
}

func NewReduceFilter[T any](reduce func(T, T) T) *ReduceFilter[T] {
	return &ReduceFilter[T]{reduce: reduce}
}

func (rf *ReduceFilter[T]) Reduce(items []T) T {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	result := items[0]
	for i := 1; i < len(items); i++ {
		result = rf.reduce(result, items[i])
	}
	return result
}

func (rf *ReduceFilter[T]) Match(item T) bool {
	return true
}

func (rf *ReduceFilter[T]) Name() string { return "reduce" }

type DistanceFilter struct {
	lat1, lon1 float64
	maxDist    float64
	mu         sync.RWMutex
}

func NewDistanceFilter(lat1, lon1, maxDistKm float64) *DistanceFilter {
	return &DistanceFilter{lat1: lat1, lon1: lon1, maxDist: maxDistKm}
}

func (df *DistanceFilter) Match(lat, lon float64) bool {
	df.mu.RLock()
	defer df.mu.RUnlock()
	dist := df.haversine(df.lat1, df.lon1, lat, lon)
	return dist <= df.maxDist
}

func (df *DistanceFilter) haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func (df *DistanceFilter) Name() string { return "distance" }

type BloomFilter struct {
	size     uint
	hashFuncs int
	bits    []bool
	mu      sync.RWMutex
}

func NewBloomFilter(size uint, hashFuncs int) *BloomFilter {
	if size == 0 {
		size = 1000
	}
	if hashFuncs <= 0 {
		hashFuncs = 3
	}
	return &BloomFilter{
		size:      size,
		hashFuncs: hashFuncs,
		bits:      make([]bool, size),
	}
}

func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	for i := 0; i < bf.hashFuncs; i++ {
		idx := bf.hash(item, i) % bf.size
		bf.bits[idx] = true
	}
}

func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	for i := 0; i < bf.hashFuncs; i++ {
		idx := bf.hash(item, i) % bf.size
		if !bf.bits[idx] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) hash(item string, seed int) uint {
	h := uint(5381)
	for _, c := range item {
		h = h*33 + uint(c) + uint(seed)
	}
	return h
}

func (bf *BloomFilter) Match(item string) bool {
	return bf.Contains(item)
}

func (bf *BloomFilter) Name() string { return "bloom" }

type CuckooFilter struct {
	buckets    [][]string
	bucketSize int
	numBuckets int
	mu         sync.RWMutex
}

func NewCuckooFilter(numBuckets, bucketSize int) *CuckooFilter {
	if numBuckets <= 0 {
		numBuckets = 1000
	}
	if bucketSize <= 0 {
		bucketSize = 4
	}
	buckets := make([][]string, numBuckets)
	for i := range buckets {
		buckets[i] = make([]string, 0, bucketSize)
	}
	return &CuckooFilter{
		buckets:    buckets,
		bucketSize: bucketSize,
		numBuckets: numBuckets,
	}
}

func (cf *CuckooFilter) Add(item string) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	idx1 := cf.hash1(item) % uint(cf.numBuckets)
	if len(cf.buckets[idx1]) < cf.bucketSize {
		cf.buckets[idx1] = append(cf.buckets[idx1], item)
		return
	}

	idx2 := cf.hash2(item) % uint(cf.numBuckets)
	if len(cf.buckets[idx2]) < cf.bucketSize {
		cf.buckets[idx2] = append(cf.buckets[idx2], item)
		return
	}

	victim := cf.buckets[idx1][0]
	cf.buckets[idx1][0] = item
	cf.buckets[idx1] = append(cf.buckets[idx1][1:], victim)
}

func (cf *CuckooFilter) Contains(item string) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	idx1 := cf.hash1(item) % uint(cf.numBuckets)
	for _, v := range cf.buckets[idx1] {
		if v == item {
			return true
		}
	}

	idx2 := cf.hash2(item) % uint(cf.numBuckets)
	for _, v := range cf.buckets[idx2] {
		if v == item {
			return true
		}
	}

	return false
}

func (cf *CuckooFilter) hash1(item string) uint {
	h := uint(0x811c9dc5)
	for _, c := range item {
		h ^= uint(c)
		h *= 0x01000193
	}
	return h
}

func (cf *CuckooFilter) hash2(item string) uint {
	h := uint(0x01000193)
	for _, c := range item {
		h ^= uint(c)
		h *= 0x811c9dc5
	}
	return h
}

func (cf *CuckooFilter) Match(item string) bool {
	return cf.Contains(item)
}

func (cf *CuckooFilter) Name() string { return "cuckoo" }

type RegexSetFilter struct {
	patterns []*regexp.Regexp
	mu       sync.RWMutex
}

func NewRegexSetFilter(patterns []string) (*RegexSetFilter, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return &RegexSetFilter{patterns: compiled}, nil
}

func (rsf *RegexSetFilter) Match(item string) bool {
	rsf.mu.RLock()
	defer rsf.mu.RUnlock()
	for _, re := range rsf.patterns {
		if re.MatchString(item) {
			return true
		}
	}
	return false
}

func (rsf *RegexSetFilter) Name() string { return "regex_set" }
