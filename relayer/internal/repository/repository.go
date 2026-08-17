package repository

import (
	"fmt"
	"sync"
)

type Entity interface {
	GetID() interface{}
}

type Repository[T Entity] interface {
	FindByID(id interface{}) (T, error)
	FindAll() ([]T, error)
	Save(entity T) error
	Delete(id interface{}) error
	Update(entity T) error
	Count() (int64, error)
	Exists(id interface{}) (bool, error)
}

type InMemoryRepository[T Entity] struct {
	items map[interface{}]T
	mu    sync.RWMutex
}

func NewInMemoryRepository[T Entity]() *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		items: make(map[interface{}]T),
	}
}

func (r *InMemoryRepository[T]) FindByID(id interface{}) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, exists := r.items[id]
	if !exists {
		var zero T
		return zero, fmt.Errorf("entity not found: %v", id)
	}
	return item, nil
}

func (r *InMemoryRepository[T]) FindAll() ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]T, 0, len(r.items))
	for _, item := range r.items {
		result = append(result, item)
	}
	return result, nil
}

func (r *InMemoryRepository[T]) Save(entity T) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[entity.GetID()] = entity
	return nil
}

func (r *InMemoryRepository[T]) Delete(id interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("entity not found: %v", id)
	}
	delete(r.items, id)
	return nil
}

func (r *InMemoryRepository[T]) Update(entity T) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := entity.GetID()
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("entity not found: %v", id)
	}
	r.items[id] = entity
	return nil
}

func (r *InMemoryRepository[T]) Count() (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.items)), nil
}

func (r *InMemoryRepository[T]) Exists(id interface{}) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.items[id]
	return exists, nil
}

type CacheRepository[T Entity] struct {
	inner Repository[T]
	cache map[interface{}]T
	mu    sync.RWMutex
}

func NewCacheRepository[T Entity](inner Repository[T]) *CacheRepository[T] {
	return &CacheRepository[T]{
		inner: inner,
		cache: make(map[interface{}]T),
	}
}

func (cr *CacheRepository[T]) FindByID(id interface{}) (T, error) {
	cr.mu.RLock()
	if item, ok := cr.cache[id]; ok {
		cr.mu.RUnlock()
		return item, nil
	}
	cr.mu.RUnlock()

	item, err := cr.inner.FindByID(id)
	if err != nil {
		return item, err
	}

	cr.mu.Lock()
	cr.cache[id] = item
	cr.mu.Unlock()

	return item, nil
}

func (cr *CacheRepository[T]) FindAll() ([]T, error) {
	return cr.inner.FindAll()
}

func (cr *CacheRepository[T]) Save(entity T) error {
	err := cr.inner.Save(entity)
	if err != nil {
		return err
	}
	cr.mu.Lock()
	cr.cache[entity.GetID()] = entity
	cr.mu.Unlock()
	return nil
}

func (cr *CacheRepository[T]) Delete(id interface{}) error {
	err := cr.inner.Delete(id)
	if err != nil {
		return err
	}
	cr.mu.Lock()
	delete(cr.cache, id)
	cr.mu.Unlock()
	return nil
}

func (cr *CacheRepository[T]) Update(entity T) error {
	err := cr.inner.Update(entity)
	if err != nil {
		return err
	}
	cr.mu.Lock()
	cr.cache[entity.GetID()] = entity
	cr.mu.Unlock()
	return nil
}

func (cr *CacheRepository[T]) Count() (int64, error) {
	return cr.inner.Count()
}

func (cr *CacheRepository[T]) Exists(id interface{}) (bool, error) {
	cr.mu.RLock()
	if _, ok := cr.cache[id]; ok {
		cr.mu.RUnlock()
		return true, nil
	}
	cr.mu.RUnlock()
	return cr.inner.Exists(id)
}

type FilteredRepository[T Entity] struct {
	inner   Repository[T]
	filters []func(T) bool
	mu      sync.RWMutex
}

func NewFilteredRepository[T Entity](inner Repository[T], filters ...func(T) bool) *FilteredRepository[T] {
	return &FilteredRepository[T]{
		inner:   inner,
		filters: filters,
	}
}

func (fr *FilteredRepository[T]) FindByID(id interface{}) (T, error) {
	item, err := fr.inner.FindByID(id)
	if err != nil {
		return item, err
	}
	for _, filter := range fr.filters {
		if !filter(item) {
			var zero T
			return zero, fmt.Errorf("entity filtered out")
		}
	}
	return item, nil
}

func (fr *FilteredRepository[T]) FindAll() ([]T, error) {
	items, err := fr.inner.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]T, 0)
	for _, item := range items {
		matches := true
		for _, filter := range fr.filters {
			if !filter(item) {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, item)
		}
	}
	return result, nil
}

