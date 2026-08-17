package saga

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepRunning    StepStatus = "running"
	StepCompleted  StepStatus = "completed"
	StepFailed     StepStatus = "failed"
	StepCompensating  StepStatus = "compensating"
	StepCompensated StepStatus = "compensated"
	StepSkipped    StepStatus = "skipped"
)

type SagaStatus string

const (
	SagaPending    SagaStatus = "pending"
	SagaRunning    SagaStatus = "running"
	SagaCompleted  SagaStatus = "completed"
	SagaFailed     SagaStatus = "failed"
	SagaCompensating SagaStatus = "compensating"
	SagaCompensated SagaStatus = "compensated"
)

type Step interface {
	Name() string
	Execute(ctx context.Context, state *SagaState) error
	Compensate(ctx context.Context, state *SagaState) error
}

type StepFunc struct {
	name          string
	executeFn     func(ctx context.Context, state *SagaState) error
	compensateFn  func(ctx context.Context, state *SagaState) error
	timeout       time.Duration
	maxRetries    int
	retryDelay    time.Duration
}

func NewStepFunc(name string, execute, compensate func(ctx context.Context, state *SagaState) error) *StepFunc {
	return &StepFunc{
		name:         name,
		executeFn:    execute,
		compensateFn: compensate,
		timeout:      30 * time.Second,
		maxRetries:   3,
		retryDelay:   1 * time.Second,
	}
}

func (s *StepFunc) WithTimeout(timeout time.Duration) *StepFunc {
	s.timeout = timeout
	return s
}

func (s *StepFunc) WithRetry(maxRetries int, delay time.Duration) *StepFunc {
	s.maxRetries = maxRetries
	s.retryDelay = delay
	return s
}

func (s *StepFunc) Name() string { return s.name }

func (s *StepFunc) Execute(ctx context.Context, state *SagaState) error {
	if s.executeFn == nil {
		return nil
	}
	return s.executeFn(ctx, state)
}

func (s *StepFunc) Compensate(ctx context.Context, state *SagaState) error {
	if s.compensateFn == nil {
		return nil
	}
	return s.compensateFn(ctx, state)
}

type SagaState struct {
	ID         string
	Status     SagaStatus
	Data       map[string]interface{}
	StepStates map[string]*StepExecution
	Error      error
	CreatedAt  time.Time
	UpdatedAt  time.Time
	mu         sync.RWMutex
}

type StepExecution struct {
	StepName   string
	Status     StepStatus
	Error      error
	StartedAt  time.Time
	FinishedAt time.Time
	Retries    int
}

func NewSagaState(id string) *SagaState {
	return &SagaState{
		ID:         id,
		Status:     SagaPending,
		Data:       make(map[string]interface{}),
		StepStates: make(map[string]*StepExecution),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (s *SagaState) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
	s.UpdatedAt = time.Now()
}

func (s *SagaState) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Data[key]
	return val, ok
}

