package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type OrderEvent struct {
	ID        string
	Type      string
	Payload   interface{}
	Timestamp time.Time
	Retries   int
}

type OrderEventBus struct {
	handlers  map[string][]OrderEventHandler
	mu        sync.RWMutex
}

type OrderEventHandler func(ctx context.Context, event OrderEvent) error

func NewOrderEventBus() *OrderEventBus {
	return &OrderEventBus{
		handlers: make(map[string][]OrderEventHandler),
	}
}

func (b *OrderEventBus) Subscribe(eventType string, handler OrderEventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *OrderEventBus) Publish(ctx context.Context, event OrderEvent) error {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("handler error for %s: %w", event.Type, err)
		}
	}
	return nil
}

func (b *OrderEventBus) PublishAsync(event OrderEvent) {
	go func() {
		_ = b.Publish(context.Background(), event)
	}()
}

type EventBusMetrics struct {
	Published   int64
	Delivered   int64
	Failed      int64
	AvgLatency  time.Duration
	mu          sync.Mutex
}

func NewEventBusMetrics() *EventBusMetrics {
	return &EventBusMetrics{}
}

func (m *EventBusMetrics) RecordPublish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Published++
}

func (m *EventBusMetrics) RecordDelivery(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Delivered++
	m.AvgLatency = (m.AvgLatency * time.Duration(m.Delivered-1) + latency) / time.Duration(m.Delivered)
}

func (m *EventBusMetrics) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Failed++
}

type InMemoryEventStore struct {
	events []OrderEvent
	maxSize int
	mu     sync.Mutex
}

func NewInMemoryEventStore(maxSize int) *InMemoryEventStore {
	return &InMemoryEventStore{
		events:  make([]OrderEvent, 0),
		maxSize: maxSize,
	}
}

func (s *InMemoryEventStore) Store(event OrderEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) >= s.maxSize {
		s.events = s.events[1:]
	}
	s.events = append(s.events, event)
	return nil
}

func (s *InMemoryEventStore) GetByID(id string) (*OrderEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("event %s not found", id)
}

func (s *InMemoryEventStore) ListByType(eventType string) []OrderEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []OrderEvent
	for _, e := range s.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

func (s *InMemoryEventStore) ListSince(since time.Time) []OrderEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []OrderEvent
	for _, e := range s.events {
		if e.Timestamp.After(since) || e.Timestamp.Equal(since) {
			result = append(result, e)
		}
	}
	return result
}

func (s *InMemoryEventStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

type EventReplay struct {
	store     *InMemoryEventStore
	bus       *OrderEventBus
	mu        sync.Mutex
}

func NewEventReplay(store *InMemoryEventStore, bus *OrderEventBus) *EventReplay {
	return &EventReplay{store: store, bus: bus}
}

func (er *EventReplay) ReplaySince(ctx context.Context, since time.Time) error {
	er.mu.Lock()
	defer er.mu.Unlock()
	events := er.store.ListSince(since)
	for _, event := range events {
		if err := er.bus.Publish(ctx, event); err != nil {
			return fmt.Errorf("replay error for event %s: %w", event.ID, err)
		}
	}
	return nil
}

func (er *EventReplay) ReplayByType(ctx context.Context, eventType string) error {
	er.mu.Lock()
	defer er.mu.Unlock()
	events := er.store.ListByType(eventType)
	for _, event := range events {
		if err := er.bus.Publish(ctx, event); err != nil {
			return fmt.Errorf("replay error for event %s: %w", event.ID, err)
		}
	}
	return nil
}

type EventSerializer struct{}

func NewEventSerializer() *EventSerializer {
	return &EventSerializer{}
}

func (s *EventSerializer) Marshal(event OrderEvent) ([]byte, error) {
	return json.Marshal(event)
}

func (s *EventSerializer) Unmarshal(data []byte) (OrderEvent, error) {
	var event OrderEvent
	err := json.Unmarshal(data, &event)
	return event, err
}
