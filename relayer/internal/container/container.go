package container

import (
	"fmt"
	"sync"
)

type Stack[T any] struct {
	items []T
	mu    sync.RWMutex
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{items: make([]T, 0)}
}

func (s *Stack[T]) Push(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item, true
}

func (s *Stack[T]) Peek() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

func (s *Stack[T]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *Stack[T]) IsEmpty() bool {
	return s.Size() == 0
}

func (s *Stack[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = s.items[:0]
}

func (s *Stack[T]) ToSlice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]T, len(s.items))
	copy(result, s.items)
	return result
}

type Queue[T any] struct {
	items []T
	mu    sync.RWMutex
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{items: make([]T, 0)}
}

func (q *Queue[T]) Enqueue(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

func (q *Queue[T]) Dequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *Queue[T]) Peek() (T, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	return q.items[0], true
}

func (q *Queue[T]) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

func (q *Queue[T]) IsEmpty() bool {
	return q.Size() == 0
}

func (q *Queue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
}

func (q *Queue[T]) ToSlice() []T {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]T, len(q.items))
	copy(result, q.items)
	return result
}

type Deque[T any] struct {
	items []T
	mu    sync.RWMutex
}

func NewDeque[T any]() *Deque[T] {
	return &Deque[T]{items: make([]T, 0)}
}

func (d *Deque[T]) PushFront(item T) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = append([]T{item}, d.items...)
}

func (d *Deque[T]) PushBack(item T) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = append(d.items, item)
}

func (d *Deque[T]) PopFront() (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.items) == 0 {
		var zero T
		return zero, false
	}
	item := d.items[0]
	d.items = d.items[1:]
	return item, true
}

func (d *Deque[T]) PopBack() (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.items) == 0 {
		var zero T
		return zero, false
	}
	item := d.items[len(d.items)-1]
	d.items = d.items[:len(d.items)-1]
	return item, true
}

func (d *Deque[T]) PeekFront() (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.items) == 0 {
		var zero T
		return zero, false
	}
	return d.items[0], true
}

func (d *Deque[T]) PeekBack() (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.items) == 0 {
		var zero T
		return zero, false
	}
	return d.items[len(d.items)-1], true
}

func (d *Deque[T]) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.items)
}

func (d *Deque[T]) IsEmpty() bool {
	return d.Size() == 0
}

func (d *Deque[T]) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = d.items[:0]
}

func (d *Deque[T]) ToSlice() []T {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]T, len(d.items))
	copy(result, d.items)
	return result
}

type CircularBuffer[T any] struct {
	items  []T
	head   int
	tail   int
	count  int
	full   bool
	size   int
	mu     sync.RWMutex
}

func NewCircularBuffer[T any](size int) *CircularBuffer[T] {
	if size <= 0 {
		size = 10
	}
	return &CircularBuffer[T]{
		items: make([]T, size),
		size:  size,
	}
}

func (cb *CircularBuffer[T]) Write(item T) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.full {
		return false
	}

	cb.items[cb.tail] = item
	cb.tail = (cb.tail + 1) % cb.size
	cb.count++
	if cb.count == cb.size {
		cb.full = true
	}
	return true
}

func (cb *CircularBuffer[T]) Read() (T, bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.count == 0 {
		var zero T
		return zero, false
	}

	item := cb.items[cb.head]
	cb.head = (cb.head + 1) % cb.size
	cb.count--
	cb.full = false
	return item, true
}

func (cb *CircularBuffer[T]) Peek() (T, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.count == 0 {
		var zero T
		return zero, false
	}
	return cb.items[cb.head], true
}

func (cb *CircularBuffer[T]) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count
}

func (cb *CircularBuffer[T]) Cap() int {
	return cb.size
}

func (cb *CircularBuffer[T]) IsFull() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.full
}

func (cb *CircularBuffer[T]) IsEmpty() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count == 0
}

func (cb *CircularBuffer[T]) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.head = 0
	cb.tail = 0
	cb.count = 0
	cb.full = false
}

type SortedContainer[T any] struct {
	items []T
	less  func(a, b T) bool
	mu    sync.RWMutex
}

