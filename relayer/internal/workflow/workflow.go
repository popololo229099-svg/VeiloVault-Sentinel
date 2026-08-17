package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ActivityStatus string

const (
	ActivityPending   ActivityStatus = "pending"
	ActivityRunning   ActivityStatus = "running"
	ActivityCompleted ActivityStatus = "completed"
	ActivityFailed    ActivityStatus = "failed"
	ActivitySkipped   ActivityStatus = "skipped"
	ActivityRetrying  ActivityStatus = "retrying"
)

type WorkflowStatus string

const (
	WorkflowPending      WorkflowStatus = "pending"
	WorkflowRunning      WorkflowStatus = "running"
	WorkflowCompleted    WorkflowStatus = "completed"
	WorkflowFailed       WorkflowStatus = "failed"
	WorkflowSuspended    WorkflowStatus = "suspended"
	WorkflowCancelled    WorkflowStatus = "cancelled"
)

type Activity interface {
	Name() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Rollback(ctx context.Context, state map[string]interface{}) error
}

type ActivityFunc struct {
	name         string
	executeFn    func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	rollbackFn   func(ctx context.Context, state map[string]interface{}) error
	timeout      time.Duration
	maxRetries   int
	retryDelay   time.Duration
	retryBackoff float64
}

func NewActivityFunc(name string, execute func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)) *ActivityFunc {
	return &ActivityFunc{
		name:         name,
		executeFn:    execute,
		timeout:      30 * time.Second,
		maxRetries:   3,
		retryDelay:   time.Second,
		retryBackoff: 2.0,
	}
}

func (a *ActivityFunc) WithTimeout(d time.Duration) *ActivityFunc {
	a.timeout = d
	return a
}

func (a *ActivityFunc) WithRetry(max int, delay time.Duration) *ActivityFunc {
	a.maxRetries = max
	a.retryDelay = delay
	return a
}

func (a *ActivityFunc) WithRollback(fn func(ctx context.Context, state map[string]interface{}) error) *ActivityFunc {
	a.rollbackFn = fn
	return a
}

func (a *ActivityFunc) Name() string { return a.name }

func (a *ActivityFunc) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return a.executeFn(ctx, input)
}

func (a *ActivityFunc) Rollback(ctx context.Context, state map[string]interface{}) error {
	if a.rollbackFn != nil {
		return a.rollbackFn(ctx, state)
	}
	return nil
}

type ActivityExecution struct {
	ActivityName string
	Status       ActivityStatus
	Input        map[string]interface{}
	Output       map[string]interface{}
	Error        error
	StartedAt    time.Time
	FinishedAt   time.Time
	Retries      int
}

type Workflow struct {
	ID          string
	Name        string
	Status      WorkflowStatus
	Activities  []Activity
	State       map[string]interface{}
	Executions  []*ActivityExecution
	Error       error
	CreatedAt   time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	mu          sync.RWMutex
}

func NewWorkflow(id, name string) *Workflow {
	return &Workflow{
		ID:         id,
		Name:       name,
		Status:     WorkflowPending,
		Activities: make([]Activity, 0),
		State:      make(map[string]interface{}),
		Executions: make([]*ActivityExecution, 0),
		CreatedAt:  time.Now(),
	}
}

func (w *Workflow) AddActivity(activity Activity) *Workflow {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Activities = append(w.Activities, activity)
	return w
}

func (w *Workflow) AddActivityFunc(name string, fn func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)) *Workflow {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Activities = append(w.Activities, NewActivityFunc(name, fn))
	return w
}

func (w *Workflow) SetState(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.State[key] = value
}

func (w *Workflow) GetState(key string) (interface{}, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	val, ok := w.State[key]
	return val, ok
}

type WorkflowEngine struct {
	workflows map[string]*Workflow
	config    WorkflowConfig
	mu        sync.RWMutex
}

type WorkflowConfig struct {
	MaxConcurrent   int
	DefaultTimeout  time.Duration
	EnableTracing   bool
	OnActivityStart func(workflowID, activityName string)
	OnActivityEnd   func(workflowID, activityName string, err error)
	OnWorkflowStart func(workflowID string)
	OnWorkflowEnd   func(workflowID string, err error)
}

func DefaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		MaxConcurrent:  10,
		DefaultTimeout: 5 * time.Minute,
		EnableTracing:  true,
	}
}

func NewWorkflowEngine(config ...WorkflowConfig) *WorkflowEngine {
	cfg := DefaultWorkflowConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &WorkflowEngine{
		workflows: make(map[string]*Workflow),
		config:    cfg,
	}
}

