package observer

import (
	"fmt"
	"sync"
	"time"
)

type EventBusMiddleware func(*Event) *Event

func LoggingMiddleware(logFunc func(string)) EventBusMiddleware {
	return func(event *Event) *Event {
		if logFunc != nil {
			logFunc(fmt.Sprintf("event: %s at %v", event.Name, event.Timestamp))
		}
		return event
	}
}

func FilteringMiddleware(filter func(*Event) bool) EventBusMiddleware {
	return func(event *Event) *Event {
		if filter != nil && !filter(event) {
			return nil
		}
		return event
	}
}

func TransformationMiddleware(transform func(*Event) *Event) EventBusMiddleware {
	return func(event *Event) *Event {
		if transform != nil {
			return transform(event)
		}
		return event
	}
}

func RetryMiddleware(maxRetries int, delay time.Duration) EventBusMiddleware {
	return func(event *Event) *Event {
		return event
	}
}

type EventInterceptor interface {
	BeforePublish(event *Event) (*Event, error)
	AfterPublish(event *Event) error
	Name() string
}

type EventInterceptorChain struct {
	interceptors []EventInterceptor
	mu           sync.RWMutex
}

func NewEventInterceptorChain() *EventInterceptorChain {
	return &EventInterceptorChain{
		interceptors: make([]EventInterceptor, 0),
	}
}

func (eic *EventInterceptorChain) Add(interceptor EventInterceptor) {
	eic.mu.Lock()
	defer eic.mu.Unlock()
	eic.interceptors = append(eic.interceptors, interceptor)
}

func (eic *EventInterceptorChain) BeforePublish(event *Event) (*Event, error) {
	eic.mu.RLock()
	defer eic.mu.RUnlock()

	current := event
	for _, interceptor := range eic.interceptors {
		var err error
		current, err = interceptor.BeforePublish(current)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, nil
		}
	}
	return current, nil
}

func (eic *EventInterceptorChain) AfterPublish(event *Event) error {
	eic.mu.RLock()
	defer eic.mu.RUnlock()

	for _, interceptor := range eic.interceptors {
		if err := interceptor.AfterPublish(event); err != nil {
			return err
		}
	}
	return nil
}

type DeadLetterQueue struct {
	events    []*Event
	maxSize   int
	callback  func(*Event)
	mu        sync.RWMutex
}

func NewDeadLetterQueue(maxSize int) *DeadLetterQueue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &DeadLetterQueue{
		events:  make([]*Event, 0),
		maxSize: maxSize,
	}
}

func (dlq *DeadLetterQueue) SetCallback(callback func(*Event)) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	dlq.callback = callback
}

func (dlq *DeadLetterQueue) Add(event *Event) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	if len(dlq.events) >= dlq.maxSize {
		dlq.events = dlq.events[1:]
	}
	dlq.events = append(dlq.events, event)

	if dlq.callback != nil {
		go dlq.callback(event)
	}
}

func (dlq *DeadLetterQueue) Retry() []*Event {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	events := make([]*Event, len(dlq.events))
	copy(events, dlq.events)
	dlq.events = dlq.events[:0]
	return events
}

func (dlq *DeadLetterQueue) Size() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.events)
}

type EventFilter interface {
	Accept(event *Event) bool
	Name() string
}

type EventFilterChain struct {
	filters []EventFilter
	mu      sync.RWMutex
}

func NewEventFilterChain(filters ...EventFilter) *EventFilterChain {
	return &EventFilterChain{filters: filters}
}

func (efc *EventFilterChain) Add(filter EventFilter) {
	efc.mu.Lock()
	defer efc.mu.Unlock()
	efc.filters = append(efc.filters, filter)
}

func (efc *EventFilterChain) Accept(event *Event) bool {
	efc.mu.RLock()
	defer efc.mu.RUnlock()
	for _, f := range efc.filters {
		if !f.Accept(event) {
			return false
		}
	}
	return true
}

type EventTypeFilter struct {
	allowedTypes map[string]bool
	mu           sync.RWMutex
}

func NewEventTypeFilter(types ...string) *EventTypeFilter {
	allowed := make(map[string]bool, len(types))
	for _, t := range types {
		allowed[t] = true
	}
	return &EventTypeFilter{allowedTypes: allowed}
}

func (etf *EventTypeFilter) Accept(event *Event) bool {
	etf.mu.RLock()
	defer etf.mu.RUnlock()
	return etf.allowedTypes[event.Name]
}

func (etf *EventTypeFilter) Name() string { return "event_type" }

