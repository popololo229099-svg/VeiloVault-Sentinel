package repository

import (
	"fmt"
	"sync"
	"time"
)

type SoftDeleteRepository[T Entity] struct {
	inner   Repository[T]
	deleted map[interface{}]time.Time
	mu      sync.RWMutex
}

func NewSoftDeleteRepository[T Entity](inner Repository[T]) *SoftDeleteRepository[T] {
	return &SoftDeleteRepository[T]{
		inner:   inner,
		deleted: make(map[interface{}]time.Time),
	}
}

func (sdr *SoftDeleteRepository[T]) FindByID(id interface{}) (T, error) {
	sdr.mu.RLock()
	if _, ok := sdr.deleted[id]; ok {
		sdr.mu.RUnlock()
		var zero T
		return zero, fmt.Errorf("entity soft-deleted: %v", id)
	}
	sdr.mu.RUnlock()
	return sdr.inner.FindByID(id)
}

func (sdr *SoftDeleteRepository[T]) FindAll() ([]T, error) {
	sdr.mu.RLock()
	deleted := make(map[interface{}]bool, len(sdr.deleted))
	for k, v := range sdr.deleted {
		deleted[k] = true
		_ = v
	}
	sdr.mu.RUnlock()

	items, err := sdr.inner.FindAll()
	if err != nil {
		return nil, err
	}

	result := make([]T, 0)
	for _, item := range items {
		if !deleted[item.GetID()] {
			result = append(result, item)
		}
	}
	return result, nil
}

func (sdr *SoftDeleteRepository[T]) Save(entity T) error {
	return sdr.inner.Save(entity)
}

func (sdr *SoftDeleteRepository[T]) Delete(id interface{}) error {
	sdr.mu.Lock()
	defer sdr.mu.Unlock()
	sdr.deleted[id] = time.Now()
	return nil
}

func (sdr *SoftDeleteRepository[T]) Update(entity T) error {
	return sdr.inner.Update(entity)
}

func (sdr *SoftDeleteRepository[T]) Count() (int64, error) {
	sdr.mu.RLock()
	deletedCount := len(sdr.deleted)
	sdr.mu.RUnlock()

	total, err := sdr.inner.Count()
	if err != nil {
		return 0, err
	}
	return total - int64(deletedCount), nil
}

func (sdr *SoftDeleteRepository[T]) Exists(id interface{}) (bool, error) {
	sdr.mu.RLock()
	if _, ok := sdr.deleted[id]; ok {
		sdr.mu.RUnlock()
		return false, nil
	}
	sdr.mu.RUnlock()
	return sdr.inner.Exists(id)
}

func (sdr *SoftDeleteRepository[T]) Restore(id interface{}) error {
	sdr.mu.Lock()
	defer sdr.mu.Unlock()
	delete(sdr.deleted, id)
	return nil
}

type TimestampRepository[T Entity] struct {
	inner     Repository[T]
	createdAt map[interface{}]time.Time
	updatedAt map[interface{}]time.Time
	mu        sync.RWMutex
}

func NewTimestampRepository[T Entity](inner Repository[T]) *TimestampRepository[T] {
	return &TimestampRepository[T]{
		inner:     inner,
		createdAt: make(map[interface{}]time.Time),
		updatedAt: make(map[interface{}]time.Time),
	}
}

func (tr *TimestampRepository[T]) FindByID(id interface{}) (T, error) {
	return tr.inner.FindByID(id)
}

func (tr *TimestampRepository[T]) FindAll() ([]T, error) {
	return tr.inner.FindAll()
}

func (tr *TimestampRepository[T]) Save(entity T) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	id := entity.GetID()
	if _, exists := tr.createdAt[id]; !exists {
		tr.createdAt[id] = time.Now()
	}
	tr.updatedAt[id] = time.Now()
	return tr.inner.Save(entity)
}

func (tr *TimestampRepository[T]) Delete(id interface{}) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	delete(tr.createdAt, id)
	delete(tr.updatedAt, id)
	return tr.inner.Delete(id)
}

func (tr *TimestampRepository[T]) Update(entity T) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.updatedAt[entity.GetID()] = time.Now()
	return tr.inner.Update(entity)
}

func (tr *TimestampRepository[T]) Count() (int64, error)             { return tr.inner.Count() }
func (tr *TimestampRepository[T]) Exists(id interface{}) (bool, error) { return tr.inner.Exists(id) }

