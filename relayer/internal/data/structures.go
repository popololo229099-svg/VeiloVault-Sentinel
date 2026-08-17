package data

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type LRU struct {
	capacity int
	items    map[string]*lruEntry
	order    []string
	mu       sync.Mutex
}

type lruEntry struct {
	key       string
	value     interface{}
	createdAt time.Time
}

func NewLRU(capacity int) *LRU {
	return &LRU{
		capacity: capacity,
		items:    make(map[string]*lruEntry),
		order:    make([]string, 0, capacity),
	}
}

func (l *LRU) Get(key string) (interface{}, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.items[key]
	if !ok {
		return nil, false
	}
	l.touch(key)
	return entry.value, true
}

func (l *LRU) Put(key string, value interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.items[key]; ok {
		l.items[key].value = value
		l.touch(key)
		return
	}
	if len(l.items) >= l.capacity {
		l.evict()
	}
	l.items[key] = &lruEntry{key: key, value: value, createdAt: time.Now()}
	l.order = append(l.order, key)
}

func (l *LRU) Delete(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.items[key]; !ok {
		return false
	}
	delete(l.items, key)
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
	return true
}

func (l *LRU) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.items)
}

func (l *LRU) Keys() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]string, len(l.order))
	copy(result, l.order)
	return result
}

func (l *LRU) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[string]*lruEntry)
	l.order = l.order[:0]
}

func (l *LRU) touch(key string) {
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			l.order = append(l.order, key)
			return
		}
	}
}

func (l *LRU) evict() {
	if len(l.order) == 0 {
		return
	}
	oldest := l.order[0]
	delete(l.items, oldest)
	l.order = l.order[1:]
}

type BloomFilter struct {
	bits    []bool
	numHash int
	size    int
}

func NewBloomFilter(size, numHash int) *BloomFilter {
	return &BloomFilter{
		bits:    make([]bool, size),
		numHash: numHash,
		size:    size,
	}
}

func (bf *BloomFilter) Add(item string) {
	for i := 0; i < bf.numHash; i++ {
		idx := bf.hash(item, i) % bf.size
		bf.bits[idx] = true
	}
}

func (bf *BloomFilter) MayContain(item string) bool {
	for i := 0; i < bf.numHash; i++ {
		idx := bf.hash(item, i) % bf.size
		if !bf.bits[idx] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) hash(item string, seed int) int {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", item, seed)))
	return int(h[0])<<24 | int(h[1])<<16 | int(h[2])<<8 | int(h[3])
}

type TrieNode struct {
	children map[byte]*TrieNode
	isEnd    bool
	value    interface{}
}

type Trie struct {
	root *TrieNode
	size int
}

func NewTrie() *Trie {
	return &Trie{root: &TrieNode{children: make(map[byte]*TrieNode)}}
}

func (t *Trie) Insert(key string, value interface{}) {
	node := t.root
	for i := 0; i < len(key); i++ {
		child, ok := node.children[key[i]]
		if !ok {
			child = &TrieNode{children: make(map[byte]*TrieNode)}
			node.children[key[i]] = child
		}
		node = child
	}
	if !node.isEnd {
		t.size++
	}
	node.isEnd = true
	node.value = value
}

func (t *Trie) Search(key string) (interface{}, bool) {
	node := t.findNode(key)
	if node == nil || !node.isEnd {
		return nil, false
	}
	return node.value, true
}

func (t *Trie) StartsWith(prefix string) []string {
	node := t.findNode(prefix)
	if node == nil {
		return nil
	}
	var results []string
	t.collect(node, prefix, &results)
	return results
}

func (t *Trie) findNode(key string) *TrieNode {
	node := t.root
	for i := 0; i < len(key); i++ {
		child, ok := node.children[key[i]]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}

func (t *Trie) collect(node *TrieNode, prefix string, results *[]string) {
	if node.isEnd {
		*results = append(*results, prefix)
	}
	for b, child := range node.children {
		t.collect(child, prefix+string(b), results)
	}
}

func (t *Trie) Size() int { return t.size }

type MinHeap []int

func (h MinHeap) Len() int            { return len(h) }
func (h MinHeap) Less(i, j int) bool  { return h[i] < h[j] }
func (h MinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *MinHeap) Push(x interface{}) { *h = append(*h, x.(int)) }
func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type PriorityQueueItem struct {
	Value    interface{}
	Priority int
	Index    int
}

type PriorityQueue struct {
	items []*PriorityQueueItem
	mu    sync.Mutex
}

func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{items: make([]*PriorityQueueItem, 0)}
}

func (pq *PriorityQueue) Push(value interface{}, priority int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	item := &PriorityQueueItem{Value: value, Priority: priority}
	pq.items = append(pq.items, item)
	pq.bubbleUp(len(pq.items) - 1)
}

func (pq *PriorityQueue) Pop() (interface{}, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if len(pq.items) == 0 {
		return nil, false
	}
	item := pq.items[0]
	last := pq.items[len(pq.items)-1]
	pq.items = pq.items[:len(pq.items)-1]
	if len(pq.items) > 0 {
		pq.items[0] = last
		pq.bubbleDown(0)
	}
	return item.Value, true
}

func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.items)
}

func (pq *PriorityQueue) bubbleUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if pq.items[i].Priority < pq.items[parent].Priority {
			pq.items[i], pq.items[parent] = pq.items[parent], pq.items[i]
			i = parent
		} else {
			break
		}
	}
}

