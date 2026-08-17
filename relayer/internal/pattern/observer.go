package pattern

import (
	"fmt"
	"sync"
)

// Observer defines the interface for objects that observe events.
type Observer interface {
	OnEvent(event Event)
	ID() string
}

// Event represents a domain event.
type Event struct {
	Type    string
	Payload interface{}
	Source  string
}

// EventEmitter implements the Observer pattern (also known as Publish-Subscribe).
// It maintains a list of interested observers and notifies them of events.
type EventEmitter struct {
	mu          sync.RWMutex
	observers   map[string][]Observer
	filters     map[string][]func(Event) bool
	globalHooks []func(Event)
}

func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		observers: make(map[string][]Observer),
		filters:   make(map[string][]func(Event) bool),
	}
}

// Subscribe registers an observer for a specific event type.
func (e *EventEmitter) Subscribe(eventType string, obs Observer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.observers[eventType] = append(e.observers[eventType], obs)
}

// Unsubscribe removes an observer from a specific event type.
func (e *EventEmitter) Unsubscribe(eventType string, obs Observer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	observers := e.observers[eventType]
	for i, o := range observers {
		if o.ID() == obs.ID() {
			e.observers[eventType] = append(observers[:i], observers[i+1:]...)
			return
		}
	}
}

// AddFilter adds a predicate filter for an event type. Events that fail the filter are dropped.
func (e *EventEmitter) AddFilter(eventType string, filter func(Event) bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filters[eventType] = append(e.filters[eventType], filter)
}

// AddGlobalHook adds a function that runs for every event regardless of type.
func (e *EventEmitter) AddGlobalHook(hook func(Event)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalHooks = append(e.globalHooks, hook)
}

// Emit notifies all observers of an event type.
func (e *EventEmitter) Emit(event Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Global hooks
	for _, hook := range e.globalHooks {
		hook(event)
	}

	// Check filters
	if filters, ok := e.filters[event.Type]; ok {
		for _, filter := range filters {
			if !filter(event) {
				return
			}
		}
	}

	// Notify observers
	if observers, ok := e.observers[event.Type]; ok {
		for _, obs := range observers {
			go obs.OnEvent(event)
		}
	}

	// Notify wildcard observers
	if observers, ok := e.observers["*"]; ok {
		for _, obs := range observers {
			go obs.OnEvent(event)
		}
	}
}

// ObserverCount returns the number of observers for a given event type.
func (e *EventEmitter) ObserverCount(eventType string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.observers[eventType])
}

// LogObserver is a simple observer that logs events to stdout.
type LogObserver struct {
	Prefix string
}

func NewLogObserver(prefix string) *LogObserver {
	return &LogObserver{Prefix: prefix}
}

func (o *LogObserver) OnEvent(event Event) {
	fmt.Printf("[%s] Event: %s | Source: %s\n", o.Prefix, event.Type, event.Source)
}

func (o *LogObserver) ID() string { return "log-" + o.Prefix }
