package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

func TestRuntimeTodoLifecycle(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	dueAt := time.Now().UTC().Add(time.Hour)
	todo, err := runtime.CreateTodo(CreateTodoInput{
		MissionID:     "mission-1",
		ThreadID:      "thread-1",
		Title:         "Create roadmap",
		Description:   "Create a detailed roadmap",
		OwnerAgentID:  "ceo",
		Priority:      missions.PriorityHigh,
		DueAt:         &dueAt,
		DependsOn:     []string{"todo-prep"},
		ArtifactPaths: []string{"docs/roadmap.md"},
	})
	if err != nil {
		t.Fatalf("CreateTodo returned error: %v", err)
	}

	todo, err = runtime.AssignTodo(todo.ID, "planner")
	if err != nil {
		t.Fatalf("AssignTodo returned error: %v", err)
	}
	if todo.OwnerAgentID != "planner" {
		t.Fatalf("expected planner owner, got %q", todo.OwnerAgentID)
	}
	todo, err = runtime.StartTodo(todo.ID)
	if err != nil {
		t.Fatalf("StartTodo returned error: %v", err)
	}
	if todo.Status != TodoStatusInProgress {
		t.Fatalf("expected in_progress status, got %q", todo.Status)
	}
	todo, err = runtime.BlockTodo(todo.ID)
	if err != nil {
		t.Fatalf("BlockTodo returned error: %v", err)
	}
	if todo.Status != TodoStatusBlocked {
		t.Fatalf("expected blocked status, got %q", todo.Status)
	}
	todo, err = runtime.CompleteTodo(todo.ID)
	if err != nil {
		t.Fatalf("CompleteTodo returned error: %v", err)
	}
	if todo.Status != TodoStatusDone {
		t.Fatalf("expected done status, got %q", todo.Status)
	}
}

func TestRuntimeTimerLifecycle(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	wakeAt := time.Now().UTC().Add(15 * time.Minute)
	timer, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       wakeAt,
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}
	dueTimers, err := runtime.ListDueTimers(wakeAt.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("ListDueTimers returned error: %v", err)
	}
	if len(dueTimers) != 1 || dueTimers[0].ID != timer.ID {
		t.Fatalf("unexpected due timers: %#v", dueTimers)
	}
	timer, err = runtime.TriggerTimer(timer.ID)
	if err != nil {
		t.Fatalf("TriggerTimer returned error: %v", err)
	}
	if timer.Status != TimerStatusTriggered || timer.TriggeredAt == nil {
		t.Fatalf("expected triggered timer with timestamp, got %#v", timer)
	}
	cancelled, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       wakeAt.Add(time.Hour),
		ActionType:   "escalate",
	})
	if err != nil {
		t.Fatalf("second ScheduleTimer returned error: %v", err)
	}
	cancelled, err = runtime.CancelTimer(cancelled.ID)
	if err != nil {
		t.Fatalf("CancelTimer returned error: %v", err)
	}
	if cancelled.Status != TimerStatusCancelled {
		t.Fatalf("expected cancelled timer status, got %#v", cancelled)
	}
}

func TestRuntimeClaimDueTimers(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	now := time.Now().UTC()
	due, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       now.Add(-time.Minute),
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer due returned error: %v", err)
	}
	_, err = runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       now.Add(time.Hour),
		ActionType:   "later_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer future returned error: %v", err)
	}

	claimed, err := runtime.ClaimDueTimers(now, 10)
	if err != nil {
		t.Fatalf("ClaimDueTimers returned error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != due.ID {
		t.Fatalf("unexpected claimed timers: %#v", claimed)
	}
	if claimed[0].Status != TimerStatusTriggered || claimed[0].TriggeredAt == nil {
		t.Fatalf("expected claimed timer to be triggered, got %#v", claimed[0])
	}

	again, err := runtime.ClaimDueTimers(now.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("second ClaimDueTimers returned error: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected due timer claim to be idempotent, got %#v", again)
	}
}

