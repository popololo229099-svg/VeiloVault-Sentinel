package sort

import (
	"sync"
)

type ParallelQuickSorter[T any] struct {
	less       func(a, b T) bool
	threshold  int
	mu         sync.RWMutex
}

func NewParallelQuickSorter[T any](less func(a, b T) bool) *ParallelQuickSorter[T] {
	return &ParallelQuickSorter[T]{less: less, threshold: 1000}
}

func (pqs *ParallelQuickSorter[T]) Sort(data []T) []T {
	pqs.mu.RLock()
	less := pqs.less
	pqs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	pqs.parallelQuickSort(result, 0, len(result)-1, less)
	return result
}

func (pqs *ParallelQuickSorter[T]) parallelQuickSort(data []T, low, high int, less func(a, b T) bool) {
	if low < high {
		if high-low < pqs.threshold {
			pqs.insertionSort(data, low, high, less)
			return
		}
		pi := pqs.partition(data, low, high, less)
		done := make(chan bool, 2)
		go func() {
			pqs.parallelQuickSort(data, low, pi-1, less)
			done <- true
		}()
		go func() {
			pqs.parallelQuickSort(data, pi+1, high, less)
			done <- true
		}()
		<-done
		<-done
	}
}

func (pqs *ParallelQuickSorter[T]) partition(data []T, low, high int, less func(a, b T) bool) int {
	pivot := data[high]
	i := low - 1
	for j := low; j < high; j++ {
		if less(data[j], pivot) {
			i++
			data[i], data[j] = data[j], data[i]
		}
	}
	data[i+1], data[high] = data[high], data[i+1]
	return i + 1
}