type EventRateFilter struct {
	maxPerSecond int
	count        int
	lastReset    time.Time
	mu           sync.Mutex
}

func NewEventRateFilter(maxPerSecond int) *EventRateFilter {
	if maxPerSecond <= 0 {
		maxPerSecond = 100
	}
	return &EventRateFilter{
		maxPerSecond: maxPerSecond,
		lastReset:    time.Now(),
	}
}

func (erf *EventRateFilter) Accept(event *Event) bool {
	erf.mu.Lock()
	defer erf.mu.Unlock()

	now := time.Now()
	if now.Sub(erf.lastReset) >= time.Second {
		erf.count = 0
		erf.lastReset = now
	}

	erf.count++
	return erf.count <= erf.maxPerSecond
}

func (erf *EventRateFilter) Name() string { return "event_rate" }

type EventHistory struct {
	events  []*Event
	maxSize int
	mu      sync.RWMutex
}

func NewEventHistory(maxSize int) *EventHistory {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &EventHistory{
		events:  make([]*Event, 0),
		maxSize: maxSize,
	}
}

func (eh *EventHistory) Record(event *Event) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	if len(eh.events) >= eh.maxSize {
		eh.events = eh.events[1:]
	}
	eh.events = append(eh.events, event)
}

func (eh *EventHistory) GetRecent(count int) []*Event {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	if count > len(eh.events) {
		count = len(eh.events)
	}
	result := make([]*Event, count)
	copy(result, eh.events[len(eh.events)-count:])
	return result
}

func (eh *EventHistory) Search(name string) []*Event {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	var results []*Event
	for _, e := range eh.events {
		if e.Name == name {
			results = append(results, e)
		}
	}
	return results
}

func (eh *EventHistory) Clear() {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.events = eh.events[:0]
}

type EventAggregator struct {
	buckets map[string][]*Event
	window  time.Duration
	mu      sync.RWMutex
}

func NewEventAggregator(window time.Duration) *EventAggregator {
	if window <= 0 {
		window = time.Minute
	}
	return &EventAggregator{
		buckets: make(map[string][]*Event),
		window:  window,
	}
}

func (ea *EventAggregator) Add(event *Event) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.buckets[event.Name] = append(ea.buckets[event.Name], event)
}

func (ea *EventAggregator) Flush(name string) []*Event {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	events := ea.buckets[name]
	delete(ea.buckets, name)
	return events
}

func (ea *EventAggregator) FlushAll() map[string][]*Event {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	result := ea.buckets
	ea.buckets = make(map[string][]*Event)
	return result
}

func (ea *EventAggregator) Count(name string) int {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return len(ea.buckets[name])
}

type EventValidator interface {
	Validate(event *Event) error
}

type CompositeEventValidator struct {
	validators []EventValidator
	mu         sync.RWMutex
}

func NewCompositeEventValidator(validators ...EventValidator) *CompositeEventValidator {
	return &CompositeEventValidator{validators: validators}
}

func (cev *CompositeEventValidator) Add(v EventValidator) {
	cev.mu.Lock()
	defer cev.mu.Unlock()
	cev.validators = append(cev.validators, v)
}

func (cev *CompositeEventValidator) Validate(event *Event) error {
	cev.mu.RLock()
	defer cev.mu.RUnlock()
	for _, v := range cev.validators {
		if err := v.Validate(event); err != nil {
			return err
		}
	}
	return nil
}

type RequiredFieldsValidator struct {
	fields []string
	mu     sync.RWMutex
}

func NewRequiredFieldsValidator(fields ...string) *RequiredFieldsValidator {
	return &RequiredFieldsValidator{fields: fields}
}

func (rfv *RequiredFieldsValidator) Validate(event *Event) error {
	rfv.mu.RLock()
	defer rfv.mu.RUnlock()
	if event.Name == "" {
		return fmt.Errorf("event name is required")
	}
	return nil
}

type SourceValidator struct {
	allowedSources map[string]bool
	mu             sync.RWMutex
}

func NewSourceValidator(sources ...string) *SourceValidator {
	allowed := make(map[string]bool, len(sources))
	for _, s := range sources {
		allowed[s] = true
	}
	return &SourceValidator{allowedSources: allowed}
}

func (sv *SourceValidator) Validate(event *Event) error {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	if len(sv.allowedSources) > 0 && !sv.allowedSources[event.Source] {
		return fmt.Errorf("source not allowed: %s", event.Source)
	}
	return nil
}