func TestTimerProcessorMarksFailedWhenHandlerErrors(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	timer, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       time.Now().UTC().Add(-time.Minute),
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}
	processor, err := NewTimerProcessor(runtime, func(_ context.Context, timer Timer) error {
		if timer.ID != "" {
			return fmt.Errorf("boom")
		}
		return nil
	}, TimerProcessorConfig{Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("NewTimerProcessor returned error: %v", err)
	}
	if err := processor.ProcessOnce(context.Background()); err == nil {
		t.Fatal("expected processor error")
	}
	stored, err := runtime.Store().GetTimer(timer.ID)
	if err != nil {
		t.Fatalf("GetTimer returned error: %v", err)
	}
	if stored.Status != TimerStatusFailed {
		t.Fatalf("expected failed timer after handler error, got %#v", stored)
	}
}

func TestTimerProcessorRunProcessesDueTimersImmediatelyOnStartup(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	_, err = runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       time.Now().UTC().Add(-time.Minute),
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}
	handled := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	processor, err := NewTimerProcessor(runtime, func(_ context.Context, timer Timer) error {
		handled <- timer.ID
		cancel()
		return nil
	}, TimerProcessorConfig{PollInterval: time.Hour})
	if err != nil {
		t.Fatalf("NewTimerProcessor returned error: %v", err)
	}
	if err := processor.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	select {
	case timerID := <-handled:
		if timerID == "" {
			t.Fatal("expected handled timer id")
		}
	default:
		t.Fatal("expected due timer to be processed immediately on processor startup")
	}
}

func TestTimerProcessorRunDoesNotReprocessClaimedTimersAfterRestart(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	timer, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       time.Now().UTC().Add(-time.Minute),
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}
	handled := 0
	processor, err := NewTimerProcessor(runtime, func(_ context.Context, _ Timer) error {
		handled++
		return nil
	}, TimerProcessorConfig{Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("NewTimerProcessor returned error: %v", err)
	}
	if err := processor.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("first ProcessOnce returned error: %v", err)
	}
	restarted, err := NewTimerProcessor(runtime, func(_ context.Context, _ Timer) error {
		handled++
		return nil
	}, TimerProcessorConfig{Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("second NewTimerProcessor returned error: %v", err)
	}
	if err := restarted.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("second ProcessOnce returned error: %v", err)
	}
	if handled != 1 {
		t.Fatalf("expected timer to be handled exactly once across restarts, got %d", handled)
	}
	stored, err := runtime.Store().GetTimer(timer.ID)
	if err != nil {
		t.Fatalf("GetTimer returned error: %v", err)
	}
	if stored.Status != TimerStatusTriggered {
		t.Fatalf("expected timer to remain triggered after restart recovery, got %#v", stored)
	}
}

func TestTimerProcessorProcessOnceContinuesAfterIndividualFailure(t *testing.T) {
	runtime, err := NewRuntime(NewMemoryStore())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	first, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       time.Now().UTC().Add(-time.Minute),
		ActionType:   "status_check",
	})
	if err != nil {
		t.Fatalf("first ScheduleTimer returned error: %v", err)
	}
	second, err := runtime.ScheduleTimer(ScheduleTimerInput{
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		SetByAgentID: "ceo",
		WakeAt:       time.Now().UTC().Add(-time.Minute),
		ActionType:   "follow_up",
	})
	if err != nil {
		t.Fatalf("second ScheduleTimer returned error: %v", err)
	}
	handled := map[string]int{}
	processor, err := NewTimerProcessor(runtime, func(_ context.Context, timer Timer) error {
		handled[timer.ID]++
		if timer.ID == first.ID {
			return fmt.Errorf("boom")
		}
		return nil
	}, TimerProcessorConfig{Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("NewTimerProcessor returned error: %v", err)
	}
	if err := processor.ProcessOnce(context.Background()); err == nil {
		t.Fatal("expected aggregated processor error")
	}
	if handled[first.ID] != 1 || handled[second.ID] != 1 {
		t.Fatalf("expected both claimed timers to be attempted once, got %#v", handled)
	}
	storedFirst, err := runtime.Store().GetTimer(first.ID)
	if err != nil {
		t.Fatalf("GetTimer first returned error: %v", err)
	}
	if storedFirst.Status != TimerStatusFailed {
		t.Fatalf("expected failed first timer, got %#v", storedFirst)
	}
	storedSecond, err := runtime.Store().GetTimer(second.ID)
	if err != nil {
		t.Fatalf("GetTimer second returned error: %v", err)
	}
	if storedSecond.Status != TimerStatusTriggered {
		t.Fatalf("expected successful second timer to remain triggered, got %#v", storedSecond)
	}
}