func (pq *PriorityQueue) bubbleDown(i int) {
	n := len(pq.items)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2
		if left < n && pq.items[left].Priority < pq.items[smallest].Priority {
			smallest = left
		}
		if right < n && pq.items[right].Priority < pq.items[smallest].Priority {
			smallest = right
		}
		if smallest != i {
			pq.items[i], pq.items[smallest] = pq.items[smallest], pq.items[i]
			i = smallest
		} else {
			break
		}
	}
}

type DAG struct {
	nodes map[string]*DAGNode
	mu    sync.RWMutex
}

type DAGNode struct {
	ID       string
	Data     interface{}
	Children []*DAGNode
}

func NewDAG() *DAG {
	return &DAG{nodes: make(map[string]*DAGNode)}
}

func (d *DAG) AddNode(id string, data interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[id] = &DAGNode{ID: id, Data: data}
}

func (d *DAG) AddEdge(from, to string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	fromNode, ok := d.nodes[from]
	if !ok {
		return fmt.Errorf("node %s not found", from)
	}
	toNode, ok := d.nodes[to]
	if !ok {
		return fmt.Errorf("node %s not found", to)
	}
	fromNode.Children = append(fromNode.Children, toNode)
	return nil
}

func (d *DAG) TopologicalSort() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	inDegree := make(map[string]int)
	for id := range d.nodes {
		inDegree[id] = 0
	}
	for _, node := range d.nodes {
		for _, child := range node.Children {
			inDegree[child.ID]++
		}
	}
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	var result []string
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		result = append(result, nodeID)
		for _, child := range d.nodes[nodeID].Children {
			inDegree[child.ID]--
			if inDegree[child.ID] == 0 {
				queue = append(queue, child.ID)
				sort.Strings(queue)
			}
		}
	}
	if len(result) != len(d.nodes) {
		return nil, fmt.Errorf("cycle detected")
	}
	return result, nil
}

type Graph struct {
	adjacency map[string][]string
	mu        sync.RWMutex
}

func NewGraph() *Graph {
	return &Graph{adjacency: make(map[string][]string)}
}

func (g *Graph) AddEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.adjacency[from] = append(g.adjacency[from], to)
}

func (g *Graph) Neighbors(node string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.adjacency[node]
}

func (g *Graph) BFS(start string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	queue := []string{start}
	visited[start] = true
	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		for _, neighbor := range g.adjacency[node] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	return result
}

func (g *Graph) DFS(start string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	var result []string
	g.dfsVisit(start, visited, &result)
	return result
}

func (g *Graph) dfsVisit(node string, visited map[string]bool, result *[]string) {
	if visited[node] {
		return
	}
	visited[node] = true
	*result = append(*result, node)
	for _, neighbor := range g.adjacency[node] {
		g.dfsVisit(neighbor, visited, result)
	}
}

func (g *Graph) HasCycle() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	for node := range g.adjacency {
		if !visited[node] {
			if g.detectCycle(node, visited, recStack) {
				return true
			}
		}
	}
	return false
}

func (g *Graph) detectCycle(node string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true
	for _, neighbor := range g.adjacency[node] {
		if !visited[neighbor] {
			if g.detectCycle(neighbor, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}
	recStack[node] = false
	return false
}

type RingBuffer struct {
	buffer []interface{}
	head   int
	tail   int
	size   int
	count  int
	mu     sync.Mutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]interface{}, size),
		size:   size,
	}
}

func (rb *RingBuffer) Write(item interface{}) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == rb.size {
		return false
	}
	rb.buffer[rb.tail] = item
	rb.tail = (rb.tail + 1) % rb.size
	rb.count++
	return true
}

func (rb *RingBuffer) Read() (interface{}, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == 0 {
		return nil, false
	}
	item := rb.buffer[rb.head]
	rb.buffer[rb.head] = nil
	rb.head = (rb.head + 1) % rb.size
	rb.count--
	return item, true
}

func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

func (rb *RingBuffer) IsFull() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count == rb.size
}

type StringSet struct {
	items map[string]struct{}
	mu    sync.RWMutex
}

func NewStringSet(items ...string) *StringSet {
	s := &StringSet{items: make(map[string]struct{})}
	for _, item := range items {
		s.items[item] = struct{}{}
	}
	return s
}

func (s *StringSet) Add(item string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item] = struct{}{}
}

func (s *StringSet) Remove(item string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, item)
}

func (s *StringSet) Contains(item string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[item]
	return ok
}

func (s *StringSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *StringSet) ToSlice() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.items))
	for item := range s.items {
		result = append(result, item)
	}
	return result
}

func (s *StringSet) Union(other *StringSet) *StringSet {
	result := NewStringSet()
	s.mu.RLock()
	for item := range s.items {
		result.items[item] = struct{}{}
	}
	s.mu.RUnlock()
	other.mu.RLock()
	for item := range other.items {
		result.items[item] = struct{}{}
	}
	other.mu.RUnlock()
	return result
}

func (s *StringSet) Intersection(other *StringSet) *StringSet {
	result := NewStringSet()
	s.mu.RLock()
	defer s.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for item := range s.items {
		if _, ok := other.items[item]; ok {
			result.items[item] = struct{}{}
		}
	}
	return result
}

func (s *StringSet) Difference(other *StringSet) *StringSet {
	result := NewStringSet()
	s.mu.RLock()
	defer s.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for item := range s.items {
		if _, ok := other.items[item]; !ok {
			result.items[item] = struct{}{}
		}
	}
	return result
}

func (s *StringSet) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]string, 0, len(s.items))
	for item := range s.items {
		items = append(items, item)
	}
	sort.Strings(items)
	return "{" + strings.Join(items, ", ") + "}"
}