func (s *SagaState) GetString(key string) (string, bool) {
	val, ok := s.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func (s *SagaState) GetInt(key string) (int, bool) {
	val, ok := s.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

func (s *SagaState) MustGet(key string) interface{} {
	val, _ := s.Get(key)
	return val
}

func (s *SagaState) UpdateStepState(stepName string, status StepStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exec, ok := s.StepStates[stepName]
	if !ok {
		exec = &StepExecution{StepName: stepName}
		s.StepStates[stepName] = exec
	}
	exec.Status = status
	exec.Error = err
	if status == StepRunning {
		exec.StartedAt = time.Now()
	}
	if status == StepCompleted || status == StepFailed || status == StepCompensated {
		exec.FinishedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
}

type SagaConfig struct {
	MaxRetries       int
	RetryDelay       time.Duration
	StepTimeout      time.Duration
	EnableTracing    bool
	OnStepComplete   func(step string, state *SagaState)
	OnStepFailed     func(step string, state *SagaState, err error)
	OnCompensation   func(step string, state *SagaState)
	OnSagaComplete   func(id string, state *SagaState)
	OnSagaFailed     func(id string, state *SagaState, err error)
}

func DefaultSagaConfig() SagaConfig {
	return SagaConfig{
		MaxRetries:  3,
		RetryDelay:  1 * time.Second,
		StepTimeout: 30 * time.Second,
	}
}

type Saga struct {
	id     string
	steps  []Step
	state  *SagaState
	config SagaConfig
	mu     sync.RWMutex
}

func NewSaga(id string, config ...SagaConfig) *Saga {
	cfg := DefaultSagaConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &Saga{
		id:     id,
		steps:  make([]Step, 0),
		state:  NewSagaState(id),
		config: cfg,
	}
}

func (s *Saga) AddStep(step Step) *Saga {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
	return s
}

func (s *Saga) Execute(ctx context.Context) error {
	s.mu.Lock()
	s.state.Status = SagaRunning
	s.mu.Unlock()

	completedSteps := make([]int, 0)

	for i, step := range s.steps {
		stepCtx := ctx
		if s.config.StepTimeout > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, s.config.StepTimeout)
			defer cancel()
		}

		s.state.UpdateStepState(step.Name(), StepRunning, nil)

		var err error
		maxRetries := s.config.MaxRetries
		if sf, ok := step.(*StepFunc); ok && sf.maxRetries > 0 {
			maxRetries = sf.maxRetries
		}

		for retry := 0; retry <= maxRetries; retry++ {
			err = step.Execute(stepCtx, s.state)
			if err == nil {
				break
			}
			if retry < maxRetries {
				delay := s.config.RetryDelay
				if sf, ok := step.(*StepFunc); ok && sf.retryDelay > 0 {
					delay = sf.retryDelay
				}
				select {
				case <-stepCtx.Done():
					err = stepCtx.Err()
					goto done
				case <-time.After(delay):
				}
			}
		}
	done:

		if err != nil {
			s.state.UpdateStepState(step.Name(), StepFailed, err)
			s.state.Error = err
			s.state.Status = SagaFailed

			if s.config.OnStepFailed != nil {
				s.config.OnStepFailed(step.Name(), s.state, err)
			}

			if compensateErr := s.compensate(ctx, completedSteps); compensateErr != nil {
				s.state.Status = SagaFailed
				s.state.Error = fmt.Errorf("compensation failed: %w (original: %w)", compensateErr, err)
				if s.config.OnSagaFailed != nil {
					s.config.OnSagaFailed(s.id, s.state, s.state.Error)
				}
				return s.state.Error
			}

			s.state.Status = SagaCompensated
			if s.config.OnSagaFailed != nil {
				s.config.OnSagaFailed(s.id, s.state, err)
			}
			return fmt.Errorf("saga %s compensated after failure: %w", s.id, err)
		}

		s.state.UpdateStepState(step.Name(), StepCompleted, nil)
		completedSteps = append(completedSteps, i)

		if s.config.OnStepComplete != nil {
			s.config.OnStepComplete(step.Name(), s.state)
		}
	}

	s.state.Status = SagaCompleted
	s.state.UpdatedAt = time.Now()

	if s.config.OnSagaComplete != nil {
		s.config.OnSagaComplete(s.id, s.state)
	}

	return nil
}

func (s *Saga) compensate(ctx context.Context, completedSteps []int) error {
	s.state.Status = SagaCompensating

	for i := len(completedSteps) - 1; i >= 0; i-- {
		idx := completedSteps[i]
		step := s.steps[idx]

		s.state.UpdateStepState(step.Name(), StepCompensating, nil)

		if err := step.Compensate(ctx, s.state); err != nil {
			s.state.UpdateStepState(step.Name(), StepFailed, err)
			return fmt.Errorf("compensation failed for step %s: %w", step.Name(), err)
		}

		s.state.UpdateStepState(step.Name(), StepCompensated, nil)

		if s.config.OnCompensation != nil {
			s.config.OnCompensation(step.Name(), s.state)
		}
	}

	return nil
}

func (s *Saga) State() *SagaState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Saga) ID() string {
	return s.id
}

