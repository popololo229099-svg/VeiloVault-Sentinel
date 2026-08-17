package sort

import (
	"sync"
)

type ExternalMergeSorter[T any] struct {
	less        func(a, b T) bool
	chunkSize   int
	mergeFunc   func([]T, []T) []T
	mu          sync.RWMutex
}

func NewExternalMergeSorter[T any](less func(a, b T) bool, chunkSize int) *ExternalMergeSorter[T] {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	return &ExternalMergeSorter[T]{
		less:      less,
		chunkSize: chunkSize,
	}
}

func (ems *ExternalMergeSorter[T]) Sort(data []T) []T {
	ems.mu.RLock()
	less := ems.less
	ems.mu.RUnlock()

	if len(data) <= ems.chunkSize {
		result := make([]T, len(data))
		copy(result, data)
		ems.insertionSort(result, less)
		return result
	}

	chunks := make([][]T, 0)
	for i := 0; i < len(data); i += ems.chunkSize {
		end := i + ems.chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]T, end-i)
		copy(chunk, data[i:end])
		ems.insertionSort(chunk, less)
		chunks = append(chunks, chunk)
	}

	for len(chunks) > 1 {
		var merged [][]T
		for i := 0; i < len(chunks); i += 2 {
			if i+1 < len(chunks) {
				merged = append(merged, ems.mergeChunks(chunks[i], chunks[i+1], less))
			} else {
				merged = append(merged, chunks[i])
			}
		}
		chunks = merged
	}

	if len(chunks) == 0 {
		return nil
	}
	return chunks[0]
}

