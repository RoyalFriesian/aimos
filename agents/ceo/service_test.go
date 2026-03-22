package ceo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func decodePayload(t *testing.T, response ResponseEnvelope) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(response.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

type stubCompletionClient struct {
	responses []string
	calls     int
	prompts   []string
	messages  [][]threads.Message
}

func (s *stubCompletionClient) Generate(_ context.Context, _ string, systemPrompt string, _ string) (string, error) {
	s.prompts = append(s.prompts, systemPrompt)
	response := s.responses[s.calls]
	s.calls++
	return response, nil
}

func (s *stubCompletionClient) GenerateFromMessages(_ context.Context, _ string, messages []threads.Message) (string, error) {
	cloned := make([]threads.Message, len(messages))
	copy(cloned, messages)
	s.messages = append(s.messages, cloned)

	response := s.responses[s.calls]
	s.calls++
	return response, nil
}

func newTestService(t *testing.T, stub *stubCompletionClient) (*Service, *missions.MemoryStore, *threads.MemoryStore, *missionstate.MemoryStore, *execution.MemoryStore) {
	t.Helper()

	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	feedbackStore := feedback.NewMemoryStore()
	service, err := NewService(
		Config{APIKey: "test-key", Model: "gpt-5.4"},
		stub,
		missionStore,
		threadStore,
		missionStateStore,
		executionStore,
		feedbackStore,
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return service, missionStore, threadStore, missionStateStore, executionStore
}

func seedMissionThread(t *testing.T, missionStore *missions.MemoryStore, threadStore *threads.MemoryStore, missionID string, threadID string) {
	t.Helper()

	programID := "program-" + missionID
	if err := missionStore.CreateProgram(missions.Program{ID: programID, ClientID: "client-1", Title: "Program " + missionID}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if err := missionStore.CreateMission(missions.Mission{
		ID:             missionID,
		ProgramID:      programID,
		RootMissionID:  missionID,
		OwningThreadID: threadID,
		OwnerAgentID:   "ceo-root",
		OwnerRole:      "CEO",
		MissionType:    "root",
		Title:          "Mission " + missionID,
		Charter:        "Own the mission",
		Goal:           "Deliver the mission",
		Scope:          "Root mission scope",
		AuthorityLevel: "global",
		Status:         missions.MissionStatusActive,
	}); err != nil {
		t.Fatalf("CreateMission returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:            threadID,
		MissionID:     missionID,
		RootMissionID: missionID,
		Kind:          "strategy",
		Title:         "Thread " + threadID,
		Summary:       "Mission thread",
		Context:       "Owns the main CEO conversation lane.",
		OwnerAgentID:  "ceo-root",
		Status:        threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
}

func TestServiceRespondUsesContextMode(t *testing.T) {
	resetModes(t)

	service, _, _, _, _ := newTestService(t, &stubCompletionClient{responses: []string{"Discovery response"}})

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "Help me define this product.",
		Context:  contextPayload,
		ThreadID: "thread-1",
		TraceID:  "trace-1",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.Mode != ModeDiscovery {
		t.Fatalf("expected discovery mode, got %q", response.Mode)
	}
}

func TestServiceRespondClassifiesMode(t *testing.T) {
	resetModes(t)

	service, _, _, _, _ := newTestService(t, &stubCompletionClient{responses: []string{"alignment", "Alignment response"}})

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "We have an idea but are not aligned on scope.",
		ThreadID: "thread-1",
		TraceID:  "trace-1",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.Mode != ModeAlignment {
		t.Fatalf("expected alignment mode, got %q", response.Mode)
	}
	if !response.RatingPrompt.Enabled {
		t.Fatal("expected rating prompt to be enabled")
	}
}

func TestServiceRespondLoadsSystemPromptFromJSONConfig(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Discovery response"}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "Help me define this product.",
		Context:  contextPayload,
		ThreadID: "thread-1",
		TraceID:  "trace-1",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if len(stub.messages) != 1 {
		t.Fatalf("expected one conversation call, got %d", len(stub.messages))
	}
	if len(stub.messages[0]) == 0 {
		t.Fatal("expected conversation messages to include a system prompt")
	}
	if !strings.Contains(stub.messages[0][0].Content, "discovery mode") {
		t.Fatalf("expected discovery system prompt from JSON config, got %q", stub.messages[0][0].Content)
	}
	if !strings.Contains(stub.messages[0][1].Content, "Mission:") {
		t.Fatalf("expected durable mission context in second system message, got %q", stub.messages[0][1].Content)
	}
	if !strings.Contains(stub.messages[0][1].Content, "Due Todos:") || !strings.Contains(stub.messages[0][1].Content, "Due Timers:") {
		t.Fatalf("expected execution context sections in second system message, got %q", stub.messages[0][1].Content)
	}
	if !strings.Contains(stub.messages[0][0].Content, "Respond as JSON") {
		t.Fatalf("expected structured output instruction in system prompt, got %q", stub.messages[0][0].Content)
	}
}

func TestFormatContextPackIncludesDueExecutionAndRollupFlags(t *testing.T) {
	dueAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	text := formatContextPack(contextpacks.ContextPack{
		Mission: missions.Mission{
			ID:              "mission-root",
			Title:           "Mission root",
			MissionType:     "root",
			Goal:            "Deliver the product",
			Scope:           "Top level",
			AuthorityLevel:  "global",
			Status:          missions.MissionStatusActive,
			ProgressPercent: 50,
		},
		Thread: threads.Thread{
			Title:   "Root thread",
			Summary: "Main summary",
			Context: "Main context",
		},
		ChildRollups: []missionstate.Rollup{{
			ChildMissionID:   "mission-child",
			Status:           missions.MissionStatusBlocked,
			ProgressPercent:  25,
			Health:           "red",
			CurrentBlocker:   "Waiting on API access",
			LatestSummary:    "Blocked pending credentials.",
			OverdueFlags:     json.RawMessage(`["todo_due","timer_due"]`),
			ExecutionSummary: json.RawMessage(`{"totalTodos":4,"openTodos":2,"inProgressTodos":1,"blockedTodos":1,"doneTodos":2,"dueTodos":1,"scheduledTimers":2,"dueTimers":1,"nextTimerAt":"2025-01-02T03:04:05Z"}`),
		}},
		DueTodos: []execution.Todo{{
			Title:        "Review backlog",
			Status:       execution.TodoStatusBlocked,
			Priority:     missions.PriorityCritical,
			OwnerAgentID: "ceo-root",
			DueAt:        &dueAt,
		}},
		DueTimers: []execution.Timer{{
			ActionType:   "follow_up",
			WakeAt:       dueAt,
			SetByAgentID: "ceo-root",
			Status:       execution.TimerStatusScheduled,
		}},
	})

	for _, fragment := range []string{
		"Overdue: todo_due, timer_due",
		"Execution: todos total=4 open=2 in_progress=1 blocked=1 done=2 due=1; timers scheduled=2 due=1; next_timer=2025-01-02T03:04:05Z",
		"Due Todos:",
		"Review backlog | Status: blocked | Priority: critical | Owner: ceo-root | Due: 2025-01-02T03:04:05Z",
		"Due Timers:",
		"follow_up | Wake: 2025-01-02T03:04:05Z | Set By: ceo-root | Status: scheduled",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected formatted context to contain %q, got %q", fragment, text)
		}
	}
}

func TestServiceHandleTriggeredTimerAppendsMissionThreadMessage(t *testing.T) {
	resetModes(t)
	service, missionStore, threadStore, missionStateStore, executionStore := newTestService(t, &stubCompletionClient{responses: []string{"Discovery response"}})
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	executionRuntime, err := execution.NewRuntime(executionStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	timer, err := executionRuntime.ScheduleTimer(execution.ScheduleTimerInput{
		MissionID:     "mission-root",
		ThreadID:      "thread-root",
		SetByAgentID:  "ceo-root",
		WakeAt:        time.Now().UTC().Add(-time.Minute),
		ActionType:    "status_check",
		ActionPayload: json.RawMessage(`{"reason":"check mission progress"}`),
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}

	if err := service.handleTriggeredTimer(context.Background(), timer); err != nil {
		t.Fatalf("handleTriggeredTimer returned error: %v", err)
	}
	history, err := service.threadStore.ListMessages("thread-root")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 1 || !strings.Contains(history[0].Content, "Reason: check mission progress") {
		t.Fatalf("expected triggered timer message, got %#v", history)
	}
	latestSummary, err := missionStateStore.GetLatestSummary("mission-root")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if !strings.Contains(latestSummary.SummaryText, "Timer triggered: status_check") {
		t.Fatalf("expected timer trigger to refresh mission summary, got %q", latestSummary.SummaryText)
	}
}

func TestServiceHandleTriggeredEscalationBlocksMissionAndPublishesRollup(t *testing.T) {
	resetModes(t)
	service, missionStore, threadStore, missionStateStore, executionStore := newTestService(t, &stubCompletionClient{responses: []string{"Discovery response"}})

	if err := missionStore.CreateProgram(missions.Program{ID: "program-root", ClientID: "client-1", Title: "Program root"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	for _, mission := range []missions.Mission{
		{
			ID:             "mission-root",
			ProgramID:      "program-root",
			RootMissionID:  "mission-root",
			OwningThreadID: "thread-root",
			OwnerAgentID:   "ceo-root",
			OwnerRole:      "CEO",
			MissionType:    "root",
			Title:          "Root mission",
			Charter:        "Own the program",
			Goal:           "Deliver the program",
			Scope:          "Root scope",
			AuthorityLevel: "global",
			Status:         missions.MissionStatusActive,
		},
		{
			ID:              "mission-child",
			ProgramID:       "program-root",
			ParentMissionID: "mission-root",
			RootMissionID:   "mission-root",
			OwningThreadID:  "thread-child",
			OwnerAgentID:    "sub-ceo",
			OwnerRole:       "sub_ceo",
			MissionType:     "domain",
			Title:           "Child mission",
			Charter:         "Own the child mission",
			Goal:            "Deliver child scope",
			Scope:           "Child scope",
			AuthorityLevel:  "domain",
			Status:          missions.MissionStatusActive,
		},
	} {
		if err := missionStore.CreateMission(mission); err != nil {
			t.Fatalf("CreateMission(%s) returned error: %v", mission.ID, err)
		}
	}
	for _, thread := range []threads.Thread{
		{ID: "thread-root", MissionID: "mission-root", RootMissionID: "mission-root", Kind: "strategy", Title: "Root thread", Summary: "Root summary", Context: "Root context", OwnerAgentID: "ceo-root", Status: threads.ThreadStatusActive},
		{ID: "thread-child", MissionID: "mission-child", RootMissionID: "mission-root", ParentThreadID: "thread-root", Kind: "execution", Title: "Child thread", Summary: "Child summary", Context: "Child context", OwnerAgentID: "sub-ceo", Status: threads.ThreadStatusActive},
	} {
		if err := threadStore.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread(%s) returned error: %v", thread.ID, err)
		}
	}

	executionRuntime, err := execution.NewRuntime(executionStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	timer, err := executionRuntime.ScheduleTimer(execution.ScheduleTimerInput{
		MissionID:     "mission-child",
		ThreadID:      "thread-child",
		SetByAgentID:  "ceo-root",
		WakeAt:        time.Now().UTC().Add(-time.Minute),
		ActionType:    "escalate",
		ActionPayload: json.RawMessage(`{"reason":"blocker unresolved"}`),
	})
	if err != nil {
		t.Fatalf("ScheduleTimer returned error: %v", err)
	}

	if err := service.handleTriggeredTimer(context.Background(), timer); err != nil {
		t.Fatalf("handleTriggeredTimer returned error: %v", err)
	}
	mission, err := missionStore.GetMission("mission-child")
	if err != nil {
		t.Fatalf("GetMission returned error: %v", err)
	}
	if mission.Status != missions.MissionStatusBlocked {
		t.Fatalf("expected escalated mission to be blocked, got %q", mission.Status)
	}
	history, err := service.threadStore.ListMessages("thread-child")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 1 || history[0].MessageType != "timer_escalated" || !strings.Contains(history[0].Content, "blocker unresolved") {
		t.Fatalf("expected escalation message, got %#v", history)
	}
	rollup, err := missionStateStore.GetRollup("mission-root", "mission-child")
	if err != nil {
		t.Fatalf("GetRollup returned error: %v", err)
	}
	if rollup.Status != missions.MissionStatusBlocked {
		t.Fatalf("expected blocked rollup status, got %q", rollup.Status)
	}
	if rollup.Health != "red" {
		t.Fatalf("expected red rollup health, got %q", rollup.Health)
	}
	if !strings.Contains(rollup.CurrentBlocker, "blocker unresolved") {
		t.Fatalf("expected escalation blocker to flow into rollup, got %q", rollup.CurrentBlocker)
	}
}

func TestServiceRespondBuildsStructuredDiscoveryPayloadFromJSON(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{`{"message":"We should clarify the ICP before solutioning.","assumptions":["B2B SaaS motion"],"gaps":["No user segment defined"],"accessNeeds":["Current sales calls","Existing analytics"],"ambitionLevel":"durable","successCriteria":["Qualified discovery brief approved"],"nextQuestions":["Who is the first buyer?"]}`}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "Help me shape this product.",
		Context:  contextPayload,
		ThreadID: "thread-structured-discovery",
		TraceID:  "trace-structured-discovery",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	payload := decodePayload(t, response)
	if payload["message"] != "We should clarify the ICP before solutioning." {
		t.Fatalf("unexpected payload message: %#v", payload["message"])
	}
	if payload["ambitionLevel"] != "durable" {
		t.Fatalf("unexpected ambition level: %#v", payload["ambitionLevel"])
	}
	assumptions, ok := payload["assumptions"].([]any)
	if !ok || len(assumptions) != 1 || assumptions[0] != "B2B SaaS motion" {
		t.Fatalf("unexpected assumptions: %#v", payload["assumptions"])
	}
	history, err := service.threadStore.ListMessages("thread-structured-discovery")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if history[1].Content != "We should clarify the ICP before solutioning." {
		t.Fatalf("expected persisted assistant message to use parsed message text, got %q", history[1].Content)
	}
}

func TestServiceRespondBuildsStructuredAlignmentPayloadFromJSON(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{`{"message":"Start with a durable admin workflow before chasing edge cases.","recommendedScopePosture":"durable","tradeoffs":["slower initial release","cleaner operations later"],"decisionPoints":["single-tenant vs multi-tenant"],"accessNeeds":["customer ops walkthrough"],"risks":["scope drift"],"nextActions":["confirm tenancy model"]}`}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeAlignment})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "Help me align on the right product scope.",
		Context:  contextPayload,
		ThreadID: "thread-structured-alignment",
		TraceID:  "trace-structured-alignment",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	payload := decodePayload(t, response)
	if payload["recommendedScopePosture"] != "durable" {
		t.Fatalf("unexpected scope posture: %#v", payload["recommendedScopePosture"])
	}
	nextActions, ok := payload["nextActions"].([]any)
	if !ok || len(nextActions) != 1 || nextActions[0] != "confirm tenancy model" {
		t.Fatalf("unexpected next actions: %#v", payload["nextActions"])
	}
}

func TestServiceRespondRefreshesMissionSummaryAfterEveryTurn(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Discovery response with clear next step."}}
	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "Help me decide what to build first.",
		Context:  contextPayload,
		ThreadID: "thread-root",
		TraceID:  "trace-root-summary-refresh",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	latestSummary, err := missionStateStore.GetLatestSummary("mission-root")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if latestSummary.ThreadID != "thread-root" {
		t.Fatalf("expected summary thread thread-root, got %q", latestSummary.ThreadID)
	}
	if !strings.Contains(latestSummary.SummaryText, "Discovery response with clear next step") {
		t.Fatalf("expected summary text to include latest CEO response, got %q", latestSummary.SummaryText)
	}
	history, err := service.threadStore.ListMessages("thread-root")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if latestSummary.CoverageEndRef != history[len(history)-1].ID {
		t.Fatalf("expected summary coverage to end at the latest assistant message, got %q", latestSummary.CoverageEndRef)
	}
	rootRollups, err := missionStateStore.ListRollups("mission-root")
	if err != nil {
		t.Fatalf("ListRollups returned error: %v", err)
	}
	if len(rootRollups) != 0 {
		t.Fatalf("expected root mission response to avoid publishing parent rollups, got %#v", rootRollups)
	}
	if len(stub.messages) != 1 {
		t.Fatalf("expected one model call, got %d", len(stub.messages))
	}
}

func TestServiceRespondPublishesParentRollupForChildMissionTurns(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Child mission response for networking execution."}}
	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	if err := missionStore.CreateMission(missions.Mission{
		ID:              "mission-network",
		ProgramID:       "program-mission-root",
		ParentMissionID: "mission-root",
		RootMissionID:   "mission-root",
		OwningThreadID:  "thread-network",
		OwnerAgentID:    "sub-ceo-networking",
		OwnerRole:       "sub_ceo",
		MissionType:     "domain",
		Title:           "Networking",
		Charter:         "Own the networking domain",
		Goal:            "Deliver networking foundations",
		Scope:           "VPC, firewall, and routing",
		AuthorityLevel:  "domain",
		Status:          missions.MissionStatusActive,
		ProgressPercent: 35,
	}); err != nil {
		t.Fatalf("CreateMission child returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:             "thread-network",
		MissionID:      "mission-network",
		RootMissionID:  "mission-root",
		ParentThreadID: "thread-root",
		Kind:           "strategy",
		Title:          "Networking thread",
		Summary:        "Networking mission thread",
		Context:        "Owns the delegated networking mission.",
		OwnerAgentID:   "sub-ceo-networking",
		Status:         threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread child returned error: %v", err)
	}

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery, "missionId": "mission-network"})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:  "What should the networking mission do next?",
		Context: contextPayload,
		TraceID: "trace-child-rollup-refresh",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	latestSummary, err := missionStateStore.GetLatestSummary("mission-network")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if !strings.Contains(latestSummary.SummaryText, "Child mission response for networking execution") {
		t.Fatalf("expected child summary text to include latest CEO response, got %q", latestSummary.SummaryText)
	}
	rootRollups, err := missionStateStore.ListRollups("mission-root")
	if err != nil {
		t.Fatalf("ListRollups returned error: %v", err)
	}
	if len(rootRollups) != 1 {
		t.Fatalf("expected one parent rollup after child mission turn, got %d", len(rootRollups))
	}
	if rootRollups[0].ChildMissionID != "mission-network" {
		t.Fatalf("expected rollup for mission-network, got %#v", rootRollups[0])
	}
	if !strings.Contains(rootRollups[0].LatestSummary, "Child mission response for networking execution") {
		t.Fatalf("expected rollup summary to include latest child response, got %q", rootRollups[0].LatestSummary)
	}
}

func TestServiceThreadStoreRefreshesMissionStateForDelegatedActivity(t *testing.T) {
	resetModes(t)

	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	if err := missionStore.CreateMission(missions.Mission{
		ID:              "mission-network",
		ProgramID:       "program-mission-root",
		ParentMissionID: "mission-root",
		RootMissionID:   "mission-root",
		OwningThreadID:  "thread-network",
		OwnerAgentID:    "sub-ceo-networking",
		OwnerRole:       "sub_ceo",
		MissionType:     "domain",
		Title:           "Networking",
		Charter:         "Own the networking domain",
		Goal:            "Deliver networking foundations",
		Scope:           "VPC, firewall, and routing",
		AuthorityLevel:  "domain",
		Status:          missions.MissionStatusActive,
		ProgressPercent: 40,
	}); err != nil {
		t.Fatalf("CreateMission child returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:             "thread-network",
		MissionID:      "mission-network",
		RootMissionID:  "mission-root",
		ParentThreadID: "thread-root",
		Kind:           "execution",
		Title:          "Networking execution",
		Summary:        "Execution thread",
		Context:        "Owns delegated execution updates.",
		OwnerAgentID:   "sub-ceo-networking",
		Status:         threads.ThreadStatusActive,
	}); err != nil {
		t.Fatalf("CreateThread child returned error: %v", err)
	}

	if err := service.threadStore.AppendMessage(threads.Message{
		ID:            "msg-delegated-1",
		ThreadID:      "thread-network",
		Role:          threads.RoleAssistant,
		AuthorAgentID: "worker-networking",
		AuthorRole:    "worker",
		MessageType:   "worker_update",
		Content:       "Completed the firewall rules execution step.",
	}); err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}

	latestSummary, err := missionStateStore.GetLatestSummary("mission-network")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if !strings.Contains(latestSummary.SummaryText, "Completed the firewall rules execution step") {
		t.Fatalf("expected delegated activity to refresh child summary, got %q", latestSummary.SummaryText)
	}
	rootRollups, err := missionStateStore.ListRollups("mission-root")
	if err != nil {
		t.Fatalf("ListRollups returned error: %v", err)
	}
	if len(rootRollups) != 1 {
		t.Fatalf("expected one root rollup after delegated activity, got %d", len(rootRollups))
	}
	if !strings.Contains(rootRollups[0].LatestSummary, "Completed the firewall rules execution step") {
		t.Fatalf("expected delegated activity to refresh parent rollup, got %q", rootRollups[0].LatestSummary)
	}
}

func TestServiceRespondBuildsReuseAwareRoadmapPayload(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{`{"message":"We should adapt the retained networking rollout and split the initiative into reusable domain missions.","reuseDecision":{"strategy":"adapt_existing","rationale":"A prior networking mission and execution lane are close enough to adapt."},"proposedMissions":[{"title":"Networking foundation","charter":"Own the network base layer.","goal":"Adapt prior VPC and firewall work into the new program.","scope":"Routing, firewall, and connectivity foundations.","missionType":"domain","authorityLevel":"domain","reuseRefs":["mission-network-legacy","thread-network-legacy"],"reasoning":"Leverages retained networking work instead of rebuilding from zero."},{"title":"Compute substrate","charter":"Own the compute base layer.","goal":"Define the VM and scheduler foundations.","scope":"Compute APIs, placement, and lifecycle.","missionType":"domain","authorityLevel":"domain","reuseRefs":[],"reasoning":"Net-new workstream required alongside reused networking."}],"nextActions":["Review proposed mission boundaries","Create child missions"]}`}}
	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	if err := missionStore.CreateProgram(missions.Program{ID: "program-legacy", ClientID: "client-1", Title: "Legacy cloud"}); err != nil {
		t.Fatalf("CreateProgram legacy returned error: %v", err)
	}
	if err := missionStore.CreateMission(missions.Mission{
		ID:             "mission-network-legacy",
		ProgramID:      "program-legacy",
		RootMissionID:  "mission-network-legacy",
		OwnerAgentID:   "ceo-network",
		OwnerRole:      "CEO",
		MissionType:    "domain",
		Title:          "Legacy networking foundation",
		Charter:        "Own the VPC and firewall roadmap",
		Goal:           "Deliver routing and firewall foundations",
		Scope:          "VPCs, firewall, and edge connectivity",
		AuthorityLevel: "domain",
		Status:         missions.MissionStatusFinished,
	}); err != nil {
		t.Fatalf("CreateMission legacy returned error: %v", err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:            "thread-network-legacy",
		MissionID:     "mission-network-legacy",
		RootMissionID: "mission-network-legacy",
		Kind:          "execution",
		Title:         "Legacy networking execution lane",
		Summary:       "Firewall and routing rollout summary",
		Context:       "Tracks the retained networking rollout, firewall policy, and routing cutover.",
		OwnerAgentID:  "ceo-network",
		Status:        threads.ThreadStatusFinished,
	}); err != nil {
		t.Fatalf("CreateThread legacy returned error: %v", err)
	}

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeRoadmap})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "Decompose this into networking and compute workstreams with strong reuse.",
		Context:  contextPayload,
		ThreadID: "thread-root",
		TraceID:  "trace-roadmap",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	payload := decodePayload(t, response)
	if payload["message"] != "We should adapt the retained networking rollout and split the initiative into reusable domain missions." {
		t.Fatalf("unexpected roadmap message: %#v", payload["message"])
	}
	reuseDecision, ok := payload["reuseDecision"].(map[string]any)
	if !ok {
		t.Fatalf("expected reuseDecision object, got %#v", payload["reuseDecision"])
	}
	if reuseDecision["strategy"] != "adapt_existing" {
		t.Fatalf("expected adapt_existing strategy, got %#v", reuseDecision["strategy"])
	}
	reusableMissions, ok := payload["reusableMissionMatches"].([]any)
	if !ok || len(reusableMissions) == 0 {
		t.Fatalf("expected reusable mission matches, got %#v", payload["reusableMissionMatches"])
	}
	firstReusableMission, ok := reusableMissions[0].(map[string]any)
	if !ok || firstReusableMission["id"] != "mission-network-legacy" {
		t.Fatalf("expected legacy mission match, got %#v", payload["reusableMissionMatches"])
	}
	proposedMissions, ok := payload["proposedMissions"].([]any)
	if !ok || len(proposedMissions) != 2 {
		t.Fatalf("expected 2 proposed missions, got %#v", payload["proposedMissions"])
	}
	firstProposed, ok := proposedMissions[0].(map[string]any)
	if !ok || firstProposed["title"] != "Networking foundation" {
		t.Fatalf("unexpected first proposed mission: %#v", payload["proposedMissions"])
	}
	if firstProposed["missionId"] != "mission-root-networking-foundation" {
		t.Fatalf("expected persisted mission id on first proposed mission, got %#v", firstProposed["missionId"])
	}
	if firstProposed["threadId"] != "thread-mission-root-networking-foundation" {
		t.Fatalf("expected persisted thread id on first proposed mission, got %#v", firstProposed["threadId"])
	}
	reuseRefs, ok := firstProposed["reuseRefs"].([]any)
	if !ok || len(reuseRefs) != 2 {
		t.Fatalf("expected reuse refs on first proposed mission, got %#v", firstProposed["reuseRefs"])
	}
	reuseTrace, ok := firstProposed["reuseTrace"].([]any)
	if !ok || len(reuseTrace) != 2 {
		t.Fatalf("expected persisted reuse trace on first proposed mission, got %#v", firstProposed["reuseTrace"])
	}
	if firstProposed["delegatedToAgentId"] != "sub-ceo-networking" {
		t.Fatalf("expected delegated agent id on first proposed mission, got %#v", firstProposed["delegatedToAgentId"])
	}
	if firstProposed["delegatedToRole"] != "sub_ceo" {
		t.Fatalf("expected delegated role on first proposed mission, got %#v", firstProposed["delegatedToRole"])
	}
	if firstProposed["selectionSource"] != "directory" {
		t.Fatalf("expected directory-backed delegate selection, got %#v", firstProposed["selectionSource"])
	}
	if firstProposed["startupState"] != "claimed" {
		t.Fatalf("expected claimed startup state, got %#v", firstProposed["startupState"])
	}
	if firstProposed["delegateStatus"] != "busy" {
		t.Fatalf("expected busy delegate status, got %#v", firstProposed["delegateStatus"])
	}
	requiredCapabilities, ok := firstProposed["requiredCapabilities"].([]any)
	if !ok || len(requiredCapabilities) < 4 {
		t.Fatalf("expected required capabilities on first proposed mission, got %#v", firstProposed["requiredCapabilities"])
	}
	matchedCapabilities, ok := firstProposed["matchedCapabilities"].([]any)
	if !ok || len(matchedCapabilities) < 4 {
		t.Fatalf("expected matched capabilities on first proposed mission, got %#v", firstProposed["matchedCapabilities"])
	}
	if firstProposed["selectionRationale"] == "" {
		t.Fatalf("expected selection rationale on first proposed mission, got %#v", firstProposed["selectionRationale"])
	}
	authorityScope, ok := firstProposed["authorityScope"].(map[string]any)
	if !ok || authorityScope["handoffType"] != "sub_ceo" {
		t.Fatalf("expected authority scope with sub_ceo handoff, got %#v", firstProposed["authorityScope"])
	}
	if authorityScope["selectionSource"] != "directory" {
		t.Fatalf("expected authority scope to preserve selection source, got %#v", authorityScope["selectionSource"])
	}
	if authorityScope["startupState"] != "claimed" || authorityScope["delegateStatus"] != "busy" {
		t.Fatalf("expected authority scope to preserve startup lifecycle, got %#v", authorityScope)
	}
	persistedChildren, ok := payload["persistedChildMissions"].([]any)
	if !ok || len(persistedChildren) != 2 {
		t.Fatalf("expected persisted child mission summary, got %#v", payload["persistedChildMissions"])
	}
	refreshedSummaries, ok := payload["refreshedSummaries"].([]any)
	if !ok || len(refreshedSummaries) != 2 {
		t.Fatalf("expected refreshed summaries for child missions, got %#v", payload["refreshedSummaries"])
	}
	publishedRollups, ok := payload["publishedRollups"].([]any)
	if !ok || len(publishedRollups) != 2 {
		t.Fatalf("expected published rollups for child missions, got %#v", payload["publishedRollups"])
	}
	handoffs, ok := payload["delegationHandoffs"].([]any)
	if !ok || len(handoffs) != 2 {
		t.Fatalf("expected delegation handoffs, got %#v", payload["delegationHandoffs"])
	}
	firstHandoff, ok := handoffs[0].(map[string]any)
	if !ok || firstHandoff["agentId"] != "sub-ceo-networking" {
		t.Fatalf("expected first handoff to target sub-ceo-networking, got %#v", payload["delegationHandoffs"])
	}
	if firstHandoff["selectionSource"] != "directory" {
		t.Fatalf("expected directory selection source on handoff, got %#v", firstHandoff["selectionSource"])
	}
	if firstHandoff["startupState"] != "claimed" || firstHandoff["delegateStatus"] != "busy" {
		t.Fatalf("expected handoff startup lifecycle metadata, got %#v", firstHandoff)
	}
	children, err := missionStore.ListChildMissions("mission-root")
	if err != nil {
		t.Fatalf("ListChildMissions returned error: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 persisted child missions, got %d", len(children))
	}
	if children[0].OwningThreadID == "" {
		t.Fatalf("expected persisted child mission owning thread, got %#v", children[0])
	}
	if string(children[0].ReuseTrace) == "" || string(children[0].ReuseTrace) == "[]" {
		t.Fatalf("expected persisted child mission reuse trace, got %s", string(children[0].ReuseTrace))
	}
	if children[0].OwnerAgentID != "sub-ceo-networking" {
		t.Fatalf("expected delegated mission owner to be updated, got %q", children[0].OwnerAgentID)
	}
	assignments, err := missionStore.ListAssignments("mission-root-networking-foundation")
	if err != nil {
		t.Fatalf("ListAssignments returned error: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected initial plus delegated assignment on child mission, got %d", len(assignments))
	}
	childThreads, err := threadStore.ListByMission("mission-root-networking-foundation")
	if err != nil {
		t.Fatalf("ListByMission returned error: %v", err)
	}
	if len(childThreads) != 1 || childThreads[0].OwnerAgentID != "sub-ceo-networking" {
		t.Fatalf("expected delegated child thread owner, got %#v", childThreads)
	}
	latestSummary, err := missionStateStore.GetLatestSummary("mission-root-networking-foundation")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if latestSummary.SummaryText == "" {
		t.Fatalf("expected generated mission summary, got %#v", latestSummary)
	}
	rootRollups, err := missionStateStore.ListRollups("mission-root")
	if err != nil {
		t.Fatalf("ListRollups returned error: %v", err)
	}
	if len(rootRollups) != 2 {
		t.Fatalf("expected two published root rollups, got %d", len(rootRollups))
	}
	history, err := service.threadStore.ListMessages("thread-root")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if history[1].Content != "We should adapt the retained networking rollout and split the initiative into reusable domain missions." {
		t.Fatalf("expected persisted roadmap message, got %q", history[1].Content)
	}
	if len(stub.messages) != 0 {
		t.Fatalf("expected roadmap planner to use Generate, not GenerateFromMessages, got %d message calls", len(stub.messages))
	}
}

func TestServiceRespondRoadmapFallsBackWhenPlannerReturnsInvalidJSON(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"not-json"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeRoadmap})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:   "Build identity, compute, and networking for this product.",
		Context:  contextPayload,
		ThreadID: "thread-root",
		TraceID:  "trace-roadmap-fallback",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	payload := decodePayload(t, response)
	reuseDecision, ok := payload["reuseDecision"].(map[string]any)
	if !ok {
		t.Fatalf("expected reuseDecision object, got %#v", payload["reuseDecision"])
	}
	if reuseDecision["strategy"] != "build_net_new" {
		t.Fatalf("expected build_net_new strategy, got %#v", reuseDecision["strategy"])
	}
	proposedMissions, ok := payload["proposedMissions"].([]any)
	if !ok || len(proposedMissions) == 0 {
		t.Fatalf("expected fallback proposed missions, got %#v", payload["proposedMissions"])
	}
	if _, exists := payload["plannerRaw"]; !exists {
		t.Fatalf("expected raw planner output to be preserved in payload, got %#v", payload)
	}
	history, err := service.threadStore.ListMessages("thread-root")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 persisted roadmap messages, got %d", len(history))
	}
}