type SagaOrchestrator struct {
	sagas  map[string]*Saga
	mu     sync.RWMutex
	config SagaConfig
}

func NewSagaOrchestrator(config ...SagaConfig) *SagaOrchestrator {
	cfg := DefaultSagaConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &SagaOrchestrator{
		sagas:  make(map[string]*Saga),
		config: cfg,
	}
}

func (o *SagaOrchestrator) CreateSaga(id string) *Saga {
	o.mu.Lock()
	defer o.mu.Unlock()
	saga := NewSaga(id, o.config)
	o.sagas[id] = saga
	return saga
}

func (o *SagaOrchestrator) GetSaga(id string) (*Saga, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	s, ok := o.sagas[id]
	return s, ok
}

func (o *SagaOrchestrator) ExecuteSaga(ctx context.Context, id string) error {
	saga, ok := o.GetSaga(id)
	if !ok {
		return fmt.Errorf("saga %s not found", id)
	}
	return saga.Execute(ctx)
}

func (o *SagaOrchestrator) SagaStatus(id string) (SagaStatus, error) {
	saga, ok := o.GetSaga(id)
	if !ok {
		return "", fmt.Errorf("saga %s not found", id)
	}
	return saga.State().Status, nil
}

type ChoreographyStep interface {
	Name() string
	Topic() string
	CanHandle(event string) bool
	Handle(ctx context.Context, event *Event) (*Event, error)
}

type Event struct {
	ID        string
	Topic     string
	Type      string
	Payload   map[string]interface{}
	Timestamp time.Time
	Metadata  map[string]string
}

func NewEvent(id, eventType string, payload map[string]interface{}) *Event {
	return &Event{
		ID:        id,
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
}

type ChoreographySaga struct {
	id      string
	steps   map[string]ChoreographyStep
	eventCh chan *Event
	status  SagaStatus
	state   *SagaState
	mu      sync.RWMutex
}

func NewChoreographySaga(id string) *ChoreographySaga {
	return &ChoreographySaga{
		id:      id,
		steps:   make(map[string]ChoreographyStep),
		eventCh: make(chan *Event, 100),
		status:  SagaPending,
		state:   NewSagaState(id),
	}
}

func (cs *ChoreographySaga) AddStep(step ChoreographyStep) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.steps[step.Name()] = step
}

func (cs *ChoreographySaga) Start(ctx context.Context, initialEvent *Event) error {
	cs.mu.Lock()
	cs.status = SagaRunning
	cs.mu.Unlock()

	go cs.processEvents(ctx)

	cs.eventCh <- initialEvent

	return nil
}

func (cs *ChoreographySaga) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			cs.mu.Lock()
			cs.status = SagaFailed
			cs.mu.Unlock()
			return
		case event, ok := <-cs.eventCh:
			if !ok {
				cs.mu.Lock()
				cs.status = SagaCompleted
				cs.mu.Unlock()
				return
			}
			cs.processEvent(ctx, event)
		}
	}
}

func (cs *ChoreographySaga) processEvent(ctx context.Context, event *Event) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, step := range cs.steps {
		if step.CanHandle(event.Type) {
			result, err := step.Handle(ctx, event)
			if err != nil {
				cs.state.UpdateStepState(step.Name(), StepFailed, err)
				return
			}
			cs.state.UpdateStepState(step.Name(), StepCompleted, nil)
			if result != nil {
				select {
				case cs.eventCh <- result:
				default:
				}
			}
		}
	}
}

func (cs *ChoreographySaga) Status() SagaStatus {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.status
}

type SagaBuilder struct {
	id     string
	steps  []Step
	config SagaConfig
}

func NewSagaBuilder(id string) *SagaBuilder {
	return &SagaBuilder{
		id:     id,
		steps:  make([]Step, 0),
		config: DefaultSagaConfig(),
	}
}

