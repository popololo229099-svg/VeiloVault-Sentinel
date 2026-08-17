package state

import (
	"fmt"
	"sync"
	"time"
)

type StateCondition[T any] interface {
	Evaluate(data T) bool
	Name() string
}

type CompositeCondition[T any] struct {
	conditions []StateCondition[T]
	operator   string
	mu         sync.RWMutex
}

func NewCompositeCondition[T any](operator string, conditions ...StateCondition[T]) *CompositeCondition[T] {
	return &CompositeCondition[T]{
		conditions: conditions,
		operator:   operator,
	}
}

func (cc *CompositeCondition[T]) Evaluate(data T) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	switch cc.operator {
	case "and":
		for _, c := range cc.conditions {
			if !c.Evaluate(data) {
				return false
			}
		}
		return true
	case "or":
		for _, c := range cc.conditions {
			if c.Evaluate(data) {
				return true
			}
		}
		return false
	case "not":
		for _, c := range cc.conditions {
			if c.Evaluate(data) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (cc *CompositeCondition[T]) Name() string { return "composite" }

type TimeoutCondition[T any] struct {
	timeout time.Duration
	start   time.Time
	mu      sync.RWMutex
}

func NewTimeoutCondition[T any](timeout time.Duration) *TimeoutCondition[T] {
	return &TimeoutCondition[T]{
		timeout: timeout,
		start:   time.Now(),
	}
}

func (tc *TimeoutCondition[T]) Evaluate(data T) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return time.Since(tc.start) > tc.timeout
}

func (tc *TimeoutCondition[T]) Name() string   { return "timeout" }
func (tc *TimeoutCondition[T]) Reset()         { tc.mu.Lock(); defer tc.mu.Unlock(); tc.start = time.Now() }

type RetryCondition[T any] struct {
	maxRetries int
	attempts   int
	mu         sync.RWMutex
}

func NewRetryCondition[T any](maxRetries int) *RetryCondition[T] {
	return &RetryCondition[T]{maxRetries: maxRetries}
}

func (rc *RetryCondition[T]) Evaluate(data T) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.attempts++
	return rc.attempts <= rc.maxRetries
}

func (rc *RetryCondition[T]) Name() string { return "retry" }
func (rc *RetryCondition[T]) Attempts() int { rc.mu.RLock(); defer rc.mu.RUnlock(); return rc.attempts }
func (rc *RetryCondition[T]) Reset()        { rc.mu.Lock(); defer rc.mu.Unlock(); rc.attempts = 0 }

type StateLogger[T any] struct {
	entries []StateLogEntry
	maxSize int
	mu      sync.RWMutex
}

type StateLogEntry struct {
	From      string
	To        string
	Timestamp time.Time
	Data      string
}

func NewStateLogger[T any](maxSize int) *StateLogger[T] {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &StateLogger[T]{
		entries: make([]StateLogEntry, 0),
		maxSize: maxSize,
	}
}

func (sl *StateLogger[T]) Log(from, to string, data interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if len(sl.entries) >= sl.maxSize {
		sl.entries = sl.entries[1:]
	}
	sl.entries = append(sl.entries, StateLogEntry{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Data:      fmt.Sprintf("%v", data),
	})
}

func (sl *StateLogger[T]) Entries() []StateLogEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	result := make([]StateLogEntry, len(sl.entries))
	copy(result, sl.entries)
	return result
}

func (sl *StateLogger[T]) Clear() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.entries = sl.entries[:0]
}

type StateMetrics[T any] struct {
	transitions map[string]int64
	errors      map[string]int64
	durations   map[string]time.Duration
	mu          sync.RWMutex
}

func NewStateMetrics[T any]() *StateMetrics[T] {
	return &StateMetrics[T]{
		transitions: make(map[string]int64),
		errors:      make(map[string]int64),
		durations:   make(map[string]time.Duration),
	}
}

func (sm *StateMetrics[T]) RecordTransition(from, to string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	key := from + "->" + to
	sm.transitions[key]++
	sm.durations[key] += duration
}

func (sm *StateMetrics[T]) RecordError(state string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.errors[state]++
}

func (sm *StateMetrics[T]) GetTransitions() map[string]int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]int64, len(sm.transitions))
	for k, v := range sm.transitions {
		result[k] = v
	}
	return result
}

func (sm *StateMetrics[T]) GetErrors() map[string]int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]int64, len(sm.errors))
	for k, v := range sm.errors {
		result[k] = v
	}
	return result
}

func (sm *StateMetrics[T]) GetDurations() map[string]time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]time.Duration, len(sm.durations))
	for k, v := range sm.durations {
		result[k] = v
	}
	return result
}

func (sm *StateMetrics[T]) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transitions = make(map[string]int64)
	sm.errors = make(map[string]int64)
	sm.durations = make(map[string]time.Duration)
}

type StateMachineBuilder[T any] struct {
	states      map[string]State[T]
	transitions map[string][]string
	initial     State[T]
	mu          sync.RWMutex
}

func NewStateMachineBuilder[T any]() *StateMachineBuilder[T] {
	return &StateMachineBuilder[T]{
		states:      make(map[string]State[T]),
		transitions: make(map[string][]string),
	}
}

