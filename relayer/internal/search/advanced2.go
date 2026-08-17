package search

import (
	"math"
	"sync"
)

type RangeSearcher struct {
	mu sync.RWMutex
}

func NewRangeSearcher() *RangeSearcher {
	return &RangeSearcher{}
}

func (rs *RangeSearcher) Search(data []int, low, high int) []int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var results []int
	for _, v := range data {
		if v >= low && v <= high {
			results = append(results, v)
		}
	}
	return results
}

func (rs *RangeSearcher) Name() string { return "range" }

type KthElementSearcher struct {
	mu sync.RWMutex
}

func NewKthElementSearcher() *KthElementSearcher {
	return &KthElementSearcher{}
}

func (kes *KthElementSearcher) Search(data []int, k int) (int, error) {
	kes.mu.RLock()
	defer kes.mu.RUnlock()

	if k < 1 || k > len(data) {
		return 0, nil
	}

	result := make([]int, len(data))
	copy(result, data)

	return kes.quickSelect(result, 0, len(result)-1, k-1), nil
}

func (kes *KthElementSearcher) quickSelect(data []int, lo, hi, k int) int {
	if lo == hi {
		return data[lo]
	}

	pivotIdx := kes.partition(data, lo, hi)

	if k == pivotIdx {
		return data[k]
	} else if k < pivotIdx {
		return kes.quickSelect(data, lo, pivotIdx-1, k)
	} else {
		return kes.quickSelect(data, pivotIdx+1, hi, k)
	}
}

func (kes *KthElementSearcher) partition(data []int, lo, hi int) int {
	pivot := data[hi]
	i := lo
	for j := lo; j < hi; j++ {
		if data[j] <= pivot {
			data[i], data[j] = data[j], data[i]
			i++
		}
	}
	data[i], data[hi] = data[hi], data[i]
	return i
}

func (kes *KthElementSearcher) Name() string { return "kth_element" }

type MedianSearcher struct {
	mu sync.RWMutex
}

func NewMedianSearcher() *MedianSearcher {
	return &MedianSearcher{}
}

func (ms *MedianSearcher) Search(data []float64) float64 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func (ms *MedianSearcher) Name() string { return "median" }

type NearestNeighborSearcher struct {
	points [][]float64
	mu     sync.RWMutex
}

func NewNearestNeighborSearcher() *NearestNeighborSearcher {
	return &NearestNeighborSearcher{}
}

func (nn *NearestNeighborSearcher) AddPoint(point []float64) {
	nn.mu.Lock()
	defer nn.mu.Unlock()
	nn.points = append(nn.points, point)
}

func (nn *NearestNeighborSearcher) Search(query []float64, k int) [][]float64 {
	nn.mu.RLock()
	defer nn.mu.RUnlock()

	if len(nn.points) == 0 || k <= 0 {
		return nil
	}

	type distPoint struct {
		point    []float64
		distance float64
	}

	distances := make([]distPoint, len(nn.points))
	for i, p := range nn.points {
		dist := 0.0
		n := len(query)
		if len(p) < n {
			n = len(p)
		}
		for j := 0; j < n; j++ {
			diff := query[j] - p[j]
			dist += diff * diff
		}
		distances[i] = distPoint{point: p, distance: math.Sqrt(dist)}
	}

	for i := 1; i < len(distances); i++ {
		for j := i; j > 0 && distances[j].distance < distances[j-1].distance; j-- {
			distances[j], distances[j-1] = distances[j-1], distances[j]
		}
	}

	if k > len(distances) {
		k = len(distances)
	}

	results := make([][]float64, k)
	for i := 0; i < k; i++ {
		results[i] = distances[i].point
	}
	return results
}

func (nn *NearestNeighborSearcher) Name() string { return "nearest_neighbor" }

type BinarySearcher2D struct {
	data [][]int
	mu   sync.RWMutex
}

