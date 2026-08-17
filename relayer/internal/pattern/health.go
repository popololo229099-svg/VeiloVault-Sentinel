package pattern

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type HealthChecker struct {
	checks     map[string]HealthCheck
	interval   time.Duration
	mu         sync.RWMutex
	results    map[string]HealthCheckResult
}

type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
	Timeout() time.Duration
}

type HealthCheckResult struct {
	Name      string
	Status    string
	Error     string
	Latency   time.Duration
	CheckedAt time.Time
}

func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		checks:  make(map[string]HealthCheck),
		interval: interval,
		results: make(map[string]HealthCheckResult),
	}
}

func (hc *HealthChecker) Register(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[check.Name()] = check
}

func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	hc.runChecks(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.runChecks(ctx)
		}
	}
}

func (hc *HealthChecker) runChecks(ctx context.Context) {
	hc.mu.RLock()
	checks := make([]HealthCheck, 0, len(hc.checks))
	for _, c := range hc.checks {
		checks = append(checks, c)
	}
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	for _, check := range checks {
		wg.Add(1)
		go func(c HealthCheck) {
			defer wg.Done()
			timeout := c.Timeout()
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			start := time.Now()
			err := c.Check(checkCtx)
			latency := time.Since(start)
			result := HealthCheckResult{
				Name:      c.Name(),
				Status:    "healthy",
				Latency:   latency,
				CheckedAt: time.Now(),
			}
			if err != nil {
				result.Status = "unhealthy"
				result.Error = err.Error()
			}
			hc.mu.Lock()
			hc.results[c.Name()] = result
			hc.mu.Unlock()
		}(check)
	}
	wg.Wait()
}

func (hc *HealthChecker) Results() map[string]HealthCheckResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	results := make(map[string]HealthCheckResult)
	for k, v := range hc.results {
		results[k] = v
	}
	return results
}

func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	for _, r := range hc.results {
		if r.Status != "healthy" {
			return false
		}
	}
	return true
}

type ReadinessProbe interface {
	Ready(ctx context.Context) error
}

type LivenessProbe interface {
	Live(ctx context.Context) error
}

type HealthAggregator struct {
	readiness map[string]ReadinessProbe
	liveness  map[string]LivenessProbe
	mu        sync.RWMutex
}

func NewHealthAggregator() *HealthAggregator {
	return &HealthAggregator{
		readiness: make(map[string]ReadinessProbe),
		liveness:  make(map[string]LivenessProbe),
	}
}

func (ha *HealthAggregator) RegisterReadiness(name string, probe ReadinessProbe) {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.readiness[name] = probe
}

func (ha *HealthAggregator) RegisterLiveness(name string, probe LivenessProbe) {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.liveness[name] = probe
}

func (ha *HealthAggregator) CheckReadiness(ctx context.Context) map[string]string {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	status := make(map[string]string)
	for name, probe := range ha.readiness {
		if err := probe.Ready(ctx); err != nil {
			status[name] = fmt.Sprintf("not ready: %v", err)
		} else {
			status[name] = "ready"
		}
	}
	return status
}

func (ha *HealthAggregator) CheckLiveness(ctx context.Context) map[string]string {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	status := make(map[string]string)
	for name, probe := range ha.liveness {
		if err := probe.Live(ctx); err != nil {
			status[name] = fmt.Sprintf("not live: %v", err)
		} else {
			status[name] = "live"
		}
	}
	return status
}
