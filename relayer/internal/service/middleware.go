package service

import (
	"fmt"
	"sync"
	"time"
)

type ServiceMiddleware func(Service) Service

func LoggingMiddleware(logFunc func(string, ...interface{})) ServiceMiddleware {
	return func(next Service) Service {
		return &loggingService{
			inner:   next,
			logFunc: logFunc,
		}
	}
}

type loggingService struct {
	inner   Service
	logFunc func(string, ...interface{})
	mu      sync.RWMutex
}

func (ls *loggingService) Name() string { return ls.inner.Name() }

func (ls *loggingService) Start() error {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	ls.logFunc("starting service %s", ls.inner.Name())
	err := ls.inner.Start()
	if err != nil {
		ls.logFunc("service %s failed to start: %v", ls.inner.Name(), err)
	} else {
		ls.logFunc("service %s started", ls.inner.Name())
	}
	return err
}

func (ls *loggingService) Stop() error {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	ls.logFunc("stopping service %s", ls.inner.Name())
	err := ls.inner.Stop()
	if err != nil {
		ls.logFunc("service %s failed to stop: %v", ls.inner.Name(), err)
	} else {
		ls.logFunc("service %s stopped", ls.inner.Name())
	}
	return err
}

func (ls *loggingService) IsRunning() bool { return ls.inner.IsRunning() }

func MetricsMiddleware(monitor *ServiceMonitor) ServiceMiddleware {
	return func(next Service) Service {
		return &metricsService{
			inner:   next,
			monitor: monitor,
		}
	}
}

type metricsService struct {
	inner   Service
	monitor *ServiceMonitor
	mu      sync.RWMutex
}

func (ms *metricsService) Name() string { return ms.inner.Name() }

func (ms *metricsService) Start() error {
	start := time.Now()
	err := ms.inner.Start()
	duration := time.Since(start)

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.monitor != nil {
		ms.monitor.RecordStart(ms.inner.Name())
		if err != nil {
			ms.monitor.RecordError(ms.inner.Name())
		}
		_ = duration
	}
	return err
}

func (ms *metricsService) Stop() error {
	start := time.Now()
	err := ms.inner.Stop()
	duration := time.Since(start)

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.monitor != nil {
		ms.monitor.RecordStop(ms.inner.Name())
		if err != nil {
			ms.monitor.RecordError(ms.inner.Name())
		}
		_ = duration
	}
	return err
}

func (ms *metricsService) IsRunning() bool { return ms.inner.IsRunning() }

func RetryMiddleware(maxRetries int, delay time.Duration) ServiceMiddleware {
	return func(next Service) Service {
		return &retryService{
			inner:      next,
			maxRetries: maxRetries,
			delay:      delay,
		}
	}
}

type retryService struct {
	inner      Service
	maxRetries int
	delay      time.Duration
	mu         sync.RWMutex
}

func (rs *retryService) Name() string { return rs.inner.Name() }

func (rs *retryService) Start() error {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var err error
	for i := 0; i <= rs.maxRetries; i++ {
		err = rs.inner.Start()
		if err == nil {
			return nil
		}
		if i < rs.maxRetries {
			time.Sleep(rs.delay)
		}
	}
	return fmt.Errorf("service %s failed to start after %d retries: %w", rs.inner.Name(), rs.maxRetries, err)
}

func (rs *retryService) Stop() error {
	return rs.inner.Stop()
}

func (rs *retryService) IsRunning() bool { return rs.inner.IsRunning() }

type CircuitBreakerService struct {
	inner         Service
	failureCount  int
	threshold     int
	state         string
	resetTimeout  time.Duration
	lastFailure   time.Time
	mu            sync.RWMutex
}

