package events

import (
	"sync"
	"time"
)

type DomainEvent struct {
	ID        string
	Type      string
	Payload   interface{}
	Timestamp time.Time
	Source    string
	Metadata  map[string]string
}

func NewDomainEvent(id, eventType, source string, payload interface{}) *DomainEvent {
	return &DomainEvent{
		ID:        id,
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
		Source:    source,
		Metadata:  make(map[string]string),
	}
}

type EventHandler interface {
	Handle(event *DomainEvent) error
	EventType() string
}

type DomainEventBus struct {
	handlers   map[string][]EventHandler
	deadLetter []*DomainEvent
	mu         sync.RWMutex
	maxDead    int
}

func NewDomainEventBus(maxDeadLetter int) *DomainEventBus {
	if maxDeadLetter <= 0 {
		maxDeadLetter = 1000
	}
	return &DomainEventBus{
		handlers:   make(map[string][]EventHandler),
		deadLetter: make([]*DomainEvent, 0),
		maxDead:    maxDeadLetter,
	}
}

func (b *DomainEventBus) Subscribe(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	et := handler.EventType()
	b.handlers[et] = append(b.handlers[et], handler)
}

func (b *DomainEventBus) Publish(event *DomainEvent) error {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.mu.Lock()
		if len(b.deadLetter) >= b.maxDead {
			b.deadLetter = b.deadLetter[1:]
		}
		b.deadLetter = append(b.deadLetter, event)
		b.mu.Unlock()
		return nil
	}

	for _, h := range handlers {
		go func(handler EventHandler) {
			_ = handler.Handle(event)
		}(h)
	}
	return nil
}

func (b *DomainEventBus) HandlerCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}

func (b *DomainEventBus) DeadLetterCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.deadLetter)
}

type OutboxEntry struct {
	ID         string
	EventType  string
	Payload    []byte
	Status     string
	CreatedAt  time.Time
	RetryCount int
	MaxRetries int
}

type Outbox struct {
	entries []*OutboxEntry
	mu      sync.Mutex
}

func NewOutbox() *Outbox {
	return &Outbox{entries: make([]*OutboxEntry, 0)}
}

func (o *Outbox) Add(entry *OutboxEntry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	entry.Status = "pending"
	entry.CreatedAt = time.Now()
	if entry.MaxRetries == 0 {
		entry.MaxRetries = 3
	}
	o.entries = append(o.entries, entry)
}

func (o *Outbox) GetPending() []*OutboxEntry {
	o.mu.Lock()
	defer o.mu.Unlock()
	var pending []*OutboxEntry
	for _, e := range o.entries {
		if e.Status == "pending" {
			pending = append(pending, e)
		}
	}
	return pending
}

func (o *Outbox) MarkProcessed(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, e := range o.entries {
		if e.ID == id {
			e.Status = "processed"
			return
		}
	}
}

func (o *Outbox) MarkFailed(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, e := range o.entries {
		if e.ID == id {
			e.RetryCount++
			if e.RetryCount >= e.MaxRetries {
				e.Status = "dead_letter"
			}
			return
		}
	}
}

func (o *Outbox) Cleanup(maxAge time.Duration) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	var kept []*OutboxEntry
	removed := 0
	for _, e := range o.entries {
		if e.CreatedAt.Before(cutoff) && e.Status == "processed" {
			removed++
		} else {
			kept = append(kept, e)
		}
	}
	o.entries = kept
	return removed
}

type DeadLetterQueue struct {
	entries []*DeadLetterEntry
	maxSize int
	mu      sync.Mutex
}

type DeadLetterEntry struct {
	Event     *DomainEvent
	Error     string
	Attempts  int
	CreatedAt time.Time
}

func NewDeadLetterQueue(maxSize int) *DeadLetterQueue {
	return &DeadLetterQueue{entries: make([]*DeadLetterEntry, 0), maxSize: maxSize}
}

func (dlq *DeadLetterQueue) Add(event *DomainEvent, err error) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	if len(dlq.entries) >= dlq.maxSize {
		dlq.entries = dlq.entries[1:]
	}
	dlq.entries = append(dlq.entries, &DeadLetterEntry{
		Event:     event,
		Error:     err.Error(),
		CreatedAt: time.Now(),
	})
}

func (dlq *DeadLetterQueue) Count() int {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	return len(dlq.entries)
}

func (dlq *DeadLetterQueue) Drain() []*DeadLetterEntry {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	result := dlq.entries
	dlq.entries = make([]*DeadLetterEntry, 0)
	return result
}
