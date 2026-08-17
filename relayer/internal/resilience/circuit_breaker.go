package resilience

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open")
)

type State int

const (
	StateClosed   State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

type CircuitBreaker struct {
	mu                   sync.Mutex
	state                State
	failureCount         int
	successCount         int
	failureThreshold     int
	successThreshold     int
	timeout              time.Duration
	halfOpenMaxRequests  int
	halfOpenRequests     int
	lastFailureTime      time.Time
	onStateChange        func(from, to State)
	metrics              *CircuitMetrics
}

type CircuitBreakerConfig struct {
	FailureThreshold    int
	SuccessThreshold    int
	Timeout             time.Duration
	HalfOpenMaxRequests int
	OnStateChange       func(from, to State)
}

type CircuitMetrics struct {
	TotalRequests   int64
	TotalFailures   int64
	TotalSuccesses  int64
	TotalTimeouts   int64
	ConsecutiveFails int
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HalfOpenMaxRequests <= 0 {
		cfg.HalfOpenMaxRequests = 3
	}
	return &CircuitBreaker{
		state:               StateClosed,
		failureThreshold:    cfg.FailureThreshold,
		successThreshold:    cfg.SuccessThreshold,
		timeout:             cfg.Timeout,
		halfOpenMaxRequests: cfg.HalfOpenMaxRequests,
		onStateChange:       cfg.OnStateChange,
		metrics:             &CircuitMetrics{},
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.allowRequest(); err != nil {
		return err
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) allowRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.setState(StateHalfOpen)
			cb.halfOpenRequests = 0
			cb.successCount = 0
			return nil
		}
		return ErrCircuitOpen
	case StateHalfOpen:
		if cb.halfOpenRequests >= cb.halfOpenMaxRequests {
			return ErrTooManyRequests
		}
		cb.halfOpenRequests++
		return nil
	}
	return nil
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.metrics.TotalRequests++
	if err != nil {
		cb.metrics.TotalFailures++
		cb.metrics.ConsecutiveFails++
		cb.failureCount++
		cb.successCount = 0

		switch cb.state {
		case StateClosed:
			if cb.failureCount >= cb.failureThreshold {
				cb.lastFailureTime = time.Now()
				cb.setState(StateOpen)
			}
		case StateHalfOpen:
			cb.lastFailureTime = time.Now()
			cb.setState(StateOpen)
		}
	} else {
		cb.metrics.TotalSuccesses++
		cb.metrics.ConsecutiveFails = 0
		cb.successCount++
		cb.failureCount = 0

		if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {
			cb.setState(StateClosed)
		}
	}
}

func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}
	old := cb.state
	cb.state = newState
	cb.failureCount = 0
	cb.successCount = 0
	if cb.onStateChange != nil {
		go cb.onStateChange(old, newState)
	}
}

func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) GetMetrics() CircuitMetrics {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return *cb.metrics
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.metrics = &CircuitMetrics{}
}

// MultiCircuitBreaker manages multiple circuit breakers by key.
type MultiCircuitBreaker struct {
	breakers map[string]*CircuitBreaker
	cfg      CircuitBreakerConfig
	mu       sync.RWMutex
}

func NewMultiCircuitBreaker(cfg CircuitBreakerConfig) *MultiCircuitBreaker {
	return &MultiCircuitBreaker{
		breakers: make(map[string]*CircuitBreaker),
		cfg:      cfg,
	}
}

func (m *MultiCircuitBreaker) Get(key string) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cb, ok := m.breakers[key]; ok {
		return cb
	}
	cb := NewCircuitBreaker(m.cfg)
	m.breakers[key] = cb
	return cb
}

func (m *MultiCircuitBreaker) Execute(key string, fn func() error) error {
	return m.Get(key).Execute(fn)
}

func (m *MultiCircuitBreaker) AllStates() map[string]State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	states := make(map[string]State)
	for k, cb := range m.breakers {
		states[k] = cb.GetState()
	}
	return states
}