func TestServiceRespondUsesThreadHistoryInNextTurn(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"First CEO reply", "Second CEO reply"}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "First client question",
		Context:  contextPayload,
		ThreadID: "thread-history",
		TraceID:  "trace-1",
	})
	if err != nil {
		t.Fatalf("first Respond returned error: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "Second client question",
		Context:  contextPayload,
		ThreadID: "thread-history",
		TraceID:  "trace-2",
	})
	if err != nil {
		t.Fatalf("second Respond returned error: %v", err)
	}

	if len(stub.messages) != 2 {
		t.Fatalf("expected two conversation calls, got %d", len(stub.messages))
	}
	secondTurn := stub.messages[1]
	if len(secondTurn) != 5 {
		t.Fatalf("expected 5 messages in second conversation, got %d", len(secondTurn))
	}
	if !strings.Contains(secondTurn[1].Content, "Mission:") {
		t.Fatalf("expected mission context system message, got %q", secondTurn[1].Content)
	}
	if secondTurn[2].Content != "First client question" {
		t.Fatalf("expected first history user message, got %q", secondTurn[1].Content)
	}
	if secondTurn[3].Content != "First CEO reply" {
		t.Fatalf("expected first history assistant message, got %q", secondTurn[3].Content)
	}
	if secondTurn[4].Content != "Second client question" {
		t.Fatalf("expected latest user prompt, got %q", secondTurn[4].Content)
	}

	history, err := service.threadStore.ListMessages("thread-history")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 persisted messages, got %d", len(history))
	}
}