func (fr *FilteredRepository[T]) Save(entity T) error     { return fr.inner.Save(entity) }
func (fr *FilteredRepository[T]) Delete(id interface{}) error { return fr.inner.Delete(id) }
func (fr *FilteredRepository[T]) Update(entity T) error    { return fr.inner.Update(entity) }
func (fr *FilteredRepository[T]) Count() (int64, error)    { return fr.inner.Count() }
func (fr *FilteredRepository[T]) Exists(id interface{}) (bool, error) { return fr.inner.Exists(id) }

type EventRepository[T Entity] struct {
	inner  Repository[T]
	events []func(string, T)
	mu     sync.RWMutex
}

func NewEventRepository[T Entity](inner Repository[T]) *EventRepository[T] {
	return &EventRepository[T]{
		inner:  inner,
		events: make([]func(string, T), 0),
	}
}

func (er *EventRepository[T]) OnEvent(handler func(string, T)) {
	er.mu.Lock()
	defer er.mu.Unlock()
	er.events = append(er.events, handler)
}

func (er *EventRepository[T]) emit(event string, entity T) {
	er.mu.RLock()
	handlers := make([]func(string, T), len(er.events))
	copy(handlers, er.events)
	er.mu.RUnlock()

	for _, h := range handlers {
		h(event, entity)
	}
}

func (er *EventRepository[T]) FindByID(id interface{}) (T, error) {
	return er.inner.FindByID(id)
}

func (er *EventRepository[T]) FindAll() ([]T, error) { return er.inner.FindAll() }

func (er *EventRepository[T]) Save(entity T) error {
	err := er.inner.Save(entity)
	if err == nil {
		er.emit("save", entity)
	}
	return err
}

func (er *EventRepository[T]) Delete(id interface{}) error {
	return er.inner.Delete(id)
}

func (er *EventRepository[T]) Update(entity T) error {
	err := er.inner.Update(entity)
	if err == nil {
		er.emit("update", entity)
	}
	return err
}

func (er *EventRepository[T]) Count() (int64, error)               { return er.inner.Count() }
func (er *EventRepository[T]) Exists(id interface{}) (bool, error) { return er.inner.Exists(id) }

type Query[T Entity] struct {
	Conditions []func(T) bool
	OrderBy    func(a, b T) bool
	Limit      int
	Offset     int
}

type QueryableRepository[T Entity] interface {
	Repository[T]
	Find(query *Query[T]) ([]T, error)
}

type QueryRepository[T Entity] struct {
	inner Repository[T]
	mu    sync.RWMutex
}

func NewQueryRepository[T Entity](inner Repository[T]) *QueryRepository[T] {
	return &QueryRepository[T]{inner: inner}
}

func (qr *QueryRepository[T]) Find(query *Query[T]) ([]T, error) {
	items, err := qr.inner.FindAll()
	if err != nil {
		return nil, err
	}

	result := make([]T, 0)
	for _, item := range items {
		matches := true
		for _, cond := range query.Conditions {
			if !cond(item) {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, item)
		}
	}

	if query.OrderBy != nil {
		for i := 1; i < len(result); i++ {
			for j := i; j > 0 && query.OrderBy(result[j], result[j-1]); j-- {
				result[j], result[j-1] = result[j-1], result[j]
			}
		}
	}

	if query.Offset > 0 && query.Offset < len(result) {
		result = result[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result, nil
}

func (qr *QueryRepository[T]) FindByID(id interface{}) (T, error)  { return qr.inner.FindByID(id) }
func (qr *QueryRepository[T]) FindAll() ([]T, error)               { return qr.inner.FindAll() }
func (qr *QueryRepository[T]) Save(entity T) error                 { return qr.inner.Save(entity) }
func (qr *QueryRepository[T]) Delete(id interface{}) error         { return qr.inner.Delete(id) }
func (qr *QueryRepository[T]) Update(entity T) error               { return qr.inner.Update(entity) }
func (qr *QueryRepository[T]) Count() (int64, error)               { return qr.inner.Count() }
func (qr *QueryRepository[T]) Exists(id interface{}) (bool, error) { return qr.inner.Exists(id) }

type UnitOfWork[T Entity] struct {
	repo    Repository[T]
	changes []func() error
	mu      sync.RWMutex
}

func NewUnitOfWork[T Entity](repo Repository[T]) *UnitOfWork[T] {
	return &UnitOfWork[T]{
		repo:    repo,
		changes: make([]func() error, 0),
	}
}

func (uow *UnitOfWork[T]) Track(fn func() error) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	uow.changes = append(uow.changes, fn)
}

func (uow *UnitOfWork[T]) Commit() error {
	uow.mu.RLock()
	changes := make([]func() error, len(uow.changes))
	copy(changes, uow.changes)
	uow.mu.RUnlock()

	for _, change := range changes {
		if err := change(); err != nil {
			return err
		}
	}

	uow.mu.Lock()
	uow.changes = uow.changes[:0]
	uow.mu.Unlock()
	return nil
}

func (uow *UnitOfWork[T]) Rollback() {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	uow.changes = uow.changes[:0]
}
