package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
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
		return "half-open"
	default:
		return "unknown"
	}
}

type CircuitBreaker struct {
	name            string
	state           State
	failureCount    int
	successCount    int
	failureThreshold int
	successThreshold int
	timeout         time.Duration
	lastFailureTime time.Time
	halfOpenMax     int
	halfOpenCount   int
	onStateChange   func(name string, from, to State)
	fallback        func(error) error
	metrics         *BreakerMetrics
	mu              sync.RWMutex
}

type CircuitBreakerConfig struct {
	Name             string
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	HalfOpenMax      int
	OnStateChange    func(string, State, State)
	Fallback         func(error) error
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:             "default",
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          60 * time.Second,
		HalfOpenMax:      3,
	}
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	if config.HalfOpenMax <= 0 {
		config.HalfOpenMax = 3
	}

	return &CircuitBreaker{
		name:             config.Name,
		state:            StateClosed,
		failureThreshold: config.FailureThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
		halfOpenMax:      config.HalfOpenMax,
		onStateChange:    config.OnStateChange,
		fallback:         config.Fallback,
		metrics:          NewBreakerMetrics(),
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case StateOpen:
		if cb.shouldTryReset() {
			cb.setState(StateHalfOpen)
		} else {
			if cb.fallback != nil {
				return cb.fallback(ErrCircuitOpen)
			}
			return ErrCircuitOpen
		}
	case StateHalfOpen:
		cb.mu.RLock()
		halfOpenCount := cb.halfOpenCount
		cb.mu.RUnlock()
		if halfOpenCount >= cb.halfOpenMax {
			if cb.fallback != nil {
				return cb.fallback(ErrTooManyRequests)
			}
			return ErrTooManyRequests
		}
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
		if cb.fallback != nil {
			return cb.fallback(err)
		}
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()
	cb.metrics.RecordFailure()

	cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.setState(StateOpen)
		return
	}

	cb.mu.RLock()
	threshold := cb.failureThreshold
	failureCount := cb.failureCount
	cb.mu.RUnlock()

	if failureCount >= threshold {
		cb.setState(StateOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.failureCount = 0
	cb.metrics.RecordSuccess()

	if cb.state == StateHalfOpen {
		cb.mu.RUnlock()
		cb.mu.RLock()
		threshold := cb.successThreshold
		successCount := cb.successCount
		cb.mu.RUnlock()

		if successCount >= threshold {
			cb.setState(StateClosed)
		}
	}
}

func (cb *CircuitBreaker) setState(newState State) {
	cb.mu.Lock()
	oldState := cb.state
	cb.state = newState

	switch newState {
	case StateClosed:
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenCount = 0
	case StateOpen:
		cb.halfOpenCount = 0
		cb.successCount = 0
	case StateHalfOpen:
		cb.halfOpenCount = 0
		cb.successCount = 0
		cb.failureCount = 0
	}

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, oldState, newState)
	}
	cb.mu.Unlock()

	cb.metrics.RecordStateChange(newState)
}

func (cb *CircuitBreaker) shouldTryReset() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return time.Since(cb.lastFailureTime) > cb.timeout
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Name() string {
	return cb.name
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCount = 0
}

func (cb *CircuitBreaker) Metrics() BreakerMetricsSnapshot {
	return cb.metrics.Snapshot()
}

type BreakerMetrics struct {
	totalRequests   int64
	successCount    int64
	failureCount    int64
	consecutiveFails int64
	lastStateChange time.Time
	stateChanges    []StateChange
	mu              sync.RWMutex
}

type StateChange struct {
	From      State
	To        State
	Timestamp time.Time
}

func NewBreakerMetrics() *BreakerMetrics {
	return &BreakerMetrics{
		lastStateChange: time.Now(),
		stateChanges:    make([]StateChange, 0),
	}
}

func (bm *BreakerMetrics) RecordSuccess() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.totalRequests++
	bm.successCount++
	bm.consecutiveFails = 0
}