func (b *SagaBuilder) WithMaxRetries(retries int) *SagaBuilder {
	b.config.MaxRetries = retries
	return b
}

func (b *SagaBuilder) WithStepTimeout(timeout time.Duration) *SagaBuilder {
	b.config.StepTimeout = timeout
	return b
}

func (b *SagaBuilder) OnStepComplete(fn func(step string, state *SagaState)) *SagaBuilder {
	b.config.OnStepComplete = fn
	return b
}

func (b *SagaBuilder) OnStepFailed(fn func(step string, state *SagaState, err error)) *SagaBuilder {
	b.config.OnStepFailed = fn
	return b
}

func (b *SagaBuilder) OnCompensation(fn func(step string, state *SagaState)) *SagaBuilder {
	b.config.OnCompensation = fn
	return b
}

func (b *SagaBuilder) OnComplete(fn func(id string, state *SagaState)) *SagaBuilder {
	b.config.OnSagaComplete = fn
	return b
}

func (b *SagaBuilder) OnFailed(fn func(id string, state *SagaState, err error)) *SagaBuilder {
	b.config.OnSagaFailed = fn
	return b
}

func (b *SagaBuilder) AddStep(step Step) *SagaBuilder {
	b.steps = append(b.steps, step)
	return b
}

func (b *SagaBuilder) AddStepFunc(name string, execute, compensate func(ctx context.Context, state *SagaState) error) *SagaBuilder {
	step := NewStepFunc(name, execute, compensate)
	b.steps = append(b.steps, step)
	return b
}

func (b *SagaBuilder) Build() *Saga {
	saga := NewSaga(b.id, b.config)
	for _, step := range b.steps {
		saga.AddStep(step)
	}
	return saga
}

type SagaEvent struct {
	ID        string
	SagaID    string
	StepName  string
	Status    StepStatus
	Error     error
	Timestamp time.Time
}

type SagaHistory struct {
	events []SagaEvent
	mu     sync.RWMutex
}

func NewSagaHistory() *SagaHistory {
	return &SagaHistory{events: make([]SagaEvent, 0)}
}

func (h *SagaHistory) Record(event SagaEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	event.Timestamp = time.Now()
	h.events = append(h.events, event)
}

func (h *SagaHistory) Events(sagaID string) []SagaEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []SagaEvent
	for _, e := range h.events {
		if e.SagaID == sagaID {
			result = append(result, e)
		}
	}
	return result
}

func (h *SagaHistory) RecentEvents(n int) []SagaEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n > len(h.events) {
		n = len(h.events)
	}
	result := make([]SagaEvent, n)
	copy(result, h.events[len(h.events)-n:])
	return result
}

type SagaMonitor struct {
	history  *SagaHistory
	running  map[string]*Saga
	completed map[string]*Saga
	mu       sync.RWMutex
}

func NewSagaMonitor() *SagaMonitor {
	return &SagaMonitor{
		history:   NewSagaHistory(),
		running:   make(map[string]*Saga),
		completed: make(map[string]*Saga),
	}
}

func (m *SagaMonitor) Track(saga *Saga) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running[saga.ID()] = saga
}

func (m *SagaMonitor) Complete(sagaID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if saga, ok := m.running[sagaID]; ok {
		m.completed[sagaID] = saga
		delete(m.running, sagaID)
	}
}

func (m *SagaMonitor) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.running)
}

func (m *SagaMonitor) CompletedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.completed)
}

type SagaMetrics struct {
	TotalExecutions   int64
	SuccessfulSagas   int64
	FailedSagas       int64
	CompensatedSagas  int64
	AvgExecutionTime  time.Duration
	StepMetrics       map[string]*StepMetrics
	mu                sync.RWMutex
}

type StepMetrics struct {
	Executions  int64
	Failures    int64
	Retries     int64
	AvgDuration time.Duration
}

func NewSagaMetrics() *SagaMetrics {
	return &SagaMetrics{
		StepMetrics: make(map[string]*StepMetrics),
	}
}

