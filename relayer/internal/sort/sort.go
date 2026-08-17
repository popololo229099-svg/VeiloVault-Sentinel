package sort

import (
	"sync"
)

type Sorter[T any] interface {
	Sort(data []T) []T
	Name() string
}

type QuickSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewQuickSorter[T any](less func(a, b T) bool) *QuickSorter[T] {
	return &QuickSorter[T]{less: less}
}

func (qs *QuickSorter[T]) Sort(data []T) []T {
	qs.mu.RLock()
	less := qs.less
	qs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	qs.quickSort(result, 0, len(result)-1, less)
	return result
}

func (qs *QuickSorter[T]) quickSort(data []T, low, high int, less func(a, b T) bool) {
	if low < high {
		pi := qs.partition(data, low, high, less)
		qs.quickSort(data, low, pi-1, less)
		qs.quickSort(data, pi+1, high, less)
	}
}

func (qs *QuickSorter[T]) partition(data []T, low, high int, less func(a, b T) bool) int {
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

func (qs *QuickSorter[T]) Name() string { return "quicksort" }

type MergeSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewMergeSorter[T any](less func(a, b T) bool) *MergeSorter[T] {
	return &MergeSorter[T]{less: less}
}

func (ms *MergeSorter[T]) Sort(data []T) []T {
	ms.mu.RLock()
	less := ms.less
	ms.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	ms.mergeSort(result, 0, len(result)-1, less)
	return result
}

func (ms *MergeSorter[T]) mergeSort(data []T, left, right int, less func(a, b T) bool) {
	if left < right {
		mid := (left + right) / 2
		ms.mergeSort(data, left, mid, less)
		ms.mergeSort(data, mid+1, right, less)
		ms.merge(data, left, mid, right, less)
	}
}

func (ms *MergeSorter[T]) merge(data []T, left, mid, right int, less func(a, b T) bool) {
	n1 := mid - left + 1
	n2 := right - mid

	leftArr := make([]T, n1)
	rightArr := make([]T, n2)

	copy(leftArr, data[left:mid+1])
	copy(rightArr, data[mid+1:right+1])

	i, j, k := 0, 0, left
	for i < n1 && j < n2 {
		if less(leftArr[i], rightArr[j]) || (!less(rightArr[j], leftArr[i])) {
			data[k] = leftArr[i]
			i++
		} else {
			data[k] = rightArr[j]
			j++
		}
		k++
	}

	for i < n1 {
		data[k] = leftArr[i]
		i++
		k++
	}

	for j < n2 {
		data[k] = rightArr[j]
		j++
		k++
	}
}

func (ms *MergeSorter[T]) Name() string { return "mergesort" }

type HeapSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewHeapSorter[T any](less func(a, b T) bool) *HeapSorter[T] {
	return &HeapSorter[T]{less: less}
}

func (hs *HeapSorter[T]) Sort(data []T) []T {
	hs.mu.RLock()
	less := hs.less
	hs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	n := len(result)

	for i := n/2 - 1; i >= 0; i-- {
		hs.heapify(result, n, i, less)
	}

	for i := n - 1; i > 0; i-- {
		result[0], result[i] = result[i], result[0]
		hs.heapify(result, i, 0, less)
	}

	return result
}

func (hs *HeapSorter[T]) heapify(data []T, n, i int, less func(a, b T) bool) {
	largest := i
	left := 2*i + 1
	right := 2*i + 2

	if left < n && less(data[largest], data[left]) {
		largest = left
	}

	if right < n && less(data[largest], data[right]) {
		largest = right
	}

	if largest != i {
		data[i], data[largest] = data[largest], data[i]
		hs.heapify(data, n, largest, less)
	}
}

func (hs *HeapSorter[T]) Name() string { return "heapsort" }

type InsertionSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewInsertionSorter[T any](less func(a, b T) bool) *InsertionSorter[T] {
	return &InsertionSorter[T]{less: less}
}

func (is *InsertionSorter[T]) Sort(data []T) []T {
	is.mu.RLock()
	less := is.less
	is.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	for i := 1; i < len(result); i++ {
		key := result[i]
		j := i - 1
		for j >= 0 && less(key, result[j]) {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = key
	}

	return result
}

func (is *InsertionSorter[T]) Name() string { return "insertionsort" }

type BubbleSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewBubbleSorter[T any](less func(a, b T) bool) *BubbleSorter[T] {
	return &BubbleSorter[T]{less: less}
}

func (bs *BubbleSorter[T]) Sort(data []T) []T {
	bs.mu.RLock()
	less := bs.less
	bs.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	n := len(result)

	for i := 0; i < n-1; i++ {
		swapped := false
		for j := 0; j < n-i-1; j++ {
			if less(result[j+1], result[j]) {
				result[j], result[j+1] = result[j+1], result[j]
				swapped = true
			}
		}
		if !swapped {
			break
		}
	}

	return result
}

func (bs *BubbleSorter[T]) Name() string { return "bubblesort" }

type ShellSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewShellSorter[T any](less func(a, b T) bool) *ShellSorter[T] {
	return &ShellSorter[T]{less: less}
}

func (ss *ShellSorter[T]) Sort(data []T) []T {
	ss.mu.RLock()
	less := ss.less
	ss.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)
	n := len(result)

	for gap := n / 2; gap > 0; gap /= 2 {
		for i := gap; i < n; i++ {
			temp := result[i]
			j := i
			for j >= gap && less(temp, result[j-gap]) {
				result[j] = result[j-gap]
				j -= gap
			}
			result[j] = temp
		}
	}

	return result
}

func (ss *ShellSorter[T]) Name() string { return "shellsort" }

type RadixSorter struct {
	mu sync.RWMutex
}

func NewRadixSorter() *RadixSorter {
	return &RadixSorter{}
}

func (rs *RadixSorter) Sort(data []int) []int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	result := make([]int, len(data))
	copy(result, data)

	max := result[0]
	for _, v := range result {
		if v > max {
			max = v
		}
	}

	for exp := 1; max/exp > 0; exp *= 10 {
		rs.countSort(result, exp)
	}

	return result
}

