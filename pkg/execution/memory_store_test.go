package execution

import (
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

func TestMemoryStoreCreateAndUpdateTodo(t *testing.T) {
	store := NewMemoryStore()
	dueAt := time.Now().UTC().Add(2 * time.Hour)
	if err := store.CreateTodo(Todo{
		ID:           "todo-1",
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		Title:        "Create roadmap",
		Description:  "Create the detailed roadmap",
		OwnerAgentID: "ceo",
		Priority:     missions.PriorityHigh,
		DueAt:        &dueAt,
	}); err != nil {
		t.Fatalf("CreateTodo returned error: %v", err)
	}

	todo, err := store.GetTodo("todo-1")
	if err != nil {
		t.Fatalf("GetTodo returned error: %v", err)
	}
	todo.Status = TodoStatusInProgress
	if err := store.UpdateTodo(todo); err != nil {
		t.Fatalf("UpdateTodo returned error: %v", err)
	}

	todos, err := store.ListTodos("mission-1")
	if err != nil {
		t.Fatalf("ListTodos returned error: %v", err)
	}
	if len(todos) != 1 || todos[0].Status != TodoStatusInProgress {
		t.Fatalf("unexpected todos after update: %#v", todos)
	}

	dueTodos, err := store.ListDueTodos(dueAt.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("ListDueTodos returned error: %v", err)
	}
	if len(dueTodos) != 1 || dueTodos[0].ID != "todo-1" {
		t.Fatalf("unexpected due todos: %#v", dueTodos)
	}
}

func TestMemoryStoreCreateAndUpdateTimer(t *testing.T) {
	store := NewMemoryStore()
	wakeAt := time.Now().UTC().Add(30 * time.Minute)
	if err := store.CreateTimer(Timer{
		ID:           "timer-1",
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       wakeAt,
		ActionType:   "status_check",
	}); err != nil {
		t.Fatalf("CreateTimer returned error: %v", err)
	}

	timer, err := store.GetTimer("timer-1")
	if err != nil {
		t.Fatalf("GetTimer returned error: %v", err)
	}
	timer.Status = TimerStatusTriggered
	now := time.Now().UTC()
	timer.TriggeredAt = &now
	if err := store.UpdateTimer(timer); err != nil {
		t.Fatalf("UpdateTimer returned error: %v", err)
	}

	timers, err := store.ListTimers("mission-1")
	if err != nil {
		t.Fatalf("ListTimers returned error: %v", err)
	}
	if len(timers) != 1 || timers[0].Status != TimerStatusTriggered {
		t.Fatalf("unexpected timers after update: %#v", timers)
	}

	dueTimers, err := store.ListDueTimers(wakeAt.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("ListDueTimers returned error: %v", err)
	}
	if len(dueTimers) != 0 {
		t.Fatalf("expected triggered timer to be excluded from due timers, got %#v", dueTimers)
	}
}

func TestMemoryStoreClaimDueTimers(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	for _, timer := range []Timer{
		{ID: "timer-due", MissionID: "mission-1", ThreadID: "thread-1", SetByAgentID: "ceo", WakeAt: now.Add(-time.Minute), ActionType: "status_check"},
		{ID: "timer-future", MissionID: "mission-1", ThreadID: "thread-1", SetByAgentID: "ceo", WakeAt: now.Add(time.Hour), ActionType: "later_check"},
	} {
		if err := store.CreateTimer(timer); err != nil {
			t.Fatalf("CreateTimer(%s) returned error: %v", timer.ID, err)
		}
	}

	claimed, err := store.ClaimDueTimers(now, 10)
	if err != nil {
		t.Fatalf("ClaimDueTimers returned error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != "timer-due" {
		t.Fatalf("unexpected claimed timers: %#v", claimed)
	}
	if claimed[0].Status != TimerStatusTriggered || claimed[0].TriggeredAt == nil {
		t.Fatalf("expected claimed timer to be triggered, got %#v", claimed[0])
	}

	claimedAgain, err := store.ClaimDueTimers(now, 10)
	if err != nil {
		t.Fatalf("second ClaimDueTimers returned error: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("expected no additional claims, got %#v", claimedAgain)
	}
}