func TestServiceRespondKeepsThreadsIsolated(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Reply A", "Reply B"}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{Prompt: "question-a", Context: contextPayload, ThreadID: "thread-a", TraceID: "trace-a"})
	if err != nil {
		t.Fatalf("Respond thread-a returned error: %v", err)
	}
	_, err = service.Respond(context.Background(), Request{Prompt: "question-b", Context: contextPayload, ThreadID: "thread-b", TraceID: "trace-b"})
	if err != nil {
		t.Fatalf("Respond thread-b returned error: %v", err)
	}

	messagesA, err := service.threadStore.ListMessages("thread-a")
	if err != nil {
		t.Fatalf("ListMessages thread-a returned error: %v", err)
	}
	messagesB, err := service.threadStore.ListMessages("thread-b")
	if err != nil {
		t.Fatalf("ListMessages thread-b returned error: %v", err)
	}
	if len(messagesA) != 2 || len(messagesB) != 2 {
		t.Fatalf("expected 2 messages per thread, got thread-a=%d thread-b=%d", len(messagesA), len(messagesB))
	}
	if messagesA[0].Content == messagesB[0].Content {
		t.Fatalf("expected isolated thread content, got same first message %q", messagesA[0].Content)
	}
}

func TestServiceRespondUsesMissionSummaryAndRollupsInPromptContext(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Discovery response"}}
	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	if err := missionStore.CreateMission(missions.Mission{
		ID:              "mission-network",
		ProgramID:       "program-mission-root",
		ParentMissionID: "mission-root",
		RootMissionID:   "mission-root",
		OwningThreadID:  "thread-network",
		OwnerAgentID:    "ceo-network",
		OwnerRole:       "CEO",
		MissionType:     "domain",
		Title:           "Networking",
		Charter:         "Own networking strategy",
		Goal:            "Deliver networking pillar",
		Scope:           "Networking domain",
		AuthorityLevel:  "domain",
		Status:          missions.MissionStatusActive,
	}); err != nil {
		t.Fatalf("CreateMission child returned error: %v", err)
	}
	if err := missionStateStore.CreateSummary(missionstate.Summary{
		ID:               "summary-root",
		MissionID:        "mission-root",
		ThreadID:         "thread-root",
		Level:            "mission",
		Kind:             "rolling",
		CoverageStartRef: "seed-1",
		CoverageEndRef:   "seed-1",
		SummaryText:      "The CEO has narrowed scope to identity, compute, and networking.",
	}); err != nil {
		t.Fatalf("CreateSummary returned error: %v", err)
	}
	nextUpdate := time.Now().UTC().Add(90 * time.Minute)
	if err := missionStateStore.UpsertRollup(missionstate.Rollup{
		ID:                   "rollup-network",
		ParentMissionID:      "mission-root",
		ChildMissionID:       "mission-network",
		Status:               missions.MissionStatusActive,
		ProgressPercent:      35,
		Health:               "green",
		CurrentBlocker:       "Awaiting network policy decision",
		LatestSummary:        "Networking has split VPC, firewall, and edge routing work.",
		NextExpectedUpdateAt: &nextUpdate,
	}); err != nil {
		t.Fatalf("UpsertRollup returned error: %v", err)
	}
	if err := threadStore.AppendMessage(threads.Message{
		ID:            "seed-1",
		ThreadID:      "thread-root",
		Role:          threads.RoleAssistant,
		AuthorAgentID: "ceo",
		AuthorRole:    "ceo",
		MessageType:   "ceo_message",
		Content:       "We should start with the platform spine before expanding into every cloud surface.",
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "What should the root CEO do next?",
		Context:  contextPayload,
		ThreadID: "thread-root",
		TraceID:  "trace-root",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	if len(stub.messages) != 1 {
		t.Fatalf("expected one conversation call, got %d", len(stub.messages))
	}
	contextMessage := stub.messages[0][1].Content
	if !strings.Contains(contextMessage, "Mission mission-root") && !strings.Contains(contextMessage, "- ID: mission-root") {
		t.Fatalf("expected mission identity in context message, got %q", contextMessage)
	}
	if !strings.Contains(contextMessage, "The CEO has narrowed scope to identity, compute, and networking.") {
		t.Fatalf("expected latest summary in context message, got %q", contextMessage)
	}
	if !strings.Contains(contextMessage, "Networking has split VPC, firewall, and edge routing work.") {
		t.Fatalf("expected child rollup summary in context message, got %q", contextMessage)
	}
	if stub.messages[0][2].Content != "We should start with the platform spine before expanding into every cloud surface." {
		t.Fatalf("expected bounded recent history after system messages, got %q", stub.messages[0][2].Content)
	}
}

func TestServiceRespondUsesMissionIDFromContextWithoutThreadID(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"mission-targeted response"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-explicit", "thread-explicit")

	contextPayload, err := json.Marshal(map[string]any{
		"mode":      ModeDiscovery,
		"missionId": "mission-explicit",
	})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:  "Work on the explicit mission.",
		Context: contextPayload,
		TraceID: "trace-explicit",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.ThreadID != "thread-explicit" {
		t.Fatalf("expected response thread thread-explicit, got %q", response.ThreadID)
	}
	if len(stub.messages) != 1 {
		t.Fatalf("expected one conversation call, got %d", len(stub.messages))
	}
	if !strings.Contains(stub.messages[0][1].Content, "- ID: mission-explicit") {
		t.Fatalf("expected explicit mission context in system message, got %q", stub.messages[0][1].Content)
	}
	if stub.messages[0][2].Content != "Work on the explicit mission." {
		t.Fatalf("expected user prompt after the two system messages, got %q", stub.messages[0][2].Content)
	}

	history, err := service.threadStore.ListMessages("thread-explicit")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 persisted messages on the owning thread, got %d", len(history))
	}
}

func TestServiceRespondUsesTopLevelMissionIDWithoutThreadID(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"top-level mission-targeted response"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-top-level", "thread-top-level")

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		Prompt:    "Work on the top-level mission field.",
		MissionID: "mission-top-level",
		Context:   contextPayload,
		TraceID:   "trace-top-level",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.ThreadID != "thread-top-level" {
		t.Fatalf("expected response thread thread-top-level, got %q", response.ThreadID)
	}
	if len(stub.messages) != 1 {
		t.Fatalf("expected one conversation call, got %d", len(stub.messages))
	}
	if !strings.Contains(stub.messages[0][1].Content, "- ID: mission-top-level") {
		t.Fatalf("expected top-level mission context in system message, got %q", stub.messages[0][1].Content)
	}
	if stub.messages[0][2].Content != "Work on the top-level mission field." {
		t.Fatalf("expected user prompt after the two system messages, got %q", stub.messages[0][2].Content)
	}
}

func TestServiceRespondRejectsMissionIDMismatchBetweenEnvelopeAndContext(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"should-not-run"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-envelope", "thread-envelope")
	seedMissionThread(t, missionStore, threadStore, "mission-context", "thread-context")

	contextPayload, err := json.Marshal(map[string]any{
		"mode":      ModeDiscovery,
		"missionId": "mission-context",
	})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:    "Try conflicting mission ids.",
		MissionID: "mission-envelope",
		Context:   contextPayload,
		TraceID:   "trace-conflict",
	})
	if err == nil {
		t.Fatal("expected Respond to fail when top-level and context missionId conflict")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected missionId mismatch error, got %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("expected model not to be called on missionId conflict, got %d calls", stub.calls)
	}
}

