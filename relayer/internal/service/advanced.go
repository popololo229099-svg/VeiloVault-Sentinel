package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type WorkerService struct {
	name     string
	workerFn func(ctx context.Context) error
	workers  int
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  bool
}

func NewWorkerService(name string, workers int, fn func(ctx context.Context) error) *WorkerService {
	if workers <= 0 {
		workers = 1
	}
	return &WorkerService{
		name:     name,
		workerFn: fn,
		workers:  workers,
	}
}

func (ws *WorkerService) Start() error {
	ws.mu.Lock()
	if ws.running {
		ws.mu.Unlock()
		return fmt.Errorf("service already running")
	}
	ws.running = true
	ws.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	ws.cancel = cancel

	for i := 0; i < ws.workers; i++ {
		ws.wg.Add(1)
		go func(id int) {
			defer ws.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := ws.workerFn(ctx); err != nil {
						time.Sleep(time.Second)
					}
				}
			}
		}(i)
	}

	return nil
}

func (ws *WorkerService) Stop() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil
	}

	ws.running = false
	if ws.cancel != nil {
		ws.cancel()
	}
	ws.wg.Wait()
	return nil
}

func (ws *WorkerService) Name() string    { return ws.name }
func (ws *WorkerService) IsRunning() bool { ws.mu.RLock(); defer ws.mu.RUnlock(); return ws.running }

type ScheduledService struct {
	name     string
	interval time.Duration
	task     func() error
	ticker   *time.Ticker
	stopCh   chan struct{}
	mu       sync.RWMutex
	running  bool
}

func NewScheduledService(name string, interval time.Duration, task func() error) *ScheduledService {
	if interval <= 0 {
		interval = time.Minute
	}
	return &ScheduledService{
		name:     name,
		interval: interval,
		task:     task,
		stopCh:   make(chan struct{}),
	}
}

func (ss *ScheduledService) Start() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.running {
		return fmt.Errorf("service already running")
	}

	ss.running = true
	ss.ticker = time.NewTicker(ss.interval)

	go func() {
		for {
			select {
			case <-ss.ticker.C:
				_ = ss.task()
			case <-ss.stopCh:
				return
			}
		}
	}()

	return nil
}

func (ss *ScheduledService) Stop() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if !ss.running {
		return nil
	}

	ss.running = false
	if ss.ticker != nil {
		ss.ticker.Stop()
	}
	close(ss.stopCh)
	return nil
}

func (ss *ScheduledService) Name() string    { return ss.name }
func (ss *ScheduledService) IsRunning() bool { ss.mu.RLock(); defer ss.mu.RUnlock(); return ss.running }

type GracefulService struct {
	inner        Service
	shutdownTimeout time.Duration
	hooks         []func()
	mu            sync.RWMutex
}

func NewGracefulService(inner Service, timeout time.Duration) *GracefulService {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GracefulService{
		inner:           inner,
		shutdownTimeout: timeout,
		hooks:           make([]func(), 0),
	}
}

func (gs *GracefulService) OnShutdown(hook func()) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.hooks = append(gs.hooks, hook)
}

func (gs *GracefulService) Start() error {
	return gs.inner.Start()
}

func (gs *GracefulService) Stop() error {
	gs.mu.RLock()
	hooks := make([]func(), len(gs.hooks))
	copy(hooks, gs.hooks)
	gs.mu.RUnlock()

	done := make(chan error, 1)
	go func() {
		done <- gs.inner.Stop()
	}()

	select {
	case err := <-done:
		for _, hook := range hooks {
			hook()
		}
		return err
	case <-time.After(gs.shutdownTimeout):
		for _, hook := range hooks {
			hook()
		}
		return fmt.Errorf("graceful shutdown timed out")
	}
}

func (gs *GracefulService) Name() string    { return gs.inner.Name() }
func (gs *GracefulService) IsRunning() bool { return gs.inner.IsRunning() }

type ServiceHealthCheck struct {
	services   []Service
	interval   time.Duration
	statusFunc func(name string, healthy bool)
	stopCh     chan struct{}
	mu         sync.RWMutex
}

func NewServiceHealthCheck(interval time.Duration, statusFunc func(string, bool)) *ServiceHealthCheck {
	return &ServiceHealthCheck{
		services:   make([]Service, 0),
		interval:   interval,
		statusFunc: statusFunc,
		stopCh:     make(chan struct{}),
	}
}

func (shc *ServiceHealthCheck) AddService(svc Service) {
	shc.mu.Lock()
	defer shc.mu.Unlock()
	shc.services = append(shc.services, svc)
}

func (shc *ServiceHealthCheck) Start() {
	go func() {
		ticker := time.NewTicker(shc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				shc.check()
			case <-shc.stopCh:
				return
			}
		}
	}()
}

func (shc *ServiceHealthCheck) Stop() {
	close(shc.stopCh)
}

