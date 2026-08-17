package search

import (
	"math"
	"sort"
	"sync"
)

type Searcher[T any] interface {
	Search(data []T, target T) int
	Name() string
}

type BinarySearcher[T any] struct {
	less func(a, b T) bool
	equal func(a, b T) bool
	mu   sync.RWMutex
}

func NewBinarySearcher[T any](less func(a, b T) bool, equal func(a, b T) bool) *BinarySearcher[T] {
	return &BinarySearcher[T]{less: less, equal: equal}
}

func (bs *BinarySearcher[T]) Search(data []T, target T) int {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	lo, hi := 0, len(data)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if bs.equal(data[mid], target) {
			return mid
		}
		if bs.less(data[mid], target) {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return -1
}

func (bs *BinarySearcher[T]) Name() string { return "binary" }

type InterpolationSearcher struct {
	mu sync.RWMutex
}

func NewInterpolationSearcher() *InterpolationSearcher {
	return &InterpolationSearcher{}
}

func (is *InterpolationSearcher) Search(data []int, target int) int {
	is.mu.RLock()
	defer is.mu.RUnlock()

	if len(data) == 0 {
		return -1
	}

	lo, hi := 0, len(data)-1

	for lo <= hi && target >= data[lo] && target <= data[hi] {
		if lo == hi {
			if data[lo] == target {
				return lo
			}
			return -1
		}

		pos := lo + ((target-data[lo])*(hi-lo))/(data[hi]-data[lo])

		if pos < lo || pos > hi {
			return -1
		}

		if data[pos] == target {
			return pos
		}

		if data[pos] < target {
			lo = pos + 1
		} else {
			hi = pos - 1
		}
	}

	return -1
}

func (is *InterpolationSearcher) Name() string { return "interpolation" }

type ExponentialSearcher struct {
	mu sync.RWMutex
}

func NewExponentialSearcher() *ExponentialSearcher {
	return &ExponentialSearcher{}
}

func (es *ExponentialSearcher) Search(data []int, target int) int {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if len(data) == 0 {
		return -1
	}

	if data[0] == target {
		return 0
	}

	bound := 1
	for bound < len(data) && data[bound] <= target {
		bound *= 2
	}

	hi := bound
	if hi >= len(data) {
		hi = len(data) - 1
	}

	return sort.SearchInts(data[bound/2:hi+1], target) + bound/2
}

func (es *ExponentialSearcher) Name() string { return "exponential" }

type JumpSearcher struct {
	mu sync.RWMutex
}

func NewJumpSearcher() *JumpSearcher {
	return &JumpSearcher{}
}

func (js *JumpSearcher) Search(data []int, target int) int {
	js.mu.RLock()
	defer js.mu.RUnlock()

	n := len(data)
	if n == 0 {
		return -1
	}

	step := int(math.Sqrt(float64(n)))
	prev := 0

	for data[int(math.Min(float64(step), float64(n-1)))] < target {
		prev = step
		step += int(math.Sqrt(float64(n)))
		if prev >= n {
			return -1
		}
	}

	for prev < int(math.Min(float64(step), float64(n))) {
		if data[prev] == target {
			return prev
		}
		prev++
	}

	return -1
}

func (js *JumpSearcher) Name() string { return "jump" }

type FibonacciSearcher struct {
	mu sync.RWMutex
}

func NewFibonacciSearcher() *FibonacciSearcher {
	return &FibonacciSearcher{}
}

func (fs *FibonacciSearcher) Search(data []int, target int) int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	n := len(data)
	if n == 0 {
		return -1
	}

	fib2 := 0
	fib1 := 1
	fib := fib1 + fib2

	for fib < n {
		fib2 = fib1
		fib1 = fib
		fib = fib1 + fib2
	}

	offset := -1

	for fib > 1 {
		i := int(math.Min(float64(offset+fib2), float64(n-1)))

		if data[i] < target {
			fib = fib1
			fib1 = fib2
			fib2 = fib - fib1
			offset = i
		} else if data[i] > target {
			fib = fib2
			fib1 = fib1 - fib2
			fib2 = fib - fib1
		} else {
			return i
		}
	}

	if fib1 > 0 && data[offset+1] == target {
		return offset + 1
	}

	return -1
}

func (fs *FibonacciSearcher) Name() string { return "fibonacci" }

type TernarySearcher struct {
	less func(a, b float64) bool
	mu   sync.RWMutex
}

