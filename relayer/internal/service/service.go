package service

import (
	"fmt"
	"sync"
	"time"
)

type Service interface {
	Name() string
	Start() error
	Stop() error
	IsRunning() bool
}

type BaseService struct {
	name    string
	running bool
	mu      sync.RWMutex
}

func NewBaseService(name string) *BaseService {
	return &BaseService{name: name}
}

func (bs *BaseService) Name() string { return bs.name }

func (bs *BaseService) Start() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.running = true
	return nil
}

func (bs *BaseService) Stop() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.running = false
	return nil
}

func (bs *BaseService) IsRunning() bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.running
}

type ServiceManager struct {
	services map[string]Service
	ordered  []string
	mu       sync.RWMutex
}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		services: make(map[string]Service),
		ordered:  make([]string, 0),
	}
}

func (sm *ServiceManager) Register(service Service) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.services[service.Name()] = service
	sm.ordered = append(sm.ordered, service.Name())
}

func (sm *ServiceManager) Unregister(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.services, name)
	for i, n := range sm.ordered {
		if n == name {
			sm.ordered = append(sm.ordered[:i], sm.ordered[i+1:]...)
			break
		}
	}
}

func (sm *ServiceManager) StartAll() error {
	sm.mu.RLock()
	ordered := make([]string, len(sm.ordered))
	copy(ordered, sm.ordered)
	services := make(map[string]Service, len(sm.services))
	for k, v := range sm.services {
		services[k] = v
	}
	sm.mu.RUnlock()

	for _, name := range ordered {
		if err := services[name].Start(); err != nil {
			return fmt.Errorf("failed to start %s: %w", name, err)
		}
	}
	return nil
}

func (sm *ServiceManager) StopAll() error {
	sm.mu.RLock()
	ordered := make([]string, len(sm.ordered))
	copy(ordered, sm.ordered)
	services := make(map[string]Service, len(sm.services))
	for k, v := range sm.services {
		services[k] = v
	}
	sm.mu.RUnlock()

	var lastErr error
	for i := len(ordered) - 1; i >= 0; i-- {
		name := ordered[i]
		if err := services[name].Stop(); err != nil {
			lastErr = fmt.Errorf("failed to stop %s: %w", name, err)
		}
	}
	return lastErr
}

func (sm *ServiceManager) Get(name string) Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.services[name]
}

func (sm *ServiceManager) List() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]string, len(sm.ordered))
	copy(result, sm.ordered)
	return result
}

func (sm *ServiceManager) Status() map[string]bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	status := make(map[string]bool, len(sm.services))
	for name, svc := range sm.services {
		status[name] = svc.IsRunning()
	}
	return status
}

type LifecycleHook struct {
	OnStart func() error
	OnStop  func() error
	mu      sync.RWMutex
}

func NewLifecycleHook(onStart, onStop func() error) *LifecycleHook {
	return &LifecycleHook{
		OnStart: onStart,
		OnStop:  onStop,
	}
}

func (lh *LifecycleHook) Start() error {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	if lh.OnStart != nil {
		return lh.OnStart()
	}
	return nil
}

func (lh *LifecycleHook) Stop() error {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	if lh.OnStop != nil {
		return lh.OnStop()
	}
	return nil
}

type ServiceHealth struct {
	Name      string
	Running   bool
	Uptime    time.Duration
	LastError error
	mu        sync.RWMutex
}

func NewServiceHealth(name string) *ServiceHealth {
	return &ServiceHealth{Name: name}
}

func (sh *ServiceHealth) SetRunning(running bool) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.Running = running
}

func (sh *ServiceHealth) SetUptime(uptime time.Duration) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.Uptime = uptime
}

func (sh *ServiceHealth) SetError(err error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.LastError = err
}

func (sh *ServiceHealth) Check() *ServiceHealth {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return &ServiceHealth{
		Name:      sh.Name,
		Running:   sh.Running,
		Uptime:    sh.Uptime,
		LastError: sh.LastError,
	}
}

type ServiceRegistry struct {
	services map[string]Service
	mu       sync.RWMutex
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]Service),
	}
}

func (sr *ServiceRegistry) Register(svc Service) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.services[svc.Name()] = svc
}

func (sr *ServiceRegistry) Get(name string) Service {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.services[name]
}

func (sr *ServiceRegistry) Remove(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.services, name)
}

func (sr *ServiceRegistry) List() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	names := make([]string, 0, len(sr.services))
	for name := range sr.services {
		names = append(names, name)
	}
	return names
}

type HealthChecker struct {
	services  []Service
	interval  time.Duration
	callback  func(name string, healthy bool)
	mu        sync.RWMutex
	stopCh    chan struct{}
}

func NewHealthChecker(interval time.Duration, callback func(string, bool)) *HealthChecker {
	return &HealthChecker{
		services: make([]Service, 0),
		interval: interval,
		callback: callback,
		stopCh:  make(chan struct{}),
	}
}

func (hc *HealthChecker) AddService(svc Service) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.services = append(hc.services, svc)
}

func (hc *HealthChecker) Start() {
	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.check()
			case <-hc.stopCh:
				return
			}
		}
	}()
}

func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

func (hc *HealthChecker) check() {
	hc.mu.RLock()
	services := make([]Service, len(hc.services))
	copy(services, hc.services)
	cb := hc.callback
	hc.mu.RUnlock()

	for _, svc := range services {
		healthy := svc.IsRunning()
		if cb != nil {
			cb(svc.Name(), healthy)
		}
	}
}

type ServiceDependency struct {
	Name     string
	Required bool
	mu       sync.RWMutex
}

func NewServiceDependency(name string, required bool) *ServiceDependency {
	return &ServiceDependency{Name: name, Required: required}
}

type DependencyGraph struct {
	nodes map[string][]string
	mu    sync.RWMutex
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string][]string),
	}
}

func (dg *DependencyGraph) AddEdge(from, to string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	dg.nodes[from] = append(dg.nodes[from], to)
}

func (dg *DependencyGraph) Dependencies(name string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	return dg.nodes[name]
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

	queue := make([]string, 0)
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, dep := range dg.nodes[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	return result
}