func (e *WorkflowEngine) Register(workflow *Workflow) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.ID] = workflow
}

func (e *WorkflowEngine) Execute(ctx context.Context, workflowID string) error {
	e.mu.RLock()
	workflow, ok := e.workflows[workflowID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow %s not found", workflowID)
	}

	workflow.mu.Lock()
	workflow.Status = WorkflowRunning
	workflow.StartedAt = time.Now()
	workflow.mu.Unlock()

	if e.config.OnWorkflowStart != nil {
		e.config.OnWorkflowStart(workflow.ID)
	}

	for _, activity := range workflow.Activities {
		select {
		case <-ctx.Done():
			workflow.mu.Lock()
			workflow.Status = WorkflowCancelled
			workflow.Error = ctx.Err()
			workflow.mu.Unlock()
			return ctx.Err()
		default:
		}

		exec := &ActivityExecution{
			ActivityName: activity.Name(),
			Status:       ActivityRunning,
			StartedAt:    time.Now(),
		}
		input := make(map[string]interface{})
		for k, v := range workflow.State {
			input[k] = v
		}
		exec.Input = input

		workflow.mu.Lock()
		workflow.Executions = append(workflow.Executions, exec)
		workflow.mu.Unlock()

		if e.config.OnActivityStart != nil {
			e.config.OnActivityStart(workflow.ID, activity.Name())
		}

		af, _ := activity.(*ActivityFunc)
		maxRetries := 3
		retryDelay := time.Second
		if af != nil {
			maxRetries = af.maxRetries
			retryDelay = af.retryDelay
		}

		var output map[string]interface{}
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			activityCtx := ctx
			if af != nil && af.timeout > 0 {
				var cancel context.CancelFunc
				activityCtx, cancel = context.WithTimeout(ctx, af.timeout)
				defer cancel()
			}

			output, err = activity.Execute(activityCtx, input)
			if err == nil {
				break
			}
			exec.Retries = attempt
			if attempt < maxRetries {
				delay := retryDelay
				if af != nil && af.retryBackoff > 0 {
					backoff := float64(retryDelay)
					for i := 0; i < attempt; i++ {
						backoff *= af.retryBackoff
					}
					delay = time.Duration(backoff)
				}
				select {
				case <-ctx.Done():
					err = ctx.Err()
					goto done
				case <-time.After(delay):
				}
				exec.Status = ActivityRetrying
			}
		}
	done:

		exec.FinishedAt = time.Now()
		if err != nil {
			exec.Status = ActivityFailed
			exec.Error = err
			workflow.mu.Lock()
			workflow.Status = WorkflowFailed
			workflow.Error = err
			workflow.mu.Unlock()

			if e.config.OnActivityEnd != nil {
				e.config.OnActivityEnd(workflow.ID, activity.Name(), err)
			}
			if e.config.OnWorkflowEnd != nil {
				e.config.OnWorkflowEnd(workflow.ID, err)
			}
			return fmt.Errorf("activity %s failed: %w", activity.Name(), err)
		}

		exec.Status = ActivityCompleted
		exec.Output = output
		for k, v := range output {
			workflow.State[k] = v
		}

		if e.config.OnActivityEnd != nil {
			e.config.OnActivityEnd(workflow.ID, activity.Name(), nil)
		}
	}

	workflow.mu.Lock()
	workflow.Status = WorkflowCompleted
	workflow.FinishedAt = time.Now()
	workflow.mu.Unlock()

	if e.config.OnWorkflowEnd != nil {
		e.config.OnWorkflowEnd(workflow.ID, nil)
	}

	return nil
}

func (e *WorkflowEngine) GetWorkflow(id string) (*Workflow, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	w, ok := e.workflows[id]
	return w, ok
}

func (e *WorkflowEngine) Cancel(id string) error {
	e.mu.RLock()
	workflow, ok := e.workflows[id]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	workflow.mu.Lock()
	defer workflow.mu.Unlock()
	workflow.Status = WorkflowCancelled
	return nil
}

type WorkflowBuilder struct {
	id       string
	name     string
	activities []Activity
	config   WorkflowConfig
}

func NewWorkflowBuilder(id, name string) *WorkflowBuilder {
	return &WorkflowBuilder{
		id:         id,
		name:       name,
		activities: make([]Activity, 0),
		config:     DefaultWorkflowConfig(),
	}
}

func (b *WorkflowBuilder) WithConfig(config WorkflowConfig) *WorkflowBuilder {
	b.config = config
	return b
}

func (b *WorkflowBuilder) AddActivity(activity Activity) *WorkflowBuilder {
	b.activities = append(b.activities, activity)
	return b
}