func NewBinarySearcher2D(data [][]int) *BinarySearcher2D {
	return &BinarySearcher2D{data: data}
}

func (bs2d *BinarySearcher2D) Search(target int) (int, int, bool) {
	bs2d.mu.RLock()
	defer bs2d.mu.RUnlock()

	if len(bs2d.data) == 0 || len(bs2d.data[0]) == 0 {
		return -1, -1, false
	}

	rows := len(bs2d.data)
	cols := len(bs2d.data[0])

	row, col := 0, cols-1
	for row < rows && col >= 0 {
		if bs2d.data[row][col] == target {
			return row, col, true
		} else if bs2d.data[row][col] > target {
			col--
		} else {
			row++
		}
	}

	return -1, -1, false
}

func (bs2d *BinarySearcher2D) Name() string { return "binary_2d" }

type TopKSearcher[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewTopKSearcher[T any](less func(a, b T) bool) *TopKSearcher[T] {
	return &TopKSearcher[T]{less: less}
}

func (tk *TopKSearcher[T]) Search(data []T, k int) []T {
	tk.mu.RLock()
	less := tk.less
	tk.mu.RUnlock()

	if k >= len(data) {
		result := make([]T, len(data))
		copy(result, data)
		return result
	}

	result := make([]T, k)
	copy(result, data[:k])

	for i := 1; i < k; i++ {
		for j := i; j > 0 && less(result[j], result[j-1]); j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}

	for i := k; i < len(data); i++ {
		if less(data[i], result[k-1]) {
			result[k-1] = data[i]
			for j := k - 1; j > 0 && less(result[j], result[j-1]); j-- {
				result[j], result[j-1] = result[j-1], result[j]
			}
		}
	}

	return result
}

func (tk *TopKSearcher[T]) Name() string { return "top_k" }

type CountSearcher struct {
	mu sync.RWMutex
}

func NewCountSearcher() *CountSearcher {
	return &CountSearcher{}
}

func (cs *CountSearcher) Count(data []string, target string) int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	count := 0
	for _, s := range data {
		if s == target {
			count++
		}
	}
	return count
}

func (cs *CountSearcher) CountOccurrences(data []byte, pattern []byte) int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if len(pattern) == 0 || len(data) < len(pattern) {
		return 0
	}

	count := 0
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			count++
		}
	}
	return count
}

func (cs *CountSearcher) Name() string { return "count" }

type SubstringSearcher struct {
	mu sync.RWMutex
}

func NewSubstringSearcher() *SubstringSearcher {
	return &SubstringSearcher{}
}

func (ss *SubstringSearcher) Search(text, pattern string) []int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var indices []int
	n, m := len(text), len(pattern)
	if m == 0 || n < m {
		return indices
	}

	for i := 0; i <= n-m; i++ {
		match := true
		for j := 0; j < m; j++ {
			if text[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			indices = append(indices, i)
		}
	}
	return indices
}

func (ss *SubstringSearcher) KMP(text, pattern string) []int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var indices []int
	if len(pattern) == 0 {
		return indices
	}

	lps := ss.computeLPS(pattern)
	i, j := 0, 0

	for i < len(text) {
		if text[i] == pattern[j] {
			i++
			j++
		}
		if j == len(pattern) {
			indices = append(indices, i-j)
			j = lps[j-1]
		} else if i < len(text) && text[i] != pattern[j] {
			if j != 0 {
				j = lps[j-1]
			} else {
				i++
			}
		}
	}
	return indices
}

func (ss *SubstringSearcher) computeLPS(pattern string) []int {
	lps := make([]int, len(pattern))
	length := 0
	i := 1

	for i < len(pattern) {
		if pattern[i] == pattern[length] {
			length++
			lps[i] = length
			i++
		} else {
			if length != 0 {
				length = lps[length-1]
			} else {
				lps[i] = 0
				i++
			}
		}
	}
	return lps
}

func (ss *SubstringSearcher) Name() string { return "substring" }