func (shc *ServiceHealthCheck) check() {
	shc.mu.RLock()
	services := make([]Service, len(shc.services))
	copy(services, shc.services)
	cb := shc.statusFunc
	shc.mu.RUnlock()

	for _, svc := range services {
		healthy := svc.IsRunning()
		if cb != nil {
			cb(svc.Name(), healthy)
		}
	}
}

type ServiceDecorator struct {
	inner    Service
	onStart  func() error
	onStop   func() error
	mu       sync.RWMutex
}

func NewServiceDecorator(inner Service) *ServiceDecorator {
	return &ServiceDecorator{inner: inner}
}

func (sd *ServiceDecorator) OnStart(fn func() error) *ServiceDecorator {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.onStart = fn
	return sd
}

func (sd *ServiceDecorator) OnStop(fn func() error) *ServiceDecorator {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.onStop = fn
	return sd
}

func (sd *ServiceDecorator) Start() error {
	sd.mu.RLock()
	onStart := sd.onStart
	sd.mu.RUnlock()

	if onStart != nil {
		if err := onStart(); err != nil {
			return err
		}
	}
	return sd.inner.Start()
}

func (sd *ServiceDecorator) Stop() error {
	sd.mu.RLock()
	onStop := sd.onStop
	sd.mu.RUnlock()

	err := sd.inner.Stop()
	if onStop != nil {
		if stopErr := onStop(); stopErr != nil && err == nil {
			err = stopErr
		}
	}
	return err
}

func (sd *ServiceDecorator) Name() string    { return sd.inner.Name() }
func (sd *ServiceDecorator) IsRunning() bool { return sd.inner.IsRunning() }

type ServicePool struct {
	services []Service
	current  int
	mu       sync.RWMutex
}

func NewServicePool(services ...Service) *ServicePool {
	return &ServicePool{services: services}
}

func (sp *ServicePool) Next() Service {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if len(sp.services) == 0 {
		return nil
	}
	svc := sp.services[sp.current]
	sp.current = (sp.current + 1) % len(sp.services)
	return svc
}

func (sp *ServicePool) StartAll() error {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	for _, svc := range sp.services {
		if err := svc.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (sp *ServicePool) StopAll() error {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	var lastErr error
	for i := len(sp.services) - 1; i >= 0; i-- {
		if err := sp.services[i].Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type ServiceMonitor struct {
	metrics map[string]*ServiceMetrics
	mu      sync.RWMutex
}

type ServiceMetrics struct {
	StartCount   int64
	StopCount    int64
	ErrorCount   int64
	TotalUptime  time.Duration
	LastStart    time.Time
	LastStop     time.Time
}

func NewServiceMonitor() *ServiceMonitor {
	return &ServiceMonitor{
		metrics: make(map[string]*ServiceMetrics),
	}
}

func (sm *ServiceMonitor) RecordStart(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.metrics[name]; !ok {
		sm.metrics[name] = &ServiceMetrics{}
	}
	sm.metrics[name].StartCount++
	sm.metrics[name].LastStart = time.Now()
}

func (sm *ServiceMonitor) RecordStop(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.metrics[name]; !ok {
		sm.metrics[name] = &ServiceMetrics{}
	}
	sm.metrics[name].StopCount++
	sm.metrics[name].LastStop = time.Now()
	sm.metrics[name].TotalUptime += time.Since(sm.metrics[name].LastStart)
}

func (sm *ServiceMonitor) RecordError(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.metrics[name]; !ok {
		sm.metrics[name] = &ServiceMetrics{}
	}
	sm.metrics[name].ErrorCount++
}

func (sm *ServiceMonitor) GetMetrics(name string) *ServiceMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	m, ok := sm.metrics[name]
	if !ok {
		return nil
	}
	cp := *m
	return &cp
}

func (sm *ServiceMonitor) Snapshot() map[string]*ServiceMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]*ServiceMetrics, len(sm.metrics))
	for k, v := range sm.metrics {
		cp := *v
		result[k] = &cp
	}
	return result
}

type ServiceLimiter struct {
	limiter  map[string]int
	counts   map[string]int
	mu       sync.RWMutex
}

func NewServiceLimiter(maxConcurrent int) *ServiceLimiter {
	return &ServiceLimiter{
		limiter: make(map[string]int),
		counts:  make(map[string]int),
	}
}

func (sl *ServiceLimiter) SetLimit(name string, max int) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.limiter[name] = max
}

func (sl *ServiceLimiter) Acquire(name string) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	max, ok := sl.limiter[name]
	if !ok {
		return true
	}
	if sl.counts[name] >= max {
		return false
	}
	sl.counts[name]++
	return true
}

func (sl *ServiceLimiter) Release(name string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.counts[name] > 0 {
		sl.counts[name]--
	}
}

func (sl *ServiceLimiter) Count(name string) int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.counts[name]
}