func TestServiceRespondRejectsMissionIDThreadMismatch(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"should-not-run"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-a", "thread-a")
	seedMissionThread(t, missionStore, threadStore, "mission-b", "thread-b")

	contextPayload, err := json.Marshal(map[string]any{
		"mode":      ModeDiscovery,
		"missionId": "mission-a",
	})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "Use the wrong thread.",
		Context:  contextPayload,
		ThreadID: "thread-b",
		TraceID:  "trace-mismatch",
	})
	if err == nil {
		t.Fatal("expected Respond to fail for mission/thread mismatch")
	}
	if !strings.Contains(err.Error(), "belongs to mission") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("expected model not to be called on mismatch, got %d calls", stub.calls)
	}
}

func TestServiceRespondRejectsUnknownMissionIDInContext(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"should-not-run"}}
	service, _, _, _, _ := newTestService(t, stub)

	contextPayload, err := json.Marshal(map[string]any{
		"mode":      ModeDiscovery,
		"missionId": "mission-missing",
	})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:  "Target a missing mission.",
		Context: contextPayload,
		TraceID: "trace-missing",
	})
	if err == nil {
		t.Fatal("expected Respond to fail for missing mission target")
	}
	if !strings.Contains(err.Error(), missions.ErrMissionNotFound.Error()) {
		t.Fatalf("expected mission not found error, got %v", err)
	}
}

