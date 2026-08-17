package graph

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"sync"
)

type Edge struct {
	From   string
	To     string
	Weight float64
}

type Graph struct {
	vertices map[string]map[string]float64
	directed bool
	mu       sync.RWMutex
}

func NewGraph(directed bool) *Graph {
	return &Graph{
		vertices: make(map[string]map[string]float64),
		directed: directed,
	}
}

func (g *Graph) AddVertex(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.vertices[name]; !exists {
		g.vertices[name] = make(map[string]float64)
	}
}

func (g *Graph) AddEdge(from, to string, weight float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.vertices[from]; !exists {
		g.vertices[from] = make(map[string]float64)
	}
	if _, exists := g.vertices[to]; !exists {
		g.vertices[to] = make(map[string]float64)
	}

	g.vertices[from][to] = weight
	if !g.directed {
		g.vertices[to][from] = weight
	}
}

func (g *Graph) RemoveVertex(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.vertices, name)
	for v := range g.vertices {
		delete(g.vertices[v], name)
	}
}

func (g *Graph) RemoveEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.vertices[from], to)
	if !g.directed {
		delete(g.vertices[to], from)
	}
}

func (g *Graph) GetNeighbors(vertex string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	neighbors := make([]string, 0, len(g.vertices[vertex]))
	for n := range g.vertices[vertex] {
		neighbors = append(neighbors, n)
	}
	return neighbors
}

func (g *Graph) GetWeight(from, to string) (float64, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges, exists := g.vertices[from]
	if !exists {
		return 0, false
	}
	w, ok := edges[to]
	return w, ok
}

func (g *Graph) Vertices() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	vertices := make([]string, 0, len(g.vertices))
	for v := range g.vertices {
		vertices = append(vertices, v)
	}
	return vertices
}

func (g *Graph) VertexCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.vertices)
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, edges := range g.vertices {
		count += len(edges)
	}
	if !g.directed {
		count /= 2
	}
	return count
}

func (g *Graph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]Edge, 0)
	seen := make(map[string]bool)
	for from, tos := range g.vertices {
		for to, weight := range tos {
			key := from + "->" + to
			if g.directed {
				edges = append(edges, Edge{From: from, To: to, Weight: weight})
			} else {
				reverseKey := to + "->" + from
				if !seen[key] && !seen[reverseKey] {
					edges = append(edges, Edge{From: from, To: to, Weight: weight})
					seen[key] = true
				}
			}
			_ = from
		}
	}
	return edges
}

func (g *Graph) HasVertex(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.vertices[name]
	return exists
}

func (g *Graph) HasEdge(from, to string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges, exists := g.vertices[from]
	if !exists {
		return false
	}
	_, ok := edges[to]
	return ok
}

type DijkstraResult struct {
	Distances map[string]float64
	Paths     map[string][]string
}

func (g *Graph) Dijkstra(start string) *DijkstraResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dist := make(map[string]float64)
	prev := make(map[string]string)
	visited := make(map[string]bool)

	for v := range g.vertices {
		dist[v] = math.MaxFloat64
	}
	dist[start] = 0

	pq := &priorityQueue{}
	heap.Push(pq, &pqItem{node: start, dist: 0})

	for pq.Len() > 0 {
		item := heap.Pop(pq).(*pqItem)
		u := item.node

		if visited[u] {
			continue
		}
		visited[u] = true

		for v, weight := range g.vertices[u] {
			if visited[v] {
				continue
			}
			alt := dist[u] + weight
			if alt < dist[v] {
				dist[v] = alt
				prev[v] = u
				heap.Push(pq, &pqItem{node: v, dist: alt})
			}
		}
	}

	paths := make(map[string][]string)
	for v := range dist {
		if v == start {
			paths[v] = []string{v}
			continue
		}
		if dist[v] == math.MaxFloat64 {
			continue
		}
		path := make([]string, 0)
		current := v
		for current != "" {
			path = append([]string{current}, path...)
			current = prev[current]
		}
		paths[v] = path
	}

	return &DijkstraResult{
		Distances: dist,
		Paths:     paths,
	}
}

type pqItem struct {
	node string
	dist float64
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].dist < pq[j].dist }
func (pq priorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x interface{}) { *pq = append(*pq, x.(*pqItem)) }
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

func (g *Graph) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.directed {
		return nil, fmt.Errorf("topological sort requires a directed graph")
	}

	inDegree := make(map[string]int)
	for v := range g.vertices {
		inDegree[v] = 0
	}
	for _, tos := range g.vertices {
		for to := range tos {
			inDegree[to]++
		}
	}

	queue := make([]string, 0)
	for v, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, v)
		}
	}

	result := make([]string, 0)
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		result = append(result, v)

		for to := range g.vertices[v] {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
			}
		}
	}

	if len(result) != len(g.vertices) {
		return nil, fmt.Errorf("graph contains a cycle")
	}

	return result, nil
}

