package a2a

import (
	"sync"
)

// A2AEvent represents a broadcast or point-to-point negotiation event on the A2A bus.
type A2AEvent struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"` // e.g. HANDOFF, NEGOTIATE, BROADCAST, STATE_CHANGED
	Sender    string      `json:"sender"`
	Payload   interface{} `json:"payload"`
}

// SubscriptionChan is a buffered channel for receiving events.
type SubscriptionChan chan A2AEvent

// EventBus implements a thread-safe in-memory pub/sub broker for A2A communications.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]SubscriptionChan
}

// NewEventBus creates a new EventBus instance.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]SubscriptionChan),
	}
}

// Subscribe registers a new subscriber channel for a specific event type.
func (b *EventBus) Subscribe(eventType string) SubscriptionChan {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(SubscriptionChan, 100) // buffered to prevent blocking callers
	b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	return ch
}

// Publish broadcasts an event asynchronously to all subscribers matching the event type.
func (b *EventBus) Publish(event A2AEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, exists := b.subscribers[event.EventType]
	if !exists {
		return
	}

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Non-blocking write: skips if the subscriber channel buffer is full
		}
	}
}

// Unsubscribe removes a channel subscription and closes the channel.
func (b *EventBus) Unsubscribe(eventType string, ch SubscriptionChan) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs, exists := b.subscribers[eventType]
	if !exists {
		return
	}

	for i, sub := range subs {
		if sub == ch {
			b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}