func (tr *TimestampRepository[T]) GetCreatedAt(id interface{}) (time.Time, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	t, ok := tr.createdAt[id]
	return t, ok
}

func (tr *TimestampRepository[T]) GetUpdatedAt(id interface{}) (time.Time, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	t, ok := tr.updatedAt[id]
	return t, ok
}

type PaginatedRepository[T Entity] struct {
	inner Repository[T]
	mu    sync.RWMutex
}

type PageRequest struct {
	Page     int
	PageSize int
}

type PageResult[T any] struct {
	Items      []T
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

func NewPaginatedRepository[T Entity](inner Repository[T]) *PaginatedRepository[T] {
	return &PaginatedRepository[T]{inner: inner}
}

func (pr *PaginatedRepository[T]) FindPaginated(page *PageRequest) (*PageResult[T], error) {
	if page == nil {
		page = &PageRequest{Page: 1, PageSize: 10}
	}
	if page.Page < 1 {
		page.Page = 1
	}
	if page.PageSize <= 0 {
		page.PageSize = 10
	}

	total, err := pr.inner.Count()
	if err != nil {
		return nil, err
	}

	items, err := pr.inner.FindAll()
	if err != nil {
		return nil, err
	}

	start := (page.Page - 1) * page.PageSize
	if start >= len(items) {
		return &PageResult[T]{
			Items:      make([]T, 0),
			Total:      total,
			Page:       page.Page,
			PageSize:   page.PageSize,
			TotalPages: int(total) / page.PageSize,
		}, nil
	}

	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}

	totalPages := int(total) / page.PageSize
	if int(total)%page.PageSize > 0 {
		totalPages++
	}

	return &PageResult[T]{
		Items:      items[start:end],
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (pr *PaginatedRepository[T]) FindByID(id interface{}) (T, error)  { return pr.inner.FindByID(id) }
func (pr *PaginatedRepository[T]) FindAll() ([]T, error)               { return pr.inner.FindAll() }
func (pr *PaginatedRepository[T]) Save(entity T) error                 { return pr.inner.Save(entity) }
func (pr *PaginatedRepository[T]) Delete(id interface{}) error         { return pr.inner.Delete(id) }
func (pr *PaginatedRepository[T]) Update(entity T) error               { return pr.inner.Update(entity) }
func (pr *PaginatedRepository[T]) Count() (int64, error)               { return pr.inner.Count() }
func (pr *PaginatedRepository[T]) Exists(id interface{}) (bool, error) { return pr.inner.Exists(id) }

type VersionedRepository[T Entity] struct {
	inner    Repository[T]
	versions map[interface{}][]versionEntry[T]
	mu       sync.RWMutex
}

type versionEntry[T any] struct {
	version   int
	entity    T
	createdAt time.Time
}

func NewVersionedRepository[T Entity](inner Repository[T]) *VersionedRepository[T] {
	return &VersionedRepository[T]{
		inner:    inner,
		versions: make(map[interface{}][]versionEntry[T]),
	}
}

func (vr *VersionedRepository[T]) FindByID(id interface{}) (T, error) {
	return vr.inner.FindByID(id)
}

func (vr *VersionedRepository[T]) FindAll() ([]T, error) {
	return vr.inner.FindAll()
}

func (vr *VersionedRepository[T]) Save(entity T) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	id := entity.GetID()
	versions := vr.versions[id]
	version := len(versions) + 1

	vr.versions[id] = append(versions, versionEntry[T]{
		version:   version,
		entity:    entity,
		createdAt: time.Now(),
	})

	return vr.inner.Save(entity)
}

func (vr *VersionedRepository[T]) Delete(id interface{}) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	delete(vr.versions, id)
	return vr.inner.Delete(id)
}

func (vr *VersionedRepository[T]) Update(entity T) error {
	return vr.Save(entity)
}

func (vr *VersionedRepository[T]) Count() (int64, error) {
	return vr.inner.Count()
}

func (vr *VersionedRepository[T]) Exists(id interface{}) (bool, error) {
	return vr.inner.Exists(id)
}

func (vr *VersionedRepository[T]) GetVersions(id interface{}) []T {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	versions, ok := vr.versions[id]
	if !ok {
		return nil
	}

	result := make([]T, len(versions))
	for i, v := range versions {
		result[i] = v.entity
	}
	return result
}

