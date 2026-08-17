package state

import (
	"fmt"
	"sync"
	"time"
)

type StateEvent struct {
	Name      string
	Data      interface{}
	Timestamp time.Time
}

type StateEventListener[T any] interface {
	OnEnter(state string, data T) error
	OnExit(state string, data T) error
	OnTransition(from, to string, data T) error
}

type StateMachineMonitor[T any] struct {
	machine    *StateMachine[T]
	listeners  []StateEventListener[T]
	history    []StateEvent
	maxHistory int
	mu         sync.RWMutex
}

func NewStateMachineMonitor[T any](machine *StateMachine[T], maxHistory int) *StateMachineMonitor[T] {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	return &StateMachineMonitor[T]{
		machine:    machine,
		listeners:  make([]StateEventListener[T], 0),
		history:    make([]StateEvent, 0),
		maxHistory: maxHistory,
	}
}

func (smm *StateMachineMonitor[T]) AddListener(listener StateEventListener[T]) {
	smm.mu.Lock()
	defer smm.mu.Unlock()
	smm.listeners = append(smm.listeners, listener)
}

func (smm *StateMachineMonitor[T]) RecordTransition(from, to string, data T) {
	smm.mu.Lock()
	defer smm.mu.Unlock()

	if len(smm.history) >= smm.maxHistory {
		smm.history = smm.history[1:]
	}

	smm.history = append(smm.history, StateEvent{
		Name:      fmt.Sprintf("%s->%s", from, to),
		Timestamp: time.Now(),
	})

	for _, listener := range smm.listeners {
		_ = listener.OnTransition(from, to, data)
	}
}

func (smm *StateMachineMonitor[T]) GetHistory() []StateEvent {
	smm.mu.RLock()
	defer smm.mu.RUnlock()
	result := make([]StateEvent, len(smm.history))
	copy(result, smm.history)
	return result
}

type StateMachineMiddleware[T any] struct {
	inner     *StateMachine[T]
	preHook   func(from, to string, data T) error
	postHook  func(from, to string, data T) error
	mu        sync.RWMutex
}

func NewStateMachineMiddleware[T any](inner *StateMachine[T]) *StateMachineMiddleware[T] {
	return &StateMachineMiddleware[T]{inner: inner}
}

func (smm *StateMachineMiddleware[T]) SetPreHook(hook func(from, to string, data T) error) {
	smm.mu.Lock()
	defer smm.mu.Unlock()
	smm.preHook = hook
}

func (smm *StateMachineMiddleware[T]) SetPostHook(hook func(from, to string, data T) error) {
	smm.mu.Lock()
	defer smm.mu.Unlock()
	smm.postHook = hook
}

func (smm *StateMachineMiddleware[T]) Trigger(data T) error {
	smm.mu.RLock()
	preHook := smm.preHook
	postHook := smm.postHook
	smm.mu.RUnlock()

	from := smm.inner.CurrentState()

	if preHook != nil {
		if err := preHook(from, "", data); err != nil {
			return fmt.Errorf("pre-hook error: %w", err)
		}
	}

	if err := smm.inner.Trigger(data); err != nil {
		return err
	}

	to := smm.inner.CurrentState()
	if postHook != nil {
		if err := postHook(from, to, data); err != nil {
			return fmt.Errorf("post-hook error: %w", err)
		}
	}

	return nil
}

func (smm *StateMachineMiddleware[T]) CurrentState() string {
	return smm.inner.CurrentState()
}

type StateMachineInspector[T any] struct {
	machine *StateMachine[T]
	mu      sync.RWMutex
}

func NewStateMachineInspector[T any](machine *StateMachine[T]) *StateMachineInspector[T] {
	return &StateMachineInspector[T]{machine: machine}
}

func (smi *StateMachineInspector[T]) CurrentState() string {
	return smi.machine.CurrentState()
}

func (smi *StateMachineInspector[T]) History() []StateTransition {
	return smi.machine.History()
}

func (smi *StateMachineInspector[T]) CanTransition(to string) bool {
	smi.mu.RLock()
	defer smi.mu.RUnlock()
	current := smi.machine.CurrentState()
	for _, t := range smi.machine.transitions[current] {
		if t == to {
			return true
		}
	}
	return false
}

func (smi *StateMachineInspector[T]) AvailableTransitions() []string {
	smi.mu.RLock()
	defer smi.mu.RUnlock()
	current := smi.machine.CurrentState()
	return smi.machine.transitions[current]
}

func (smi *StateMachineInspector[T]) AllStates() []string {
	smi.mu.RLock()
	defer smi.mu.RUnlock()
	states := make([]string, 0, len(smi.machine.states))
	for name := range smi.machine.states {
		states = append(states, name)
	}
	return states
}

type StateTransitionGuard[T any] struct {
	condition func(T) bool
	errorMsg  string
	mu        sync.RWMutex
}

