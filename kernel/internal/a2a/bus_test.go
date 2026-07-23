package a2a

import (
	"testing"
	"time"
)

func TestEventBus_PubSub(t *testing.T) {
	bus := NewEventBus()

	// Subscribe to a topic
	ch := bus.Subscribe("STATE_CHANGED")

	// Publish event
	event := A2AEvent{
		EventID:   "evt_1",
		EventType: "STATE_CHANGED",
		Sender:    "test_runner",
		Payload:   "Planning",
	}
	bus.Publish(event)

	// Verify receipt
	select {
	case received := <-ch:
		if received.EventID != "evt_1" {
			t.Errorf("expected event_id 'evt_1', got '%s'", received.EventID)
		}
		if received.Payload != "Planning" {
			t.Errorf("expected payload 'Planning', got '%v'", received.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event message")
	}

	// Unsubscribe and check close
	bus.Unsubscribe("STATE_CHANGED", ch)

	// Verify channel is closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("expected subscription channel to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}
