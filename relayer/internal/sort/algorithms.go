package sort

import (
	"sync"
)

type CountingSorterGeneric[T any] struct {
	keyFunc func(T) int
	max     int
	mu      sync.RWMutex
}

func NewCountingSorterGeneric[T any](keyFunc func(T) int, max int) *CountingSorterGeneric[T] {
	if max <= 0 {
		max = 10000
	}
	return &CountingSorterGeneric[T]{keyFunc: keyFunc, max: max}
}

func (cs *CountingSorterGeneric[T]) Sort(data []T) []T {
	cs.mu.RLock()
	keyFunc := cs.keyFunc
	max := cs.max
	cs.mu.RUnlock()

	count := make([]int, max+1)
	for _, item := range data {
		key := keyFunc(item)
		if key >= 0 && key <= max {
			count[key]++
		}
	}

	for i := 1; i <= max; i++ {
		count[i] += count[i-1]
	}

	result := make([]T, len(data))
	for i := len(data) - 1; i >= 0; i-- {
		key := keyFunc(data[i])
		if key >= 0 && key <= max {
			count[key]--
			result[count[key]] = data[i]
		}
	}

	return result
}

func (cs *CountingSorterGeneric[T]) Name() string { return "counting" }

type BucketSorterGeneric[T any] struct {
	bucketCount int
	keyFunc     func(T) float64
	less        func(a, b T) bool
	mu          sync.RWMutex
}

func NewBucketSorterGeneric[T any](bucketCount int, keyFunc func(T) float64, less func(a, b T) bool) *BucketSorterGeneric[T] {
	if bucketCount <= 0 {
		bucketCount = 10
	}
	return &BucketSorterGeneric[T]{
		bucketCount: bucketCount,
		keyFunc:     keyFunc,
		less:        less,
	}
}