func (m *SagaMetrics) RecordExecution(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalExecutions++
	m.AvgExecutionTime = (m.AvgExecutionTime * time.Duration(m.TotalExecutions-1) + duration) / time.Duration(m.TotalExecutions)
}

func (m *SagaMetrics) RecordSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuccessfulSagas++
}

func (m *SagaMetrics) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedSagas++
}

func (m *SagaMetrics) RecordCompensation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CompensatedSagas++
}

func (m *SagaMetrics) RecordStepExecution(stepName string, duration time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sm, ok := m.StepMetrics[stepName]
	if !ok {
		sm = &StepMetrics{}
		m.StepMetrics[stepName] = sm
	}
	sm.Executions++
	if failed {
		sm.Failures++
	}
	sm.AvgDuration = (sm.AvgDuration * time.Duration(sm.Executions-1) + duration) / time.Duration(sm.Executions)
}

func (m *SagaMetrics) RecordStepRetry(stepName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sm, ok := m.StepMetrics[stepName]
	if ok {
		sm.Retries++
	}
}

type ParallelSaga struct {
	*Saga
	groups [][]Step
	mu     sync.Mutex
}

func NewParallelSaga(id string, config ...SagaConfig) *ParallelSaga {
	return &ParallelSaga{
		Saga:   NewSaga(id, config...),
		groups: make([][]Step, 0),
	}
}

func (ps *ParallelSaga) AddParallelGroup(steps ...Step) *ParallelSaga {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.groups = append(ps.groups, steps)
	return ps
}

func (ps *ParallelSaga) Execute(ctx context.Context) error {
	for _, group := range ps.groups {
		errCh := make(chan error, len(group))
		for _, step := range group {
			go func(s Step) {
				errCh <- s.Execute(ctx, ps.State())
			}(step)
		}

		for range group {
			if err := <-errCh; err != nil {
				return fmt.Errorf("parallel step failed: %w", err)
			}
		}
	}
	return nil
}

type ConditionalStep struct {
	name      string
	condition func(state *SagaState) bool
	step      Step
}

func NewConditionalStep(name string, condition func(state *SagaState) bool, step Step) *ConditionalStep {
	return &ConditionalStep{
		name:      name,
		condition: condition,
		step:      step,
	}
}

func (cs *ConditionalStep) Name() string { return cs.name }

func (cs *ConditionalStep) Execute(ctx context.Context, state *SagaState) error {
	if cs.condition(state) {
		return cs.step.Execute(ctx, state)
	}
	state.UpdateStepState(cs.name, StepSkipped, nil)
	return nil
}

func (cs *ConditionalStep) Compensate(ctx context.Context, state *SagaState) error {
	if cs.condition(state) {
		return cs.step.Compensate(ctx, state)
	}
	return nil
}

type TimeoutStep struct {
	name     string
	inner    Step
	timeout  time.Duration
}

func NewTimeoutStep(name string, inner Step, timeout time.Duration) *TimeoutStep {
	return &TimeoutStep{name: name, inner: inner, timeout: timeout}
}

func (ts *TimeoutStep) Name() string { return ts.name }

func (ts *TimeoutStep) Execute(ctx context.Context, state *SagaState) error {
	ctx, cancel := context.WithTimeout(ctx, ts.timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ts.inner.Execute(ctx, state)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("step %s timed out after %v", ts.name, ts.timeout)
	case err := <-done:
		return err
	}
}

func (ts *TimeoutStep) Compensate(ctx context.Context, state *SagaState) error {
	return ts.inner.Compensate(ctx, state)
}

type CompensatingSagaError struct {
	OriginalError error
	CompErr       error
	StepName      string
}

func (e *CompensatingSagaError) Error() string {
	return fmt.Sprintf("step %s failed: %v, compensation: %v", e.StepName, e.OriginalError, e.CompErr)
}

func (e *CompensatingSagaError) Unwrap() error {
	return e.OriginalError
}