func (bm *BreakerMetrics) RecordFailure() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.totalRequests++
	bm.failureCount++
	bm.consecutiveFails++
}

func (bm *BreakerMetrics) RecordStateChange(to State) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.stateChanges = append(bm.stateChanges, StateChange{
		To:        to,
		Timestamp: time.Now(),
	})
	bm.lastStateChange = time.Now()
}

func (bm *BreakerMetrics) Snapshot() BreakerMetricsSnapshot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return BreakerMetricsSnapshot{
		TotalRequests:   bm.totalRequests,
		SuccessCount:    bm.successCount,
		FailureCount:    bm.failureCount,
		ConsecutiveFails: bm.consecutiveFails,
		LastStateChange: bm.lastStateChange,
		StateChanges:    len(bm.stateChanges),
	}
}

func (bm *BreakerMetrics) Reset() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.totalRequests = 0
	bm.successCount = 0
	bm.failureCount = 0
	bm.consecutiveFails = 0
	bm.stateChanges = bm.stateChanges[:0]
}

type BreakerMetricsSnapshot struct {
	TotalRequests    int64
	SuccessCount     int64
	FailureCount     int64
	ConsecutiveFails int64
	LastStateChange  time.Time
	StateChanges     int
}

type BreakerGroup struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

func NewBreakerGroup() *BreakerGroup {
	return &BreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (bg *BreakerGroup) GetOrCreate(name string, config CircuitBreakerConfig) *CircuitBreaker {
	bg.mu.RLock()
	cb, exists := bg.breakers[name]
	bg.mu.RUnlock()

	if exists {
		return cb
	}

	bg.mu.Lock()
	defer bg.mu.Unlock()

	cb, exists = bg.breakers[name]
	if exists {
		return cb
	}

	config.Name = name
	cb = NewCircuitBreaker(config)
	bg.breakers[name] = cb
	return cb
}

func (bg *BreakerGroup) Get(name string) *CircuitBreaker {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.breakers[name]
}

func (bg *BreakerGroup) Remove(name string) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	delete(bg.breakers, name)
}

func (bg *BreakerGroup) Execute(name string, fn func() error) error {
	cb := bg.Get(name)
	if cb == nil {
		return fn()
	}
	return cb.Execute(fn)
}

func (bg *BreakerGroup) ResetAll() {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	for _, cb := range bg.breakers {
		cb.Reset()
	}
}

func (bg *BreakerGroup) Count() int {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return len(bg.breakers)
}

func (bg *BreakerGroup) States() map[string]State {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	states := make(map[string]State)
	for name, cb := range bg.breakers {
		states[name] = cb.State()
	}
	return states
}

type FallbackStrategy interface {
	Execute(err error) error
}

type StaticFallback struct {
	result error
}

func NewStaticFallback(err error) *StaticFallback {
	return &StaticFallback{result: err}
}

func (sf *StaticFallback) Execute(_ error) error {
	return sf.result
}

type RetryFallback struct {
	retries int
	delay   time.Duration
	fn      func() error
	mu      sync.RWMutex
}

func NewRetryFallback(retries int, delay time.Duration, fn func() error) *RetryFallback {
	return &RetryFallback{
		retries: retries,
		delay:   delay,
		fn:      fn,
	}
}

func (rf *RetryFallback) Execute(_ error) error {
	rf.mu.RLock()
	retries := rf.retries
	delay := rf.delay
	fn := rf.fn
	rf.mu.RUnlock()

	var err error
	for i := 0; i < retries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}

type EventListener struct {
	callbacks map[string][]func(string, State, State)
	mu        sync.RWMutex
}

func NewEventListener() *EventListener {
	return &EventListener{
		callbacks: make(map[string][]func(string, State, State)),
	}
}

func (el *EventListener) On(name string, callback func(string, State, State)) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.callbacks[name] = append(el.callbacks[name], callback)
}