func (g *Graph) StronglyConnectedComponents() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	index := 0
	indices := make(map[string]int)
	lowlink := make(map[string]int)
	onStack := make(map[string]bool)
	stack := make([]string, 0)
	sccs := make([][]string, 0)

	var strongConnect func(v string)
	strongConnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for w := range g.vertices[v] {
			if _, visited := indices[w]; !visited {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			scc := make([]string, 0)
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for v := range g.vertices {
		if _, visited := indices[v]; !visited {
			strongConnect(v)
		}
	}

	return sccs
}

type MSTResult struct {
	Edges []Edge
	Total float64
}

type edgeHeap []Edge

func (h edgeHeap) Len() int           { return len(h) }
func (h edgeHeap) Less(i, j int) bool { return h[i].Weight < h[j].Weight }
func (h edgeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *edgeHeap) Push(x interface{}) { *h = append(*h, x.(Edge)) }
func (h *edgeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (g *Graph) Prim() *MSTResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	vertices := make([]string, 0)
	for v := range g.vertices {
		vertices = append(vertices, v)
	}

	if len(vertices) == 0 {
		return &MSTResult{}
	}

	visited := make(map[string]bool)
	edges := &edgeHeap{}
	heap.Init(edges)
	mstEdges := make([]Edge, 0)
	totalWeight := 0.0

	start := vertices[0]
	visited[start] = true

	for to, weight := range g.vertices[start] {
		heap.Push(edges, Edge{From: start, To: to, Weight: weight})
	}

	for edges.Len() > 0 && len(visited) < len(vertices) {
		edge := heap.Pop(edges).(Edge)

		if visited[edge.To] {
			continue
		}

		visited[edge.To] = true
		mstEdges = append(mstEdges, edge)
		totalWeight += edge.Weight

		for to, weight := range g.vertices[edge.To] {
			if !visited[to] {
				heap.Push(edges, Edge{From: edge.To, To: to, Weight: weight})
			}
		}
	}

	return &MSTResult{Edges: mstEdges, Total: totalWeight}
}

func (g *Graph) IsBipartite() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	colors := make(map[string]int)

	for v := range g.vertices {
		if _, exists := colors[v]; exists {
			continue
		}

		queue := []string{v}
		colors[v] = 1

		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]

			for w := range g.vertices[u] {
				if _, exists := colors[w]; !exists {
					colors[w] = -colors[u]
					queue = append(queue, w)
				} else if colors[w] == colors[u] {
					return false
				}
			}
		}
	}

	return true
}

func (g *Graph) HasCycle() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(v string) bool
	dfs = func(v string) bool {
		visited[v] = true
		recStack[v] = true

		for w := range g.vertices[v] {
			if !visited[w] {
				if dfs(w) {
					return true
				}
			} else if recStack[w] {
				return true
			}
		}

		recStack[v] = false
		return false
	}

	for v := range g.vertices {
		if !visited[v] {
			if dfs(v) {
				return true
			}
		}
	}

	return false
}

func (g *Graph) BFS(start string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	queue := []string{start}
	visited[start] = true
	result := make([]string, 0)

	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		result = append(result, v)

		for w := range g.vertices[v] {
			if !visited[w] {
				visited[w] = true
				queue = append(queue, w)
			}
		}
	}

	return result
}

func (g *Graph) DFS(start string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	result := make([]string, 0)

	var dfs func(v string)
	dfs = func(v string) {
		visited[v] = true
		result = append(result, v)
		for w := range g.vertices[v] {
			if !visited[w] {
				dfs(w)
			}
		}
	}

	dfs(start)
	return result
}

func (g *Graph) ConnectedComponents() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	components := make([][]string, 0)

	var dfs func(v string, component *[]string)
	dfs = func(v string, component *[]string) {
		visited[v] = true
		*component = append(*component, v)
		for w := range g.vertices[v] {
			if !visited[w] {
				dfs(w, component)
			}
		}
	}

	for v := range g.vertices {
		if !visited[v] {
			component := make([]string, 0)
			dfs(v, &component)
			sort.Strings(component)
			components = append(components, component)
		}
	}

	return components
}

func (g *Graph) ShortestPathAllPairs() map[string]map[string]float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dist := make(map[string]map[string]float64)
	for v := range g.vertices {
		dist[v] = make(map[string]float64)
		for u := range g.vertices {
			dist[v][u] = math.MaxFloat64
		}
		dist[v][v] = 0
	}

	for from, tos := range g.vertices {
		for to, weight := range tos {
			dist[from][to] = weight
		}
	}

	vertices := make([]string, 0)
	for v := range g.vertices {
		vertices = append(vertices, v)
	}

	for k := range vertices {
		for i := range vertices {
			for j := range vertices {
				if dist[vertices[i]][vertices[k]]+dist[vertices[k]][vertices[j]] < dist[vertices[i]][vertices[j]] {
					dist[vertices[i]][vertices[j]] = dist[vertices[i]][vertices[k]] + dist[vertices[k]][vertices[j]]
				}
			}
		}
	}

	return dist
}

func (g *Graph) Clone() *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	clone := NewGraph(g.directed)
	for v, edges := range g.vertices {
		clone.vertices[v] = make(map[string]float64)
		for w, weight := range edges {
			clone.vertices[v][w] = weight
		}
	}
	return clone
}

func (g *Graph) Subgraph(vertices []string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	vertexSet := make(map[string]bool)
	for _, v := range vertices {
		vertexSet[v] = true
	}

	sub := NewGraph(g.directed)
	for v := range g.vertices {
		if vertexSet[v] {
			sub.AddVertex(v)
			for w, weight := range g.vertices[v] {
				if vertexSet[w] {
					sub.vertices[v][w] = weight
				}
			}
		}
	}
	return sub
}