func (rs *RadixSorter) countSort(data []int, exp int) {
	n := len(data)
	output := make([]int, n)
	count := make([]int, 10)

	for _, v := range data {
		count[(v/exp)%10]++
	}

	for i := 1; i < 10; i++ {
		count[i] += count[i-1]
	}

	for i := n - 1; i >= 0; i-- {
		output[count[(data[i]/exp)%10]-1] = data[i]
		count[(data[i]/exp)%10]--
	}

	copy(data, output)
}

func (rs *RadixSorter) Name() string { return "radixsort" }

type CountingSorter struct {
	mu sync.RWMutex
}

func NewCountingSorter() *CountingSorter {
	return &CountingSorter{}
}

func (cs *CountingSorter) Sort(data []int) []int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	result := make([]int, len(data))
	copy(result, data)

	min, max := result[0], result[0]
	for _, v := range result {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	count := make([]int, max-min+1)
	for _, v := range result {
		count[v-min]++
	}

	idx := 0
	for i, c := range count {
		for j := 0; j < c; j++ {
			result[idx] = i + min
			idx++
		}
	}

	return result
}

func (cs *CountingSorter) Name() string { return "countingsort" }

type BucketSorter struct {
	bucketCount int
	less        func(a, b float64) bool
	mu          sync.RWMutex
}

func NewBucketSorter(bucketCount int) *BucketSorter {
	if bucketCount <= 0 {
		bucketCount = 10
	}
	return &BucketSorter{bucketCount: bucketCount}
}

func (bs *BucketSorter) Sort(data []float64) []float64 {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	result := make([]float64, len(data))
	copy(result, data)

	min, max := result[0], result[0]
	for _, v := range result {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	bucketRange := (max - min) / float64(bs.bucketCount)
	buckets := make([][]float64, bs.bucketCount)

	for _, v := range result {
		idx := int((v - min) / bucketRange)
		if idx >= bs.bucketCount {
			idx = bs.bucketCount - 1
		}
		buckets[idx] = append(buckets[idx], v)
	}

	idx := 0
	for _, bucket := range buckets {
		for i := 1; i < len(bucket); i++ {
			key := bucket[i]
			j := i - 1
			for j >= 0 && bucket[j] > key {
				bucket[j+1] = bucket[j]
				j--
			}
			bucket[j+1] = key
		}
		for _, v := range bucket {
			result[idx] = v
			idx++
		}
	}

	return result
}

func (bs *BucketSorter) Name() string { return "bucketsort" }

type TimSorter[T any] struct {
	less func(a, b T) bool
	mu   sync.RWMutex
}

func NewTimSorter[T any](less func(a, b T) bool) *TimSorter[T] {
	return &TimSorter[T]{less: less}
}

func (ts *TimSorter[T]) Sort(data []T) []T {
	ts.mu.RLock()
	less := ts.less
	ts.mu.RUnlock()

	result := make([]T, len(data))
	copy(result, data)

	n := len(result)
	if n < 2 {
		return result
	}

	const minRun = 32

	for i := 0; i < n; i += minRun {
		end := i + minRun - 1
		if end >= n {
			end = n - 1
		}
		ts.insertionSort(result, i, end, less)
	}

	for size := minRun; size < n; size *= 2 {
		for start := 0; start < n; start += 2 * size {
			mid := start + size - 1
			end := start + 2*size - 1
			if mid >= n {
				mid = n - 1
			}
			if end >= n {
				end = n - 1
			}
			if mid < end {
				ts.merge(result, start, mid, end, less)
			}
		}
	}

	return result
}

func (ts *TimSorter[T]) insertionSort(data []T, left, right int, less func(a, b T) bool) {
	for i := left + 1; i <= right; i++ {
		key := data[i]
		j := i - 1
		for j >= left && less(key, data[j]) {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}

func (ts *TimSorter[T]) merge(data []T, left, mid, right int, less func(a, b T) bool) {
	leftArr := make([]T, mid-left+1)
	rightArr := make([]T, right-mid)

	copy(leftArr, data[left:mid+1])
	copy(rightArr, data[mid+1:right+1])

	i, j, k := 0, 0, left
	for i < len(leftArr) && j < len(rightArr) {
		if less(leftArr[i], rightArr[j]) || (!less(rightArr[j], leftArr[i])) {
			data[k] = leftArr[i]
			i++
		} else {
			data[k] = rightArr[j]
			j++
		}
		k++
	}

	for i < len(leftArr) {
		data[k] = leftArr[i]
		i++
		k++
	}

	for j < len(rightArr) {
		data[k] = rightArr[j]
		j++
		k++
	}
}

func (ts *TimSorter[T]) Name() string { return "timsort" }

type SortRegistry[T any] struct {
	sorters map[string]Sorter[T]
	mu      sync.RWMutex
}

func NewSortRegistry[T any]() *SortRegistry[T] {
	return &SortRegistry[T]{
		sorters: make(map[string]Sorter[T]),
	}
}

func (sr *SortRegistry[T]) Register(sorter Sorter[T]) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.sorters[sorter.Name()] = sorter
}

func (sr *SortRegistry[T]) Get(name string) Sorter[T] {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.sorters[name]
}

func (sr *SortRegistry[T]) List() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	names := make([]string, 0, len(sr.sorters))
	for name := range sr.sorters {
		names = append(names, name)
	}
	return names
}