func (el *EventListener) Notify(name string, from, to State) {
	el.mu.RLock()
	defer el.mu.RUnlock()

	if callbacks, exists := el.callbacks[name]; exists {
		for _, cb := range callbacks {
			cb(name, from, to)
		}
	}

	if callbacks, exists := el.callbacks["*"]; exists {
		for _, cb := range callbacks {
			cb(name, from, to)
		}
	}
}

func (el *EventListener) Remove(name string) {
	el.mu.Lock()
	defer el.mu.Unlock()
	delete(el.callbacks, name)
}

func (el *EventListener) RemoveAll() {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.callbacks = make(map[string][]func(string, State, State))
}

type HealthChecker struct {
	breakers  *BreakerGroup
	interval  time.Duration
	callbacks []func(map[string]State)
	mu        sync.RWMutex
	stopCh    chan struct{}
}

func NewHealthChecker(breakers *BreakerGroup, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		breakers:  breakers,
		interval:  interval,
		callbacks: make([]func(map[string]State), 0),
		stopCh:    make(chan struct{}),
	}
}

func (hc *HealthChecker) OnHealthChange(fn func(map[string]State)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.callbacks = append(hc.callbacks, fn)
}

func (hc *HealthChecker) Check() map[string]State {
	return hc.breakers.States()
}

func (hc *HealthChecker) IsHealthy() bool {
	states := hc.breakers.States()
	for _, state := range states {
		if state == StateOpen {
			return false
		}
	}
	return true
}

func (hc *HealthChecker) Start() {
	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				states := hc.Check()
				hc.mu.RLock()
				callbacks := hc.callbacks
				hc.mu.RUnlock()
				for _, cb := range callbacks {
					cb(states)
				}
			case <-hc.stopCh:
				return
			}
		}
	}()
}

func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

type StatePersistence struct {
	store   map[string]State
	history []StateRecord
	mu      sync.RWMutex
}

type StateRecord struct {
	Name      string
	From      State
	To        State
	Timestamp time.Time
}

func NewStatePersistence() *StatePersistence {
	return &StatePersistence{
		store:   make(map[string]State),
		history: make([]StateRecord, 0),
	}
}

func (sp *StatePersistence) Save(name string, state State) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.store[name] = state
	sp.history = append(sp.history, StateRecord{
		Name:      name,
		To:        state,
		Timestamp: time.Now(),
	})
}

func (sp *StatePersistence) Load(name string) (State, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	state, exists := sp.store[name]
	return state, exists
}

func (sp *StatePersistence) History(name string) []StateRecord {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	result := make([]StateRecord, 0)
	for _, record := range sp.history {
		if record.Name == name {
			result = append(result, record)
		}
	}
	return result
}

func (sp *StatePersistence) Clear() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.store = make(map[string]State)
	sp.history = sp.history[:0]
}

type BreakerMiddleware struct {
	group   *BreakerGroup
	config  CircuitBreakerConfig
	mu      sync.RWMutex
}

func NewBreakerMiddleware(group *BreakerGroup, config CircuitBreakerConfig) *BreakerMiddleware {
	return &BreakerMiddleware{
		group:  group,
		config: config,
	}
}

func (bm *BreakerMiddleware) Execute(name string, fn func() error) error {
	cb := bm.group.GetOrCreate(name, bm.config)
	return cb.Execute(fn)
}

func (bm *BreakerMiddleware) SetConfig(config CircuitBreakerConfig) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.config = config
}

var (
	ErrCircuitOpen    = &BreakerError{Code: "CIRCUIT_OPEN", Message: "circuit breaker is open"}
	ErrTooManyRequests = &BreakerError{Code: "TOO_MANY_REQUESTS", Message: "too many requests in half-open state"}
)

type BreakerError struct {
	Code    string
	Message string
}

func (be *BreakerError) Error() string {
	return be.Message
}
