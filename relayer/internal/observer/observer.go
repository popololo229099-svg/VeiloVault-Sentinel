package observer

import (
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Name      string
	Data      interface{}
	Timestamp time.Time
	Source    string
}

func NewEvent(name string, data interface{}) *Event {
	return &Event{
		Name:      name,
		Data:      data,
		Timestamp: time.Now(),
	}
}

func (e *Event) SetSource(source string) {
	e.Source = source
}

type EventHandler interface {
	Handle(event *Event) error
	Name() string
}

type EventEmitter struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		handlers: make(map[string][]EventHandler),
	}
}

func (ee *EventEmitter) On(eventName string, handler EventHandler) {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	ee.handlers[eventName] = append(ee.handlers[eventName], handler)
}

func (ee *EventEmitter) Off(eventName string, handler EventHandler) {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	handlers := ee.handlers[eventName]
	for i, h := range handlers {
		if h.Name() == handler.Name() {
			ee.handlers[eventName] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

func (ee *EventEmitter) Emit(event *Event) error {
	ee.mu.RLock()
	handlers := make([]EventHandler, len(ee.handlers[event.Name]))
	copy(handlers, ee.handlers[event.Name])
	ee.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler.Handle(event); err != nil {
			return fmt.Errorf("handler %s error: %w", handler.Name(), err)
		}
	}
	return nil
}

func (ee *EventEmitter) EmitAsync(event *Event) {
	go func() {
		_ = ee.Emit(event)
	}()
}

func (ee *EventEmitter) Listeners(eventName string) int {
	ee.mu.RLock()
	defer ee.mu.RUnlock()
	return len(ee.handlers[eventName])
}

type EventBus struct {
	emitter    *EventEmitter
	middleware []func(*Event) *Event
	queue      chan *Event
	closed     bool
	mu         sync.RWMutex
}

func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventBus{
		emitter:    NewEventEmitter(),
		middleware: make([]func(*Event) *Event, 0),
		queue:      make(chan *Event, bufferSize),
	}
}

func (eb *EventBus) Use(mw func(*Event) *Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.middleware = append(eb.middleware, mw)
}

func (eb *EventBus) Subscribe(eventName string, handler EventHandler) {
	eb.emitter.On(eventName, handler)
}

func (eb *EventBus) Publish(event *Event) error {
	eb.mu.RLock()
	mws := make([]func(*Event) *Event, len(eb.middleware))
	copy(mws, eb.middleware)
	eb.mu.RUnlock()

	for _, mw := range mws {
		event = mw(event)
	}

	return eb.emitter.Emit(event)
}

func (eb *EventBus) Start(workers int) {
	if workers <= 0 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		go func() {
			for event := range eb.queue {
				_ = eb.emitter.Emit(event)
			}
		}()
	}
}

func (eb *EventBus) Enqueue(event *Event) error {
	eb.mu.RLock()
	closed := eb.closed
	eb.mu.RUnlock()

	if closed {
		return fmt.Errorf("event bus closed")
	}

	eb.queue <- event
	return nil
}

func (eb *EventBus) Close() {
	eb.mu.Lock()
	eb.closed = true
	eb.mu.Unlock()
	close(eb.queue)
}

type Observer[T any] interface {
	Update(data T) error
	Name() string
}

type Observable[T any] struct {
	observers []Observer[T]
	mu        sync.RWMutex
}

func NewObservable[T any]() *Observable[T] {
	return &Observable[T]{
		observers: make([]Observer[T], 0),
	}
}

func (o *Observable[T]) Attach(observer Observer[T]) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.observers = append(o.observers, observer)
}

func (o *Observable[T]) Detach(observer Observer[T]) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, obs := range o.observers {
		if obs.Name() == observer.Name() {
			o.observers = append(o.observers[:i], o.observers[i+1:]...)
			break
		}
	}
}

func (o *Observable[T]) Notify(data T) error {
	o.mu.RLock()
	observers := make([]Observer[T], len(o.observers))
	copy(observers, o.observers)
	o.mu.RUnlock()

	var lastErr error
	for _, obs := range observers {
		if err := obs.Update(data); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (o *Observable[T]) ObserverCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.observers)
}

type EventWrapper struct {
	Event Event
}

type Subject[T any] struct {
	observable *Observable[T]
	events     map[string]*Observable[*EventWrapper]
	mu         sync.RWMutex
}

func NewSubject[T any]() *Subject[T] {
	return &Subject[T]{
		observable: NewObservable[T](),
		events:     make(map[string]*Observable[*EventWrapper]),
	}
}

func (s *Subject[T]) Subscribe(observer Observer[T]) {
	s.observable.Attach(observer)
}

func (s *Subject[T]) Unsubscribe(observer Observer[T]) {
	s.observable.Detach(observer)
}

func (s *Subject[T]) Publish(data T) error {
	return s.observable.Notify(data)
}

func (s *Subject[T]) OnEvent(eventName string, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.events[eventName]; !exists {
		s.events[eventName] = NewObservable[*EventWrapper]()
	}
	s.events[eventName].Attach(&eventObserver{handler: handler})
}

func (s *Subject[T]) EmitEvent(event *Event) error {
	s.mu.RLock()
	obs, exists := s.events[event.Name]
	s.mu.RUnlock()

	if !exists {
		return nil
	}
	return obs.Notify(&EventWrapper{Event: *event})
}

type eventObserver struct {
	handler EventHandler
}

func (eo *eventObserver) Update(wrapper *EventWrapper) error {
	return eo.handler.Handle(&wrapper.Event)
}

func (eo *eventObserver) Name() string { return eo.handler.Name() }

type Mediator struct {
	handlers map[string]func(interface{}) (interface{}, error)
	mu       sync.RWMutex
}

func NewMediator() *Mediator {
	return &Mediator{
		handlers: make(map[string]func(interface{}) (interface{}, error)),
	}
}

func (m *Mediator) Register(name string, handler func(interface{}) (interface{}, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
}

func (m *Mediator) Send(name string, request interface{}) (interface{}, error) {
	m.mu.RLock()
	handler, exists := m.handlers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("handler not found: %s", name)
	}
	return handler(request)
}

type MediatorRequest struct {
	Name    string
	Payload interface{}
}

type MediatorResponse struct {
	Data  interface{}
	Error error
}
