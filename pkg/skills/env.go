package skills

import (
	"context"

	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/microai"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// NodeStore is the subset of the agent node store interface needed by skills.
// It lives here to avoid a circular import between pkg/skills and pkg/agents.
type NodeStore interface {
	GetNode(agentID string) (NodeInfo, error)
	ListChildren(parentAgentID string) ([]NodeInfo, error)
	UpdateStatus(agentID string, status string) error
}

// NodeInfo is a minimal representation of an agent node used by skill handlers.
type NodeInfo struct {
	ID               string
	ParentAgentID    string
	RootAgentID      string
	ProjectID        string
	ThreadID         string
	MissionID        string
	Name             string
	Role             string
	Depth            int
	ProblemStatement string
	Status           string
	Model            string
	Paused           bool
}

// LoopManager is the interface for starting/stopping child agent loops.
type LoopManager interface {
	StartChildLoop(ctx context.Context, config ChildLoopConfig) error
	StopLoop(agentID string)
}

// ChildLoopConfig holds the parameters for launching a child agent loop.
type ChildLoopConfig struct {
	AgentID         string
	AgentRole       string
	MissionID       string
	ThreadID        string
	ProjectID       string
	ProjectLocation string
	Depth           int
	MaxDepth        int
	Model           string
	SystemPrompt    string
}

// TestingValidator is the interface for the testing agent used by skill handlers.
type TestingValidator interface {
	Validate(ctx context.Context, input TestInput) (TestResult, error)
}

// TestInput holds the inputs for testing agent validation.
type TestInput struct {
	Deliverable        string
	TodoTitle          string
	TodoDescription    string
	AcceptanceCriteria string
	MissionGoal        string
}

// TestResult holds the output of a testing agent validation.
type TestResult struct {
	Status  string
	Summary string
	Issues  []string
}

// QAValidator is the interface for the QA agent used by skill handlers.
type QAValidator interface {
	ValidateProject(ctx context.Context, input QAInput) (QAResult, error)
}

// QAInput holds the inputs for QA agent validation.
type QAInput struct {
	ProjectDir  string
	MissionGoal string
	AgentID     string
}

// QAResult holds the output of a QA agent validation.
type QAResult struct {
	Status  string
	Summary string
}

// Env provides all dependencies to skill handlers without coupling them
// to the agent loop implementation. The agent loop constructs an Env at
// the start of each turn and passes it to every handler invocation.
type Env struct {
	// Identity
	ProjectDir string
	ProjectID  string
	MissionID  string
	ThreadID   string
	AgentID    string
	AgentRole  string // "CEO", "Manager", "Worker", "Tester"
	Depth      int
	MaxDepth   int
	Model      string

	// Stores
	NodeStore       NodeStore
	ThreadStore     threads.Store
	MissionStore    missions.Store
	AttachmentStore attachments.Store

	// Runtimes
	ExecutionRuntime    *execution.Runtime
	MissionRuntime      *missions.Runtime
	MissionStateRuntime *missionstate.Runtime
	ContextBuilder      *contextpacks.Builder

	// Agent services
	LoopManager LoopManager
	Testing     TestingValidator
	QA          QAValidator
	Reasoner    microai.Interface

	// Callbacks — the agent loop sets these to update its own state
	// when a handler performs side effects.
	OnFilesChanged  func(ctx context.Context, projectPath string)
	OnSelfTerminate func()
	OnThreadSwitch  func(newThreadID string, todoID string)
}
