package missionstate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestRuntimeRefreshMissionStateGeneratesSummaryAndParentRollup(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	stateStore := NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	runtime, err := NewRuntime(stateStore, missionStore, threadStore, executionStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if err := missionStore.CreateProgram(missions.Program{ID: "program-1", ClientID: "client-1", Title: "Cloud replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	for _, mission := range []missions.Mission{
		{
			ID:             "mission-root",
			ProgramID:      "program-1",
			RootMissionID:  "mission-root",
			OwningThreadID: "thread-root",
			OwnerAgentID:   "ceo-root",
			OwnerRole:      "CEO",
			MissionType:    "root",
			Title:          "Build cloud",
			Charter:        "Own full cloud strategy",
			Goal:           "Deliver cloud platform",
			Scope:          "Top-level program",
			AuthorityLevel: "global",
			Status:         missions.MissionStatusActive,
		},
		{
			ID:              "mission-network",
			ProgramID:       "program-1",
			ParentMissionID: "mission-root",
			RootMissionID:   "mission-root",
			OwningThreadID:  "thread-network",
			OwnerAgentID:    "sub-ceo-networking",
			OwnerRole:       "sub_ceo",
			MissionType:     "domain",
			Title:           "Networking foundation",
			Charter:         "Own the networking base layer",
			Goal:            "Deliver networking foundations",
			Scope:           "Routing and firewall",
			AuthorityLevel:  "domain",
			Status:          missions.MissionStatusActive,
			ProgressPercent: 15,
		},
	} {
		if err := missionStore.CreateMission(mission); err != nil {
			t.Fatalf("CreateMission(%s) returned error: %v", mission.ID, err)
		}
	}
	for _, thread := range []threads.Thread{
		{ID: "thread-root", MissionID: "mission-root", RootMissionID: "mission-root", Kind: "strategy", Title: "Root thread", Summary: "Root summary", Context: "Root context", OwnerAgentID: "ceo-root", Status: threads.ThreadStatusActive},
		{ID: "thread-network", MissionID: "mission-network", RootMissionID: "mission-root", ParentThreadID: "thread-root", Kind: "strategy", Title: "Networking thread", Summary: "Networking summary", Context: "Networking context", OwnerAgentID: "sub-ceo-networking", Status: threads.ThreadStatusActive},
	} {
		if err := threadStore.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread(%s) returned error: %v", thread.ID, err)
		}
	}
	for _, message := range []threads.Message{
		{ID: "msg-1", ThreadID: "thread-network", Role: threads.RoleAssistant, AuthorAgentID: "sub-ceo-networking", AuthorRole: "sub_ceo", MessageType: "ceo_message", Content: "Networking mission is decomposed into VPC and firewall tracks."},
		{ID: "msg-2", ThreadID: "thread-network", Role: threads.RoleAssistant, AuthorAgentID: "sub-ceo-networking", AuthorRole: "sub_ceo", MessageType: "ceo_message", Content: "Next step is execution planning for firewall policy."},
	} {
		if err := threadStore.AppendMessage(message); err != nil {
			t.Fatalf("AppendMessage(%s) returned error: %v", message.ID, err)
		}
	}

	dueAt := time.Now().UTC().Add(-10 * time.Minute)
	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-network-done",
		MissionID:    "mission-network",
		ThreadID:     "thread-network",
		Title:        "Ship VPC draft",
		Description:  "Completed the VPC draft.",
		OwnerAgentID: "sub-ceo-networking",
		Priority:     missions.PriorityMedium,
		Status:       execution.TodoStatusDone,
	}); err != nil {
		t.Fatalf("CreateTodo done returned error: %v", err)
	}
	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-network-due",
		MissionID:    "mission-network",
		ThreadID:     "thread-network",
		Title:        "Validate firewall dependencies",
		Description:  "Produce the overdue firewall execution plan.",
		OwnerAgentID: "sub-ceo-networking",
		Priority:     missions.PriorityHigh,
		Status:       execution.TodoStatusBlocked,
		DueAt:        &dueAt,
	}); err != nil {
		t.Fatalf("CreateTodo returned error: %v", err)
	}
	if err := executionStore.CreateTimer(execution.Timer{
		ID:           "timer-network-due",
		MissionID:    "mission-network",
		ThreadID:     "thread-network",
		SetByAgentID: "sub-ceo-networking",
		WakeAt:       time.Now().UTC().Add(-5 * time.Minute),
		ActionType:   "follow_up",
	}); err != nil {
		t.Fatalf("CreateTimer returned error: %v", err)
	}

	summary, rollup, err := runtime.RefreshMissionState("mission-network", "thread-network")
	if err != nil {
		t.Fatalf("RefreshMissionState returned error: %v", err)
	}
	if summary.MissionID != "mission-network" || summary.ThreadID != "thread-network" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.CoverageStartRef != "msg-1" || summary.CoverageEndRef != "msg-2" {
		t.Fatalf("expected message coverage refs, got start=%q end=%q", summary.CoverageStartRef, summary.CoverageEndRef)
	}
	if rollup == nil {
		t.Fatal("expected parent rollup to be published")
	}
	if rollup.ParentMissionID != "mission-root" || rollup.ChildMissionID != "mission-network" {
		t.Fatalf("unexpected rollup linkage: %#v", rollup)
	}
	if rollup.LatestSummary == "" {
		t.Fatalf("expected rollup latest summary, got %#v", rollup)
	}
	if rollup.Health != "red" {
		t.Fatalf("expected red health, got %q", rollup.Health)
	}
	if rollup.ProgressPercent != 50 {
		t.Fatalf("expected execution-derived 50%% progress, got %v", rollup.ProgressPercent)
	}
	if rollup.CurrentBlocker != "Blocked todo: Validate firewall dependencies" {
		t.Fatalf("unexpected current blocker: %q", rollup.CurrentBlocker)
	}
	var overdueFlags []string
	if err := json.Unmarshal(rollup.OverdueFlags, &overdueFlags); err != nil {
		t.Fatalf("unmarshal overdue flags: %v", err)
	}
	if len(overdueFlags) != 2 || overdueFlags[0] != "todo_due" || overdueFlags[1] != "timer_due" {
		t.Fatalf("unexpected overdue flags: %#v", overdueFlags)
	}
	var executionSummary map[string]any
	if err := json.Unmarshal(rollup.ExecutionSummary, &executionSummary); err != nil {
		t.Fatalf("unmarshal execution summary: %v", err)
	}
	for key, want := range map[string]float64{
		"totalTodos":      2,
		"openTodos":       1,
		"blockedTodos":    1,
		"doneTodos":       1,
		"dueTodos":        1,
		"scheduledTimers": 1,
		"dueTimers":       1,
	} {
		got, ok := executionSummary[key].(float64)
		if !ok || got != want {
			t.Fatalf("unexpected execution summary %s: %#v", key, executionSummary[key])
		}
	}
}