func (smb *StateMachineBuilder[T]) AddState(state State[T]) *StateMachineBuilder[T] {
	smb.mu.Lock()
	defer smb.mu.Unlock()
	smb.states[state.Name()] = state
	return smb
}

func (smb *StateMachineBuilder[T]) AddTransition(from, to string) *StateMachineBuilder[T] {
	smb.mu.Lock()
	defer smb.mu.Unlock()
	smb.transitions[from] = append(smb.transitions[from], to)
	return smb
}

func (smb *StateMachineBuilder[T]) SetInitial(state State[T]) *StateMachineBuilder[T] {
	smb.mu.Lock()
	defer smb.mu.Unlock()
	smb.initial = state
	return smb
}

func (smb *StateMachineBuilder[T]) Build() *StateMachine[T] {
	smb.mu.RLock()
	defer smb.mu.RUnlock()

	sm := NewStateMachine(smb.initial)
	for name, state := range smb.states {
		sm.AddState(state)
		_ = name
	}
	for from, tos := range smb.transitions {
		for _, to := range tos {
			sm.AddTransition(from, to)
		}
	}
	return sm
}

type StateChart[T any] struct {
	states     map[string]*StateNode[T]
	transitions []*StateTransitionDef[T]
	mu         sync.RWMutex
}

type StateNode[T any] struct {
	Name       string
	Entry      func(T)
	Exit       func(T)
	Children   []string
	IsParallel bool
}

type StateTransitionDef[T any] struct {
	From      string
	To        string
	Guard     func(T) bool
	Action    func(T)
	EventType string
}

func NewStateChart[T any]() *StateChart[T] {
	return &StateChart[T]{
		states:      make(map[string]*StateNode[T]),
		transitions: make([]*StateTransitionDef[T], 0),
	}
}

func (sc *StateChart[T]) AddState(node *StateNode[T]) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.states[node.Name] = node
}

func (sc *StateChart[T]) AddTransition(from, to string, guard func(T) bool, action func(T)) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.transitions = append(sc.transitions, &StateTransitionDef[T]{
		From:   from,
		To:     to,
		Guard:  guard,
		Action: action,
	})
}

func (sc *StateChart[T]) GetState(name string) *StateNode[T] {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.states[name]
}

func (sc *StateChart[T]) GetTransitions(from string) []*StateTransitionDef[T] {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	var result []*StateTransitionDef[T]
	for _, t := range sc.transitions {
		if t.From == from {
			result = append(result, t)
		}
	}
	return result
}

type StateTimer[T any] struct {
	timers map[string]*time.Timer
	mu     sync.RWMutex
}

func NewStateTimer[T any]() *StateTimer[T] {
	return &StateTimer[T]{
		timers: make(map[string]*time.Timer),
	}
}

func (st *StateTimer[T]) Start(stateName string, duration time.Duration, callback func()) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if timer, ok := st.timers[stateName]; ok {
		timer.Stop()
	}

	st.timers[stateName] = time.AfterFunc(duration, callback)
}

func (st *StateTimer[T]) Stop(stateName string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if timer, ok := st.timers[stateName]; ok {
		timer.Stop()
		delete(st.timers, stateName)
	}
}

func (st *StateTimer[T]) StopAll() {
	st.mu.Lock()
	defer st.mu.Unlock()
	for name, timer := range st.timers {
		timer.Stop()
		delete(st.timers, name)
	}
}

type StateTransitionValidator[T any] struct {
	validators map[string]func(T) error
	mu         sync.RWMutex
}

func NewStateTransitionValidator[T any]() *StateTransitionValidator[T] {
	return &StateTransitionValidator[T]{
		validators: make(map[string]func(T) error),
	}
}

func (stv *StateTransitionValidator[T]) Register(stateName string, validator func(T) error) {
	stv.mu.Lock()
	defer stv.mu.Unlock()
	stv.validators[stateName] = validator
}

func (stv *StateTransitionValidator[T]) Validate(stateName string, data T) error {
	stv.mu.RLock()
	defer stv.mu.RUnlock()
	if validator, ok := stv.validators[stateName]; ok {
		return validator(data)
	}
	return nil
}

type StateSerializer[T any] struct {
	serializers map[string]func(T) ([]byte, error)
	deserializers map[string]func([]byte) (T, error)
	mu sync.RWMutex
}

func NewStateSerializer[T any]() *StateSerializer[T] {
	return &StateSerializer[T]{
		serializers:   make(map[string]func(T) ([]byte, error)),
		deserializers: make(map[string]func([]byte) (T, error)),
	}
}

func (ss *StateSerializer[T]) RegisterSerializer(stateName string, fn func(T) ([]byte, error)) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.serializers[stateName] = fn
}

func (ss *StateSerializer[T]) RegisterDeserializer(stateName string, fn func([]byte) (T, error)) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.deserializers[stateName] = fn
}

func (ss *StateSerializer[T]) Serialize(stateName string, data T) ([]byte, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	fn, ok := ss.serializers[stateName]
	if !ok {
		return nil, fmt.Errorf("no serializer for state: %s", stateName)
	}
	return fn(data)
}

func (ss *StateSerializer[T]) Deserialize(stateName string, data []byte) (T, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	fn, ok := ss.deserializers[stateName]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no deserializer for state: %s", stateName)
	}
	return fn(data)
}
