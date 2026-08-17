package lifecycle

import (
	"sync"
	"time"
)

type CircuitState int

const (
	CircuitClosed   CircuitState = 0
	CircuitOpen     CircuitState = 1
	CircuitHalfOpen CircuitState = 2
)

type CircuitBreaker struct {
	state         CircuitState
	failureCount  int
	successCount  int
	threshold     int
	resetTimeout  time.Duration
	lastFailure   time.Time
	mu            sync.Mutex
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CircuitClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == CircuitHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.threshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.failureCount >= cb.threshold {
		cb.state = CircuitOpen
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
}

type WatchdogConfig struct {
	Interval    time.Duration
	Timeout     time.Duration
	MaxRetries  int
}

type Watchdog struct {
	config  WatchdogConfig
	healthy bool
	lastCheck time.Time
	mu      sync.RWMutex
	stopCh  chan struct{}
}

func NewWatchdog(config WatchdogConfig) *Watchdog {
	return &Watchdog{
		config:  config,
		healthy: true,
		stopCh:  make(chan struct{}),
	}
}

func (w *Watchdog) Start(checkFn func() error) {
	go func() {
		ticker := time.NewTicker(w.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				return
			case <-ticker.C:
				w.mu.Lock()
				err := checkFn()
				w.healthy = err == nil
				w.lastCheck = time.Now()
				w.mu.Unlock()
			}
		}
	}()
}

func (w *Watchdog) Stop() {
	close(w.stopCh)
}

func (w *Watchdog) IsHealthy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.healthy
}

func (w *Watchdog) LastCheck() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastCheck
}

type ServiceRegistry struct {
	services map[string]*ServiceEntry
	mu       sync.RWMutex
}

type ServiceEntry struct {
	Name      string
	URL       string
	Healthy   bool
	Metadata  map[string]string
	LastSeen  time.Time
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*ServiceEntry),
	}
}

func (sr *ServiceRegistry) Register(name, url string, metadata map[string]string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.services[name] = &ServiceEntry{
		Name:     name,
		URL:      url,
		Healthy:  true,
		Metadata: metadata,
		LastSeen: time.Now(),
	}
}

func (sr *ServiceRegistry) Deregister(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.services, name)
}

func (sr *ServiceRegistry) Get(name string) (*ServiceEntry, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	entry, ok := sr.services[name]
	return entry, ok
}

func (sr *ServiceRegistry) MarkHealthy(name string, healthy bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if entry, ok := sr.services[name]; ok {
		entry.Healthy = healthy
		entry.LastSeen = time.Now()
	}
}

func (sr *ServiceRegistry) HealthyServices() []*ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	var result []*ServiceEntry
	for _, entry := range sr.services {
		if entry.Healthy {
			result = append(result, entry)
		}
	}
	return result
}

func (sr *ServiceRegistry) AllServices() []*ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	result := make([]*ServiceEntry, 0, len(sr.services))
	for _, entry := range sr.services {
		result = append(result, entry)
	}
	return result
}

func (sr *ServiceRegistry) StaleServices(maxAge time.Duration) []*ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	var result []*ServiceEntry
	for _, entry := range sr.services {
		if time.Since(entry.LastSeen) > maxAge {
			result = append(result, entry)
		}
	}
	return result
}

type DependencyGraph struct {
	nodes map[string][]string
	mu    sync.RWMutex
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{nodes: make(map[string][]string)}
}

func (dg *DependencyGraph) AddDependency(from, to string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	dg.nodes[from] = append(dg.nodes[from], to)
}

func (dg *DependencyGraph) Dependencies(name string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	return dg.nodes[name]
}

func (dg *DependencyGraph) Dependents(name string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	var result []string
	for node, deps := range dg.nodes {
		for _, dep := range deps {
			if dep == name {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

func (dg *DependencyGraph) HasCycle() bool {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	for node := range dg.nodes {
		if !visited[node] {
			if dg.dfs(node, visited, recStack) {
				return true
			}
		}
	}
	return false
}

func (dg *DependencyGraph) dfs(node string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true
	for _, dep := range dg.nodes[node] {
		if !visited[dep] {
			if dg.dfs(dep, visited, recStack) {
				return true
			}
		} else if recStack[dep] {
			return true
		}
	}
	recStack[node] = false
	return false
}

func (dg *DependencyGraph) TopologicalSort() []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	inDegree := make(map[string]int)
	for node := range dg.nodes {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range dg.nodes[node] {
			inDegree[dep]++
		}
	}
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}
	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)
		for _, dep := range dg.nodes[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	return sorted
}