func NewCircuitBreakerService(inner Service, threshold int, resetTimeout time.Duration) *CircuitBreakerService {
	if threshold <= 0 {
		threshold = 3
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreakerService{
		inner:        inner,
		threshold:    threshold,
		state:        "closed",
		resetTimeout: resetTimeout,
	}
}

func (cbs *CircuitBreakerService) Name() string { return cbs.inner.Name() }

func (cbs *CircuitBreakerService) Start() error {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	if cbs.state == "open" {
		if time.Since(cbs.lastFailure) > cbs.resetTimeout {
			cbs.state = "half-open"
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := cbs.inner.Start()
	if err != nil {
		cbs.failureCount++
		cbs.lastFailure = time.Now()
		if cbs.failureCount >= cbs.threshold {
			cbs.state = "open"
		}
		return err
	}

	cbs.failureCount = 0
	cbs.state = "closed"
	return nil
}

func (cbs *CircuitBreakerService) Stop() error {
	return cbs.inner.Stop()
}

func (cbs *CircuitBreakerService) IsRunning() bool { return cbs.inner.IsRunning() }

type TimeoutService struct {
	inner   Service
	timeout time.Duration
	mu      sync.RWMutex
}

func NewTimeoutService(inner Service, timeout time.Duration) *TimeoutService {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &TimeoutService{inner: inner, timeout: timeout}
}

func (ts *TimeoutService) Name() string { return ts.inner.Name() }

func (ts *TimeoutService) Start() error {
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() { ch <- result{ts.inner.Start()} }()

	select {
	case r := <-ch:
		return r.err
	case <-time.After(ts.timeout):
		return fmt.Errorf("service %s start timed out", ts.inner.Name())
	}
}

func (ts *TimeoutService) Stop() error {
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() { ch <- result{ts.inner.Stop()} }()

	select {
	case r := <-ch:
		return r.err
	case <-time.After(ts.timeout):
		return fmt.Errorf("service %s stop timed out", ts.inner.Name())
	}
}

func (ts *TimeoutService) IsRunning() bool { return ts.inner.IsRunning() }

type ServiceGroup struct {
	services []Service
	mu       sync.RWMutex
}

func NewServiceGroup(services ...Service) *ServiceGroup {
	return &ServiceGroup{services: services}
}

func (sg *ServiceGroup) Start() error {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	for _, svc := range sg.services {
		if err := svc.Start(); err != nil {
			return fmt.Errorf("failed to start %s: %w", svc.Name(), err)
		}
	}
	return nil
}

func (sg *ServiceGroup) Stop() error {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	var lastErr error
	for i := len(sg.services) - 1; i >= 0; i-- {
		if err := sg.services[i].Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (sg *ServiceGroup) Name() string    { return "group" }
func (sg *ServiceGroup) IsRunning() bool { return true }

type ServiceWrapper struct {
	name    string
	startFn func() error
	stopFn  func() error
	running bool
	mu      sync.RWMutex
}

func NewServiceWrapper(name string) *ServiceWrapper {
	return &ServiceWrapper{name: name}
}

func (sw *ServiceWrapper) OnStart(fn func() error) *ServiceWrapper {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.startFn = fn
	return sw
}

func (sw *ServiceWrapper) OnStop(fn func() error) *ServiceWrapper {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.stopFn = fn
	return sw
}

func (sw *ServiceWrapper) Start() error {
	sw.mu.RLock()
	fn := sw.startFn
	sw.mu.RUnlock()

	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	sw.mu.Lock()
	sw.running = true
	sw.mu.Unlock()
	return nil
}

func (sw *ServiceWrapper) Stop() error {
	sw.mu.RLock()
	fn := sw.stopFn
	sw.mu.RUnlock()

	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	sw.mu.Lock()
	sw.running = false
	sw.mu.Unlock()
	return nil
}

func (sw *ServiceWrapper) Name() string    { return sw.name }
func (sw *ServiceWrapper) IsRunning() bool { sw.mu.RLock(); defer sw.mu.RUnlock(); return sw.running }

type ServiceLifecycle struct {
	name       string
	onStart    func() error
	onStop     func() error
	onPause    func() error
	onResume   func() error
	state      string
	startTime  time.Time
	mu         sync.RWMutex
}

func NewServiceLifecycle(name string) *ServiceLifecycle {
	return &ServiceLifecycle{
		name:  name,
		state: "stopped",
	}
}

func (sl *ServiceLifecycle) OnStart(fn func() error) *ServiceLifecycle {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.onStart = fn
	return sl
}

func (sl *ServiceLifecycle) OnStop(fn func() error) *ServiceLifecycle {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.onStop = fn
	return sl
}

func (sl *ServiceLifecycle) OnPause(fn func() error) *ServiceLifecycle {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.onPause = fn
	return sl
}

func (sl *ServiceLifecycle) OnResume(fn func() error) *ServiceLifecycle {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.onResume = fn
	return sl
}

func (sl *ServiceLifecycle) Start() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.state != "stopped" {
		return fmt.Errorf("cannot start: state is %s", sl.state)
	}

	if sl.onStart != nil {
		if err := sl.onStart(); err != nil {
			return err
		}
	}

	sl.state = "running"
	sl.startTime = time.Now()
	return nil
}

func (sl *ServiceLifecycle) Stop() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.state == "stopped" {
		return fmt.Errorf("cannot stop: already stopped")
	}

	if sl.onStop != nil {
		if err := sl.onStop(); err != nil {
			return err
		}
	}

	sl.state = "stopped"
	return nil
}

func (sl *ServiceLifecycle) Pause() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.state != "running" {
		return fmt.Errorf("cannot pause: state is %s", sl.state)
	}

	if sl.onPause != nil {
		if err := sl.onPause(); err != nil {
			return err
		}
	}

	sl.state = "paused"
	return nil
}

func (sl *ServiceLifecycle) Resume() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.state != "paused" {
		return fmt.Errorf("cannot resume: state is %s", sl.state)
	}

	if sl.onResume != nil {
		if err := sl.onResume(); err != nil {
			return err
		}
	}

	sl.state = "running"
	return nil
}

func (sl *ServiceLifecycle) Name() string    { return sl.name }
func (sl *ServiceLifecycle) IsRunning() bool { sl.mu.RLock(); defer sl.mu.RUnlock(); return sl.state == "running" }
func (sl *ServiceLifecycle) State() string   { sl.mu.RLock(); defer sl.mu.RUnlock(); return sl.state }

func (sl *ServiceLifecycle) Uptime() time.Duration {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	if sl.state == "running" {
		return time.Since(sl.startTime)
	}
	return 0
}

type ServiceRegistry2 struct {
	services map[string]Service
	aliases  map[string]string
	mu       sync.RWMutex
}

func NewServiceRegistry2() *ServiceRegistry2 {
	return &ServiceRegistry2{
		services: make(map[string]Service),
		aliases:  make(map[string]string),
	}
}

func (sr *ServiceRegistry2) Register(svc Service) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.services[svc.Name()] = svc
}

func (sr *ServiceRegistry2) RegisterAlias(alias, name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.aliases[alias] = name
}

func (sr *ServiceRegistry2) Get(name string) Service {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	if actual, ok := sr.aliases[name]; ok {
		return sr.services[actual]
	}
	return sr.services[name]
}

func (sr *ServiceRegistry2) Remove(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.services, name)
	for alias, target := range sr.aliases {
		if target == name {
			delete(sr.aliases, alias)
		}
	}
}

func (sr *ServiceRegistry2) StartAll() error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	for _, svc := range sr.services {
		if err := svc.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (sr *ServiceRegistry2) StopAll() error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	var lastErr error
	for _, svc := range sr.services {
		if err := svc.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (sr *ServiceRegistry2) Status() map[string]bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	status := make(map[string]bool)
	for name, svc := range sr.services {
		status[name] = svc.IsRunning()
	}
	return status
}
