package data

import (
	"sync"
	"time"
)

type Specification[T any] interface {
	IsSatisfiedBy(T) bool
	And(other Specification[T]) Specification[T]
	Or(other Specification[T]) Specification[T]
	Not() Specification[T]
}

type BaseSpec[T any] struct {
	predicate func(T) bool
}

func NewSpec[T any](predicate func(T) bool) *BaseSpec[T] {
	return &BaseSpec[T]{predicate: predicate}
}

func (s *BaseSpec[T]) IsSatisfiedBy(item T) bool {
	return s.predicate(item)
}

func (s *BaseSpec[T]) And(other Specification[T]) Specification[T] {
	return &BaseSpec[T]{
		predicate: func(item T) bool {
			return s.IsSatisfiedBy(item) && other.IsSatisfiedBy(item)
		},
	}
}

func (s *BaseSpec[T]) Or(other Specification[T]) Specification[T] {
	return &BaseSpec[T]{
		predicate: func(item T) bool {
			return s.IsSatisfiedBy(item) || other.IsSatisfiedBy(item)
		},
	}
}

func (s *BaseSpec[T]) Not() Specification[T] {
	return &BaseSpec[T]{
		predicate: func(item T) bool {
			return !s.IsSatisfiedBy(item)
		},
	}
}

type UnitOfWork struct {
	newObjs      map[string]interface{}
	dirtyObjs    map[string]interface{}
	deletedObjs  map[string]interface{}
	completed    bool
	mu           sync.Mutex
	commitFn     func() error
	rollbackFn   func() error
}

func NewUnitOfWork(commitFn, rollbackFn func() error) *UnitOfWork {
	return &UnitOfWork{
		newObjs:     make(map[string]interface{}),
		dirtyObjs:   make(map[string]interface{}),
		deletedObjs: make(map[string]interface{}),
		commitFn:    commitFn,
		rollbackFn:  rollbackFn,
	}
}

func (uow *UnitOfWork) RegisterNew(key string, obj interface{}) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	uow.newObjs[key] = obj
}

func (uow *UnitOfWork) RegisterDirty(key string, obj interface{}) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	uow.dirtyObjs[key] = obj
}

func (uow *UnitOfWork) RegisterDeleted(key string) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	uow.deletedObjs[key] = true
	delete(uow.newObjs, key)
	delete(uow.dirtyObjs, key)
}

func (uow *UnitOfWork) Commit() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	if uow.completed {
		return nil
	}
	err := uow.commitFn()
	if err != nil && uow.rollbackFn != nil {
		uow.rollbackFn()
	}
	uow.completed = true
	return err
}

func (uow *UnitOfWork) Rollback() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	if uow.rollbackFn != nil {
		return uow.rollbackFn()
	}
	return nil
}

func (uow *UnitOfWork) HasChanges() bool {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	return len(uow.newObjs) > 0 || len(uow.dirtyObjs) > 0 || len(uow.deletedObjs) > 0
}

type EventSourcedEntity struct {
	ID        string
	Events    []Event
	Version   int
}

type Event struct {
	Type      string
	Data      interface{}
	Version   int
	Timestamp time.Time
}

type EventStore struct {
	entities map[string]*EventSourcedEntity
	mu       sync.RWMutex
}

func NewEventStore() *EventStore {
	return &EventStore{entities: make(map[string]*EventSourcedEntity)}
}

func (es *EventStore) Append(entityID string, event Event) {
	es.mu.Lock()
	defer es.mu.Unlock()
	entity, ok := es.entities[entityID]
	if !ok {
		entity = &EventSourcedEntity{ID: entityID, Events: make([]Event, 0)}
		es.entities[entityID] = entity
	}
	event.Version = entity.Version + 1
	event.Timestamp = time.Now()
	entity.Events = append(entity.Events, event)
	entity.Version = event.Version
}

func (es *EventStore) GetEvents(entityID string) []Event {
	es.mu.RLock()
	defer es.mu.RUnlock()
	entity, ok := es.entities[entityID]
	if !ok {
		return nil
	}
	result := make([]Event, len(entity.Events))
	copy(result, entity.Events)
	return result
}

func (es *EventStore) GetVersion(entityID string) int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	if entity, ok := es.entities[entityID]; ok {
		return entity.Version
	}
	return 0
}

func (es *EventStore) Replay(entityID string, handler func(Event)) {
	for _, event := range es.GetEvents(entityID) {
		handler(event)
	}
}

type CQRSBus struct {
	commandHandlers map[string]func(interface{}) error
	queryHandlers   map[string]func(interface{}) (interface{}, error)
	mu              sync.RWMutex
}

func NewCQRSBus() *CQRSBus {
	return &CQRSBus{
		commandHandlers: make(map[string]func(interface{}) error),
		queryHandlers:   make(map[string]func(interface{}) (interface{}, error)),
	}
}

func (b *CQRSBus) RegisterCommand(name string, handler func(interface{}) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commandHandlers[name] = handler
}

func (b *CQRSBus) RegisterQuery(name string, handler func(interface{}) (interface{}, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queryHandlers[name] = handler
}

func (b *CQRSBus) ExecuteCommand(name string, cmd interface{}) error {
	b.mu.RLock()
	handler, ok := b.commandHandlers[name]
	b.mu.RUnlock()
	if !ok {
		return ErrHandlerNotFound
	}
	return handler(cmd)
}

func (b *CQRSBus) ExecuteQuery(name string, query interface{}) (interface{}, error) {
	b.mu.RLock()
	handler, ok := b.queryHandlers[name]
	b.mu.RUnlock()
	if !ok {
		return nil, ErrHandlerNotFound
	}
	return handler(query)
}

var ErrHandlerNotFound = &DataError{"handler not found"}

type DataError struct{ msg string }

func (e *DataError) Error() string { return e.msg }