func (ems *ExternalMergeSorter[T]) mergeChunks(a, b []T, less func(a, b T) bool) []T {
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

func (ems *ExternalMergeSorter[T]) insertionSort(data []T, less func(a, b T) bool) {
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

func (ems *ExternalMergeSorter[T]) Name() string { return "external_merge" }

type BitonicSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewBitonicSorter[T any](less func(a, b T) bool) *BitonicSorter[T] {
	return &BitonicSorter[T]{less: less}
}

func (bs *BitonicSorter[T]) Sort(data []T) []T {
	bs.mu.RLock()
	less := bs.less
	bs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	bs.bitonicSort(result, 0, len(result), true, less)
	return result
}

func (bs *BitonicSorter[T]) bitonicSort(data []T, low, cnt int, dir bool, less func(a, b T) bool) {
	if cnt > 1 {
		k := cnt / 2
		bs.bitonicSort(data, low, k, true, less)
		bs.bitonicSort(data, low+k, k, false, less)
		bs.bitonicMerge(data, low, cnt, dir, less)
	}
}

func (bs *BitonicSorter[T]) bitonicMerge(data []T, low, cnt int, dir bool, less func(a, b T) bool) {
	if cnt > 1 {
		k := cnt / 2
		for i := low; i < low+k; i++ {
			if dir == less(data[i+k], data[i]) {
				data[i], data[i+k] = data[i+k], data[i]
			}
		}
		bs.bitonicMerge(data, low, k, dir, less)
		bs.bitonicMerge(data, low+k, k, dir, less)
	}
}

func (bs *BitonicSorter[T]) Name() string { return "bitonic" }

type PancakeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewPancakeSorter[T any](less func(a, b T) bool) *PancakeSorter[T] {
	return &PancakeSorter[T]{less: less}
}

func (ps *PancakeSorter[T]) Sort(data []T) []T {
	ps.mu.RLock()
	less := ps.less
	ps.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	for i := len(result); i > 1; i-- {
		maxIdx := 0
		for j := 1; j < i; j++ {
			if less(result[maxIdx], result[j]) {
				maxIdx = j
			}
		}

		if maxIdx != i-1 {
			ps.flip(result, maxIdx)
			ps.flip(result, i-1)
		}
	}

	return result
}

func (ps *PancakeSorter[T]) flip(data []T, k int) {
	for i, j := 0, k; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

func (ps *PancakeSorter[T]) Name() string { return "pancake" }

type SelectionSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewSelectionSorter[T any](less func(a, b T) bool) *SelectionSorter[T] {
	return &SelectionSorter[T]{less: less}
}

func (ss *SelectionSorter[T]) Sort(data []T) []T {
	ss.mu.RLock()
	less := ss.less
	ss.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	for i := 0; i < len(result)-1; i++ {
		minIdx := i
		for j := i + 1; j < len(result); j++ {
			if less(result[j], result[minIdx]) {
				minIdx = j
			}
		}
		if minIdx != i {
			result[i], result[minIdx] = result[minIdx], result[i]
		}
	}

	return result
}

func (ss *SelectionSorter[T]) Name() string { return "selection" }

type CocktailSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewCocktailSorter[T any](less func(a, b T) bool) *CocktailSorter[T] {
	return &CocktailSorter[T]{less: less}
}

func (cs *CocktailSorter[T]) Sort(data []T) []T {
	cs.mu.RLock()
	less := cs.less
	cs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	n := len(result)
	start, end := 0, n-1
	swapped := true

	for swapped {
		swapped = false

		for i := start; i < end; i++ {
			if less(result[i+1], result[i]) {
				result[i], result[i+1] = result[i+1], result[i]
				swapped = true
			}
		}
		end--

		if !swapped {
			break
		}

		swapped = false
		for i := end; i > start; i-- {
			if less(result[i], result[i-1]) {
				result[i], result[i-1] = result[i-1], result[i]
				swapped = true
			}
		}
		start++
	}

	return result
}

func (cs *CocktailSorter[T]) Name() string { return "cocktail" }

type GnomeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewGnomeSorter[T any](less func(a, b T) bool) *GnomeSorter[T] {
	return &GnomeSorter[T]{less: less}
}

func (gs *GnomeSorter[T]) Sort(data []T) []T {
	gs.mu.RLock()
	less := gs.less
	gs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	i := 0
	n := len(result)

	for i < n {
		if i == 0 || !less(result[i], result[i-1]) {
			i++
		} else {
			result[i], result[i-1] = result[i-1], result[i]
			i--
		}
	}

	return result
}

func (gs *GnomeSorter[T]) Name() string { return "gnome" }

type OddEvenSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewOddEvenSorter[T any](less func(a, b T) bool) *OddEvenSorter[T] {
	return &OddEvenSorter[T]{less: less}
}

func (oes *OddEvenSorter[T]) Sort(data []T) []T {
	oes.mu.RLock()
	less := oes.less
	oes.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	n := len(result)
	sorted := false

	for !sorted {
		sorted = true

		for i := 1; i < n-1; i += 2 {
			if less(result[i+1], result[i]) {
				result[i], result[i+1] = result[i+1], result[i]
				sorted = false
			}
		}

		for i := 0; i < n-1; i += 2 {
			if less(result[i+1], result[i]) {
				result[i], result[i+1] = result[i+1], result[i]
				sorted = false
			}
		}
	}

	return result
}

func (oes *OddEvenSorter[T]) Name() string { return "odd_even" }

type LibrarySorter[T any] struct {
	less     func(a, b T) bool
	gapSize  float64
	mu       sync.RWMutex
}

func NewLibrarySorter[T any](less func(a, b T) bool) *LibrarySorter[T] {
	return &LibrarySorter[T]{less: less, gapSize: 0.5}
}

func (ls *LibrarySorter[T]) Sort(data []T) []T {
	ls.mu.RLock()
	less := ls.less
	ls.mu.RUnlock()

	if len(data) <= 1 {
		result := make([]T, len(data))
		copy(result, data)
		return result
	}

	result := make([]T, 0, len(data)*2)
	result = append(result, data[0])

	for i := 1; i < len(data); i++ {
		pos := ls.binarySearch(result, data[i], less)
		var zero T
		result = append(result, zero)
		copy(result[pos+1:], result[pos:])
		result[pos] = data[i]
	}

	return result
}

func (ls *LibrarySorter[T]) binarySearch(data []T, target T, less func(a, b T) bool) int {
	lo, hi := 0, len(data)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if less(data[mid], target) {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func (ls *LibrarySorter[T]) Name() string { return "library" }
