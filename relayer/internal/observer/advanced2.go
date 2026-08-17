package observer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type EventQueue struct {
	events    chan *Event
	priority  chan *Event
	workers   int
	handler   func(*Event) error
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

func NewEventQueue(workers, bufferSize int) *EventQueue {
	if workers <= 0 {
		workers = 2
	}
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	return &EventQueue{
		events:   make(chan *Event, bufferSize),
		priority: make(chan *Event, bufferSize),
		workers:  workers,
		stopCh:   make(chan struct{}),
	}
}

func (eq *EventQueue) SetHandler(handler func(*Event) error) {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	eq.handler = handler
}

func (eq *EventQueue) Start() {
	eq.mu.RLock()
	handler := eq.handler
	eq.mu.RUnlock()

	for i := 0; i < eq.workers; i++ {
		eq.wg.Add(1)
		go eq.worker(handler)
	}
}

func (eq *EventQueue) worker(handler func(*Event) error) {
	defer eq.wg.Done()
	for {
		select {
		case event := <-eq.priority:
			if handler != nil {
				_ = handler(event)
			}
		case event := <-eq.events:
			if handler != nil {
				_ = handler(event)
			}
		case <-eq.stopCh:
			return
		}
	}
}

func (eq *EventQueue) Enqueue(event *Event) {
	eq.events <- event
}

func (eq *EventQueue) EnqueuePriority(event *Event) {
	eq.priority <- event
}

func (eq *EventQueue) Stop() {
	close(eq.stopCh)
	eq.wg.Wait()
}

func (eq *EventQueue) Size() int {
	return len(eq.events) + len(eq.priority)
}

type EventProcessor struct {
	processors map[string]func(*Event) error
	middleware []func(*Event) *Event
	mu        sync.RWMutex
}

func NewEventProcessor() *EventProcessor {
	return &EventProcessor{
		processors: make(map[string]func(*Event) error),
		middleware:  make([]func(*Event) *Event, 0),
	}
}

func (ep *EventProcessor) Register(name string, processor func(*Event) error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.processors[name] = processor
}

func (ep *EventProcessor) Use(mw func(*Event) *Event) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.middleware = append(ep.middleware, mw)
}

func (ep *EventProcessor) Process(event *Event) error {
	ep.mu.RLock()
	mws := make([]func(*Event) *Event, len(ep.middleware))
	copy(mws, ep.middleware)
	ep.mu.RUnlock()

	for _, mw := range mws {
		event = mw(event)
		if event == nil {
			return nil
		}
	}

	ep.mu.RLock()
	processor, ok := ep.processors[event.Name]
	ep.mu.RUnlock()

	if ok {
		return processor(event)
	}

	return fmt.Errorf("no processor for event: %s", event.Name)
}

func (ep *EventProcessor) ProcessAsync(event *Event) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- ep.Process(event)
	}()
	return ch
}

type EventBatch struct {
	events  []*Event
	batchSize int
	flushFn func([]*Event) error
	mu      sync.RWMutex
}

func NewEventBatch(batchSize int, flushFn func([]*Event) error) *EventBatch {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &EventBatch{
		events:    make([]*Event, 0, batchSize),
		batchSize: batchSize,
		flushFn:   flushFn,
	}
}

func (eb *EventBatch) Add(event *Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.events = append(eb.events, event)
	if len(eb.events) >= eb.batchSize {
		eb.flush()
	}
}

func (eb *EventBatch) Flush() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.flush()
}

func (eb *EventBatch) flush() {
	if eb.flushFn != nil && len(eb.events) > 0 {
		batch := make([]*Event, len(eb.events))
		copy(batch, eb.events)
		eb.events = eb.events[:0]
		_ = eb.flushFn(batch)
	}
}

func (eb *EventBatch) Size() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.events)
}

type EventRateLimiter struct {
	limit    int
	window   time.Duration
	counts   map[string]int
	lastTime map[string]time.Time
	mu       sync.Mutex
}

