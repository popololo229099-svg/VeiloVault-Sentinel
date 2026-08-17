package data

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Set[T comparable] struct {
	items map[T]struct{}
	mu    sync.RWMutex
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{items: make(map[T]struct{})}
}

func (s *Set[T]) Add(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item] = struct{}{}
}

func (s *Set[T]) Remove(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, item)
}

func (s *Set[T]) Contains(item T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[item]
	return ok
}

func (s *Set[T]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *Set[T]) Items() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]T, 0, len(s.items))
	for item := range s.items {
		result = append(result, item)
	}
	return result
}

func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	for item := range s.items {
		result.Add(item)
	}
	for item := range other.items {
		result.Add(item)
	}
	return result
}

func (s *Set[T]) Intersection(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	for item := range s.items {
		if other.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	for item := range s.items {
		if !other.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

func (s *Set[T]) Equals(other *Set[T]) bool {
	if s.Size() != other.Size() {
		return false
	}
	for item := range s.items {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

func (s *Set[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[T]struct{})
}

type BoundedQueue[T any] struct {
	items    []T
	maxSize  int
	mu       sync.Mutex
}

func NewBoundedQueue[T any](maxSize int) *BoundedQueue[T] {
	return &BoundedQueue[T]{items: make([]T, 0), maxSize: maxSize}
}

func (bq *BoundedQueue[T]) Push(item T) bool {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	if len(bq.items) >= bq.maxSize {
		return false
	}
	bq.items = append(bq.items, item)
	return true
}

func (bq *BoundedQueue[T]) Pop() (T, bool) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	var zero T
	if len(bq.items) == 0 {
		return zero, false
	}
	item := bq.items[0]
	bq.items = bq.items[1:]
	return item, true
}

func (bq *BoundedQueue[T]) Peek() (T, bool) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	var zero T
	if len(bq.items) == 0 {
		return zero, false
	}
	return bq.items[0], true
}

func (bq *BoundedQueue[T]) Size() int {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return len(bq.items)
}

func (bq *BoundedQueue[T]) IsFull() bool {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return len(bq.items) >= bq.maxSize
}

func (bq *BoundedQueue[T]) Items() []T {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	result := make([]T, len(bq.items))
	copy(result, bq.items)
	return result
}

type Heap[T any] struct {
	data []T
	less func(a, b T) bool
	mu   sync.Mutex
}

func NewHeap[T any](less func(a, b T) bool) *Heap[T] {
	return &Heap[T]{data: make([]T, 0), less: less}
}

func (h *Heap[T]) Push(item T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.data = append(h.data, item)
	h.bubbleUp(len(h.data) - 1)
}

func (h *Heap[T]) Pop() (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var zero T
	if len(h.data) == 0 {
		return zero, false
	}
	item := h.data[0]
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	if len(h.data) > 0 {
		h.bubbleDown(0)
	}
	return item, true
}

func (h *Heap[T]) Peek() (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var zero T
	if len(h.data) == 0 {
		return zero, false
	}
	return h.data[0], true
}

func (h *Heap[T]) Size() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.data)
}

func (h *Heap[T]) bubbleUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.less(h.data[i], h.data[parent]) {
			h.data[i], h.data[parent] = h.data[parent], h.data[i]
			i = parent
		} else {
			break
		}
	}
}

func (h *Heap[T]) bubbleDown(i int) {
	n := len(h.data)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2
		if left < n && h.less(h.data[left], h.data[smallest]) {
			smallest = left
		}
		if right < n && h.less(h.data[right], h.data[smallest]) {
			smallest = right
		}
		if smallest != i {
			h.data[i], h.data[smallest] = h.data[smallest], h.data[i]
			i = smallest
		} else {
			break
		}
	}
}

type TimeSeries struct {
	points []TimeSeriesPoint
	mu     sync.RWMutex
}

type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

func NewTimeSeries() *TimeSeries {
	return &TimeSeries{points: make([]TimeSeriesPoint, 0)}
}

func (ts *TimeSeries) Record(value float64, labels map[string]string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.points = append(ts.points, TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	})
}

