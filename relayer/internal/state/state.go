package state

import (
	"fmt"
	"sync"
	"time"
)

type State[T any] interface {
	Enter(data T)
	Exit(data T)
	Update(data T) (State[T], error)
	Name() string
}

type StateMachine[T any] struct {
	current    State[T]
	states     map[string]State[T]
	transitions map[string][]string
	history    []StateTransition
	listeners  []func(from, to string, data T)
	mu         sync.RWMutex
}

type StateTransition struct {
	From      string
	To        string
	Timestamp time.Time
	Data      interface{}
}

func NewStateMachine[T any](initial State[T]) *StateMachine[T] {
	sm := &StateMachine[T]{
		states:      make(map[string]State[T]),
		transitions: make(map[string][]string),
		history:     make([]StateTransition, 0),
		listeners:   make([]func(from, to string, data T), 0),
	}
	if initial != nil {
		sm.current = initial
		sm.states[initial.Name()] = initial
	}
	return sm
}

func (sm *StateMachine[T]) AddState(state State[T]) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[state.Name()] = state
}

func (sm *StateMachine[T]) AddTransition(from, to string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transitions[from] = append(sm.transitions[from], to)
}

func (sm *StateMachine[T]) OnTransition(listener func(from, to string, data T)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.listeners = append(sm.listeners, listener)
}

func (sm *StateMachine[T]) Trigger(data T) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current == nil {
		return fmt.Errorf("no initial state set")
	}

	fromName := sm.current.Name()
	newState, err := sm.current.Update(data)
	if err != nil {
		return err
	}

	if newState == nil {
		return nil
	}

	toName := newState.Name()
	allowed := false
	for _, t := range sm.transitions[fromName] {
		if t == toName {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("transition from %s to %s not allowed", fromName, toName)
	}

	sm.current.Exit(data)
	sm.current = newState
	sm.current.Enter(data)

	sm.history = append(sm.history, StateTransition{
		From:      fromName,
		To:        toName,
		Timestamp: time.Now(),
	})

	for _, listener := range sm.listeners {
		listener(fromName, toName, data)
	}

	return nil
}

func (sm *StateMachine[T]) CurrentState() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.current == nil {
		return ""
	}
	return sm.current.Name()
}

func (sm *StateMachine[T]) History() []StateTransition {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]StateTransition, len(sm.history))
	copy(result, sm.history)
	return result
}

type StateManager[T any] struct {
	machines map[string]*StateMachine[T]
	mu       sync.RWMutex
}

func NewStateManager[T any]() *StateManager[T] {
	return &StateManager[T]{
		machines: make(map[string]*StateMachine[T]),
	}
}

func (sm *StateManager[T]) Create(id string, initial State[T]) *StateMachine[T] {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	machine := NewStateMachine(initial)
	sm.machines[id] = machine
	return machine
}

func (sm *StateManager[T]) Get(id string) *StateMachine[T] {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.machines[id]
}

func (sm *StateManager[T]) Remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.machines, id)
}

func (sm *StateManager[T]) List() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	ids := make([]string, 0, len(sm.machines))
	for id := range sm.machines {
		ids = append(ids, id)
	}
	return ids
}

type StatePersistence[T any] struct {
	storage map[string]StateSnapshot
	mu      sync.RWMutex
}

type StateSnapshot struct {
	StateName string
	Timestamp time.Time
	Data      []byte
}

func NewStatePersistence[T any]() *StatePersistence[T] {
	return &StatePersistence[T]{
		storage: make(map[string]StateSnapshot),
	}
}

func (sp *StatePersistence[T]) Save(id string, stateName string, data []byte) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.storage[id] = StateSnapshot{
		StateName: stateName,
		Timestamp: time.Now(),
		Data:      data,
	}
}

func (sp *StatePersistence[T]) Load(id string) (StateSnapshot, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	snap, exists := sp.storage[id]
	return snap, exists
}

func (sp *StatePersistence[T]) Delete(id string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	delete(sp.storage, id)
}