func NewStateTransitionGuard[T any](condition func(T) bool, errorMsg string) *StateTransitionGuard[T] {
	return &StateTransitionGuard[T]{
		condition: condition,
		errorMsg:  errorMsg,
	}
}

func (stg *StateTransitionGuard[T]) Check(data T) error {
	stg.mu.RLock()
	defer stg.mu.RUnlock()
	if !stg.condition(data) {
		return fmt.Errorf("%s", stg.errorMsg)
	}
	return nil
}

type StateMachineFactory[T any] struct {
	definitions map[string]*StateMachineDefinition[T]
	mu          sync.RWMutex
}

type StateMachineDefinition[T any] struct {
	Name        string
	States      []string
	Initial     string
	Transitions map[string][]string
}

func NewStateMachineFactory[T any]() *StateMachineFactory[T] {
	return &StateMachineFactory[T]{
		definitions: make(map[string]*StateMachineDefinition[T]),
	}
}

func (smf *StateMachineFactory[T]) Register(definition *StateMachineDefinition[T]) {
	smf.mu.Lock()
	defer smf.mu.Unlock()
	smf.definitions[definition.Name] = definition
}

func (smf *StateMachineFactory[T]) Create(name string) (*StateMachine[T], error) {
	smf.mu.RLock()
	def, ok := smf.definitions[name]
	smf.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("definition not found: %s", name)
	}

	_ = def
	return NewStateMachine[T](nil), nil
}

func (smf *StateMachineFactory[T]) List() []string {
	smf.mu.RLock()
	defer smf.mu.RUnlock()
	names := make([]string, 0, len(smf.definitions))
	for name := range smf.definitions {
		names = append(names, name)
	}
	return names
}

type StateMachinePool[T any] struct {
	machines []*StateMachine[T]
	current  int
	mu       sync.RWMutex
}

func NewStateMachinePool[T any](machines ...*StateMachine[T]) *StateMachinePool[T] {
	return &StateMachinePool[T]{machines: machines}
}

func (smp *StateMachinePool[T]) Next() *StateMachine[T] {
	smp.mu.Lock()
	defer smp.mu.Unlock()
	if len(smp.machines) == 0 {
		return nil
	}
	m := smp.machines[smp.current]
	smp.current = (smp.current + 1) % len(smp.machines)
	return m
}

func (smp *StateMachinePool[T]) Size() int {
	smp.mu.RLock()
	defer smp.mu.RUnlock()
	return len(smp.machines)
}

type StateMachineReplay[T any] struct {
	machine     *StateMachine[T]
	snapshots   []SnapshotEntry[T]
	mu          sync.RWMutex
}

type SnapshotEntry[T any] struct {
	StateName string
	Data      T
	Timestamp time.Time
}

func NewStateMachineReplay[T any](machine *StateMachine[T]) *StateMachineReplay[T] {
	return &StateMachineReplay[T]{
		machine:   machine,
		snapshots: make([]SnapshotEntry[T], 0),
	}
}

func (smr *StateMachineReplay[T]) Snapshot(data T) {
	smr.mu.Lock()
	defer smr.mu.Unlock()
	smr.snapshots = append(smr.snapshots, SnapshotEntry[T]{
		StateName: smr.machine.CurrentState(),
		Data:      data,
		Timestamp: time.Now(),
	})
}

func (smr *StateMachineReplay[T]) GetSnapshots() []SnapshotEntry[T] {
	smr.mu.RLock()
	defer smr.mu.RUnlock()
	result := make([]SnapshotEntry[T], len(smr.snapshots))
	copy(result, smr.snapshots)
	return result
}

func (smr *StateMachineReplay[T]) ReplayTo(index int) error {
	smr.mu.RLock()
	defer smr.mu.RUnlock()

	if index < 0 || index >= len(smr.snapshots) {
		return fmt.Errorf("invalid snapshot index: %d", index)
	}

	_ = smr.snapshots[index]
	return nil
}

type StateMachineValidator[T any] struct {
	rules []ValidationRule[T]
	mu    sync.RWMutex
}

type ValidationRule[T any] struct {
	From      string
	To        string
	Validator func(T) error
}

func NewStateMachineValidator[T any]() *StateMachineValidator[T] {
	return &StateMachineValidator[T]{
		rules: make([]ValidationRule[T], 0),
	}
}

func (smv *StateMachineValidator[T]) AddRule(rule ValidationRule[T]) {
	smv.mu.Lock()
	defer smv.mu.Unlock()
	smv.rules = append(smv.rules, rule)
}

func (smv *StateMachineValidator[T]) Validate(from, to string, data T) error {
	smv.mu.RLock()
	defer smv.mu.RUnlock()

	for _, rule := range smv.rules {
		if rule.From == from && rule.To == to {
			if err := rule.Validator(data); err != nil {
				return fmt.Errorf("validation failed for %s->%s: %w", from, to, err)
			}
		}
	}
	return nil
}