func TestServiceRespondBoundsRecentMessagesInPrompt(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"bounded response"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-bounded", "thread-bounded")

	baseTime := time.Now().UTC()
	for i := 0; i < 10; i++ {
		role := threads.RoleUser
		authorAgentID := "user"
		authorRole := "client"
		messageType := "client_message"
		if i%2 == 1 {
			role = threads.RoleAssistant
			authorAgentID = "ceo"
			authorRole = "ceo"
			messageType = "ceo_message"
		}
		if err := threadStore.AppendMessage(threads.Message{
			ID:            fmt.Sprintf("msg-%d", i),
			ThreadID:      "thread-bounded",
			Role:          role,
			AuthorAgentID: authorAgentID,
			AuthorRole:    authorRole,
			MessageType:   messageType,
			Content:       fmt.Sprintf("message-%d", i),
			CreatedAt:     baseTime.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("AppendMessage returned error: %v", err)
		}
	}

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	_, err = service.Respond(context.Background(), Request{
		Prompt:   "Give me the next step.",
		Context:  contextPayload,
		ThreadID: "thread-bounded",
		TraceID:  "trace-bounded",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	conversation := stub.messages[0]
	if len(conversation) != 11 {
		t.Fatalf("expected 11 messages in bounded conversation, got %d", len(conversation))
	}
	if conversation[2].Content != "message-2" {
		t.Fatalf("expected oldest retained recent message to be message-2, got %q", conversation[2].Content)
	}
	if conversation[9].Content != "message-9" {
		t.Fatalf("expected newest retained recent message to be message-9, got %q", conversation[9].Content)
	}
	if conversation[10].Content != "Give me the next step." {
		t.Fatalf("expected latest user prompt at the end, got %q", conversation[10].Content)
	}
}

func TestServiceRespondCreatesMissionScopedTodoAction(t *testing.T) {
	resetModes(t)

	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")

	actionPayload, err := json.Marshal(map[string]any{
		"title":         "Create detailed roadmap",
		"description":   "Generate the first execution-ready roadmap slice.",
		"ownerAgentId":  "planner-agent",
		"priority":      missions.PriorityHigh,
		"dependsOn":     []string{"todo-brief"},
		"artifactPaths": []string{"docs/roadmap.md"},
	})
	if err != nil {
		t.Fatalf("marshal action payload: %v", err)
	}

	response, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-create-todo",
		Action: &ActionRequest{
			Type:    ActionCreateTodo,
			Payload: actionPayload,
		},
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.Mode != ModeExecutionPrep {
		t.Fatalf("expected execution prep mode, got %q", response.Mode)
	}
	payload := decodePayload(t, response)
	todo, ok := payload["todo"].(map[string]any)
	if !ok {
		t.Fatalf("expected todo payload, got %#v", payload)
	}
	if todo["missionId"] != "mission-root" {
		t.Fatalf("expected mission-root todo, got %#v", todo)
	}
	if todo["ownerAgentId"] != "planner-agent" {
		t.Fatalf("expected planner-agent owner, got %#v", todo)
	}
	openTodos, err := service.executionRuntime.ListOpenTodos("mission-root")
	if err != nil {
		t.Fatalf("ListOpenTodos returned error: %v", err)
	}
	if len(openTodos) != 1 {
		t.Fatalf("expected 1 open todo, got %d", len(openTodos))
	}
	latestSummary, err := missionStateStore.GetLatestSummary("mission-root")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if !strings.Contains(latestSummary.SummaryText, "Created todo") {
		t.Fatalf("expected action message to refresh mission summary, got %q", latestSummary.SummaryText)
	}
}

func TestServiceRespondMutatesTodoWithinMissionScope(t *testing.T) {
	resetModes(t)

	service, missionStore, threadStore, _, executionStore := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")
	seedMissionThread(t, missionStore, threadStore, "mission-other", "thread-other")

	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-root-1",
		MissionID:    "mission-root",
		ThreadID:     "thread-root",
		Title:        "Root todo",
		Description:  "Owned by root mission",
		OwnerAgentID: "ceo",
	}); err != nil {
		t.Fatalf("CreateTodo root returned error: %v", err)
	}
	if err := executionStore.CreateTodo(execution.Todo{
		ID:           "todo-other-1",
		MissionID:    "mission-other",
		ThreadID:     "thread-other",
		Title:        "Other todo",
		Description:  "Owned by other mission",
		OwnerAgentID: "ceo",
	}); err != nil {
		t.Fatalf("CreateTodo other returned error: %v", err)
	}

	assignPayload, err := json.Marshal(map[string]any{"todoId": "todo-root-1", "ownerAgentId": "worker-1"})
	if err != nil {
		t.Fatalf("marshal assign payload: %v", err)
	}
	if _, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-assign-todo",
		Action:    &ActionRequest{Type: ActionAssignTodo, Payload: assignPayload},
	}); err != nil {
		t.Fatalf("assign todo action returned error: %v", err)
	}
	blockPayload, err := json.Marshal(map[string]any{"todoId": "todo-root-1"})
	if err != nil {
		t.Fatalf("marshal block payload: %v", err)
	}
	if _, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-block-todo",
		Action:    &ActionRequest{Type: ActionBlockTodo, Payload: blockPayload},
	}); err != nil {
		t.Fatalf("block todo action returned error: %v", err)
	}
	if _, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-complete-todo",
		Action:    &ActionRequest{Type: ActionCompleteTodo, Payload: blockPayload},
	}); err != nil {
		t.Fatalf("complete todo action returned error: %v", err)
	}
	updatedTodo, err := executionStore.GetTodo("todo-root-1")
	if err != nil {
		t.Fatalf("GetTodo returned error: %v", err)
	}
	if updatedTodo.OwnerAgentID != "worker-1" || updatedTodo.Status != execution.TodoStatusDone {
		t.Fatalf("unexpected updated todo: %#v", updatedTodo)
	}
	crossScopePayload, err := json.Marshal(map[string]any{"todoId": "todo-other-1"})
	if err != nil {
		t.Fatalf("marshal cross-scope payload: %v", err)
	}
	if _, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-cross-scope",
		Action:    &ActionRequest{Type: ActionBlockTodo, Payload: crossScopePayload},
	}); err == nil {
		t.Fatal("expected cross-mission todo mutation to fail")
	}
}

