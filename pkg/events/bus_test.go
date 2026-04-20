package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus(100)
	ch := bus.Subscribe("test-sub", nil, 10)

	bus.Publish(Event{ID: "evt-1", Type: EventMissionStarted, MissionID: "m-1"})

	select {
	case evt := <-ch:
		if evt.ID != "evt-1" {
			t.Errorf("expected evt-1, got %q", evt.ID)
		}
		if evt.Type != EventMissionStarted {
			t.Errorf("expected mission.started, got %q", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBus_FilteredSubscription(t *testing.T) {
	bus := NewBus(100)
	ch := bus.Subscribe("mission-only", func(e Event) bool {
		return e.MissionID == "m-target"
	}, 10)

	bus.Publish(Event{ID: "evt-1", Type: EventMissionStarted, MissionID: "m-other"})
	bus.Publish(Event{ID: "evt-2", Type: EventMissionCompleted, MissionID: "m-target"})

	select {
	case evt := <-ch:
		if evt.ID != "evt-2" {
			t.Errorf("expected evt-2, got %q — filter should have blocked evt-1", evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for filtered event")
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus(100)
	ch := bus.Subscribe("unsub-test", nil, 10)
	bus.Unsubscribe("unsub-test")
	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("expected closed channel after unsubscribe")
	}
}

func TestBus_RecentEvents(t *testing.T) {
	bus := NewBus(100)
	for i := 0; i < 5; i++ {
		bus.Publish(Event{
			ID:        string(rune('a' + i)),
			Type:      EventTodoCreated,
			MissionID: "m-1",
		})
	}
	events := bus.RecentEvents(3)
	if len(events) != 3 {
		t.Fatalf("expected 3, got %d", len(events))
	}
	// Newest first.
	if events[0].ID != "e" {
		t.Errorf("expected 'e' (newest), got %q", events[0].ID)
	}
}

func TestBus_RecentEventsByMission(t *testing.T) {
	bus := NewBus(100)
	bus.Publish(Event{ID: "1", Type: EventTodoCreated, MissionID: "m-A"})
	bus.Publish(Event{ID: "2", Type: EventTodoCreated, MissionID: "m-B"})
	bus.Publish(Event{ID: "3", Type: EventTodoCompleted, MissionID: "m-A"})

	events := bus.RecentEventsByMission("m-A", 10)
	if len(events) != 2 {
		t.Fatalf("expected 2, got %d", len(events))
	}
}

func TestBus_HistoryLimit(t *testing.T) {
	bus := NewBus(3)
	for i := 0; i < 10; i++ {
		bus.Publish(Event{ID: string(rune('0' + i)), Type: EventAgentHeartbeat})
	}
	events := bus.RecentEvents(100)
	if len(events) != 3 {
		t.Fatalf("expected 3 (history max), got %d", len(events))
	}
}

func TestBus_EventWithPayload(t *testing.T) {
	bus := NewBus(10)
	ch := bus.Subscribe("payload-test", nil, 10)

	payload, _ := json.Marshal(map[string]string{"action": "test"})
	bus.Publish(Event{
		ID:      "p-1",
		Type:    EventTeamApproved,
		Payload: payload,
	})

	evt := <-ch
	var data map[string]string
	if err := json.Unmarshal(evt.Payload, &data); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if data["action"] != "test" {
		t.Errorf("expected action=test, got %q", data["action"])
	}
}
