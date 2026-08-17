package data

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type EventLog struct {
	entries []LogEntry
	maxSize int
	mu      sync.Mutex
}

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Category  string
	Message   string
	Fields    map[string]interface{}
}

func NewEventLog(maxSize int) *EventLog {
	return &EventLog{entries: make([]LogEntry, 0), maxSize: maxSize}
}

func (el *EventLog) Log(level, category, message string, fields map[string]interface{}) {
	el.mu.Lock()
	defer el.mu.Unlock()
	if len(el.entries) >= el.maxSize {
		el.entries = el.entries[1:]
	}
	el.entries = append(el.entries, LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    fields,
	})
}

func (el *EventLog) Info(category, message string) {
	el.Log("info", category, message, nil)
}

func (el *EventLog) Warn(category, message string) {
	el.Log("warn", category, message, nil)
}

func (el *EventLog) Error(category, message string) {
	el.Log("error", category, message, nil)
}

func (el *EventLog) Recent(n int) []LogEntry {
	el.mu.Lock()
	defer el.mu.Unlock()
	if n > len(el.entries) {
		n = len(el.entries)
	}
	result := make([]LogEntry, n)
	copy(result, el.entries[len(el.entries)-n:])
	return result
}

func (el *EventLog) ByLevel(level string) []LogEntry {
	el.mu.Lock()
	defer el.mu.Unlock()
	var result []LogEntry
	for _, e := range el.entries {
		if e.Level == level {
			result = append(result, e)
		}
	}
	return result
}

func (el *EventLog) ByCategory(category string) []LogEntry {
	el.mu.Lock()
	defer el.mu.Unlock()
	var result []LogEntry
	for _, e := range el.entries {
		if e.Category == category {
			result = append(result, e)
		}
	}
	return result
}

func (el *EventLog) Search(query string) []LogEntry {
	el.mu.Lock()
	defer el.mu.Unlock()
	var result []LogEntry
	q := strings.ToLower(query)
	for _, e := range el.entries {
		if strings.Contains(strings.ToLower(e.Message), q) {
			result = append(result, e)
		}
	}
	return result
}

func (el *EventLog) Count() int {
	el.mu.Lock()
	defer el.mu.Unlock()
	return len(el.entries)
}

type ObjectPool struct {
	factory func() interface{}
	objects chan interface{}
	mu      sync.Mutex
}

func NewObjectPool(size int, factory func() interface{}) *ObjectPool {
	pool := &ObjectPool{
		factory: factory,
		objects: make(chan interface{}, size),
	}
	for i := 0; i < size; i++ {
		pool.objects <- factory()
	}
	return pool
}

func (op *ObjectPool) Get() interface{} {
	select {
	case obj := <-op.objects:
		return obj
	default:
		return op.factory()
	}
}

func (op *ObjectPool) Put(obj interface{}) {
	select {
	case op.objects <- obj:
	default:
	}
}

func (op *ObjectPool) Size() int {
	return cap(op.objects)
}

func (op *ObjectPool) Available() int {
	return len(op.objects)
}

type Reducer[T any] struct {
	state T
	mu    sync.Mutex
}

func NewReducer[T any](initial T) *Reducer[T] {
	return &Reducer[T]{state: initial}
}

func (r *Reducer[T]) Dispatch(action func(T) T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = action(r.state)
}

func (r *Reducer[T]) State() T {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

type Result[T any] struct {
	Value T
	Error error
}

func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

func Err[T any](err error) Result[T] {
	return Result[T]{Error: err}
}

func (r Result[T]) IsOk() bool { return r.Error == nil }
func (r Result[T]) IsErr() bool { return r.Error != nil }

func (r Result[T]) Unwrap() T {
	if r.Error != nil {
		panic(r.Error)
	}
	return r.Value
}

func (r Result[T]) UnwrapOr(defaultVal T) T {
	if r.Error != nil {
		return defaultVal
	}
	return r.Value
}

func (r Result[T]) Map(fn func(T) T) Result[T] {
	if r.Error != nil {
		return r
	}
	return Ok(fn(r.Value))
}

func (r Result[T]) FlatMap(fn func(T) Result[T]) Result[T] {
	if r.Error != nil {
		return r
	}
	return fn(r.Value)
}

type Option[T any] struct {
	value *T
}

func Some[T any](value T) Option[T] {
	return Option[T]{value: &value}
}

func None[T any]() Option[T] {
	return Option[T]{}
}

func (o Option[T]) IsSome() bool { return o.value != nil }
func (o Option[T]) IsNone() bool { return o.value == nil }

func (o Option[T]) Unwrap() T {
	if o.value == nil {
		panic("unwrap on None")
	}
	return *o.value
}

func (o Option[T]) UnwrapOr(defaultVal T) T {
	if o.value == nil {
		return defaultVal
	}
	return *o.value
}

func (o Option[T]) Map(fn func(T) T) Option[T] {
	if o.value == nil {
		return o
	}
	return Some(fn(*o.value))
}

func (o Option[T]) Filter(predicate func(T) bool) Option[T] {
	if o.value == nil || !predicate(*o.value) {
		return None[T]()
	}
	return o
}

type JSONPatch struct {
	Operations []PatchOperation
}

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewJSONPatch() *JSONPatch {
	return &JSONPatch{Operations: make([]PatchOperation, 0)}
}

func (jp *JSONPatch) Add(path string, value interface{}) *JSONPatch {
	jp.Operations = append(jp.Operations, PatchOperation{Op: "add", Path: path, Value: value})
	return jp
}

func (jp *JSONPatch) Remove(path string) *JSONPatch {
	jp.Operations = append(jp.Operations, PatchOperation{Op: "remove", Path: path})
	return jp
}

func (jp *JSONPatch) Replace(path string, value interface{}) *JSONPatch {
	jp.Operations = append(jp.Operations, PatchOperation{Op: "replace", Path: path, Value: value})
	return jp
}

func (jp *JSONPatch) Marshal() ([]byte, error) {
	return json.Marshal(jp.Operations)
}

func (jp *JSONPatch) String() string {
	data, _ := json.Marshal(jp.Operations)
	return string(data)
}

type SortedMap[K comparable, V any] struct {
	keys []K
	data map[K]V
	mu   sync.RWMutex
}

func NewSortedMap[K comparable, V any]() *SortedMap[K, V] {
	return &SortedMap[K, V]{data: make(map[K]V)}
}

func (sm *SortedMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.data[key]; !ok {
		sm.keys = append(sm.keys, key)
		sort.Slice(sm.keys, func(i, j int) bool {
			return fmt.Sprintf("%v", sm.keys[i]) < fmt.Sprintf("%v", sm.keys[j])
		})
	}
	sm.data[key] = value
}

func (sm *SortedMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.data[key]
	return val, ok
}

func (sm *SortedMap[K, V]) Keys() []K {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]K, len(sm.keys))
	copy(result, sm.keys)
	return result
}

func (sm *SortedMap[K, V]) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.data)
}