func TestServiceRespondSchedulesAndCancelsMissionTimerAction(t *testing.T) {
	resetModes(t)

	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-root", "thread-root")
	wakeAt := time.Now().UTC().Add(45 * time.Minute)
	schedulePayload, err := json.Marshal(map[string]any{
		"setByAgentId":  "ceo-root",
		"wakeAt":        wakeAt,
		"actionType":    "status_check",
		"actionPayload": map[string]any{"reason": "check mission progress"},
	})
	if err != nil {
		t.Fatalf("marshal schedule payload: %v", err)
	}
	response, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-schedule-timer",
		Action:    &ActionRequest{Type: ActionScheduleTimer, Payload: schedulePayload},
	})
	if err != nil {
		t.Fatalf("schedule timer action returned error: %v", err)
	}
	payload := decodePayload(t, response)
	timerPayload, ok := payload["timer"].(map[string]any)
	if !ok {
		t.Fatalf("expected timer payload, got %#v", payload)
	}
	timerID, ok := timerPayload["id"].(string)
	if !ok || timerID == "" {
		t.Fatalf("expected timer id, got %#v", timerPayload)
	}
	cancelPayload, err := json.Marshal(map[string]any{"timerId": timerID})
	if err != nil {
		t.Fatalf("marshal cancel payload: %v", err)
	}
	if _, err := service.Respond(context.Background(), Request{
		MissionID: "mission-root",
		ThreadID:  "thread-root",
		TraceID:   "trace-action-cancel-timer",
		Action:    &ActionRequest{Type: ActionCancelTimer, Payload: cancelPayload},
	}); err != nil {
		t.Fatalf("cancel timer action returned error: %v", err)
	}
	timer, err := service.executionRuntime.Store().GetTimer(timerID)
	if err != nil {
		t.Fatalf("GetTimer returned error: %v", err)
	}
	if timer.Status != execution.TimerStatusCancelled {
		t.Fatalf("expected cancelled timer, got %#v", timer)
	}
	latestSummary, err := missionStateStore.GetLatestSummary("mission-root")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if !strings.Contains(latestSummary.SummaryText, "Cancelled timer") {
		t.Fatalf("expected timer action to refresh mission summary, got %q", latestSummary.SummaryText)
	}
}