func (vr *VersionedRepository[T]) GetVersion(id interface{}, version int) (T, error) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	versions, ok := vr.versions[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no versions for entity: %v", id)
	}

	for _, v := range versions {
		if v.version == version {
			return v.entity, nil
		}
	}

	var zero T
	return zero, fmt.Errorf("version %d not found for entity: %v", version, id)
}

type FilterableRepository[T Entity] struct {
	inner     Repository[T]
	filters   map[string]func(T) bool
	sortFunc  func([]T) []T
	mu        sync.RWMutex
}

func NewFilterableRepository[T Entity](inner Repository[T]) *FilterableRepository[T] {
	return &FilterableRepository[T]{
		inner:   inner,
		filters: make(map[string]func(T) bool),
	}
}

func (fr *FilterableRepository[T]) AddFilter(name string, filter func(T) bool) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.filters[name] = filter
}

func (fr *FilterableRepository[T]) SetSort(sortFunc func([]T) []T) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.sortFunc = sortFunc
}

func (fr *FilterableRepository[T]) FindWithFilters(activeFilters []string) ([]T, error) {
	items, err := fr.inner.FindAll()
	if err != nil {
		return nil, err
	}

	fr.mu.RLock()
	defer fr.mu.RUnlock()

	result := make([]T, 0)
	for _, item := range items {
		matches := true
		for _, filterName := range activeFilters {
			if filter, ok := fr.filters[filterName]; ok {
				if !filter(item) {
					matches = false
					break
				}
			}
		}
		if matches {
			result = append(result, item)
		}
	}

	if fr.sortFunc != nil {
		result = fr.sortFunc(result)
	}

	return result, nil
}

func (fr *FilterableRepository[T]) FindByID(id interface{}) (T, error)  { return fr.inner.FindByID(id) }
func (fr *FilterableRepository[T]) FindAll() ([]T, error)               { return fr.inner.FindAll() }
func (fr *FilterableRepository[T]) Save(entity T) error                 { return fr.inner.Save(entity) }
func (fr *FilterableRepository[T]) Delete(id interface{}) error         { return fr.inner.Delete(id) }
func (fr *FilterableRepository[T]) Update(entity T) error               { return fr.inner.Update(entity) }
func (fr *FilterableRepository[T]) Count() (int64, error)               { return fr.inner.Count() }
func (fr *FilterableRepository[T]) Exists(id interface{}) (bool, error) { return fr.inner.Exists(id) }

type MetricsRepository[T Entity] struct {
	inner     Repository[T]
	queryCount int64
	saveCount  int64
	deleteCount int64
	mu        sync.RWMutex
}

func NewMetricsRepository[T Entity](inner Repository[T]) *MetricsRepository[T] {
	return &MetricsRepository[T]{inner: inner}
}

func (mr *MetricsRepository[T]) FindByID(id interface{}) (T, error) {
	mr.mu.Lock()
	mr.queryCount++
	mr.mu.Unlock()
	return mr.inner.FindByID(id)
}

func (mr *MetricsRepository[T]) FindAll() ([]T, error) {
	mr.mu.Lock()
	mr.queryCount++
	mr.mu.Unlock()
	return mr.inner.FindAll()
}

func (mr *MetricsRepository[T]) Save(entity T) error {
	mr.mu.Lock()
	mr.saveCount++
	mr.mu.Unlock()
	return mr.inner.Save(entity)
}

func (mr *MetricsRepository[T]) Delete(id interface{}) error {
	mr.mu.Lock()
	mr.deleteCount++
	mr.mu.Unlock()
	return mr.inner.Delete(id)
}

func (mr *MetricsRepository[T]) Update(entity T) error {
	mr.mu.Lock()
	mr.saveCount++
	mr.mu.Unlock()
	return mr.inner.Update(entity)
}

func (mr *MetricsRepository[T]) Count() (int64, error) {
	return mr.inner.Count()
}

func (mr *MetricsRepository[T]) Exists(id interface{}) (bool, error) {
	return mr.inner.Exists(id)
}

func (mr *MetricsRepository[T]) Metrics() (queries, saves, deletes int64) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return mr.queryCount, mr.saveCount, mr.deleteCount
}