func (b *WorkflowBuilder) AddStep(name string, fn func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)) *WorkflowBuilder {
	b.activities = append(b.activities, NewActivityFunc(name, fn))
	return b
}

func (b *WorkflowBuilder) Build() *Workflow {
	w := NewWorkflow(b.id, b.name)
	for _, a := range b.activities {
		w.AddActivity(a)
	}
	return w
}

type WorkflowScheduler struct {
	engine    *WorkflowEngine
	schedules map[string]*Schedule
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

type Schedule struct {
	WorkflowID string
	Interval   time.Duration
	Enabled    bool
	LastRun    time.Time
	NextRun    time.Time
}

func NewWorkflowScheduler(engine *WorkflowEngine) *WorkflowScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkflowScheduler{
		engine:    engine,
		schedules: make(map[string]*Schedule),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *WorkflowScheduler) Schedule(workflowID string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedules[workflowID] = &Schedule{
		WorkflowID: workflowID,
		Interval:   interval,
		Enabled:    true,
		NextRun:    time.Now().Add(interval),
	}
}

func (s *WorkflowScheduler) Start() {
	s.wg.Add(1)
	go s.run()
}

func (s *WorkflowScheduler) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *WorkflowScheduler) run() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkSchedules()
		}
	}
}

func (s *WorkflowScheduler) checkSchedules() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	for _, sched := range s.schedules {
		if sched.Enabled && now.After(sched.NextRun) {
			go func(wfID string) {
				_ = s.engine.Execute(context.Background(), wfID)
			}(sched.WorkflowID)
			sched.LastRun = now
			sched.NextRun = now.Add(sched.Interval)
		}
	}
}

func (s *WorkflowScheduler) Enable(workflowID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sched, ok := s.schedules[workflowID]; ok {
		sched.Enabled = true
		sched.NextRun = time.Now().Add(sched.Interval)
	}
}

func (s *WorkflowScheduler) Disable(workflowID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sched, ok := s.schedules[workflowID]; ok {
		sched.Enabled = false
	}
}

type WorkflowMetrics struct {
	TotalExecutions    int64
	SuccessfulRuns     int64
	FailedRuns         int64
	AvgExecutionTime   time.Duration
	ActivityMetrics    map[string]*ActivityMetrics
	mu                 sync.RWMutex
}

type ActivityMetrics struct {
	Executions  int64
	Failures    int64
	Retries     int64
	AvgDuration time.Duration
}

func NewWorkflowMetrics() *WorkflowMetrics {
	return &WorkflowMetrics{
		ActivityMetrics: make(map[string]*ActivityMetrics),
	}
}

func (m *WorkflowMetrics) RecordExecution(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalExecutions++
	m.AvgExecutionTime = (m.AvgExecutionTime * time.Duration(m.TotalExecutions-1) + duration) / time.Duration(m.TotalExecutions)
}

func (m *WorkflowMetrics) RecordSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuccessfulRuns++
}

func (m *WorkflowMetrics) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedRuns++
}

func (m *WorkflowMetrics) RecordActivity(name string, duration time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	am, ok := m.ActivityMetrics[name]
	if !ok {
		am = &ActivityMetrics{}
		m.ActivityMetrics[name] = am
	}
	am.Executions++
	if failed {
		am.Failures++
	}
	am.AvgDuration = (am.AvgDuration * time.Duration(am.Executions-1) + duration) / time.Duration(am.Executions)
}

type WorkflowHistory struct {
	entries []HistoryEntry
	maxSize int
	mu      sync.Mutex
}

type HistoryEntry struct {
	WorkflowID string
	Status     WorkflowStatus
	Error      error
	StartedAt  time.Time
	FinishedAt time.Time
}

func NewWorkflowHistory(maxSize int) *WorkflowHistory {
	return &WorkflowHistory{
		entries: make([]HistoryEntry, 0),
		maxSize: maxSize,
	}
}

func (h *WorkflowHistory) Record(entry HistoryEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= h.maxSize {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)
}

func (h *WorkflowHistory) Recent(n int) []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n > len(h.entries) {
		n = len(h.entries)
	}
	result := make([]HistoryEntry, n)
	copy(result, h.entries[len(h.entries)-n:])
	return result
}

func (h *WorkflowHistory) ByWorkflow(workflowID string) []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	var result []HistoryEntry
	for _, e := range h.entries {
		if e.WorkflowID == workflowID {
			result = append(result, e)
		}
	}
	return result
}

type ConditionalActivity struct {
	name      string
	condition func(state map[string]interface{}) bool
	trueAct   Activity
	falseAct  Activity
}