func TestServiceRespondPersistsResponsePayloadAndResponseID(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Structured discovery reply"}}
	service, missionStore, threadStore, _, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-feedback", "thread-feedback")

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}
	response, err := service.Respond(context.Background(), Request{
		Prompt:   "What should we build first?",
		Context:  contextPayload,
		ThreadID: "thread-feedback",
		TraceID:  "trace-feedback",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if response.ResponseID == "" {
		t.Fatal("expected response id to be set")
	}
	history, err := service.threadStore.ListMessages("thread-feedback")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected two persisted messages, got %d", len(history))
	}
	if history[1].ID != response.ResponseID {
		t.Fatalf("expected assistant message id %q, got %#v", response.ResponseID, history[1])
	}
	if history[1].ReplyToMessageID != history[0].ID {
		t.Fatalf("expected assistant reply to user message, got %#v", history[1])
	}
	if string(history[1].ContentJSON) != string(response.Payload) {
		t.Fatalf("expected assistant content json to match response payload, got %s want %s", string(history[1].ContentJSON), string(response.Payload))
	}
	feedbackStore := service.feedbackStore.(*feedback.MemoryStore)
	records, err := feedbackStore.ListByThread("thread-feedback")
	if err != nil {
		t.Fatalf("ListByThread returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no feedback yet, got %#v", records)
	}
}

