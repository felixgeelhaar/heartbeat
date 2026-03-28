package events

import (
	"sync"

	"github.com/felixgeelhaar/go-teamhealthcheck/sdk"
)

// EventType and Event are aliased from the SDK package to avoid duplication.
// The SDK package is the canonical source of event type definitions.
type EventType = sdk.EventType
type Event = sdk.Event

// Re-export constants for internal use.
const (
	VoteSubmitted            = sdk.VoteSubmitted
	HealthCheckCreated       = sdk.HealthCheckCreated
	HealthCheckStatusChanged = sdk.HealthCheckStatusChanged
	HealthCheckDeleted       = sdk.HealthCheckDeleted
)

// Listener receives events after successful store mutations.
type Listener interface {
	OnEvent(event Event)
}

// ListenerFunc adapts a plain function to the Listener interface.
type ListenerFunc func(Event)

func (f ListenerFunc) OnEvent(e Event) { f(e) }

// Bus is a fan-out event bus. Each listener is called in its own goroutine
// so it does not block the store mutation path.
type Bus struct {
	mu        sync.RWMutex
	listeners []Listener
}

// NewBus creates a new event bus.
func NewBus() *Bus { return &Bus{} }

// Subscribe registers a listener to receive all future events.
func (b *Bus) Subscribe(l Listener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, l)
}

// Publish sends an event to all registered listeners asynchronously.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, l := range b.listeners {
		go l.OnEvent(e)
	}
}