func (sp *StatePersistence[T]) List() map[string]StateSnapshot {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	result := make(map[string]StateSnapshot, len(sp.storage))
	for k, v := range sp.storage {
		result[k] = v
	}
	return result
}

type StateGuard[T any] struct {
	machine   *StateMachine[T]
	guardFunc func(from, to string, data T) bool
	mu        sync.RWMutex
}

func NewStateGuard[T any](machine *StateMachine[T], guard func(from, to string, data T) bool) *StateGuard[T] {
	return &StateGuard[T]{
		machine:   machine,
		guardFunc: guard,
	}
}

func (sg *StateGuard[T]) CanTransition(to string, data T) bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	current := sg.machine.CurrentState()
	return sg.guardFunc(current, to, data)
}

type StateHook[T any] struct {
	onEnter func(state string, data T)
	onExit  func(state string, data T)
	mu      sync.RWMutex
}

func NewStateHook[T any](onEnter, onExit func(string, T)) *StateHook[T] {
	return &StateHook[T]{
		onEnter: onEnter,
		onExit:  onExit,
	}
}

func (sh *StateHook[T]) Enter(state string, data T) {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	if sh.onEnter != nil {
		sh.onEnter(state, data)
	}
}

func (sh *StateHook[T]) Exit(state string, data T) {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	if sh.onExit != nil {
		sh.onExit(state, data)
	}
}

type StateValidator[T any] struct {
	validators map[string]func(T) error
	mu         sync.RWMutex
}

func NewStateValidator[T any]() *StateValidator[T] {
	return &StateValidator[T]{
		validators: make(map[string]func(T) error),
	}
}

func (sv *StateValidator[T]) Register(stateName string, validator func(T) error) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.validators[stateName] = validator
}

func (sv *StateValidator[T]) Validate(stateName string, data T) error {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	validator, exists := sv.validators[stateName]
	if !exists {
		return nil
	}
	return validator(data)
}

type StateTimeout[T any] struct {
	machine  *StateMachine[T]
	timeouts map[string]time.Duration
	timers   map[string]*time.Timer
	mu       sync.RWMutex
}

func NewStateTimeout[T any](machine *StateMachine[T]) *StateTimeout[T] {
	return &StateTimeout[T]{
		machine:  machine,
		timeouts: make(map[string]time.Duration),
		timers:   make(map[string]*time.Timer),
	}
}

func (st *StateTimeout[T]) SetTimeout(stateName string, timeout time.Duration) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.timeouts[stateName] = timeout
}

func (st *StateTimeout[T]) Start(id string, data T) {
	st.mu.Lock()
	defer st.mu.Unlock()

	currentState := st.machine.CurrentState()
	timeout, exists := st.timeouts[currentState]
	if !exists {
		return
	}

	if timer, ok := st.timers[id]; ok {
		timer.Stop()
	}

	st.timers[id] = time.AfterFunc(timeout, func() {
		_ = st.machine.Trigger(data)
	})
}

func (st *StateTimeout[T]) Stop(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if timer, ok := st.timers[id]; ok {
		timer.Stop()
		delete(st.timers, id)
	}
}

type CompositeState[T any] struct {
	name      string
	subStates map[string]State[T]
	current   string
	mu        sync.RWMutex
}

func NewCompositeState[T any](name string) *CompositeState[T] {
	return &CompositeState[T]{
		name:      name,
		subStates: make(map[string]State[T]),
	}
}

func (cs *CompositeState[T]) AddSubState(state State[T]) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.subStates[state.Name()] = state
}

func (cs *CompositeState[T]) SetCurrent(subStateName string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.current = subStateName
}

func (cs *CompositeState[T]) Enter(data T) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if s, ok := cs.subStates[cs.current]; ok {
		s.Enter(data)
	}
}

func (cs *CompositeState[T]) Exit(data T) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if s, ok := cs.subStates[cs.current]; ok {
		s.Exit(data)
	}
}

func (cs *CompositeState[T]) Update(data T) (State[T], error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if s, ok := cs.subStates[cs.current]; ok {
		return s.Update(data)
	}
	return nil, fmt.Errorf("no current sub-state")
}

func (cs *CompositeState[T]) Name() string { return cs.name }
