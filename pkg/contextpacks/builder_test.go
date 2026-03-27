package contextpacks

import (
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestNewBuilderRequiresAllStores(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()

	if _, err := NewBuilder(nil, threadStore, missionStateStore, executionStore, nil); err == nil {
		t.Fatal("expected mission store requirement error")
	}
	if _, err := NewBuilder(missionStore, nil, missionStateStore, executionStore, nil); err == nil {
		t.Fatal("expected thread store requirement error")
	}
	if _, err := NewBuilder(missionStore, threadStore, nil, executionStore, nil); err == nil {
		t.Fatal("expected mission state store requirement error")
	}
	if _, err := NewBuilder(missionStore, threadStore, missionStateStore, nil, nil); err == nil {
		t.Fatal("expected execution store requirement error")
	}
}

func TestBuildRootCEOPackIncludesSummaryRollupsAndRecentMessages(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	builder, err := NewBuilder(missionStore, threadStore, missionStateStore, executionStore, nil)
	if err != nil {
		t.Fatalf("NewBuilder returned error: %v", err)
	}

	if err := missionStore.CreateProgram(missions.Program{ID: "program-1", ClientID: "client-1", Title: "Cloud replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if err := missionStore.CreateMission(missions.Mission{
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
	}); err != nil {
		t.Fatalf("CreateMission root returned error: %v", err)
	}
	if err := missionStore.CreateMission(missions.Mission{
		ID:              "mission-network",
		ProgramID:       "program-1",
		ParentMissionID: "mission-root",
		RootMissionID:   "mission-root",
		OwningThreadID:  "thread-network",
		OwnerAgentID:    "ceo-network",
		OwnerRole:       "CEO",
		MissionType:     "domain",
		Title:           "Networking",
		Charter:         "Own networking domain",
		Goal:            "Build networking",
		Scope:           "Networking only",
		AuthorityLevel:  "domain",
		Status:          missions.MissionStatusActive,
	}); err != nil {
		t.Fatalf("CreateMission child returned error: %v", err)
	}

	if err := threadStore.CreateThread(threads.Thread{
		ID:            "thread-root",
		MissionID:     "mission-root",
		RootMissionID: "mission-root",
		Kind:          "strategy",
		Title:         "Root CEO thread",
		Summary:       "Top-level CEO thread",
		Context:       "Owns the main client-facing strategy lane.",
		OwnerAgentID:  "ceo-root",
		Status:        threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread root returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:             "thread-network",
		MissionID:      "mission-network",
		RootMissionID:  "mission-root",
		ParentThreadID: "thread-root",
		Kind:           "strategy",
		Title:          "Networking CEO thread",
		Summary:        "Networking mission thread",
		Context:        "Owns networking strategy and execution supervision.",
		OwnerAgentID:   "ceo-network",
		Status:         threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread child returned error: %v", err)
	}

	baseTime := time.Now().UTC()
	for i, content := range []string{"one", "two", "three"} {
		if err := threadStore.AppendMessage(threads.Message{
			ID:            "msg-" + content,
			ThreadID:      "thread-root",
			Role:          threads.RoleUser,
			AuthorAgentID: "user",
			AuthorRole:    "client",
			MessageType:   "client_message",
			Content:       content,
			CreatedAt:     baseTime.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("AppendMessage returned error: %v", err)
		}
	}

	if err := missionStateStore.CreateSummary(missionstate.Summary{
		ID:               "summary-1",
		MissionID:        "mission-root",
		ThreadID:         "thread-root",
		Level:            "mission",
		Kind:             "rolling",
		CoverageStartRef: "msg-one",
		CoverageEndRef:   "msg-two",
		SummaryText:      "Initial exploration complete.",
	}); err != nil {
		t.Fatalf("CreateSummary returned error: %v", err)
	}
	if err := missionStateStore.CreateSummary(missionstate.Summary{
		ID:               "summary-2",
		MissionID:        "mission-root",
		ThreadID:         "thread-root",
		Level:            "mission",
		Kind:             "checkpoint",
		CoverageStartRef: "msg-two",
		CoverageEndRef:   "msg-three",
		SummaryText:      "Scope narrowed to initial cloud pillars.",
	}); err != nil {
		t.Fatalf("CreateSummary second returned error: %v", err)
	}

	nextUpdate := baseTime.Add(2 * time.Hour)
	if err := missionStateStore.UpsertRollup(missionstate.Rollup{
		ID:                   "rollup-1",
		ParentMissionID:      "mission-root",
		ChildMissionID:       "mission-network",
		Status:               missions.MissionStatusActive,
		ProgressPercent:      35,
		Health:               "green",
		CurrentBlocker:       "",
		LatestSummary:        "Networking mission has decomposed VPC and firewall work.",
		NextExpectedUpdateAt: &nextUpdate,
	}); err != nil {
		t.Fatalf("UpsertRollup returned error: %v", err)
	}

	dueAt := baseTime.Add(-15 * time.Minute)
	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-root-due",
		MissionID:    "mission-root",
		ThreadID:     "thread-root",
		Title:        "Review strategy gaps",
		Description:  "Review the open strategy questions with the client.",
		OwnerAgentID: "ceo-root",
		Priority:     missions.PriorityHigh,
		Status:       execution.TodoStatusTodo,
		DueAt:        &dueAt,
	}); err != nil {
		t.Fatalf("CreateTodo returned error: %v", err)
	}
	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-other-due",
		MissionID:    "mission-network",
		ThreadID:     "thread-network",
		Title:        "Child due todo",
		Description:  "Should not appear in root context pack due list.",
		OwnerAgentID: "ceo-network",
		Priority:     missions.PriorityMedium,
		Status:       execution.TodoStatusTodo,
		DueAt:        &dueAt,
	}); err != nil {
		t.Fatalf("CreateTodo other returned error: %v", err)
	}
	if err := executionStore.CreateTimer(execution.Timer{
		ID:           "timer-root-due",
		MissionID:    "mission-root",
		ThreadID:     "thread-root",
		SetByAgentID: "ceo-root",
		WakeAt:       baseTime.Add(-5 * time.Minute),
		ActionType:   "status_check",
	}); err != nil {
		t.Fatalf("CreateTimer returned error: %v", err)
	}

	pack, err := builder.BuildRootCEOPack("mission-root", BuildOptions{RecentMessagesLimit: 2, IncludeChildRollups: true})
	if err != nil {
		t.Fatalf("BuildRootCEOPack returned error: %v", err)
	}
	if pack.Mission.ID != "mission-root" {
		t.Fatalf("expected mission-root, got %q", pack.Mission.ID)
	}
	if pack.Thread.ID != "thread-root" {
		t.Fatalf("expected thread-root, got %q", pack.Thread.ID)
	}
	if pack.LatestSummary == nil || pack.LatestSummary.ID != "summary-2" {
		t.Fatalf("expected latest summary summary-2, got %#v", pack.LatestSummary)
	}
	if len(pack.ChildRollups) != 1 {
		t.Fatalf("expected 1 child rollup, got %d", len(pack.ChildRollups))
	}
	if len(pack.RecentMessages) != 2 {
		t.Fatalf("expected 2 recent messages, got %d", len(pack.RecentMessages))
	}
	if pack.RecentMessages[0].Content != "two" || pack.RecentMessages[1].Content != "three" {
		t.Fatalf("unexpected recent messages: %#v", pack.RecentMessages)
	}
	if len(pack.DueTodos) != 1 || pack.DueTodos[0].ID != "todo-root-due" {
		t.Fatalf("expected only mission-root due todo, got %#v", pack.DueTodos)
	}
	if len(pack.DueTimers) != 1 || pack.DueTimers[0].ID != "timer-root-due" {
		t.Fatalf("expected mission-root due timer, got %#v", pack.DueTimers)
	}
}

func TestBuildMissionPackUsesOwningThreadWhenThreadIDMissing(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	builder, err := NewBuilder(missionStore, threadStore, missionStateStore, executionStore, nil)
	if err != nil {
		t.Fatalf("NewBuilder returned error: %v", err)
	}

	if err := missionStore.CreateProgram(missions.Program{ID: "program-1", ClientID: "client-1", Title: "Cloud replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if err := missionStore.CreateMission(missions.Mission{
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
	}); err != nil {
		t.Fatalf("CreateMission returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:            "thread-root",
		MissionID:     "mission-root",
		RootMissionID: "mission-root",
		Kind:          "strategy",
		Title:         "Root CEO thread",
		Summary:       "Top-level CEO thread",
		Context:       "Owns the main client-facing strategy lane.",
		OwnerAgentID:  "ceo-root",
		Status:        threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}

	pack, err := builder.BuildMissionPack("mission-root", "", BuildOptions{})
	if err != nil {
		t.Fatalf("BuildMissionPack returned error: %v", err)
	}
	if pack.Thread.ID != "thread-root" {
		t.Fatalf("expected owning thread thread-root, got %q", pack.Thread.ID)
	}
}