func NewTernarySearcher(less func(a, b float64) bool) *TernarySearcher {
	if less == nil {
		less = func(a, b float64) bool { return a < b }
	}
	return &TernarySearcher{less: less}
}

func (ts *TernarySearcher) Search(data []float64, target float64) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if len(data) == 0 {
		return -1
	}

	lo, hi := 0, len(data)-1
	for lo <= hi {
		if lo == hi {
			if math.Abs(data[lo]-target) < 1e-9 {
				return lo
			}
			return -1
		}

		mid1 := lo + (hi-lo)/3
		mid2 := hi - (hi-lo)/3

		if math.Abs(data[mid1]-target) < 1e-9 {
			return mid1
		}
		if math.Abs(data[mid2]-target) < 1e-9 {
			return mid2
		}

		if target < data[mid1] {
			hi = mid1 - 1
		} else if target > data[mid2] {
			lo = mid2 + 1
		} else {
			lo = mid1 + 1
			hi = mid2 - 1
		}
	}
	return -1
}

func (ts *TernarySearcher) Name() string { return "ternary" }

type FuzzySearcher struct {
	maxDistance int
	mu         sync.RWMutex
}

func NewFuzzySearcher(maxDistance int) *FuzzySearcher {
	if maxDistance <= 0 {
		maxDistance = 2
	}
	return &FuzzySearcher{maxDistance: maxDistance}
}

func (fs *FuzzySearcher) Search(data []string, target string) []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	results := make([]string, 0)
	for _, item := range data {
		if fs.levenshtein(item, target) <= fs.maxDistance {
			results = append(results, item)
		}
	}
	return results
}

func (fs *FuzzySearcher) levenshtein(s, t string) int {
	if len(s) == 0 {
		return len(t)
	}
	if len(t) == 0 {
		return len(s)
	}

	dp := make([][]int, len(s)+1)
	for i := range dp {
		dp[i] = make([]int, len(t)+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}

	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			cost := 0
			if s[i-1] != t[j-1] {
				cost = 1
			}
			dp[i][j] = min3(
				dp[i-1][j]+1,
				dp[i][j-1]+1,
				dp[i-1][j-1]+cost,
			)
		}
	}

	return dp[len(s)][len(t)]
}

func (fs *FuzzySearcher) Name() string { return "fuzzy" }

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

type ApproximateSearcher struct {
	threshold float64
	mu        sync.RWMutex
}

func NewApproximateSearcher(threshold float64) *ApproximateSearcher {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.8
	}
	return &ApproximateSearcher{threshold: threshold}
}

func (as *ApproximateSearcher) Search(data []string, target string) []string {
	as.mu.RLock()
	defer as.mu.RUnlock()

	results := make([]string, 0)
	for _, item := range data {
		similarity := as.jaccardSimilarity(item, target)
		if similarity >= as.threshold {
			results = append(results, item)
		}
	}
	return results
}

func (as *ApproximateSearcher) jaccardSimilarity(s, t string) float64 {
	set1 := make(map[rune]bool)
	set2 := make(map[rune]bool)

	for _, c := range s {
		set1[c] = true
	}
	for _, c := range t {
		set2[c] = true
	}

	intersection := 0
	for c := range set1 {
		if set2[c] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

func (as *ApproximateSearcher) Name() string { return "approximate" }

type LinearSearcher[T any] struct {
	equal func(a, b T) bool
	mu    sync.RWMutex
}

func NewLinearSearcher[T any](equal func(a, b T) bool) *LinearSearcher[T] {
	return &LinearSearcher[T]{equal: equal}
}

func (ls *LinearSearcher[T]) Search(data []T, target T) int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	for i, item := range data {
		if ls.equal(item, target) {
			return i
		}
	}
	return -1
}

func (ls *LinearSearcher[T]) Name() string { return "linear" }

type SearchRegistry[T any] struct {
	searchers map[string]Searcher[T]
	mu        sync.RWMutex
}

func NewSearchRegistry[T any]() *SearchRegistry[T] {
	return &SearchRegistry[T]{
		searchers: make(map[string]Searcher[T]),
	}
}

func (sr *SearchRegistry[T]) Register(searcher Searcher[T]) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.searchers[searcher.Name()] = searcher
}

func (sr *SearchRegistry[T]) Get(name string) Searcher[T] {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.searchers[name]
}

func (sr *SearchRegistry[T]) List() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	names := make([]string, 0, len(sr.searchers))
	for name := range sr.searchers {
		names = append(names, name)
	}
	return names
}
