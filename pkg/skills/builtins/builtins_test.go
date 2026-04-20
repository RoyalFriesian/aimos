package builtins_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/skills"
	"github.com/Sarnga/agent-platform/pkg/skills/builtins"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func newEnv(t *testing.T) *skills.Env {
	t.Helper()
	ts := threads.NewMemoryStore()
	ms := missions.NewMemoryStore()
	es := execution.NewMemoryStore()
	exRT, err := execution.NewRuntime(es)
	if err != nil {
		t.Fatal(err)
	}
	_ = ts.CreateThread(threads.Thread{
		ID:        "thread-1",
		MissionID: "mission-1",
		Title:     "Test Thread",
		Status:    threads.ThreadStatusActive,
	})
	return &skills.Env{
		ProjectDir:       t.TempDir(),
		ProjectID:        "proj-1",
		MissionID:        "mission-1",
		ThreadID:         "thread-1",
		AgentID:          "agent-1",
		AgentRole:        "Worker",
		Depth:            1,
		MaxDepth:         5,
		Model:            "gpt-4.1-nano",
		ThreadStore:      ts,
		MissionStore:     ms,
		ExecutionRuntime: exRT,
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestRegisterAll(t *testing.T) {
	reg := skills.NewRegistry()
	builtins.RegisterAll(reg)
	all := reg.All()
	if len(all) != 18 {
		names := make([]string, len(all))
		for i, s := range all {
			names[i] = s.Name
		}
		t.Fatalf("expected 18 skills, got %d: %v", len(all), names)
	}
}

func TestPostMessage(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandlePostMessage(context.Background(), env, mustJSON(map[string]string{
		"content": "hello world",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "thread-1") {
		t.Fatalf("expected thread-1 in result, got %s", result)
	}
	msgs, _ := env.ThreadStore.ListMessages("thread-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "hello world" {
		t.Fatalf("unexpected content: %s", msgs[0].Content)
	}
}

func TestPostMessageEmptyContent(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandlePostMessage(context.Background(), env, mustJSON(map[string]string{
		"content": "",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "skipping") {
		t.Fatalf("expected skip message, got %s", result)
	}
}

func TestPostMessageTargetThread(t *testing.T) {
	env := newEnv(t)
	_ = env.ThreadStore.CreateThread(threads.Thread{
		ID:        "thread-other",
		MissionID: "mission-1",
		Title:     "Other Thread",
		Status:    threads.ThreadStatusActive,
	})
	result, err := builtins.HandlePostMessage(context.Background(), env, mustJSON(map[string]string{
		"content":          "hi there",
		"target_thread_id": "thread-other",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "thread-other") {
		t.Fatalf("expected thread-other in result, got %s", result)
	}
	msgs, _ := env.ThreadStore.ListMessages("thread-other")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message on target thread, got %d", len(msgs))
	}
}

func TestNoOp(t *testing.T) {
	result, err := builtins.HandleNoOp(context.Background(), &skills.Env{}, json.RawMessage("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "no action taken" {
		t.Fatalf("unexpected: %s", result)
	}
}

func TestCreateTodo(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandleCreateTodo(context.Background(), env, mustJSON(map[string]string{
		"title":       "Write tests",
		"description": "Add unit tests for builtins",
		"priority":    "high",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Write tests") {
		t.Fatalf("expected title in result, got %s", result)
	}
	if !strings.Contains(result, "created") {
		t.Fatalf("expected created in result, got %s", result)
	}
}

func TestCreateTodoMissingTitle(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleCreateTodo(context.Background(), env, mustJSON(map[string]string{
		"description": "no title",
	}))
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestCreateTodoNoRuntime(t *testing.T) {
	env := newEnv(t)
	env.ExecutionRuntime = nil
	_, err := builtins.HandleCreateTodo(context.Background(), env, mustJSON(map[string]string{
		"title": "test",
	}))
	if err == nil {
		t.Fatal("expected error when execution runtime is nil")
	}
}

func TestCompleteTodo(t *testing.T) {
	env := newEnv(t)
	todo, err := env.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    env.MissionID,
		ThreadID:     env.ThreadID,
		Title:        "Test todo",
		Description:  "A test todo item",
		OwnerAgentID: env.AgentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = env.ExecutionRuntime.StartTodo(todo.ID)
	result, err := builtins.HandleCompleteTodo(context.Background(), env, mustJSON(map[string]string{
		"todo_id": todo.ID,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "completed") {
		t.Fatalf("expected completed in result, got %s", result)
	}
}

func TestBlockTodo(t *testing.T) {
	env := newEnv(t)
	todo, _ := env.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    env.MissionID,
		ThreadID:     env.ThreadID,
		Title:        "Blockable todo",
		Description:  "A blockable todo item",
		OwnerAgentID: env.AgentID,
	})
	_, _ = env.ExecutionRuntime.StartTodo(todo.ID)
	result, err := builtins.HandleBlockTodo(context.Background(), env, mustJSON(map[string]string{
		"todo_id": todo.ID,
		"reason":  "waiting on data",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "blocked") {
		t.Fatalf("expected blocked in result, got %s", result)
	}
}

func TestStartTodo(t *testing.T) {
	env := newEnv(t)
	todo, _ := env.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    env.MissionID,
		ThreadID:     env.ThreadID,
		Title:        "Startable todo",
		Description:  "A startable todo item",
		OwnerAgentID: env.AgentID,
	})
	var switched bool
	env.OnThreadSwitch = func(threadID string, todoID string) {
		switched = true
	}
	result, err := builtins.HandleStartTodo(context.Background(), env, mustJSON(map[string]string{
		"todo_id":   todo.ID,
		"thread_id": "thread-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "started") {
		t.Fatalf("expected started in result, got %s", result)
	}
	if !switched {
		t.Fatal("expected OnThreadSwitch to be called")
	}
}

func TestCompleteTodoMissingID(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleCompleteTodo(context.Background(), env, mustJSON(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for empty todo_id")
	}
}

func TestWriteAndReadFile(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	var filesChangedCalled bool
	env.OnFilesChanged = func(ctx context.Context, path string) {
		filesChangedCalled = true
	}
	writeResult, err := builtins.HandleWriteFile(ctx, env, mustJSON(map[string]string{
		"path":    "subdir/test.txt",
		"content": "hello from test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(writeResult, "test.txt") {
		t.Fatalf("expected filename in result, got %s", writeResult)
	}
	if !filesChangedCalled {
		t.Fatal("expected OnFilesChanged to be called")
	}
	readResult, err := builtins.HandleReadFile(ctx, env, mustJSON(map[string]string{
		"path": "subdir/test.txt",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if readResult != "hello from test" {
		t.Fatalf("expected hello from test, got %s", readResult)
	}
}

func TestWriteFilePathTraversal(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleWriteFile(context.Background(), env, mustJSON(map[string]string{
		"path":    "../../../etc/passwd",
		"content": "malicious",
	}))
	if err == nil {
		t.Fatal("expected path traversal to be blocked")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("expected traversal error, got: %s", err.Error())
	}
}

func TestWriteFileMissingFields(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleWriteFile(context.Background(), env, mustJSON(map[string]string{
		"path": "test.txt",
	}))
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestReadFileNotFound(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleReadFile(context.Background(), env, mustJSON(map[string]string{
		"path": "nonexistent.txt",
	}))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestWriteFileNoProjectDir(t *testing.T) {
	env := newEnv(t)
	env.ProjectDir = ""
	_, err := builtins.HandleWriteFile(context.Background(), env, mustJSON(map[string]string{
		"path":    "test.txt",
		"content": "data",
	}))
	if err == nil {
		t.Fatal("expected error when project dir is empty")
	}
}

func TestReadFileLargeFileTruncation(t *testing.T) {
	env := newEnv(t)
	largeContent := strings.Repeat("x", 100_001)
	fullPath := filepath.Join(env.ProjectDir, "large.txt")
	if err := os.WriteFile(fullPath, []byte(largeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := builtins.HandleReadFile(context.Background(), env, mustJSON(map[string]string{
		"path": "large.txt",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[truncated]") {
		t.Fatal("expected truncation marker for large files")
	}
}

func TestMarkDone(t *testing.T) {
	env := newEnv(t)
	_ = env.MissionStore.CreateProgram(missions.Program{
		ID:       "prog-1",
		ClientID: "client-1",
		Title:    "Test Program",
	})
	_ = env.MissionStore.CreateMission(missions.Mission{
		ID:           "mission-1",
		ProgramID:    "prog-1",
		Title:        "Test Mission",
		OwnerAgentID: "agent-1",
		OwnerRole:    "Worker",
		MissionType:  "execution",
		Status:       missions.MissionStatusActive,
	})
	var terminated bool
	env.OnSelfTerminate = func() { terminated = true }
	result, err := builtins.HandleMarkDone(context.Background(), env, mustJSON(map[string]string{
		"summary": "all done",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "marked done") {
		t.Fatalf("expected marked done in result, got %s", result)
	}
	if !terminated {
		t.Fatal("expected OnSelfTerminate to be called")
	}
	m, _ := env.MissionStore.GetMission("mission-1")
	if m.Status != missions.MissionStatusCompleted {
		t.Fatalf("expected mission status completed, got %s", m.Status)
	}
	msgs, _ := env.ThreadStore.ListMessages("thread-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestDeliverWork(t *testing.T) {
	env := newEnv(t)
	var filesChanged bool
	env.OnFilesChanged = func(ctx context.Context, path string) { filesChanged = true }
	result, err := builtins.HandleDeliverWork(context.Background(), env, mustJSON(map[string]string{
		"deliverable":  "implemented feature X",
		"todo_title":   "Build feature X",
		"mission_goal": "Ship feature X",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result != "Work delivered successfully" {
		t.Fatalf("unexpected result: %s", result)
	}
	if !filesChanged {
		t.Fatal("expected OnFilesChanged to be called")
	}
}

func TestDeliverWorkEmptyDeliverable(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleDeliverWork(context.Background(), env, mustJSON(map[string]string{
		"deliverable": "",
	}))
	if err == nil {
		t.Fatal("expected error for empty deliverable")
	}
}

func TestDeliverWorkTestingRejects(t *testing.T) {
	env := newEnv(t)
	env.Testing = &mockTesting{status: "FAIL", summary: "tests failed", issues: []string{"broken"}}
	result, err := builtins.HandleDeliverWork(context.Background(), env, mustJSON(map[string]string{
		"deliverable": "bad code",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "REJECTED") {
		t.Fatalf("expected rejection, got %s", result)
	}
}

func TestEscalate(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandleEscalate(context.Background(), env, mustJSON(map[string]string{
		"issue":      "blocked on API access",
		"severity":   "high",
		"suggestion": "grant API key",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Escalation sent") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestEscalateEmptyIssue(t *testing.T) {
	env := newEnv(t)
	_, err := builtins.HandleEscalate(context.Background(), env, mustJSON(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for empty issue")
	}
}

func TestScheduleFollowup(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandleScheduleFollowup(context.Background(), env, mustJSON(map[string]any{
		"delay_minutes": 10,
		"reason":        "check back on progress",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "scheduled") {
		t.Fatalf("expected scheduled in result, got %s", result)
	}
}

func TestScheduleFollowupDefaultDelay(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandleScheduleFollowup(context.Background(), env, mustJSON(map[string]any{
		"delay_minutes": 0,
		"reason":        "default timing",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "scheduled") {
		t.Fatalf("expected scheduled in result, got %s", result)
	}
}

func TestScheduleFollowupNoRuntime(t *testing.T) {
	env := newEnv(t)
	env.ExecutionRuntime = nil
	result, err := builtins.HandleScheduleFollowup(context.Background(), env, mustJSON(map[string]any{
		"delay_minutes": 5,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "not available") {
		t.Fatalf("expected not available, got %s", result)
	}
}

func TestCheckChildNoNodeStore(t *testing.T) {
	env := newEnv(t)
	env.NodeStore = nil
	result, err := builtins.HandleCheckChild(context.Background(), env, mustJSON(map[string]string{
		"child_agent_id": "child-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "no node store") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCheckChildNoChildren(t *testing.T) {
	env := newEnv(t)
	env.NodeStore = &mockNodeStore{children: nil}
	result, err := builtins.HandleCheckChild(context.Background(), env, mustJSON(map[string]string{
		"child_agent_id": "child-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No child agents") {
		t.Fatalf("expected no children message, got %s", result)
	}
}

func TestCheckChildPostsMessage(t *testing.T) {
	env := newEnv(t)
	_ = env.ThreadStore.CreateThread(threads.Thread{
		ID:        "child-thread",
		MissionID: "mission-1",
		Title:     "Child Thread",
		Status:    threads.ThreadStatusActive,
	})
	env.NodeStore = &mockNodeStore{
		children: []skills.NodeInfo{
			{ID: "child-1", ThreadID: "child-thread", Status: "active"},
		},
	}
	result, err := builtins.HandleCheckChild(context.Background(), env, mustJSON(map[string]string{
		"child_agent_id": "child-1",
		"question":       "How is it going?",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "child-1") {
		t.Fatalf("expected child-1 in result, got %s", result)
	}
	msgs, _ := env.ThreadStore.ListMessages("child-thread")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message on child thread, got %d", len(msgs))
	}
	if msgs[0].Content != "How is it going?" {
		t.Fatalf("unexpected message: %s", msgs[0].Content)
	}
}

func TestResolveConflict(t *testing.T) {
	env := newEnv(t)
	_ = env.ThreadStore.CreateThread(threads.Thread{
		ID:        "child-thread",
		MissionID: "mission-1",
		Title:     "Resolve Thread",
		Status:    threads.ThreadStatusActive,
	})
	env.NodeStore = &mockNodeStore{
		children: []skills.NodeInfo{
			{ID: "child-1", ThreadID: "child-thread", Status: "active"},
		},
	}
	result, err := builtins.HandleResolveConflict(context.Background(), env, mustJSON(map[string]string{
		"target_child_id": "child-1",
		"resolution":      "use approach B",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "child-1") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestResolveConflictChildNotFound(t *testing.T) {
	env := newEnv(t)
	env.NodeStore = &mockNodeStore{children: nil}
	_, err := builtins.HandleResolveConflict(context.Background(), env, mustJSON(map[string]string{
		"target_child_id": "nonexistent",
		"resolution":      "does not matter",
	}))
	if err == nil {
		t.Fatal("expected error for missing child")
	}
}

func TestCreateWorkerAtMaxDepth(t *testing.T) {
	env := newEnv(t)
	env.Depth = 5
	env.MaxDepth = 5
	result, err := builtins.HandleCreateWorker(context.Background(), env, mustJSON(map[string]string{
		"problem_statement": "do stuff",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "maximum depth") {
		t.Fatalf("expected max depth message, got %s", result)
	}
}

func TestCreateWorkerNoProblem(t *testing.T) {
	env := newEnv(t)
	env.LoopManager = &mockLoopManager{}
	_, err := builtins.HandleCreateWorker(context.Background(), env, mustJSON(map[string]string{
		"name": "worker-1",
	}))
	if err == nil {
		t.Fatal("expected error for missing problem_statement")
	}
}

func TestCreateWorkerNoLoopManager(t *testing.T) {
	env := newEnv(t)
	env.LoopManager = nil
	_, err := builtins.HandleCreateWorker(context.Background(), env, mustJSON(map[string]string{
		"problem_statement": "do stuff",
	}))
	if err == nil {
		t.Fatal("expected error when loop manager is nil")
	}
}

func TestCreateWorkerSuccess(t *testing.T) {
	env := newEnv(t)
	env.LoopManager = &mockLoopManager{}
	env.MissionRuntime = nil
	result, err := builtins.HandleCreateWorker(context.Background(), env, mustJSON(map[string]string{
		"problem_statement": "implement auth module",
		"name":              "auth-worker",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "auth-worker") {
		t.Fatalf("expected worker name in result, got %s", result)
	}
}

func TestMergeBranchDeferred(t *testing.T) {
	env := newEnv(t)
	result, err := builtins.HandleMergeBranch(context.Background(), env, mustJSON(map[string]string{
		"worker_name": "some-worker",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "deferred") {
		t.Fatalf("expected deferred message, got %s", result)
	}
}

func TestMergeBranchNoProjectDir(t *testing.T) {
	env := newEnv(t)
	env.ProjectDir = ""
	_, err := builtins.HandleMergeBranch(context.Background(), env, mustJSON(map[string]string{
		"worker_name": "w",
	}))
	if err == nil {
		t.Fatal("expected error when project dir is empty")
	}
}

func TestRunQANoAgent(t *testing.T) {
	env := newEnv(t)
	env.QA = nil
	result, err := builtins.HandleRunQA(context.Background(), env, json.RawMessage("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "not available") {
		t.Fatalf("expected not available, got %s", result)
	}
}

func TestRunQASuccess(t *testing.T) {
	env := newEnv(t)
	env.QA = &mockQA{status: "PASS", summary: "all good"}
	result, err := builtins.HandleRunQA(context.Background(), env, json.RawMessage("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "PASS") {
		t.Fatalf("expected PASS, got %s", result)
	}
}

func TestUpdateSummaryNoRuntime(t *testing.T) {
	env := newEnv(t)
	env.MissionStateRuntime = nil
	result, err := builtins.HandleUpdateSummary(context.Background(), env, mustJSON(map[string]string{
		"summary_text": "progress update",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "skipped") {
		t.Fatalf("expected skip message, got %s", result)
	}
}

// --- Mocks ---

type mockNodeStore struct {
	nodes    map[string]skills.NodeInfo
	children []skills.NodeInfo
}

func (m *mockNodeStore) GetNode(agentID string) (skills.NodeInfo, error) {
	if m.nodes != nil {
		if n, ok := m.nodes[agentID]; ok {
			return n, nil
		}
	}
	return skills.NodeInfo{}, nil
}

func (m *mockNodeStore) ListChildren(_ string) ([]skills.NodeInfo, error) {
	return m.children, nil
}

func (m *mockNodeStore) UpdateStatus(_ string, _ string) error {
	return nil
}

type mockLoopManager struct {
	started []skills.ChildLoopConfig
}

func (m *mockLoopManager) StartChildLoop(_ context.Context, cfg skills.ChildLoopConfig) error {
	m.started = append(m.started, cfg)
	return nil
}

func (m *mockLoopManager) StopLoop(_ string) {}

type mockTesting struct {
	status  string
	summary string
	issues  []string
}

func (m *mockTesting) Validate(_ context.Context, _ skills.TestInput) (skills.TestResult, error) {
	return skills.TestResult{Status: m.status, Summary: m.summary, Issues: m.issues}, nil
}

type mockQA struct {
	status  string
	summary string
}

func (m *mockQA) ValidateProject(_ context.Context, _ skills.QAInput) (skills.QAResult, error) {
	return skills.QAResult{Status: m.status, Summary: m.summary}, nil
}