func (ts *TimeSeries) Query(since time.Duration) []TimeSeriesPoint {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	cutoff := time.Now().Add(-since)
	var result []TimeSeriesPoint
	for _, p := range ts.points {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	return result
}

func (ts *TimeSeries) Latest() (TimeSeriesPoint, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if len(ts.points) == 0 {
		return TimeSeriesPoint{}, false
	}
	return ts.points[len(ts.points)-1], true
}

func (ts *TimeSeries) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.points)
}

func (ts *TimeSeries) Truncate(before time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	cutoff := time.Now().Add(-before)
	var kept []TimeSeriesPoint
	for _, p := range ts.points {
		if p.Timestamp.After(cutoff) {
			kept = append(kept, p)
		}
	}
	ts.points = kept
}

type SortedList[T any] struct {
	items []T
	less  func(a, b T) bool
	mu    sync.Mutex
}

func NewSortedList[T any](less func(a, b T) bool) *SortedList[T] {
	return &SortedList[T]{items: make([]T, 0), less: less}
}

func (sl *SortedList[T]) Insert(item T) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	idx := sort.Search(len(sl.items), func(i int) bool {
		return !sl.less(sl.items[i], item)
	})
	sl.items = append(sl.items, item)
	copy(sl.items[idx+1:], sl.items[idx:])
	sl.items[idx] = item
}

func (sl *SortedList[T]) RemoveAt(index int) T {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	item := sl.items[index]
	sl.items = append(sl.items[:index], sl.items[index+1:]...)
	return item
}

func (sl *SortedList[T]) Get(index int) T {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.items[index]
}

func (sl *SortedList[T]) Size() int {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return len(sl.items)
}

func (sl *SortedList[T]) Items() []T {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	result := make([]T, len(sl.items))
	copy(result, sl.items)
	return result
}

type ExpiringMap[K comparable, V any] struct {
	items map[K]*ExpiringItem[V]
	ttl   time.Duration
	mu    sync.RWMutex
}

type ExpiringItem[V any] struct {
	Value     V
	ExpiresAt time.Time
}

func NewExpiringMap[K comparable, V any](ttl time.Duration) *ExpiringMap[K, V] {
	em := &ExpiringMap[K, V]{
		items: make(map[K]*ExpiringItem[V]),
		ttl:   ttl,
	}
	go em.cleanup()
	return em
}

func (em *ExpiringMap[K, V]) Set(key K, value V) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.items[key] = &ExpiringItem[V]{
		Value:     value,
		ExpiresAt: time.Now().Add(em.ttl),
	}
}

func (em *ExpiringMap[K, V]) Get(key K) (V, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	var zero V
	item, ok := em.items[key]
	if !ok {
		return zero, false
	}
	if time.Now().After(item.ExpiresAt) {
		return zero, false
	}
	return item.Value, true
}

func (em *ExpiringMap[K, V]) Delete(key K) {
	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.items, key)
}

func (em *ExpiringMap[K, V]) Size() int {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return len(em.items)
}

func (em *ExpiringMap[K, V]) cleanup() {
	ticker := time.NewTicker(em.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		em.mu.Lock()
		now := time.Now()
		for k, v := range em.items {
			if now.After(v.ExpiresAt) {
				delete(em.items, k)
			}
		}
		em.mu.Unlock()
	}
}

type MultiMap[K comparable, V any] struct {
	items map[K][]V
	mu    sync.RWMutex
}

func NewMultiMap[K comparable, V any]() *MultiMap[K, V] {
	return &MultiMap[K, V]{items: make(map[K][]V)}
}

func (mm *MultiMap[K, V]) Add(key K, value V) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.items[key] = append(mm.items[key], value)
}

func (mm *MultiMap[K, V]) Get(key K) []V {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.items[key]
}

func (mm *MultiMap[K, V]) Remove(key K, value V) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	values := mm.items[key]
	for i, v := range values {
		if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value) {
			mm.items[key] = append(values[:i], values[i+1:]...)
			break
		}
	}
	if len(mm.items[key]) == 0 {
		delete(mm.items, key)
	}
}

func (mm *MultiMap[K, V]) Keys() []K {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	keys := make([]K, 0, len(mm.items))
	for k := range mm.items {
		keys = append(keys, k)
	}
	return keys
}

func (mm *MultiMap[K, V]) Size() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	total := 0
	for _, v := range mm.items {
		total += len(v)
	}
	return total
}