func (pqs *ParallelQuickSorter[T]) insertionSort(data []T, low, high int, less func(a, b T) bool) {
	for i := low + 1; i <= high; i++ {
		key := data[i]
		j := i - 1
		for j >= low && less(key, data[j]) {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}

func (pqs *ParallelQuickSorter[T]) Name() string { return "parallel_quicksort" }

type NaturalMergeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewNaturalMergeSorter[T any](less func(a, b T) bool) *NaturalMergeSorter[T] {
	return &NaturalMergeSorter[T]{less: less}
}

func (nms *NaturalMergeSorter[T]) Sort(data []T) []T {
	nms.mu.RLock()
	less := nms.less
	nms.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	if len(result) <= 1 {
		return result
	}

	for {
		left, right := nms.findRun(result, less)
		if right >= len(result)-1 {
			break
		}
		nextLeft, nextRight := nms.findRun(result[right+1:], less)
		if nextLeft != 0 {
			nms.merge(result, left, right, right+1, right+nextRight, less)
		}
	}

	return result
}

func (nms *NaturalMergeSorter[T]) findRun(data []T, less func(a, b T) bool) (int, int) {
	if len(data) <= 1 {
		return 0, 0
	}

	start := 0
	if less(data[1], data[0]) {
		for i := 1; i < len(data) && less(data[i], data[i-1]); i++ {
			start = i
		}
		for l, r := 0, start; l < r; l, r = l+1, r-1 {
			data[l], data[r] = data[r], data[l]
		}
	} else {
		for i := 1; i < len(data) && !less(data[i], data[i-1]); i++ {
			start = i
		}
	}

	return 0, start
}

func (nms *NaturalMergeSorter[T]) merge(data []T, l1, r1, l2, r2 int, less func(a, b T) bool) {
	left := make([]T, r1-l1+1)
	right := make([]T, r2-l2+1)
	copy(left, data[l1:r1+1])
	copy(right, data[l2:r2+1])

	i, j, k := 0, 0, l1
	for i < len(left) && j < len(right) {
		if less(left[i], right[j]) || (!less(right[j], left[i])) {
			data[k] = left[i]
			i++
		} else {
			data[k] = right[j]
			j++
		}
		k++
	}

	for i < len(left) {
		data[k] = left[i]
		i++
		k++
	}

	for j < len(right) {
		data[k] = right[j]
		j++
		k++
	}
}

func (nms *NaturalMergeSorter[T]) Name() string { return "natural_merge" }

type IntroSorter[T any] struct {
	less      func(a, b T) bool
	maxDepth  int
	mu        sync.RWMutex
}

func NewIntroSorter[T any](less func(a, b T) bool) *IntroSorter[T] {
	return &IntroSorter[T]{less: less, maxDepth: 2 * log2(16)}
}

func log2(n int) int {
	result := 0
	for n > 1 {
		n >>= 1
		result++
	}
	return result
}

func (is *IntroSorter[T]) Sort(data []T) []T {
	is.mu.RLock()
	less := is.less
	is.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	is.introSort(result, 0, len(result)-1, is.maxDepth, less)
	return result
}

func (is *IntroSorter[T]) introSort(data []T, lo, hi, depthLimit int, less func(a, b T) bool) {
	size := hi - lo + 1
	for size > 16 {
		if depthLimit == 0 {
			is.heapSort(data, lo, hi, less)
			return
		}
		depthLimit--
		p := is.medianOfThree(data, lo, hi, less)
		p = is.partition(data, lo, hi, p, less)
		is.introSort(data, p+1, hi, depthLimit, less)
		hi = p - 1
	}
	is.insertionSort(data, lo, hi, less)
}

func (is *IntroSorter[T]) medianOfThree(data []T, lo, hi int, less func(a, b T) bool) int {
	mid := (lo + hi) / 2
	if less(data[mid], data[lo]) {
		data[lo], data[mid] = data[mid], data[lo]
	}
	if less(data[hi], data[lo]) {
		data[lo], data[hi] = data[hi], data[lo]
	}
	if less(data[hi], data[mid]) {
		data[mid], data[hi] = data[hi], data[mid]
	}
	return mid
}

func (is *IntroSorter[T]) partition(data []T, lo, hi, pivotIdx int, less func(a, b T) bool) int {
	pivot := data[pivotIdx]
	data[pivotIdx], data[hi] = data[hi], data[pivotIdx]
	i := lo
	for j := lo; j < hi; j++ {
		if less(data[j], pivot) {
			data[i], data[j] = data[j], data[i]
			i++
		}
	}
	data[i], data[hi] = data[hi], data[i]
	return i
}

func (is *IntroSorter[T]) heapSort(data []T, lo, hi int, less func(a, b T) bool) {
	n := hi - lo + 1
	for i := n/2 - 1; i >= 0; i-- {
		is.siftDown(data, lo, n, i, less)
	}
	for i := n - 1; i > 0; i-- {
		data[lo], data[lo+i] = data[lo+i], data[lo]
		is.siftDown(data, lo, i, 0, less)
	}
}

func (is *IntroSorter[T]) siftDown(data []T, lo, n, i int, less func(a, b T) bool) {
 largest := i
 left := 2*i + 1
 right := 2*i + 2

	if left < n && less(data[lo+largest], data[lo+left]) {
		largest = left
	}
	if right < n && less(data[lo+largest], data[lo+right]) {
		largest = right
	}
	if largest != i {
		data[lo+i], data[lo+largest] = data[lo+largest], data[lo+i]
		is.siftDown(data, lo, n, largest, less)
	}
}

func (is *IntroSorter[T]) insertionSort(data []T, lo, hi int, less func(a, b T) bool) {
	for i := lo + 1; i <= hi; i++ {
		key := data[i]
		j := i - 1
		for j >= lo && less(key, data[j]) {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}

func (is *IntroSorter[T]) Name() string { return "introsort" }

type TreeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewTreeSorter[T any](less func(a, b T) bool) *TreeSorter[T] {
	return &TreeSorter[T]{less: less}
}

type treeNode[T any] struct {
	value T
	left  *treeNode[T]
	right *treeNode[T]
}

func (ts *TreeSorter[T]) Sort(data []T) []T {
	ts.mu.RLock()
	less := ts.less
	ts.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	var root *treeNode[T]
	for _, v := range data {
		root = ts.insert(root, v, less)
	}

	result := make([]T, 0, len(data))
	result = ts.inOrder(root, result)
	return result
}

func (ts *TreeSorter[T]) insert(node *treeNode[T], value T, less func(a, b T) bool) *treeNode[T] {
	if node == nil {
		return &treeNode[T]{value: value}
	}
	if less(value, node.value) {
		node.left = ts.insert(node.left, value, less)
	} else {
		node.right = ts.insert(node.right, value, less)
	}
	return node
}

func (ts *TreeSorter[T]) inOrder(node *treeNode[T], result []T) []T {
	if node == nil {
		return result
	}
	result = ts.inOrder(node.left, result)
	result = append(result, node.value)
	result = ts.inOrder(node.right, result)
	return result
}

func (ts *TreeSorter[T]) Name() string { return "treesort" }

type DualPivotQuickSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewDualPivotQuickSorter[T any](less func(a, b T) bool) *DualPivotQuickSorter[T] {
	return &DualPivotQuickSorter[T]{less: less}
}

func (dpq *DualPivotQuickSorter[T]) Sort(data []T) []T {
	dpq.mu.RLock()
	less := dpq.less
	dpq.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	dpq.dualPivotQuickSort(result, 0, len(result)-1, less)
	return result
}

func (dpq *DualPivotQuickSorter[T]) dualPivotQuickSort(data []T, lo, hi int, less func(a, b T) bool) {
	if lo >= hi {
		return
	}

	if less(data[hi], data[lo]) {
		data[lo], data[hi] = data[hi], data[lo]
	}

	pivot1 := data[lo]
	pivot2 := data[hi]

	i := lo + 1
	k := lo + 1
	j := hi - 1

	for k <= j {
		if less(data[k], pivot1) {
			data[i], data[k] = data[k], data[i]
			i++
			k++
		} else if !less(data[k], pivot2) {
			for j >= k && !less(pivot2, data[j]) {
				j--
			}
			if j < k {
				break
			}
			data[k], data[j] = data[j], data[k]
			j--
			if less(data[k], pivot1) {
				data[i], data[k] = data[k], data[i]
				i++
			}
			k++
		} else {
			k++
		}
	}

	i--
	j++
	data[lo], data[i] = data[i], data[lo]
	data[hi], data[j] = data[j], data[hi]

	dpq.dualPivotQuickSort(data, lo, i-1, less)
	dpq.dualPivotQuickSort(data, i+1, j-1, less)
	dpq.dualPivotQuickSort(data, j+1, hi, less)
}

func (dpq *DualPivotQuickSorter[T]) Name() string { return "dual_pivot_quicksort" }