func TestServiceSubmitFeedbackPersistsLinkedEvidence(t *testing.T) {
	resetModes(t)

	stub := &stubCompletionClient{responses: []string{"Structured discovery reply"}}
	service, missionStore, threadStore, missionStateStore, _ := newTestService(t, stub)
	seedMissionThread(t, missionStore, threadStore, "mission-feedback", "thread-feedback")
	if err := missionStateStore.CreateSummary(missionstate.Summary{
		ID:               "summary-feedback",
		MissionID:        "mission-feedback",
		ThreadID:         "thread-feedback",
		Level:            "mission",
		Kind:             "rollup",
		CoverageStartRef: "seed",
		CoverageEndRef:   "seed",
		SummaryText:      "Current mission summary",
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateSummary returned error: %v", err)
	}

	contextPayload, err := json.Marshal(map[string]any{"mode": ModeDiscovery})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}
	response, err := service.Respond(context.Background(), Request{
		Prompt:   "What should we build first?",
		Context:  contextPayload,
		ThreadID: "thread-feedback",
		TraceID:  "trace-feedback",
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	record, err := service.SubmitFeedback(context.Background(), FeedbackSubmission{
		ThreadID:   "thread-feedback",
		ResponseID: response.ResponseID,
		TraceID:    response.TraceID,
		Rating:     2,
		Reason:     "Too shallow and unclear on the next step.",
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SubmitFeedback returned error: %v", err)
	}
	if record.ThreadID != "thread-feedback" || record.ResponseID != response.ResponseID {
		t.Fatalf("unexpected feedback record linkage: %#v", record)
	}
	if record.ClientMessage != "What should we build first?" {
		t.Fatalf("expected triggering client message, got %#v", record)
	}
	if record.CEOResponse != "Structured discovery reply" {
		t.Fatalf("expected CEO response text, got %#v", record)
	}
	if !strings.Contains(record.ContextSummary, "Structured discovery reply") {
		t.Fatalf("expected refreshed context summary to reference the response, got %#v", record)
	}
	var categories []string
	if err := json.Unmarshal(record.Categories, &categories); err != nil {
		t.Fatalf("unmarshal categories: %v", err)
	}
	if len(categories) == 0 || categories[0] == "" {
		t.Fatalf("expected classified categories, got %#v", categories)
	}
	var evidence []string
	if err := json.Unmarshal(record.EvidenceRefs, &evidence); err != nil {
		t.Fatalf("unmarshal evidence refs: %v", err)
	}
	if len(evidence) < 3 {
		t.Fatalf("expected evidence refs, got %#v", evidence)
	}
	feedbackStore := service.feedbackStore.(*feedback.MemoryStore)
	persisted, err := feedbackStore.GetFeedback(record.ID)
	if err != nil {
		t.Fatalf("GetFeedback returned error: %v", err)
	}
	if persisted.TraceID != response.TraceID {
		t.Fatalf("expected persisted trace id %q, got %#v", response.TraceID, persisted)
	}
}

func TestLoadSystemPromptReturnsErrorForMissingConfig(t *testing.T) {
	resetModes(t)
	RegisterModes(Mode("custom_mode"))

	if _, err := loadSystemPrompt(Mode("custom_mode")); err == nil {
		t.Fatal("expected loadSystemPrompt to fail when mode config file is missing")
	}
}

func TestLoadConfigValidatesAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")

	if _, err := LoadConfig("missing.env"); err == nil {
		t.Fatal("expected LoadConfig to fail without OPENAI_API_KEY")
	}
}

func TestNewServiceRequiresExplicitStore(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	feedbackStore := feedback.NewMemoryStore()

	if _, err := NewService(Config{APIKey: "test-key", Model: "gpt-5.4"}, &stubCompletionClient{responses: []string{"ok"}}, nil, threadStore, missionStateStore, executionStore, feedbackStore); err == nil {
		t.Fatal("expected NewService to fail without an explicit mission store")
	}
	if _, err := NewService(Config{APIKey: "test-key", Model: "gpt-5.4"}, &stubCompletionClient{responses: []string{"ok"}}, missionStore, nil, missionStateStore, executionStore, feedbackStore); err == nil {
		t.Fatal("expected NewService to fail without an explicit thread store")
	}
	if _, err := NewService(Config{APIKey: "test-key", Model: "gpt-5.4"}, &stubCompletionClient{responses: []string{"ok"}}, missionStore, threadStore, nil, executionStore, feedbackStore); err == nil {
		t.Fatal("expected NewService to fail without an explicit mission state store")
	}
	if _, err := NewService(Config{APIKey: "test-key", Model: "gpt-5.4"}, &stubCompletionClient{responses: []string{"ok"}}, missionStore, threadStore, missionStateStore, nil, feedbackStore); err == nil {
		t.Fatal("expected NewService to fail without an explicit execution store")
	}
	if _, err := NewService(Config{APIKey: "test-key", Model: "gpt-5.4"}, &stubCompletionClient{responses: []string{"ok"}}, missionStore, threadStore, missionStateStore, executionStore, nil); err == nil {
		t.Fatal("expected NewService to fail without an explicit feedback store")
	}
}