func NewSortedContainer[T any](less func(a, b T) bool) *SortedContainer[T] {
	return &SortedContainer[T]{
		items: make([]T, 0),
		less:  less,
	}
}

func (sc *SortedContainer[T]) Insert(item T) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	i := 0
	for i < len(sc.items) && sc.less(sc.items[i], item) {
		i++
	}
	sc.items = append(sc.items[:i], append([]T{item}, sc.items[i:]...)...)
}

func (sc *SortedContainer[T]) Remove(index int) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if index < 0 || index >= len(sc.items) {
		return false
	}
	sc.items = append(sc.items[:index], sc.items[index+1:]...)
	return true
}

func (sc *SortedContainer[T]) Get(index int) (T, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if index < 0 || index >= len(sc.items) {
		var zero T
		return zero, false
	}
	return sc.items[index], true
}

func (sc *SortedContainer[T]) Size() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.items)
}

func (sc *SortedContainer[T]) ToSlice() []T {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]T, len(sc.items))
	copy(result, sc.items)
	return result
}

func (sc *SortedContainer[T]) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.items = sc.items[:0]
}

type ConcurrentMap[K comparable, V any] struct {
	buckets map[K]V
	mu      sync.RWMutex
}

func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return &ConcurrentMap[K, V]{
		buckets: make(map[K]V),
	}
}

func (cm *ConcurrentMap[K, V]) Set(key K, value V) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.buckets[key] = value
}

func (cm *ConcurrentMap[K, V]) Get(key K) (V, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	val, exists := cm.buckets[key]
	return val, exists
}

func (cm *ConcurrentMap[K, V]) Delete(key K) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.buckets, key)
}

func (cm *ConcurrentMap[K, V]) Has(key K) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, exists := cm.buckets[key]
	return exists
}

func (cm *ConcurrentMap[K, V]) Len() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.buckets)
}

func (cm *ConcurrentMap[K, V]) Keys() []K {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	keys := make([]K, 0, len(cm.buckets))
	for k := range cm.buckets {
		keys = append(keys, k)
	}
	return keys
}

func (cm *ConcurrentMap[K, V]) Values() []V {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	values := make([]V, 0, len(cm.buckets))
	for _, v := range cm.buckets {
		values = append(values, v)
	}
	return values
}

func (cm *ConcurrentMap[K, V]) Range(fn func(K, V) bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for k, v := range cm.buckets {
		if !fn(k, v) {
			break
		}
	}
}

func (cm *ConcurrentMap[K, V]) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.buckets = make(map[K]V)
}

func (cm *ConcurrentMap[K, V]) Merge(other *ConcurrentMap[K, V]) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for k, v := range other.buckets {
		cm.buckets[k] = v
	}
}

type FixedArray[T any] struct {
	items []T
	size  int
	mu    sync.RWMutex
}

func NewFixedArray[T any](size int) *FixedArray[T] {
	return &FixedArray[T]{
		items: make([]T, size),
		size:  size,
	}
}

func (fa *FixedArray[T]) Set(index int, value T) error {
	fa.mu.Lock()
	defer fa.mu.Unlock()
	if index < 0 || index >= fa.size {
		return fmt.Errorf("index out of range: %d", index)
	}
	fa.items[index] = value
	return nil
}

func (fa *FixedArray[T]) Get(index int) (T, error) {
	fa.mu.RLock()
	defer fa.mu.RUnlock()
	if index < 0 || index >= fa.size {
		var zero T
		return zero, fmt.Errorf("index out of range: %d", index)
	}
	return fa.items[index], nil
}

func (fa *FixedArray[T]) Size() int {
	return fa.size
}

func (fa *FixedArray[T]) ToSlice() []T {
	fa.mu.RLock()
	defer fa.mu.RUnlock()
	result := make([]T, fa.size)
	copy(result, fa.items)
	return result
}

func (fa *FixedArray[T]) Fill(value T) {
	fa.mu.Lock()
	defer fa.mu.Unlock()
	for i := range fa.items {
		fa.items[i] = value
	}
}

func (fa *FixedArray[T]) Each(fn func(int, T)) {
	fa.mu.RLock()
	defer fa.mu.RUnlock()
	for i, item := range fa.items {
		fn(i, item)
	}
}

