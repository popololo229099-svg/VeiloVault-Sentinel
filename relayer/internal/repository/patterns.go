package repository

import (
	"fmt"
	"sync"
	"time"
)

type SyncRepository[T Entity] struct {
	inner Repository[T]
	mu    sync.RWMutex
}

func NewSyncRepository[T Entity](inner Repository[T]) *SyncRepository[T] {
	return &SyncRepository[T]{inner: inner}
}

func (sr *SyncRepository[T]) FindByID(id interface{}) (T, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.inner.FindByID(id)
}

func (sr *SyncRepository[T]) FindAll() ([]T, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.inner.FindAll()
}

func (sr *SyncRepository[T]) Save(entity T) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.inner.Save(entity)
}

func (sr *SyncRepository[T]) Delete(id interface{}) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.inner.Delete(id)
}

func (sr *SyncRepository[T]) Update(entity T) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.inner.Update(entity)
}

func (sr *SyncRepository[T]) Count() (int64, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.inner.Count()
}

func (sr *SyncRepository[T]) Exists(id interface{}) (bool, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.inner.Exists(id)
}

type BatchRepository[T Entity] struct {
	inner     Repository[T]
	batchSize int
	mu        sync.RWMutex
}

func NewBatchRepository[T Entity](inner Repository[T], batchSize int) *BatchRepository[T] {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchRepository[T]{
		inner:     inner,
		batchSize: batchSize,
	}
}

func (br *BatchRepository[T]) SaveAll(entities []T) error {
	br.mu.RLock()
	defer br.mu.RUnlock()

	for i := 0; i < len(entities); i += br.batchSize {
		end := i + br.batchSize
		if end > len(entities) {
			end = len(entities)
		}

		for _, entity := range entities[i:end] {
			if err := br.inner.Save(entity); err != nil {
				return fmt.Errorf("batch save failed at index %d: %w", i, err)
			}
		}
	}
	return nil
}

func (br *BatchRepository[T]) DeleteAll(ids []interface{}) error {
	br.mu.RLock()
	defer br.mu.RUnlock()

	for i := 0; i < len(ids); i += br.batchSize {
		end := i + br.batchSize
		if end > len(ids) {
			end = len(ids)
		}

		for _, id := range ids[i:end] {
			if err := br.inner.Delete(id); err != nil {
				return fmt.Errorf("batch delete failed at index %d: %w", i, err)
			}
		}
	}
	return nil
}

func (br *BatchRepository[T]) FindByID(id interface{}) (T, error) { return br.inner.FindByID(id) }
func (br *BatchRepository[T]) FindAll() ([]T, error)              { return br.inner.FindAll() }
func (br *BatchRepository[T]) Save(entity T) error                { return br.inner.Save(entity) }
func (br *BatchRepository[T]) Delete(id interface{}) error        { return br.inner.Delete(id) }
func (br *BatchRepository[T]) Update(entity T) error              { return br.inner.Update(entity) }
func (br *BatchRepository[T]) Count() (int64, error)              { return br.inner.Count() }
func (br *BatchRepository[T]) Exists(id interface{}) (bool, error) { return br.inner.Exists(id) }

type AuditableRepository[T Entity] struct {
	inner      Repository[T]
	auditLog   []AuditEntry
	maxLogSize int
	mu         sync.RWMutex
}

type AuditEntry struct {
	Action    string
	EntityID  interface{}
	Timestamp time.Time
	Details   string
}

func NewAuditableRepository[T Entity](inner Repository[T], maxLogSize int) *AuditableRepository[T] {
	if maxLogSize <= 0 {
		maxLogSize = 1000
	}
	return &AuditableRepository[T]{
		inner:      inner,
		auditLog:   make([]AuditEntry, 0),
		maxLogSize: maxLogSize,
	}
}

func (ar *AuditableRepository[T]) addAudit(action string, id interface{}, details string) {
	if len(ar.auditLog) >= ar.maxLogSize {
		ar.auditLog = ar.auditLog[1:]
	}
	ar.auditLog = append(ar.auditLog, AuditEntry{
		Action:    action,
		EntityID:  id,
		Timestamp: time.Now(),
		Details:   details,
	})
}