func NewConditionalActivity(name string, condition func(state map[string]interface{}) bool, trueAct, falseAct Activity) *ConditionalActivity {
	return &ConditionalActivity{
		name:      name,
		condition: condition,
		trueAct:   trueAct,
		falseAct:  falseAct,
	}
}

func (ca *ConditionalActivity) Name() string { return ca.name }

func (ca *ConditionalActivity) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if ca.condition(input) {
		if ca.trueAct != nil {
			return ca.trueAct.Execute(ctx, input)
		}
		return input, nil
	}
	if ca.falseAct != nil {
		return ca.falseAct.Execute(ctx, input)
	}
	return input, nil
}

func (ca *ConditionalActivity) Rollback(ctx context.Context, state map[string]interface{}) error {
	if ca.trueAct != nil {
		_ = ca.trueAct.Rollback(ctx, state)
	}
	if ca.falseAct != nil {
		_ = ca.falseAct.Rollback(ctx, state)
	}
	return nil
}

type ParallelActivity struct {
	name       string
	activities []Activity
}

func NewParallelActivity(name string, activities ...Activity) *ParallelActivity {
	return &ParallelActivity{name: name, activities: activities}
}

func (pa *ParallelActivity) Name() string { return pa.name }

func (pa *ParallelActivity) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	errCh := make(chan error, len(pa.activities))
	outputCh := make(chan map[string]interface{}, len(pa.activities))

	for _, activity := range pa.activities {
		go func(a Activity) {
			output, err := a.Execute(ctx, input)
			if err != nil {
				errCh <- err
				return
			}
			outputCh <- output
		}(activity)
	}

	merged := make(map[string]interface{})
	for i := 0; i < len(pa.activities); i++ {
		select {
		case err := <-errCh:
			return nil, err
		case output := <-outputCh:
			for k, v := range output {
				merged[k] = v
			}
		}
	}
	return merged, nil
}

func (pa *ParallelActivity) Rollback(ctx context.Context, state map[string]interface{}) error {
	for _, activity := range pa.activities {
		_ = activity.Rollback(ctx, state)
	}
	return nil
}

type WorkflowVersion struct {
	Version    int
	Workflow   *Workflow
	CreatedAt  time.Time
}

type VersionedWorkflowRegistry struct {
	versions map[string][]WorkflowVersion
	mu       sync.RWMutex
}

func NewVersionedWorkflowRegistry() *VersionedWorkflowRegistry {
	return &VersionedWorkflowRegistry{
		versions: make(map[string][]WorkflowVersion),
	}
}

func (r *VersionedWorkflowRegistry) Register(name string, workflow *Workflow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	versions := r.versions[name]
	version := len(versions) + 1
	r.versions[name] = append(versions, WorkflowVersion{
		Version:   version,
		Workflow:  workflow,
		CreatedAt: time.Now(),
	})
}

func (r *VersionedWorkflowRegistry) GetLatest(name string) (*Workflow, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.versions[name]
	if !ok || len(versions) == 0 {
		return nil, false
	}
	return versions[len(versions)-1].Workflow, true
}

func (r *VersionedWorkflowRegistry) GetVersion(name string, version int) (*Workflow, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.versions[name]
	if !ok {
		return nil, false
	}
	for _, v := range versions {
		if v.Version == version {
			return v.Workflow, true
		}
	}
	return nil, false
}

func (r *VersionedWorkflowRegistry) ListVersions(name string) []int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.versions[name]
	if !ok {
		return nil
	}
	result := make([]int, len(versions))
	for i, v := range versions {
		result[i] = v.Version
	}
	return result
}

type WorkflowValidator struct {
	rules []ValidationRule
}

type ValidationRule func(w *Workflow) error

func NewWorkflowValidator() *WorkflowValidator {
	return &WorkflowValidator{rules: make([]ValidationRule, 0)}
}

func (v *WorkflowValidator) AddRule(rule ValidationRule) *WorkflowValidator {
	v.rules = append(v.rules, rule)
	return v
}

func (v *WorkflowValidator) Validate(w *Workflow) error {
	for _, rule := range v.rules {
		if err := rule(w); err != nil {
			return err
		}
	}
	return nil
}

func MustHaveActivities(w *Workflow) error {
	if len(w.Activities) == 0 {
		return fmt.Errorf("workflow %s must have at least one activity", w.ID)
	}
	return nil
}

func MustHaveUniqueNames(w *Workflow) error {
	names := make(map[string]bool)
	for _, a := range w.Activities {
		if names[a.Name()] {
			return fmt.Errorf("duplicate activity name: %s", a.Name())
		}
		names[a.Name()] = true
	}
	return nil
}
