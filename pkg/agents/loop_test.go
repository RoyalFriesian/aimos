package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/microai"

	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// --- Mock LLM ---

type mockLLM struct {
	mu        sync.Mutex
	responses []string
	callCount int
}

func newMockLLM(responses ...string) *mockLLM {
	return &mockLLM{responses: responses}
}

func (m *mockLLM) Generate(_ context.Context, _ string, _ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCount >= len(m.responses) {
		return `{"thinking":"no more responses","summary":"idle","actions":[{"type":"no_op","payload":{}}]}`, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func (m *mockLLM) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// --- Test Helpers ---

func setupTestInfra(t *testing.T) (missions.Store, threads.Store, missionstate.Store, execution.Store, NodeStore) {
	t.Helper()
	missionStore := missions.NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	missionStateStore := missionstate.NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	nodeStore := NewMemoryNodeStore()
	return missionStore, threadStore, missionStateStore, executionStore, nodeStore
}

func createTestMission(t *testing.T, missionStore missions.Store, threadStore threads.Store, missionID, threadID string) {
	t.Helper()
	if err := missionStore.CreateProgram(missions.Program{
		ID:            "prog-" + missionID,
		ClientID:      "test-client",
		Title:         "Test Program",
		RootMissionID: missionID,
		Status:        missions.ProgramStatusActive,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := missionStore.CreateMission(missions.Mission{
		ID:             missionID,
		ProgramID:      "prog-" + missionID,
		RootMissionID:  missionID,
		OwningThreadID: threadID,
		OwnerAgentID:   "test-agent",
		OwnerRole:      "ceo",
		MissionType:    "execution",
		Title:          "Test Mission",
		Charter:        "Test charter",
		Goal:           "Test goal",
		Scope:          "Test scope",
		AuthorityLevel: "full",
		Status:         missions.MissionStatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := threadStore.CreateThread(threads.Thread{
		ID:            threadID,
		MissionID:     missionID,
		RootMissionID: missionID,
		Kind:          "execution",
		Title:         "Test Thread",
		Summary:       "Test thread summary",
		OwnerAgentID:  "test-agent",
		Status:        threads.ThreadStatusActive,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}

func buildTestDeps(t *testing.T, llm *mockLLM) (AgentLoopDeps, missions.Store, threads.Store, execution.Store, NodeStore) {
	t.Helper()
	missionStore, threadStore, missionStateStore, executionStore, nodeStore := setupTestInfra(t)

	missionStateRuntime, err := missionstate.NewRuntime(missionStateStore, missionStore, threadStore, executionStore)
	if err != nil {
		t.Fatal(err)
	}
	missionRuntime, err := missions.NewRuntime(missionStore, threadStore)
	if err != nil {
		t.Fatal(err)
	}
	executionRuntime, err := execution.NewRuntime(executionStore)
	if err != nil {
		t.Fatal(err)
	}
	contextBuilder, err := contextpacks.NewBuilder(missionStore, threadStore, missionStateStore, executionStore, nil)
	if err != nil {
		t.Fatal(err)
	}

	deps := AgentLoopDeps{
		LLM:                 llm,
		NodeStore:           nodeStore,
		ThreadStore:         threadStore,
		MissionStore:        missionStore,
		ExecutionRuntime:    executionRuntime,
		MissionRuntime:      missionRuntime,
		MissionStateRuntime: missionStateRuntime,
		ContextBuilder:      contextBuilder,
		LoopManager:         NewAgentLoopManager(),
		TestingAgent:        NewTestingAgent(llm, "test-model"),
	}
	return deps, missionStore, threadStore, executionStore, nodeStore
}

// --- Tests ---

func TestAgentLoop_BasicTurnExecution(t *testing.T) {
	llm := newMockLLM(`{"thinking":"checking status","summary":"All looks good","actions":[{"type":"no_op","payload":{}}]}`)
	deps, _, _, _, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-1", "thread-1")

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "test-ceo",
		AgentRole:    NodeRoleCEO,
		MissionID:    "mission-1",
		ThreadID:     "thread-1",
		ProjectID:    "thread-1",
		Depth:        0,
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a test agent.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = loop.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if llm.CallCount() == 0 {
		t.Error("expected at least one LLM call")
	}

	// Check that a turn summary was posted to the thread.
	messages, _ := deps.ThreadStore.ListMessages("thread-1")
	hasSummary := false
	for _, msg := range messages {
		if msg.MessageType == "agent_turn_summary" {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("expected an agent_turn_summary message to be posted")
	}
}

func TestAgentLoop_PostMessageAction(t *testing.T) {
	action := json.RawMessage(`{"content":"Hello from agent"}`)
	llm := newMockLLM(fmt.Sprintf(`{"thinking":"posting","summary":"sent message","actions":[{"type":"post_message","payload":%s}]}`, string(action)))
	deps, _, _, _, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-2", "thread-2")

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "test-agent",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-2",
		ThreadID:     "thread-2",
		ProjectID:    "thread-2",
		Depth:        3,
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a test worker.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	messages, _ := deps.ThreadStore.ListMessages("thread-2")
	found := false
	for _, msg := range messages {
		if msg.MessageType == "agent_message" && msg.Content == "Hello from agent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected post_message action to create a message in the thread")
	}
}

func TestAgentLoop_CompleteTodoAction(t *testing.T) {
	llm := newMockLLM(`{"thinking":"completing","summary":"done","actions":[{"type":"complete_todo","payload":{"todoId":"todo-1"}}]}`)
	deps, _, _, executionStore, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-3", "thread-3")

	// Create a todo to complete.
	todo, err := deps.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    "mission-3",
		ThreadID:     "thread-3",
		Title:        "Test todo",
		Description:  "A test todo",
		OwnerAgentID: "test-worker",
		Priority:     missions.PriorityHigh,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Override the LLM response to use the actual todo ID.
	llm.responses[0] = fmt.Sprintf(`{"thinking":"completing","summary":"done","actions":[{"type":"complete_todo","payload":{"todoId":"%s"}}]}`, todo.ID)

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "test-worker",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-3",
		ThreadID:     "thread-3",
		ProjectID:    "thread-3",
		Depth:        3,
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a test worker.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify the todo is complete.
	updatedTodo, err := executionStore.GetTodo(todo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTodo.Status != execution.TodoStatusDone {
		t.Errorf("expected todo status %q, got %q", execution.TodoStatusDone, updatedTodo.Status)
	}
}

func TestAgentLoop_MarkDoneStopsLoop(t *testing.T) {
	llm := newMockLLM(`{"thinking":"all done","summary":"mission complete","actions":[{"type":"mark_done","payload":{}}]}`)
	deps, _, _, _, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-4", "thread-4")

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "test-worker",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-4",
		ThreadID:     "thread-4",
		ProjectID:    "thread-4",
		Depth:        3,
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a test worker.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify mission is completed.
	mission, err := deps.MissionStore.GetMission("mission-4")
	if err != nil {
		t.Fatal(err)
	}
	if mission.Status != missions.MissionStatusCompleted {
		t.Errorf("expected mission status %q, got %q", missions.MissionStatusCompleted, mission.Status)
	}

	// Verify completion message was posted.
	messages, _ := deps.ThreadStore.ListMessages("thread-4")
	found := false
	for _, msg := range messages {
		if msg.MessageType == "mission_completed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mission_completed message")
	}
}

func TestAgentLoop_CreateWorkerEnforcesMaxDepth(t *testing.T) {
	// Worker at maxDepth should not be able to create sub-workers.
	action := `{"thinking":"need help","summary":"trying to delegate","actions":[{"type":"create_worker","payload":{"name":"Sub Worker","role":"worker","problemStatement":"sub task","todoTitle":"Sub todo","todoDescription":"do sub work"}}]}`
	llm := newMockLLM(action)
	deps, _, _, _, nodeStore := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-5", "thread-5")

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "deep-worker",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-5",
		ThreadID:     "thread-5",
		ProjectID:    "thread-5",
		Depth:        3, // at max depth
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a leaf worker.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify no child nodes were created.
	children, _ := nodeStore.ListChildren("deep-worker")
	if len(children) > 0 {
		t.Errorf("expected no children at max depth, got %d", len(children))
	}
}

func TestAgentLoop_CreateWorkerAtAllowedDepth(t *testing.T) {
	action := `{"thinking":"need help","summary":"delegating","actions":[{"type":"create_worker","payload":{"name":"Sub Worker","role":"worker","problemStatement":"sub task","todoTitle":"Sub todo","todoDescription":"do sub work"}}]}`
	llm := newMockLLM(action)
	deps, _, _, _, nodeStore := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-6", "thread-6")

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "mid-manager",
		AgentRole:    NodeRoleManager,
		MissionID:    "mission-6",
		ThreadID:     "thread-6",
		ProjectID:    "thread-6",
		Depth:        1, // below max, can create
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a manager.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify a child node was created.
	children, _ := nodeStore.ListChildren("mid-manager")
	if len(children) == 0 {
		t.Error("expected a child node to be created at allowed depth")
	}
	if len(children) > 0 && children[0].Depth != 2 {
		t.Errorf("expected child depth 2, got %d", children[0].Depth)
	}
}

func TestAgentLoop_TerminalMissionStopsLoop(t *testing.T) {
	llm := newMockLLM() // no responses needed since the mission is already terminal
	deps, _, _, _, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-7", "thread-7")

	// Mark mission as completed before the loop starts.
	mission, _ := deps.MissionStore.GetMission("mission-7")
	mission.Status = missions.MissionStatusCompleted
	_ = deps.MissionStore.UpdateMission(mission)

	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:      "done-agent",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-7",
		ThreadID:     "thread-7",
		ProjectID:    "thread-7",
		Depth:        1,
		MaxDepth:     3,
		WakeInterval: 100 * time.Millisecond,
		Model:        "test-model",
		SystemPrompt: "You are a worker.",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// The LLM should not have been called since the mission was already terminal.
	if llm.CallCount() > 0 {
		t.Error("expected no LLM calls for terminal mission")
	}
}

func TestAgentLoopManager_StartStopList(t *testing.T) {
	manager := NewAgentLoopManager()
	defer manager.StopAll()

	llm := newMockLLM() // will return no_op forever
	deps, _, _, _, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-mgr", "thread-mgr")

	config := AgentLoopConfig{
		AgentID:      "mgr-test",
		AgentRole:    NodeRoleWorker,
		MissionID:    "mission-mgr",
		ThreadID:     "thread-mgr",
		ProjectID:    "thread-mgr",
		Depth:        1,
		MaxDepth:     3,
		WakeInterval: 60 * time.Second, // long interval so it doesn't tick during test
		Model:        "test-model",
		SystemPrompt: "You are a worker.",
	}

	if err := manager.StartLoop(config, deps); err != nil {
		t.Fatal(err)
	}

	if !manager.IsRunning("mgr-test") {
		t.Error("expected loop to be running")
	}
	if manager.RunningCount() != 1 {
		t.Errorf("expected 1 running loop, got %d", manager.RunningCount())
	}

	running := manager.ListRunning()
	if len(running) != 1 || running[0] != "mgr-test" {
		t.Errorf("expected [mgr-test], got %v", running)
	}

	// Starting same agent again should be a no-op.
	if err := manager.StartLoop(config, deps); err != nil {
		t.Fatal(err)
	}
	if manager.RunningCount() != 1 {
		t.Errorf("expected still 1 running loop after duplicate start, got %d", manager.RunningCount())
	}

	manager.StopLoop("mgr-test")
	time.Sleep(50 * time.Millisecond) // let goroutine clean up

	if manager.IsRunning("mgr-test") {
		t.Error("expected loop to be stopped")
	}
}

func TestAgentLoopManager_StopAll(t *testing.T) {
	manager := NewAgentLoopManager()
	llm := newMockLLM()
	deps, _, _, _, _ := buildTestDeps(t, llm)

	for i := 0; i < 3; i++ {
		mID := fmt.Sprintf("mission-all-%d", i)
		tID := fmt.Sprintf("thread-all-%d", i)
		createTestMission(t, deps.MissionStore, deps.ThreadStore, mID, tID)
		_ = manager.StartLoop(AgentLoopConfig{
			AgentID:      fmt.Sprintf("agent-all-%d", i),
			AgentRole:    NodeRoleWorker,
			MissionID:    mID,
			ThreadID:     tID,
			ProjectID:    tID,
			Depth:        1,
			MaxDepth:     3,
			WakeInterval: 60 * time.Second,
			Model:        "test-model",
			SystemPrompt: "test",
		}, deps)
	}

	if manager.RunningCount() != 3 {
		t.Fatalf("expected 3 loops, got %d", manager.RunningCount())
	}

	manager.StopAll()
	time.Sleep(50 * time.Millisecond)

	if manager.RunningCount() != 0 {
		t.Errorf("expected 0 loops after StopAll, got %d", manager.RunningCount())
	}
}

func TestParseLoopTurnResponse_Valid(t *testing.T) {
	raw := `{"thinking":"test","summary":"ok","actions":[{"type":"no_op","payload":{}}]}`
	resp, err := ParseLoopTurnResponse(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetThinking() != "test" {
		t.Errorf("expected thinking 'test', got %q", resp.GetThinking())
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionNoOp {
		t.Errorf("expected no_op, got %q", resp.Actions[0].Type)
	}
}

func TestParseLoopTurnResponse_Invalid(t *testing.T) {
	_, err := ParseLoopTurnResponse(context.Background(), nil, "not json")
	if err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestParseLoopTurnResponse_JSONStringify(t *testing.T) {
	raw := "```json\n" + `{
  "thinking": "writing package.json",
  "summary": "creating project config",
  "actions": [
    {
      "action": "write_file",
      "file_path": "package.json",
      "content": JSON.stringify({
        "name": "todo-app",
        "version": "1.0.0"
      }, null, 2)
    }
  ]
}` + "\n```"
	// The raw JSON contains JSON.stringify() which is invalid. AI repair fixes it.
	mock := &microai.MockReasoner{Fn: func(task, input string) string {
		return `{
  "thinking": "writing package.json",
  "summary": "creating project config",
  "actions": [
    {
      "action": "write_file",
      "file_path": "package.json",
      "content": "{\"name\":\"todo-app\",\"version\":\"1.0.0\"}"
    }
  ]
}`
	}}
	resp, err := ParseLoopTurnResponse(context.Background(), mock, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionWriteFile {
		t.Errorf("expected write_file, got %q", resp.Actions[0].Type)
	}
}

func TestParseLoopTurnResponse_ThinkingObject(t *testing.T) {
	raw := `{"thinking":{"step":"analyze"},"summary":"ok","actions":[{"type":"no_op","payload":{}}]}`
	resp, err := ParseLoopTurnResponse(context.Background(), nil, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetThinking() == "" {
		t.Error("expected non-empty thinking")
	}
}

func TestParseLoopTurnResponse_ActionKeyAlias(t *testing.T) {
	raw := `{"thinking":"test","summary":"ok","actions":[{"action":"write_file","file_path":"main.go","content":"package main"}]}`
	resp, err := ParseLoopTurnResponse(context.Background(), nil, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionWriteFile {
		t.Errorf("expected write_file, got %q", resp.Actions[0].Type)
	}
}

func TestParseLoopTurnResponse_ActionTypeKeyAlias(t *testing.T) {
	raw := `{"thinking":"test","summary":"ok","actions":[{"action_type":"write_file","file_path":"go.mod","content":"module todo\n\ngo 1.21"}]}`
	resp, err := ParseLoopTurnResponse(context.Background(), nil, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionWriteFile {
		t.Errorf("expected write_file, got %q", resp.Actions[0].Type)
	}
}

func TestParseLoopTurnResponse_BacktickContent(t *testing.T) {
	raw := "```json\n{\"thinking\":\"creating go.mod\",\"summary\":\"\",\"actions\":[{\"type\":\"write_file\",\"file_path\":\"go.mod\",\"content\":`module example.com/todo\n\ngo 1.21\n`}]}\n```"
	// The raw JSON contains backtick-delimited strings which are invalid. AI repair fixes it.
	mock := &microai.MockReasoner{Fn: func(task, input string) string {
		return `{"thinking":"creating go.mod","summary":"","actions":[{"type":"write_file","file_path":"go.mod","content":"module example.com/todo\n\ngo 1.21\n"}]}`
	}}
	resp, err := ParseLoopTurnResponse(context.Background(), mock, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionWriteFile {
		t.Errorf("expected write_file, got %q", resp.Actions[0].Type)
	}
}

func TestTestingAgent_Validate_Pass(t *testing.T) {
	llm := newMockLLM(`{"status":"PASS","summary":"Looks great","issues":[]}`)
	agent := NewTestingAgent(llm, "test-model")
	result, err := agent.Validate(context.Background(), TestInput{
		Deliverable:     "Here is the result",
		TodoTitle:       "Write code",
		TodoDescription: "Write clean Go code",
		MissionGoal:     "Build the system",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "PASS" {
		t.Errorf("expected PASS, got %q", result.Status)
	}
}

func TestTestingAgent_Validate_Fail(t *testing.T) {
	llm := newMockLLM(`{"status":"FAIL","summary":"Missing tests","issues":["No unit tests"]}`)
	agent := NewTestingAgent(llm, "test-model")
	result, err := agent.Validate(context.Background(), TestInput{
		Deliverable:     "Code without tests",
		TodoTitle:       "Write tested code",
		TodoDescription: "Write code with tests",
		MissionGoal:     "Quality system",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "FAIL" {
		t.Errorf("expected FAIL, got %q", result.Status)
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(result.Issues))
	}
}

func TestWakeIntervalForRole(t *testing.T) {
	if WakeIntervalForRole(NodeRoleCEO) != DefaultCEOWakeInterval {
		t.Errorf("unexpected CEO interval")
	}
	if WakeIntervalForRole(NodeRoleManager) != DefaultManagerWakeInterval {
		t.Errorf("unexpected Manager interval")
	}
	if WakeIntervalForRole(NodeRoleWorker) != DefaultWorkerWakeInterval {
		t.Errorf("unexpected Worker interval")
	}
}

func TestAgentLoop_StartTodoCreatesSubThread(t *testing.T) {
	// First turn: start_todo, second turn: no-op (keeps loop alive for verification).
	llm := &mockLLM{}
	deps, _, threadStore, executionStore, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-sub", "thread-sub")

	// Create a todo.
	todo, err := deps.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    "mission-sub",
		ThreadID:     "thread-sub",
		Title:        "Implement login page",
		Description:  "Build the login page with email/password",
		OwnerAgentID: "test-worker",
		Priority:     missions.PriorityHigh,
	})
	if err != nil {
		t.Fatal(err)
	}

	llm.responses = []string{
		fmt.Sprintf(`{"thinking":"starting todo","summary":"working","actions":[{"type":"start_todo","payload":{"todoId":"%s"}}]}`, todo.ID),
		`{"thinking":"idle","summary":"waiting","actions":[{"type":"noop","payload":{}}]}`,
	}

	projectDir := t.TempDir()
	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:         "test-worker",
		AgentRole:       NodeRoleWorker,
		MissionID:       "mission-sub",
		ThreadID:        "thread-sub",
		ProjectID:       "thread-sub",
		ProjectLocation: projectDir,
		Depth:           2,
		MaxDepth:        3,
		WakeInterval:    100 * time.Millisecond,
		Model:           "test-model",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify todo was started.
	updatedTodo, err := executionStore.GetTodo(todo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTodo.Status != execution.TodoStatusInProgress {
		t.Errorf("expected todo status %q, got %q", execution.TodoStatusInProgress, updatedTodo.Status)
	}

	// Verify a child thread was created.
	childThreads, _ := threadStore.ListByMission("mission-sub")
	foundSubThread := false
	for _, th := range childThreads {
		if th.ParentThreadID == "thread-sub" && th.Kind == "task" {
			foundSubThread = true
			break
		}
	}
	if !foundSubThread {
		t.Error("expected a child sub-thread to be created for the todo")
	}

	// Verify the loop's activeThreadID was set.
	if loop.activeThreadID == "" {
		t.Error("expected activeThreadID to be set after start_todo")
	}
}

func TestAgentLoop_DeliverWorkClosesSubThread(t *testing.T) {
	llm := &mockLLM{}
	deps, _, threadStore, executionStore, _ := buildTestDeps(t, llm)
	createTestMission(t, deps.MissionStore, deps.ThreadStore, "mission-deliver", "thread-deliver")

	// Remove testing agent to simplify the flow.
	deps.TestingAgent = nil

	todo, err := deps.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    "mission-deliver",
		ThreadID:     "thread-deliver",
		Title:        "Build API endpoint",
		Description:  "Create the /api/users endpoint",
		OwnerAgentID: "test-worker",
		Priority:     missions.PriorityHigh,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Turn 1: start_todo (creates sub-thread).
	// Turn 2: deliver_work (closes sub-thread, posts summary to parent).
	// Turn 3: no-op.
	llm.responses = []string{
		fmt.Sprintf(`{"thinking":"start","summary":"starting","actions":[{"type":"start_todo","payload":{"todoId":"%s"}}]}`, todo.ID),
		fmt.Sprintf(`{"thinking":"done","summary":"finished","actions":[{"type":"deliver_work","payload":{"todoId":"%s","deliverable":"Implemented /api/users endpoint with GET and POST handlers."}}]}`, todo.ID),
		`{"thinking":"idle","summary":"waiting","actions":[{"type":"noop","payload":{}}]}`,
	}

	projectDir := t.TempDir()
	loop, err := NewAgentLoop(AgentLoopConfig{
		AgentID:         "test-worker",
		AgentRole:       NodeRoleWorker,
		MissionID:       "mission-deliver",
		ThreadID:        "thread-deliver",
		ProjectID:       "thread-deliver",
		ProjectLocation: projectDir,
		Depth:           2,
		MaxDepth:        3,
		WakeInterval:    100 * time.Millisecond,
		Model:           "test-model",
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = loop.Run(ctx)

	// Verify todo was completed.
	updatedTodo, err := executionStore.GetTodo(todo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTodo.Status != execution.TodoStatusDone {
		t.Errorf("expected todo status %q, got %q", execution.TodoStatusDone, updatedTodo.Status)
	}

	// Verify the sub-thread was closed.
	childThreads, _ := threadStore.ListByMission("mission-deliver")
	for _, th := range childThreads {
		if th.ParentThreadID == "thread-deliver" && th.Kind == "task" {
			if th.Status != threads.ThreadStatusCompleted {
				t.Errorf("expected sub-thread status %q, got %q", threads.ThreadStatusCompleted, th.Status)
			}
		}
	}

	// Verify a completion summary was posted to the parent thread.
	parentMessages, _ := threadStore.ListMessages("thread-deliver")
	foundSummary := false
	for _, msg := range parentMessages {
		if msg.MessageType == "todo_completion_summary" {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Error("expected a todo_completion_summary message on the parent thread")
	}

	// Verify the agent switched back to main thread.
	if loop.activeThreadID != "" {
		t.Errorf("expected activeThreadID to be cleared after deliver_work, got %q", loop.activeThreadID)
	}
}

func TestNormalizeActionType_EmbeddedDescription(t *testing.T) {
	// Cases that work without AI (exact match, alias, or prefix stripping).
	structural := []struct {
		input    ActionType
		expected ActionType
	}{
		{"create_worker: Auth/session + canonical API contract", ActionCreateWorker},
		{"write_file: create the main.go file", ActionWriteFile},
		{"mark_done", ActionMarkDone},
		{"run_qa- validate output", ActionRunQA},
		{"create_worker", ActionCreateWorker},
		{"unknown_thing", ActionType("unknown_thing")},
		{"delegate", ActionCreateWorker},
		{"deliver_work_after_implementation", ActionDeliverWork},
		// Totally unknown garbage should pass through unchanged.
		{"if_all_artifacts_present", ActionType("if_all_artifacts_present")},
		{"proceed_if_ready", ActionType("proceed_if_ready")},
	}
	for _, tt := range structural {
		got := NormalizeActionType(context.Background(), nil, tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeActionType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}

	// Sentence-like types that need AI classification.
	mock := &microai.MockReasoner{Fn: func(task, input string) string {
		switch {
		case len(input) > 5 && (input[:5] == "Poll " || input[:6] == "Check " || input[:8] == "monitor_"):
			return "check_child"
		default:
			return input
		}
	}}
	aiCases := []struct {
		input    ActionType
		expected ActionType
	}{
		{"Poll child agent agent--1234 (repo layout/config/env).", ActionCheckChild},
		{"Check child agent results (auth/todo scan).", ActionCheckChild},
		{"monitor_child_worktrees", ActionCheckChild},
	}
	for _, tt := range aiCases {
		got := NormalizeActionType(context.Background(), mock, tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeActionType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractEmbeddedPayload(t *testing.T) {
	// Type with embedded JSON payload.
	at, payload := extractEmbeddedPayload(context.Background(), nil, `create_worker: {"name":"auth","problemStatement":"build auth"}`)
	if at != "create_worker" {
		t.Errorf("expected action type 'create_worker', got %q", at)
	}
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}
	var m map[string]string
	if err := json.Unmarshal(payload, &m); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if m["name"] != "auth" {
		t.Errorf("expected name=auth, got %q", m["name"])
	}

	// Type with no embedded JSON, just description text.
	at2, payload2 := extractEmbeddedPayload(context.Background(), nil, "create_worker: build the API service")
	if at2 != "create_worker" {
		t.Errorf("expected 'create_worker', got %q", at2)
	}
	if payload2 != nil {
		t.Errorf("expected nil payload for text description, got %s", string(payload2))
	}

	// Normal action type with no extra text.
	at3, payload3 := extractEmbeddedPayload(context.Background(), nil, "write_file")
	if at3 != "write_file" {
		t.Errorf("expected 'write_file', got %q", at3)
	}
	if payload3 != nil {
		t.Errorf("expected nil payload, got %s", string(payload3))
	}

	// Unknown action type — should pass through unchanged.
	at4, payload4 := extractEmbeddedPayload(context.Background(), nil, "custom_action: do something")
	if at4 != "custom_action: do something" {
		t.Errorf("expected passthrough, got %q", at4)
	}
	if payload4 != nil {
		t.Errorf("expected nil payload, got %s", string(payload4))
	}
}

func TestParseLoopTurnResponse_EmbeddedTypePayload(t *testing.T) {
	raw := `{
		"thinking": "creating a sub-worker",
		"summary": "delegating auth work",
		"actions": [
			{"type": "create_worker: {\"name\":\"auth-agent\",\"problemStatement\":\"implement auth\"}", "payload": {}}
		]
	}`
	resp, err := ParseLoopTurnResponse(context.Background(), nil, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != ActionCreateWorker {
		t.Errorf("expected action type %q, got %q", ActionCreateWorker, resp.Actions[0].Type)
	}
	// The payload should have been recovered from the embedded JSON.
	var p CreateWorkerPayload
	if err := json.Unmarshal(resp.Actions[0].Payload, &p); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if p.Name != "auth-agent" {
		t.Errorf("expected name=auth-agent, got %q", p.Name)
	}
	if p.ProblemStatement != "implement auth" {
		t.Errorf("expected problemStatement='implement auth', got %q", p.ProblemStatement)
	}
}