func (ar *AuditableRepository[T]) FindByID(id interface{}) (T, error) {
	ar.mu.Lock()
	ar.addAudit("find", id, "")
	ar.mu.Unlock()
	return ar.inner.FindByID(id)
}

func (ar *AuditableRepository[T]) FindAll() ([]T, error) {
	ar.mu.Lock()
	ar.addAudit("find_all", nil, "")
	ar.mu.Unlock()
	return ar.inner.FindAll()
}

func (ar *AuditableRepository[T]) Save(entity T) error {
	ar.mu.Lock()
	ar.addAudit("save", entity.GetID(), "")
	ar.mu.Unlock()
	return ar.inner.Save(entity)
}

func (ar *AuditableRepository[T]) Delete(id interface{}) error {
	ar.mu.Lock()
	ar.addAudit("delete", id, "")
	ar.mu.Unlock()
	return ar.inner.Delete(id)
}

func (ar *AuditableRepository[T]) Update(entity T) error {
	ar.mu.Lock()
	ar.addAudit("update", entity.GetID(), "")
	ar.mu.Unlock()
	return ar.inner.Update(entity)
}

func (ar *AuditableRepository[T]) Count() (int64, error) { return ar.inner.Count() }
func (ar *AuditableRepository[T]) Exists(id interface{}) (bool, error) { return ar.inner.Exists(id) }

func (ar *AuditableRepository[T]) AuditLog() []AuditEntry {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	result := make([]AuditEntry, len(ar.auditLog))
	copy(result, ar.auditLog)
	return result
}

type ReadOnlyRepository[T Entity] struct {
	inner Repository[T]
	mu    sync.RWMutex
}

func NewReadOnlyRepository[T Entity](inner Repository[T]) *ReadOnlyRepository[T] {
	return &ReadOnlyRepository[T]{inner: inner}
}

func (ror *ReadOnlyRepository[T]) FindByID(id interface{}) (T, error) {
	return ror.inner.FindByID(id)
}

func (ror *ReadOnlyRepository[T]) FindAll() ([]T, error) {
	return ror.inner.FindAll()
}

func (ror *ReadOnlyRepository[T]) Count() (int64, error) {
	return ror.inner.Count()
}

func (ror *ReadOnlyRepository[T]) Exists(id interface{}) (bool, error) {
	return ror.inner.Exists(id)
}

func (ror *ReadOnlyRepository[T]) Save(entity T) error {
	return fmt.Errorf("read-only repository: save not allowed")
}

func (ror *ReadOnlyRepository[T]) Delete(id interface{}) error {
	return fmt.Errorf("read-only repository: delete not allowed")
}

func (ror *ReadOnlyRepository[T]) Update(entity T) error {
	return fmt.Errorf("read-only repository: update not allowed")
}

type ConditionalRepository[T Entity] struct {
	inner     Repository[T]
	condition func(T) bool
	mu        sync.RWMutex
}

func NewConditionalRepository[T Entity](inner Repository[T], condition func(T) bool) *ConditionalRepository[T] {
	return &ConditionalRepository[T]{
		inner:     inner,
		condition: condition,
	}
}

func (cr *ConditionalRepository[T]) Save(entity T) error {
	cr.mu.RLock()
	cond := cr.condition
	cr.mu.RUnlock()

	if !cond(entity) {
		return fmt.Errorf("entity does not satisfy condition")
	}
	return cr.inner.Save(entity)
}

func (cr *ConditionalRepository[T]) Update(entity T) error {
	cr.mu.RLock()
	cond := cr.condition
	cr.mu.RUnlock()

	if !cond(entity) {
		return fmt.Errorf("entity does not satisfy condition")
	}
	return cr.inner.Update(entity)
}

func (cr *ConditionalRepository[T]) FindByID(id interface{}) (T, error)  { return cr.inner.FindByID(id) }
func (cr *ConditionalRepository[T]) FindAll() ([]T, error)               { return cr.inner.FindAll() }
func (cr *ConditionalRepository[T]) Delete(id interface{}) error         { return cr.inner.Delete(id) }
func (cr *ConditionalRepository[T]) Count() (int64, error)               { return cr.inner.Count() }
func (cr *ConditionalRepository[T]) Exists(id interface{}) (bool, error) { return cr.inner.Exists(id) }