type MinHeap[T any] struct {
	items []T
	less  func(a, b T) bool
	mu    sync.RWMutex
}

func NewMinHeap[T any](less func(a, b T) bool) *MinHeap[T] {
	return &MinHeap[T]{
		items: make([]T, 0),
		less:  less,
	}
}

func (h *MinHeap[T]) Push(item T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = append(h.items, item)
	h.siftUp(len(h.items) - 1)
}

func (h *MinHeap[T]) Pop() (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.items) == 0 {
		var zero T
		return zero, false
	}
	item := h.items[0]
	h.items[0] = h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	if len(h.items) > 0 {
		h.siftDown(0)
	}
	return item, true
}

func (h *MinHeap[T]) Peek() (T, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.items) == 0 {
		var zero T
		return zero, false
	}
	return h.items[0], true
}

func (h *MinHeap[T]) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.items)
}

func (h *MinHeap[T]) IsEmpty() bool {
	return h.Size() == 0
}

func (h *MinHeap[T]) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.less(h.items[i], h.items[parent]) {
			h.items[i], h.items[parent] = h.items[parent], h.items[i]
			i = parent
		} else {
			break
		}
	}
}

func (h *MinHeap[T]) siftDown(i int) {
	n := len(h.items)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2

		if left < n && h.less(h.items[left], h.items[smallest]) {
			smallest = left
		}
		if right < n && h.less(h.items[right], h.items[smallest]) {
			smallest = right
		}

		if smallest != i {
			h.items[i], h.items[smallest] = h.items[smallest], h.items[i]
			i = smallest
		} else {
			break
		}
	}
}

func (h *MinHeap[T]) ToSlice() []T {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]T, len(h.items))
	copy(result, h.items)
	return result
}

type MaxHeap[T any] struct {
	items []T
	less  func(a, b T) bool
	mu    sync.RWMutex
}

func NewMaxHeap[T any](less func(a, b T) bool) *MaxHeap[T] {
	return &MaxHeap[T]{
		items: make([]T, 0),
		less:  less,
	}
}

func (h *MaxHeap[T]) Push(item T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = append(h.items, item)
	for i := len(h.items) - 1; i > 0; {
		parent := (i - 1) / 2
		if h.less(h.items[parent], h.items[i]) {
			h.items[i], h.items[parent] = h.items[parent], h.items[i]
			i = parent
		} else {
			break
		}
	}
}

func (h *MaxHeap[T]) Pop() (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.items) == 0 {
		var zero T
		return zero, false
	}
	item := h.items[0]
	h.items[0] = h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]

	n := len(h.items)
	for i := 0; ; {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2
		if left < n && h.less(h.items[smallest], h.items[left]) {
			smallest = left
		}
		if right < n && h.less(h.items[smallest], h.items[right]) {
			smallest = right
		}
		if smallest != i {
			h.items[i], h.items[smallest] = h.items[smallest], h.items[i]
			i = smallest
		} else {
			break
		}
	}

	return item, true
}

func (h *MaxHeap[T]) Peek() (T, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.items) == 0 {
		var zero T
		return zero, false
	}
	return h.items[0], true
}

func (h *MaxHeap[T]) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.items)
}

type BloomFilterContainer struct {
	bits  []bool
	size  uint
	count uint64
	mu    sync.RWMutex
}

func NewBloomFilterContainer(size uint) *BloomFilterContainer {
	return &BloomFilterContainer{
		bits: make([]bool, size),
		size: size,
	}
}

func (bfc *BloomFilterContainer) Add(item string) {
	bfc.mu.Lock()
	defer bfc.mu.Unlock()
	hash := fnvHash(item)
	idx := hash % bfc.size
	bfc.bits[idx] = true
	bfc.count++
}

func (bfc *BloomFilterContainer) Contains(item string) bool {
	bfc.mu.RLock()
	defer bfc.mu.RUnlock()
	hash := fnvHash(item)
	idx := hash % bfc.size
	return bfc.bits[idx]
}

func (bfc *BloomFilterContainer) Count() uint64 {
	bfc.mu.RLock()
	defer bfc.mu.RUnlock()
	return bfc.count
}

func fnvHash(s string) uint {
	h := uint(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint(s[i])
		h *= 16777619
	}
	return h
}