func (bsg *BucketSorterGeneric[T]) Sort(data []T) []T {
	bsg.mu.RLock()
	keyFunc := bsg.keyFunc
	less := bsg.less
	bucketCount := bsg.bucketCount
	bsg.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	minVal := keyFunc(data[0])
	maxVal := keyFunc(data[0])
	for _, item := range data {
		v := keyFunc(item)
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	bucketRange := (maxVal - minVal) / float64(bucketCount)
	if bucketRange == 0 {
		bucketRange = 1
	}

	buckets := make([][]T, bucketCount)
	for _, item := range data {
		v := keyFunc(item)
		idx := int((v - minVal) / bucketRange)
		if idx >= bucketCount {
			idx = bucketCount - 1
		}
		buckets[idx] = append(buckets[idx], item)
	}

	result := make([]T, 0, len(data))
	for _, bucket := range buckets {
		insertionSortGeneric(bucket, less)
		result = append(result, bucket...)
	}

	return result
}

func insertionSortGeneric[T any](data []T, less func(a, b T) bool) {
	for i := 1; i < len(data); i++ {
		key := data[i]
		j := i - 1
		for j >= 0 && less(key, data[j]) {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}

func (bsg *BucketSorterGeneric[T]) Name() string { return "bucket_generic" }

type PigeonholeSorter[T any] struct {
	keyFunc func(T) int
	range_  int
	mu      sync.RWMutex
}

func NewPigeonholeSorter[T any](keyFunc func(T) int, range_ int) *PigeonholeSorter[T] {
	if range_ <= 0 {
		range_ = 10000
	}
	return &PigeonholeSorter[T]{keyFunc: keyFunc, range_: range_}
}

func (ps *PigeonholeSorter[T]) Sort(data []T) []T {
	ps.mu.RLock()
	keyFunc := ps.keyFunc
	range_ := ps.range_
	ps.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	minVal := keyFunc(data[0])
	for _, item := range data {
		v := keyFunc(item)
		if v < minVal {
			minVal = v
		}
	}

	holes := make([][]T, range_)
	for _, item := range data {
		idx := keyFunc(item) - minVal
		if idx >= 0 && idx < range_ {
			holes[idx] = append(holes[idx], item)
		}
	}

	result := make([]T, 0, len(data))
	for _, hole := range holes {
		result = append(result, hole...)
	}

	return result
}

func (ps *PigeonholeSorter[T]) Name() string { return "pigeonhole" }

type CombSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewCombSorter[T any](less func(a, b T) bool) *CombSorter[T] {
	return &CombSorter[T]{less: less}
}

func (cs *CombSorter[T]) Sort(data []T) []T {
	cs.mu.RLock()
	less := cs.less
	cs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	n := len(result)
	gap := n
	shrink := 1.3
	sorted := false

	for !sorted {
		gap = int(float64(gap) / shrink)
		if gap <= 1 {
			gap = 1
			sorted = true
		}

		for i := 0; i+gap < n; i++ {
			if less(result[i+gap], result[i]) {
				result[i], result[i+gap] = result[i+gap], result[i]
				sorted = false
			}
		}
	}

	return result
}

func (cs *CombSorter[T]) Name() string { return "comb" }

type StoogeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewStoogeSorter[T any](less func(a, b T) bool) *StoogeSorter[T] {
	return &StoogeSorter[T]{less: less}
}

func (ss *StoogeSorter[T]) Sort(data []T) []T {
	ss.mu.RLock()
	less := ss.less
	ss.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	ss.stoogeSort(result, 0, len(result)-1, less)
	return result
}

func (ss *StoogeSorter[T]) stoogeSort(data []T, lo, hi int, less func(a, b T) bool) {
	if lo >= hi {
		return
	}

	if less(data[hi], data[lo]) {
		data[lo], data[hi] = data[hi], data[lo]
	}

	if hi-lo+1 > 2 {
		t := (hi - lo + 1) / 3
		ss.stoogeSort(data, lo, hi-t, less)
		ss.stoogeSort(data, lo+t, hi, less)
		ss.stoogeSort(data, lo, hi-t, less)
	}
}

func (ss *StoogeSorter[T]) Name() string { return "stooge" }

type CycleSorter[T comparable] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewCycleSorter[T comparable](less func(a, b T) bool) *CycleSorter[T] {
	return &CycleSorter[T]{less: less}
}

func (cs *CycleSorter[T]) Sort(data []T) []T {
	cs.mu.RLock()
	less := cs.less
	cs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	n := len(result)

	for cycleStart := 0; cycleStart < n-1; cycleStart++ {
		item := result[cycleStart]
		pos := cycleStart

		for i := cycleStart + 1; i < n; i++ {
			if less(result[i], item) {
				pos++
			}
		}

		if pos == cycleStart {
			continue
		}

		for item == result[pos] {
			pos++
		}

		result[pos], item = item, result[pos]

		for pos != cycleStart {
			pos = cycleStart
			for i := cycleStart + 1; i < n; i++ {
				if less(result[i], item) {
					pos++
				}
			}

			for item == result[pos] {
				pos++
			}

			result[pos], item = item, result[pos]
		}
	}

	return result
}

func (cs *CycleSorter[T]) Name() string { return "cycle" }

type StrandSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewStrandSorter[T any](less func(a, b T) bool) *StrandSorter[T] {
	return &StrandSorter[T]{less: less}
}

func (ss *StrandSorter[T]) Sort(data []T) []T {
	ss.mu.RLock()
	less := ss.less
	ss.mu.RUnlock()

	if len(data) <= 1 {
		result := make([]T, len(data))
		copy(result, data)
		return result
	}

	input := make([]T, len(data))
	copy(input, data)

	var result []T

	for len(input) > 0 {
	 strand := []T{input[0]}
	 input = input[1:]

		j := 0
		for j < len(input) {
			if less(strand[len(strand)-1], input[j]) {
				strand = append(strand, input[j])
				input = append(input[:j], input[j+1:]...)
			} else {
				j++
			}
		}

		result = mergeStrands(result, strand, less)
	}

	return result
}

func mergeStrands[T any](a, b []T, less func(a, b T) bool) []T {
	result := make([]T, 0, len(a)+len(b))
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if less(a[i], b[j]) || (!less(b[j], a[i])) {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}

	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func (ss *StrandSorter[T]) Name() string { return "strand" }

type TournamentSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewTournamentSorter[T any](less func(a, b T) bool) *TournamentSorter[T] {
	return &TournamentSorter[T]{less: less}
}

func (ts *TournamentSorter[T]) Sort(data []T) []T {
	ts.mu.RLock()
	less := ts.less
	ts.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	n := len(result)
	tree := make([]int, 2*n)

	for i := 0; i < n; i++ {
		tree[n+i] = i
	}

	for i := n - 1; i > 0; i-- {
		left := tree[2*i]
		right := tree[2*i+1]
		if left < n && right < n {
			if less(result[left], result[right]) {
				tree[i] = left
			} else {
				tree[i] = right
			}
		} else if left < n {
			tree[i] = left
		} else {
			tree[i] = right
		}
	}

	sorted := make([]T, 0, n)
	for len(sorted) < n {
		winner := tree[1]
		if winner >= n {
			break
		}
		sorted = append(sorted, result[winner])
		result[winner] = result[n-1]
		n--
		tree[n] = winner

		for i := n / 2; i > 0; i-- {
			left := tree[2*i]
			right := tree[2*i+1]
			if left < n && right < n {
				if less(result[left], result[right]) {
					tree[i] = left
				} else {
					tree[i] = right
				}
			} else if left < n {
				tree[i] = left
			} else {
				tree[i] = right
			}
		}
	}

	return sorted
}

func (ts *TournamentSorter[T]) Name() string { return "tournament" }