type CachedRepository[T Entity] struct {
	inner     Repository[T]
	cache     map[interface{}]T
	ttl       time.Duration
	timestamps map[interface{}]time.Time
	mu        sync.RWMutex
}

func NewCachedRepository[T Entity](inner Repository[T], ttl time.Duration) *CachedRepository[T] {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedRepository[T]{
		inner:      inner,
		cache:      make(map[interface{}]T),
		ttl:        ttl,
		timestamps: make(map[interface{}]time.Time),
	}
}

func (cr *CachedRepository[T]) FindByID(id interface{}) (T, error) {
	cr.mu.RLock()
	if cached, ok := cr.cache[id]; ok {
		if time.Since(cr.timestamps[id]) < cr.ttl {
			cr.mu.RUnlock()
			return cached, nil
		}
	}
	cr.mu.RUnlock()

	entity, err := cr.inner.FindByID(id)
	if err != nil {
		return entity, err
	}

	cr.mu.Lock()
	cr.cache[id] = entity
	cr.timestamps[id] = time.Now()
	cr.mu.Unlock()

	return entity, nil
}

func (cr *CachedRepository[T]) FindAll() ([]T, error) {
	return cr.inner.FindAll()
}

func (cr *CachedRepository[T]) Save(entity T) error {
	cr.mu.Lock()
	delete(cr.cache, entity.GetID())
	delete(cr.timestamps, entity.GetID())
	cr.mu.Unlock()
	return cr.inner.Save(entity)
}

func (cr *CachedRepository[T]) Delete(id interface{}) error {
	cr.mu.Lock()
	delete(cr.cache, id)
	delete(cr.timestamps, id)
	cr.mu.Unlock()
	return cr.inner.Delete(id)
}

func (cr *CachedRepository[T]) Update(entity T) error {
	cr.mu.Lock()
	delete(cr.cache, entity.GetID())
	delete(cr.timestamps, entity.GetID())
	cr.mu.Unlock()
	return cr.inner.Update(entity)
}

func (cr *CachedRepository[T]) Count() (int64, error)               { return cr.inner.Count() }
func (cr *CachedRepository[T]) Exists(id interface{}) (bool, error) { return cr.inner.Exists(id) }

func (cr *CachedRepository[T]) InvalidateCache(id interface{}) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	delete(cr.cache, id)
	delete(cr.timestamps, id)
}

func (cr *CachedRepository[T]) InvalidateAll() {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.cache = make(map[interface{}]T)
	cr.timestamps = make(map[interface{}]time.Time)
}

type RepositoryChain[T Entity] struct {
	repositories []Repository[T]
	fallback     Repository[T]
	mu           sync.RWMutex
}

func NewRepositoryChain[T Entity](fallback Repository[T], repos ...Repository[T]) *RepositoryChain[T] {
	return &RepositoryChain[T]{
		repositories: repos,
		fallback:     fallback,
	}
}

func (rc *RepositoryChain[T]) FindByID(id interface{}) (T, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, repo := range rc.repositories {
		entity, err := repo.FindByID(id)
		if err == nil {
			return entity, nil
		}
	}

	return rc.fallback.FindByID(id)
}

func (rc *RepositoryChain[T]) FindAll() ([]T, error) {
	return rc.fallback.FindAll()
}

func (rc *RepositoryChain[T]) Save(entity T) error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, repo := range rc.repositories {
		if err := repo.Save(entity); err == nil {
			return nil
		}
	}

	return rc.fallback.Save(entity)
}

func (rc *RepositoryChain[T]) Delete(id interface{}) error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, repo := range rc.repositories {
		if err := repo.Delete(id); err == nil {
			return nil
		}
	}

	return rc.fallback.Delete(id)
}

func (rc *RepositoryChain[T]) Update(entity T) error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, repo := range rc.repositories {
		if err := repo.Update(entity); err == nil {
			return nil
		}
	}

	return rc.fallback.Update(entity)
}

func (rc *RepositoryChain[T]) Count() (int64, error) { return rc.fallback.Count() }
func (rc *RepositoryChain[T]) Exists(id interface{}) (bool, error) { return rc.fallback.Exists(id) }