func NewEventRateLimiter(limit int, window time.Duration) *EventRateLimiter {
	if limit <= 0 {
		limit = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &EventRateLimiter{
		limit:    limit,
		window:   window,
		counts:   make(map[string]int),
		lastTime: make(map[string]time.Time),
	}
}

func (erl *EventRateLimiter) Allow(eventName string) bool {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	now := time.Now()
	if now.Sub(erl.lastTime[eventName]) > erl.window {
		erl.counts[eventName] = 0
		erl.lastTime[eventName] = now
	}

	erl.counts[eventName]++
	return erl.counts[eventName] <= erl.limit
}

type EventDeduplicator struct {
	seen   map[string]time.Time
	ttl    time.Duration
	mu     sync.RWMutex
}

func NewEventDeduplicator(ttl time.Duration) *EventDeduplicator {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &EventDeduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
}

func (ed *EventDeduplicator) IsDuplicate(event *Event) bool {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	key := fmt.Sprintf("%s:%v", event.Name, event.Data)
	if lastSeen, ok := ed.seen[key]; ok {
		if time.Since(lastSeen) < ed.ttl {
			return true
		}
	}

	ed.seen[key] = time.Now()
	return false
}

func (ed *EventDeduplicator) Cleanup() {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	now := time.Now()
	for key, t := range ed.seen {
		if now.Sub(t) > ed.ttl {
			delete(ed.seen, key)
		}
	}
}

type EventTransformer interface {
	Transform(event *Event) (*Event, error)
	Name() string
}

type EventTransformerChain struct {
	transformers []EventTransformer
	mu           sync.RWMutex
}

func NewEventTransformerChain() *EventTransformerChain {
	return &EventTransformerChain{
		transformers: make([]EventTransformer, 0),
	}
}

func (etc *EventTransformerChain) Add(transformer EventTransformer) {
	etc.mu.Lock()
	defer etc.mu.Unlock()
	etc.transformers = append(etc.transformers, transformer)
}

func (etc *EventTransformerChain) Transform(event *Event) (*Event, error) {
	etc.mu.RLock()
	transformers := make([]EventTransformer, len(etc.transformers))
	copy(transformers, etc.transformers)
	etc.mu.RUnlock()

	current := event
	for _, t := range transformers {
		var err error
		current, err = t.Transform(current)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, nil
		}
	}
	return current, nil
}

type EventContext struct {
	CorrelationID string
	Metadata      map[string]interface{}
	Timeout       time.Duration
	mu            sync.RWMutex
}

func NewEventContext(correlationID string) *EventContext {
	return &EventContext{
		CorrelationID: correlationID,
		Metadata:      make(map[string]interface{}),
		Timeout:       30 * time.Second,
	}
}

func (ec *EventContext) SetMetadata(key string, value interface{}) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Metadata[key] = value
}

func (ec *EventContext) GetMetadata(key string) interface{} {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.Metadata[key]
}

type ContextualEventEmitter struct {
	emitter *EventEmitter
	mu      sync.RWMutex
}

func NewContextualEventEmitter() *ContextualEventEmitter {
	return &ContextualEventEmitter{
		emitter: NewEventEmitter(),
	}
}

func (cee *ContextualEventEmitter) EmitWithContext(ctx context.Context, event *Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return cee.emitter.Emit(event)
	}
}

func (cee *ContextualEventEmitter) On(eventName string, handler EventHandler) {
	cee.emitter.On(eventName, handler)
}

type EventCounter struct {
	counts  map[string]int64
	total   int64
	mu      sync.RWMutex
}

func NewEventCounter() *EventCounter {
	return &EventCounter{
		counts: make(map[string]int64),
	}
}

func (ec *EventCounter) Increment(eventName string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.counts[eventName]++
	ec.total++
}

func (ec *EventCounter) Count(eventName string) int64 {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.counts[eventName]
}

func (ec *EventCounter) Total() int64 {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.total
}

func (ec *EventCounter) Snapshot() map[string]int64 {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	result := make(map[string]int64, len(ec.counts))
	for k, v := range ec.counts {
		result[k] = v
	}
	return result
}

func (ec *EventCounter) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.counts = make(map[string]int64)
	ec.total = 0
}

type EventScope struct {
	id       string
	events   []*Event
	startAt  time.Time
	metadata map[string]interface{}
	mu       sync.RWMutex
}

func NewEventScope(id string) *EventScope {
	return &EventScope{
		id:       id,
		events:   make([]*Event, 0),
		startAt:  time.Now(),
		metadata: make(map[string]interface{}),
	}
}

func (es *EventScope) Record(event *Event) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.events = append(es.events, event)
}

func (es *EventScope) Events() []*Event {
	es.mu.RLock()
	defer es.mu.RUnlock()
	result := make([]*Event, len(es.events))
	copy(result, es.events)
	return result
}

func (es *EventScope) Duration() time.Duration {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return time.Since(es.startAt)
}

func (es *EventScope) SetMetadata(key string, value interface{}) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.metadata[key] = value
}

func (es *EventScope) GetMetadata(key string) interface{} {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.metadata[key]
}

type EventScopeManager struct {
	scopes map[string]*EventScope
	mu     sync.RWMutex
}

func NewEventScopeManager() *EventScopeManager {
	return &EventScopeManager{
		scopes: make(map[string]*EventScope),
	}
}

func (esm *EventScopeManager) Start(id string) *EventScope {
	esm.mu.Lock()
	defer esm.mu.Unlock()
	scope := NewEventScope(id)
	esm.scopes[id] = scope
	return scope
}

func (esm *EventScopeManager) Get(id string) *EventScope {
	esm.mu.RLock()
	defer esm.mu.RUnlock()
	return esm.scopes[id]
}

func (esm *EventScopeManager) End(id string) *EventScope {
	esm.mu.Lock()
	defer esm.mu.Unlock()
	scope := esm.scopes[id]
	delete(esm.scopes, id)
	return scope
}

func (esm *EventScopeManager) Active() []string {
	esm.mu.RLock()
	defer esm.mu.RUnlock()
	ids := make([]string, 0, len(esm.scopes))
	for id := range esm.scopes {
		ids = append(ids, id)
	}
	return ids
}
