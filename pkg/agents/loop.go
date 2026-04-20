package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/contextdocs"
	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/gitops"
	"github.com/Sarnga/agent-platform/pkg/microai"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/skills"
	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

var loopLogger = slog.Default()

const (
	DefaultCEOWakeInterval       = 15 * time.Second
	DefaultManagerWakeInterval   = 20 * time.Second
	DefaultWorkerWakeInterval    = 15 * time.Second
	DefaultMaxDepth              = 3
	DefaultSummaryEveryNTurns    = 5
	DefaultMaxSelfCritiquePasses = 3
	DefaultMaxTestRetries        = 3
)

// LoopCompletionClient is the LLM interface used by the agent loop.
type LoopCompletionClient interface {
	Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error)
}

// AgentLoopConfig holds all parameters for creating an AgentLoop.
type AgentLoopConfig struct {
	AgentID         string
	AgentRole       NodeRole
	MissionID       string
	ThreadID        string
	ProjectID       string
	ProjectLocation string
	Depth           int
	MaxDepth        int
	WakeInterval    time.Duration
	Model           string
	SystemPrompt    string
}

// AgentLoopDeps holds injected dependencies for the agent loop runtime.
type AgentLoopDeps struct {
	LLM                 LoopCompletionClient
	NodeStore           NodeStore
	ThreadStore         threads.Store
	MissionStore        missions.Store
	AttachmentStore     attachments.Store
	ExecutionRuntime    *execution.Runtime
	MissionRuntime      *missions.Runtime
	MissionStateRuntime *missionstate.Runtime
	ContextBuilder      *contextpacks.Builder
	LoopManager         *AgentLoopManager
	TestingAgent        *TestingAgent
	QAAgent             *QAAgent
	Reasoner            microai.Interface
	// OnFilesChanged is called once per turn after any write_file actions
	// succeed. The callback receives the project path and can trigger a
	// knowledge re-index. Optional — nil means no-op.
	OnFilesChanged func(ctx context.Context, projectPath string)
	// SkillRegistry provides function-calling tools for the agent loop.
	// When set, executeTurn uses GenerateWithTools with the OpenAI tool-calling
	// loop instead of the legacy JSON-action parsing path.
	SkillRegistry *skills.Registry
	// WakeConfig holds runtime-tunable wake intervals shared across all loops.
	// When non-nil, loops dynamically adjust their ticker on each tick.
	WakeConfig *WakeIntervalConfig
}

// AgentLoop runs a background loop for a single agent.
type AgentLoop struct {
	config AgentLoopConfig
	deps   AgentLoopDeps

	turnCount            int
	lastChildCheck       map[string]time.Time
	mu                   sync.Mutex
	cancel               context.CancelFunc
	paused               bool
	nextWakeAt           time.Time
	filesChangedThisTurn bool
	// turnsWithPendingFiles tracks consecutive turns where files were written
	// but no deliver_work was called. Auto-deliver triggers only after at least
	// 2 such turns or when the LLM stops producing write_file actions.
	turnsWithPendingFiles int

	// lastProcessedMessageID tracks the most recent message ID seen on the
	// thread. When no new messages arrive between ticks, the loop skips the
	// LLM call entirely to avoid wasting tokens on idle polls.
	lastProcessedMessageID string
	// idleSkips counts consecutive turns skipped due to no new messages.
	idleSkips int

	// activeThreadID is set when the agent is working within a todo
	// sub-thread. When non-empty, messages and context are scoped to
	// this thread instead of config.ThreadID. Cleared on deliver_work.
	activeThreadID string
	// activeTodoID tracks the todo whose sub-thread is currently active.
	activeTodoID string

	// chatHistory writes structured conversation logs to project files.
	chatHistory *ChatHistoryWriter
	// chatSummarizer generates periodic conversation summaries via LLM.
	chatSummarizer *ChatSummarizer
}

// NewAgentLoop creates a loop but does not start it. Call Run(ctx) to start.
func NewAgentLoop(config AgentLoopConfig, deps AgentLoopDeps) (*AgentLoop, error) {
	if config.AgentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if config.MissionID == "" {
		return nil, fmt.Errorf("mission id is required")
	}
	if config.ThreadID == "" {
		return nil, fmt.Errorf("thread id is required")
	}
	if config.WakeInterval <= 0 {
		config.WakeInterval = DefaultWorkerWakeInterval
	}
	if config.MaxDepth <= 0 {
		config.MaxDepth = DefaultMaxDepth
	}
	if config.Model == "" {
		config.Model = "gpt-4.1"
	}
	if deps.LLM == nil {
		return nil, fmt.Errorf("LLM client is required")
	}
	if deps.NodeStore == nil {
		return nil, fmt.Errorf("node store is required")
	}
	if deps.ThreadStore == nil {
		return nil, fmt.Errorf("thread store is required")
	}
	if deps.MissionStore == nil {
		return nil, fmt.Errorf("mission store is required")
	}
	if deps.ExecutionRuntime == nil {
		return nil, fmt.Errorf("execution runtime is required")
	}
	if deps.MissionRuntime == nil {
		return nil, fmt.Errorf("mission runtime is required")
	}
	if deps.MissionStateRuntime == nil {
		return nil, fmt.Errorf("mission state runtime is required")
	}
	if deps.ContextBuilder == nil {
		return nil, fmt.Errorf("context builder is required")
	}
	return &AgentLoop{
		config:         config,
		deps:           deps,
		lastChildCheck: make(map[string]time.Time),
		chatHistory:    NewChatHistoryWriter(),
		chatSummarizer: NewChatSummarizer(deps.LLM, config.Model),
	}, nil
}

// runContext creates a background context for use when started by the loop manager.
func (l *AgentLoop) runContext() context.Context {
	return context.Background()
}

// Run starts the agent loop. Blocks until ctx is cancelled.
func (l *AgentLoop) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	l.mu.Lock()
	l.cancel = cancel
	l.mu.Unlock()
	defer cancel()

	loopLogger.Info("agent loop started",
		"agentID", l.config.AgentID,
		"role", l.config.AgentRole,
		"missionID", l.config.MissionID,
		"depth", l.config.Depth,
		"interval", l.config.WakeInterval,
	)

	// Register this agent in the chat history index so other agents can
	// discover its conversation log file.
	if l.config.ProjectLocation != "" {
		if err := l.chatHistory.UpdateIndex(l.config.ProjectLocation, ChatHistoryAgentEntry{
			AgentID:   l.config.AgentID,
			Role:      string(l.config.AgentRole),
			ThreadID:  l.config.ThreadID,
			MissionID: l.config.MissionID,
			File:      sanitizeFilename(l.config.AgentID) + ".jsonl",
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			loopLogger.Warn("failed to update chat history index", "agentID", l.config.AgentID, "error", err)
		}
	}

	currentInterval := l.effectiveInterval()
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	l.mu.Lock()
	l.nextWakeAt = time.Now().Add(currentInterval)
	l.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			loopLogger.Info("agent loop stopped", "agentID", l.config.AgentID)
			return nil
		case <-ticker.C:
			// Dynamically adjust ticker if the shared wake config changed.
			newInterval := l.effectiveInterval()
			if newInterval != currentInterval {
				loopLogger.Info("wake interval changed",
					"agentID", l.config.AgentID,
					"old", currentInterval,
					"new", newInterval,
				)
				ticker.Reset(newInterval)
				currentInterval = newInterval
			}

			l.mu.Lock()
			isPaused := l.paused
			l.nextWakeAt = time.Now().Add(currentInterval)
			l.mu.Unlock()

			if isPaused {
				loopLogger.Debug("agent loop paused, skipping turn", "agentID", l.config.AgentID)
				continue
			}

			if err := l.executeTurn(ctx); err != nil {
				loopLogger.Error("agent loop turn failed",
					"agentID", l.config.AgentID,
					"error", err,
				)
			}
		}
	}
}

// effectiveInterval returns the current wake interval for this loop's role,
// reading from the shared WakeConfig if available, otherwise using the static config.
func (l *AgentLoop) effectiveInterval() time.Duration {
	if wc := l.deps.WakeConfig; wc != nil {
		var secs int64
		switch l.config.AgentRole {
		case NodeRoleCEO:
			secs = wc.CEOSeconds()
		case NodeRoleManager:
			secs = wc.ManagerSeconds()
		default:
			secs = wc.WorkerSeconds()
		}
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return l.config.WakeInterval
}

// Stop cancels the loop.
func (l *AgentLoop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

// Pause pauses the loop — the ticker keeps running but turns are skipped.
func (l *AgentLoop) Pause() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = true
}

// Resume resumes a paused loop.
func (l *AgentLoop) Resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = false
}

// IsPaused returns whether the loop is paused.
func (l *AgentLoop) IsPaused() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.paused
}

// NextWakeAt returns the time of the next scheduled tick.
func (l *AgentLoop) NextWakeAt() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nextWakeAt
}

// Config returns the loop config (read-only).
func (l *AgentLoop) Config() AgentLoopConfig {
	return l.config
}

// currentThreadID returns the active sub-thread if one is set (todo sub-thread),
// otherwise the main thread from the config.
func (l *AgentLoop) currentThreadID() string {
	if l.activeThreadID != "" {
		return l.activeThreadID
	}
	return l.config.ThreadID
}

func (l *AgentLoop) executeTurn(ctx context.Context) error {
	l.turnCount++

	// 0. Inject LogContext so all LLM calls from this turn carry proper
	// project/mission/thread metadata for logging and file routing.
	ctx = aiclients.WithLogContext(ctx, aiclients.LogContext{
		ProjectSlug: l.config.ProjectLocation,
		MissionID:   l.config.MissionID,
		ThreadID:    l.currentThreadID(),
	})

	// 1. Check if our mission is terminal — if so, self-terminate.
	mission, err := l.deps.MissionStore.GetMission(l.config.MissionID)
	if err != nil {
		return fmt.Errorf("load mission: %w", err)
	}
	if missions.IsTerminalMissionStatus(mission.Status) {
		loopLogger.Info("mission is terminal, stopping loop", "agentID", l.config.AgentID, "status", mission.Status)
		l.selfTerminate()
		return nil
	}

	// 1b. Staleness check — skip LLM call if no new messages since last turn.
	// This avoids burning tokens every 15s when the thread is idle.
	if changed, latestID := l.hasNewMessages(); !changed {
		l.idleSkips++
		if l.idleSkips%20 == 1 { // log every 20th skip to avoid spam
			loopLogger.Debug("skipping idle turn, no new messages",
				"agentID", l.config.AgentID,
				"threadID", l.currentThreadID(),
				"idleSkips", l.idleSkips,
			)
		}
		return nil
	} else {
		if l.idleSkips > 0 {
			loopLogger.Info("resuming after idle",
				"agentID", l.config.AgentID,
				"skippedTurns", l.idleSkips,
			)
		}
		l.idleSkips = 0
		l.lastProcessedMessageID = latestID
	}

	// 2. Build context pack.
	contextPack, err := l.deps.ContextBuilder.BuildMissionPack(l.config.MissionID, l.currentThreadID(), contextpacks.BuildOptions{
		RecentMessagesLimit: 8,
		IncludeChildRollups: true,
	})
	if err != nil {
		return fmt.Errorf("build context pack: %w", err)
	}

	// 3. Load child agents.
	children, err := l.deps.NodeStore.ListChildren(l.config.AgentID)
	if err != nil {
		return fmt.Errorf("list children: %w", err)
	}

	// 3b. Auto-completion check for CEO/Manager: if all children are
	// completed and all todos are done, mark this mission done without
	// an LLM call. This prevents idle spinning when the LLM fails to
	// detect it should call mark_done.
	if (l.config.AgentRole == NodeRoleCEO || l.config.AgentRole == NodeRoleManager) && len(children) > 0 {
		if l.allChildrenCompleted(children) && l.allTodosDone() {
			loopLogger.Info("auto-completing: all children completed and all todos done",
				"agentID", l.config.AgentID, "missionID", l.config.MissionID)
			_ = l.postAgentMessage(ctx, "All delegated work completed. Marking mission done.", "agent_auto_complete")
			return l.handleMarkDone(ctx)
		}
	}

	// 4. Build user prompt with context.
	userPrompt := l.buildUserPrompt(contextPack, children)

	// 5. Call LLM — use tool-calling path when skill registry is available.
	if l.deps.SkillRegistry != nil {
		return l.executeTurnWithTools(ctx, userPrompt)
	}

	// Legacy path: generate raw text and parse actions from JSON.
	rawResponse, err := l.deps.LLM.Generate(ctx, l.config.Model, l.config.SystemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM generate: %w", err)
	}

	// DEBUG: log raw LLM response (first 1000 chars) and parsed action count.
	rawPreview := rawResponse
	if len(rawPreview) > 1000 {
		rawPreview = rawPreview[:1000] + "..."
	}
	loopLogger.Info("LLM raw response preview",
		"agentID", l.config.AgentID,
		"len", len(rawResponse),
		"preview", rawPreview,
	)

	// 6. Parse actions.
	turnResponse, err := ParseLoopTurnResponse(ctx, l.deps.Reasoner, rawResponse)
	if err != nil {
		// If parsing fails, post the raw response as a message and move on.
		loopLogger.Warn("failed to parse loop turn response, posting raw",
			"agentID", l.config.AgentID,
			"error", err,
		)
		return l.postAgentMessage(ctx, rawResponse, "agent_raw_response")
	}

	// DEBUG: log parsed action types.
	actionTypes := make([]string, len(turnResponse.Actions))
	for i, a := range turnResponse.Actions {
		actionTypes[i] = string(a.Type)
	}
	loopLogger.Info("parsed actions",
		"agentID", l.config.AgentID,
		"count", len(turnResponse.Actions),
		"types", strings.Join(actionTypes, ", "),
	)

	// 7. Execute actions.
	l.filesChangedThisTurn = false
	hadDeliverWork := false
	hadMarkDone := false
	for _, action := range turnResponse.Actions {
		if action.Type == ActionDeliverWork {
			hadDeliverWork = true
		}
		if action.Type == ActionMarkDone {
			hadMarkDone = true
		}
		if execErr := l.executeAction(ctx, action, children); execErr != nil {
			loopLogger.Error("action execution failed",
				"agentID", l.config.AgentID,
				"actionType", action.Type,
				"error", execErr,
			)
		}
	}

	// 7a. Auto-deliver: if the worker has an active todo and wrote files
	// across turns but never called deliver_work, auto-deliver once the
	// LLM stops producing write_file actions (indicating it has nothing
	// more to write) or after accumulating enough turns with pending files.
	if l.config.AgentRole == NodeRoleWorker && l.activeTodoID != "" && !hadDeliverWork && !hadMarkDone {
		if l.filesChangedThisTurn {
			// Worker wrote files this turn — don't deliver yet, let it keep writing.
			l.turnsWithPendingFiles++
			loopLogger.Info("files written, deferring auto-deliver",
				"agentID", l.config.AgentID, "todoID", l.activeTodoID,
				"turnsWithPendingFiles", l.turnsWithPendingFiles)
		} else if l.turnsWithPendingFiles > 0 {
			// Worker had pending files from prior turn(s) but wrote nothing
			// this turn — the model is done. Auto-deliver now.
			loopLogger.Info("auto-delivering work: no new files this turn after prior writes",
				"agentID", l.config.AgentID, "todoID", l.activeTodoID,
				"turnsWithPendingFiles", l.turnsWithPendingFiles)
			autoPayload := DeliverWorkPayload{
				TodoID:      l.activeTodoID,
				Deliverable: "Auto-delivered: files written across prior turns",
				Confidence:  0.8,
			}
			if deliverErr := l.handleDeliverWork(ctx, autoPayload); deliverErr != nil {
				loopLogger.Error("auto-deliver failed", "agentID", l.config.AgentID, "error", deliverErr)
			}
			l.turnsWithPendingFiles = 0
		}
	} else if hadDeliverWork || hadMarkDone {
		l.turnsWithPendingFiles = 0
	}

	// 7b. Trigger knowledge re-index once per turn when files were written.
	if l.filesChangedThisTurn && l.deps.OnFilesChanged != nil && l.config.ProjectLocation != "" {
		l.deps.OnFilesChanged(ctx, l.config.ProjectLocation)
	}

	// 8. Post agent summary/thinking to thread.
	turnSummary := turnResponse.GetSummary()
	if turnSummary != "" {
		if err := l.postAgentMessage(ctx, turnSummary, "agent_turn_summary"); err != nil {
			loopLogger.Error("failed to post turn summary", "agentID", l.config.AgentID, "error", err)
		}
	}

	// 8a. Write turn to chat history file so any agent can read it.
	l.writeTurnToChatHistory(turnResponse, turnSummary)

	// 8b. Periodic conversation summarization — every N turns, summarize
	// recent messages and insert a compact chat_summary into the thread.
	// Future context building will start from the last summary instead of
	// replaying all raw messages, saving significant tokens.
	if l.turnCount%DefaultChatSummaryEveryNTurns == 0 {
		l.runChatSummarization(ctx)
	}

	// 9. Periodic mission-state summary refresh.
	if l.turnCount%DefaultSummaryEveryNTurns == 0 {
		if _, _, refreshErr := l.deps.MissionStateRuntime.RefreshMissionState(l.config.MissionID, l.config.ThreadID); refreshErr != nil {
			loopLogger.Error("periodic summary refresh failed", "agentID", l.config.AgentID, "error", refreshErr)
		}
	}

	return nil
}

// LoopToolCallingClient extends the base LLM interface with GenerateWithTools support.
// The agent loop checks for this via type assertion at runtime.
type LoopToolCallingClient interface {
	LoopCompletionClient
	GenerateWithTools(
		ctx context.Context,
		model string,
		systemPrompt string,
		userPrompt string,
		tools []responses.ToolUnionParam,
		executor aiclients.ToolExecutor,
		maxRounds int,
	) (aiclients.ToolCallResult, error)
}

// executeTurnWithTools runs a single turn using the OpenAI function-calling loop.
// The LLM sees registered skills as tools and calls them directly; results are
// fed back into the model within the same API round-trip.
func (l *AgentLoop) executeTurnWithTools(ctx context.Context, userPrompt string) error {
	toolClient, ok := l.deps.LLM.(LoopToolCallingClient)
	if !ok {
		return fmt.Errorf("LLM client does not support GenerateWithTools")
	}

	env := l.buildSkillEnv(ctx)
	toolParams := l.deps.SkillRegistry.ToolParamsForRole(string(l.config.AgentRole))

	loopLogger.Info("executing turn with tools",
		"agentID", l.config.AgentID,
		"toolCount", len(toolParams),
	)

	result, err := toolClient.GenerateWithTools(
		ctx,
		l.config.Model,
		l.config.SystemPrompt,
		userPrompt,
		toolParams,
		func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			loopLogger.Info("tool call",
				"agentID", l.config.AgentID,
				"tool", name,
				"argsLen", len(args),
			)
			out, execErr := l.deps.SkillRegistry.Execute(ctx, env, name, args)
			if execErr != nil {
				loopLogger.Error("tool execution failed",
					"agentID", l.config.AgentID,
					"tool", name,
					"error", execErr,
				)
				return fmt.Sprintf("ERROR: %s", execErr.Error()), nil
			}
			return out, nil
		},
		10,
	)
	if err != nil {
		return fmt.Errorf("GenerateWithTools: %w", err)
	}

	// Log tool call summary.
	if len(result.ToolCalls) > 0 {
		names := make([]string, len(result.ToolCalls))
		for i, tc := range result.ToolCalls {
			names[i] = tc.Name
		}
		loopLogger.Info("tool calls completed",
			"agentID", l.config.AgentID,
			"count", len(result.ToolCalls),
			"tools", strings.Join(names, ", "),
		)
	}

	// Post the model's text response as a turn summary.
	if result.Text != "" {
		if err := l.postAgentMessage(ctx, result.Text, "agent_turn_summary"); err != nil {
			loopLogger.Error("failed to post tool turn summary", "agentID", l.config.AgentID, "error", err)
		}
	}

	// Trigger knowledge re-index if files changed during tool calls.
	if l.filesChangedThisTurn && l.deps.OnFilesChanged != nil && l.config.ProjectLocation != "" {
		l.deps.OnFilesChanged(ctx, l.config.ProjectLocation)
	}

	// Periodic summaries.
	if l.turnCount%DefaultChatSummaryEveryNTurns == 0 {
		l.runChatSummarization(ctx)
	}
	if l.turnCount%DefaultSummaryEveryNTurns == 0 {
		if _, _, refreshErr := l.deps.MissionStateRuntime.RefreshMissionState(l.config.MissionID, l.config.ThreadID); refreshErr != nil {
			loopLogger.Error("periodic summary refresh failed", "agentID", l.config.AgentID, "error", refreshErr)
		}
	}

	return nil
}

// buildSkillEnv constructs a skills.Env from the current loop state.
func (l *AgentLoop) buildSkillEnv(ctx context.Context) *skills.Env {
	return &skills.Env{
		ProjectDir: l.config.ProjectLocation,
		ProjectID:  l.config.ProjectID,
		MissionID:  l.config.MissionID,
		ThreadID:   l.currentThreadID(),
		AgentID:    l.config.AgentID,
		AgentRole:  string(l.config.AgentRole),
		Depth:      l.config.Depth,
		MaxDepth:   l.config.MaxDepth,
		Model:      l.config.Model,

		NodeStore:       l.wrapNodeStore(),
		ThreadStore:     l.deps.ThreadStore,
		MissionStore:    l.deps.MissionStore,
		AttachmentStore: l.deps.AttachmentStore,

		ExecutionRuntime:    l.deps.ExecutionRuntime,
		MissionRuntime:      l.deps.MissionRuntime,
		MissionStateRuntime: l.deps.MissionStateRuntime,
		ContextBuilder:      l.deps.ContextBuilder,

		LoopManager: l.wrapLoopManager(),
		Testing:     l.wrapTestingAgent(),
		QA:          l.wrapQAAgent(),
		Reasoner:    l.deps.Reasoner,

		OnFilesChanged: func(ctx context.Context, projectPath string) {
			l.filesChangedThisTurn = true
		},
		OnSelfTerminate: func() {
			l.selfTerminate()
		},
		OnThreadSwitch: func(newThreadID string, todoID string) {
			l.activeThreadID = newThreadID
			l.activeTodoID = todoID
		},
	}
}

func (l *AgentLoop) executeAction(ctx context.Context, action LoopAction, children []AgentNode) error {
	// Role-based action filtering: prevent CEO/Manager from running worker-only actions
	// (CEO can use write_file but not start_todo or deliver_work)
	if l.config.AgentRole != NodeRoleWorker && l.config.AgentRole != NodeRoleTester {
		switch action.Type {
		case ActionStartTodo, ActionDeliverWork:
			loopLogger.Warn("blocked non-worker from executing worker-only action",
				"action", action.Type, "role", l.config.AgentRole, "agentID", l.config.AgentID)
			return nil // silently skip
		}
	}
	// Block run_qa for workers/testers — only CEO/Manager can run QA.
	if (l.config.AgentRole == NodeRoleWorker || l.config.AgentRole == NodeRoleTester) && action.Type == ActionRunQA {
		loopLogger.Warn("blocked worker/tester from running QA", "agentID", l.config.AgentID)
		return nil
	}
	switch action.Type {
	case ActionPostMessage:
		var p PostMessagePayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handlePostMessage(ctx, p)

	case ActionCheckChild:
		var p CheckChildPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleCheckChild(ctx, p, children)

	case ActionCreateWorker:
		var p CreateWorkerPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleCreateWorker(ctx, p)

	case ActionCompleteTodo:
		var p TodoActionPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		todoID := l.resolveTodoID(p.GetTodoID(), "in_progress")
		_, err := l.deps.ExecutionRuntime.CompleteTodo(todoID)
		return err

	case ActionBlockTodo:
		var p TodoActionPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		todoID := l.resolveTodoID(p.GetTodoID(), "in_progress")
		_, err := l.deps.ExecutionRuntime.BlockTodo(todoID)
		return err

	case ActionStartTodo:
		var p TodoActionPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		// Auto-resolve: if the LLM provides a wrong or empty todo ID,
		// find the first open (status=todo) todo for this mission.
		resolved := l.resolveTodoID(p.GetTodoID(), "todo")
		p.TodoID = resolved
		return l.handleStartTodo(ctx, p)

	case ActionUpdateSummary:
		_, _, err := l.deps.MissionStateRuntime.RefreshMissionState(l.config.MissionID, l.config.ThreadID)
		return err

	case ActionEscalate:
		var p EscalatePayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleEscalate(ctx, p)

	case ActionResolveConflict:
		var p ResolveConflictPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleResolveConflict(ctx, p, children)

	case ActionDeliverWork:
		var p DeliverWorkPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleDeliverWork(ctx, p)

	case ActionWriteFile:
		var p WriteFilePayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleWriteFile(ctx, p)

	case ActionReadFile:
		var p ReadFilePayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleReadFile(ctx, p)

	case ActionMergeBranch:
		var p MergeBranchPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleMergeBranch(ctx, p)

	case ActionRunQA:
		var p RunQAPayload
		if err := DecodePayload(action, &p); err != nil {
			return err
		}
		return l.handleRunQA(ctx, p)

	case ActionMarkDone:
		return l.handleMarkDone(ctx)

	case ActionNoOp:
		return nil

	default:
		loopLogger.Warn("unknown loop action type", "type", action.Type, "agentID", l.config.AgentID)
		return nil
	}
}

// --- Action handlers ---

func (l *AgentLoop) handlePostMessage(ctx context.Context, p PostMessagePayload) error {
	content := p.GetContent()
	if content == "" {
		return nil // nothing to post
	}
	targetThread := p.TargetThreadID
	// If no thread but a child agent ID was given, resolve the child's thread.
	if targetThread == "" && p.GetTargetAgentID() != "" {
		children, _ := l.deps.NodeStore.ListChildren(l.config.AgentID)
		for _, ch := range children {
			if ch.ID == p.GetTargetAgentID() {
				targetThread = ch.ThreadID
				break
			}
		}
	}
	if targetThread == "" {
		targetThread = l.currentThreadID()
	}
	return l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("agent-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      targetThread,
		Role:          threads.RoleAssistant,
		AuthorAgentID: l.config.AgentID,
		AuthorRole:    string(l.config.AgentRole),
		MessageType:   "agent_message",
		Content:       content,
		CreatedAt:     time.Now().UTC(),
	})
}

func (l *AgentLoop) handleCheckChild(ctx context.Context, p CheckChildPayload, children []AgentNode) error {
	// If no children exist yet, this is a no-op — the worker hasn't created any sub-workers.
	if len(children) == 0 {
		loopLogger.Info("check_child called but no children exist yet, skipping",
			"agentID", l.config.AgentID)
		return l.postAgentMessage(ctx,
			"No child agents exist yet. You should either create sub-workers with create_worker, or do the work directly with write_file.",
			"agent_guidance")
	}

	childID := p.GetChildAgentID()
	var targetChild *AgentNode
	for i := range children {
		if children[i].ID == childID {
			targetChild = &children[i]
			break
		}
	}
	// If the LLM provided an empty or wrong child ID, try to find the first
	// active child that hasn't been checked recently.
	if targetChild == nil {
		for i := range children {
			if children[i].Status == "active" || children[i].Status == "busy" {
				if _, checked := l.lastChildCheck[children[i].ID]; !checked {
					targetChild = &children[i]
					break
				}
			}
		}
		// If all have been checked, pick the first active one.
		if targetChild == nil {
			for i := range children {
				if children[i].Status == "active" || children[i].Status == "busy" {
					targetChild = &children[i]
					break
				}
			}
		}
	}
	if targetChild == nil {
		// All children are completed/terminated — nothing to check.
		loopLogger.Info("check_child: no active children to check",
			"agentID", l.config.AgentID, "totalChildren", len(children))
		return nil
	}

	l.lastChildCheck[targetChild.ID] = time.Now().UTC()

	content := p.GetQuestion()
	if content == "" {
		content = fmt.Sprintf("Status check from %s: What is your current progress?", l.config.AgentID)
	}
	return l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("check-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      targetChild.ThreadID,
		Role:          threads.RoleUser,
		AuthorAgentID: l.config.AgentID,
		AuthorRole:    string(l.config.AgentRole),
		MessageType:   "parent_check",
		Content:       content,
		CreatedAt:     time.Now().UTC(),
	})
}

func (l *AgentLoop) handleCreateWorker(ctx context.Context, p CreateWorkerPayload) error {
	// Enforce max depth — if at max, refuse to create sub-workers.
	if l.config.Depth >= l.config.MaxDepth {
		loopLogger.Warn("cannot create sub-worker at max depth, skipping",
			"agentID", l.config.AgentID, "depth", l.config.Depth, "maxDepth", l.config.MaxDepth)
		return nil
	}

	// Resolve problem statement from all possible LLM field variants.
	problemStatement := p.GetProblemStatement()
	todoTitle := p.GetTodoTitle()
	todoDescription := p.GetTodoDescription()

	// If problemStatement is still empty after all fallbacks, generate one
	// from the parent mission context so the child mission is still created.
	if problemStatement == "" {
		if parentMission, err := l.deps.MissionStore.GetMission(l.config.MissionID); err == nil {
			problemStatement = fmt.Sprintf("Sub-task of: %s", parentMission.Title)
		} else {
			problemStatement = fmt.Sprintf("Sub-task delegated by %s", l.config.AgentID)
		}
		loopLogger.Warn("create_worker payload had empty problem statement, using fallback",
			"agentID", l.config.AgentID, "fallback", problemStatement)
	}

	// If name is empty, derive one from the problem statement.
	workerName := p.Name
	if workerName == "" {
		workerName = sanitizeSlug(problemStatement)
		if workerName == "" {
			workerName = fmt.Sprintf("sub-worker-%d", time.Now().UnixNano())
		}
	}

	childDepth := l.config.Depth + 1
	missionID := fmt.Sprintf("sub-%s-%d", l.config.MissionID, time.Now().UnixNano())
	threadID := fmt.Sprintf("thread-%s", missionID)
	agentID := fmt.Sprintf("agent-%s-%d", sanitizeSlug(workerName), time.Now().UnixNano())

	role := strings.ToLower(strings.TrimSpace(p.Role))
	if role == "" {
		role = "worker"
	}

	nodeRole := NodeRoleWorker
	switch role {
	case "manager", "lead":
		nodeRole = NodeRoleManager
	case "tester", "qa", "quality":
		nodeRole = NodeRoleTester
	}

	parentMission, err := l.deps.MissionStore.GetMission(l.config.MissionID)
	if err != nil {
		return fmt.Errorf("load parent mission for sub-worker: %w", err)
	}

	_, childThread, err := l.deps.MissionRuntime.CreateChildMission(missions.ChildMissionInput{
		MissionID:       missionID,
		ParentMissionID: l.config.MissionID,
		ThreadID:        threadID,
		OwnerAgentID:    agentID,
		OwnerRole:       role,
		MissionType:     "execution",
		ThreadKind:      "execution",
		MissionTitle:    problemStatement,
		Charter:         problemStatement,
		Goal:            problemStatement,
		Scope:           parentMission.Scope,
		AuthorityLevel:  "execution",
		ThreadTitle:     fmt.Sprintf("%s execution thread", workerName),
		ThreadSummary:   problemStatement,
		ThreadContext:   fmt.Sprintf("Execution thread for sub-worker %s under parent agent %s.", workerName, l.config.AgentID),
		ParentThreadID:  l.config.ThreadID,
	})
	if err != nil {
		return fmt.Errorf("create child mission for sub-worker: %w", err)
	}

	// Create todo for the sub-worker.
	if todoTitle == "" {
		todoTitle = fmt.Sprintf("Execute: %s", problemStatement)
	}
	if todoDescription == "" {
		todoDescription = problemStatement
	}
	_, err = l.deps.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    missionID,
		ThreadID:     childThread.ID,
		Title:        todoTitle,
		Description:  todoDescription,
		OwnerAgentID: agentID,
		Priority:     missions.PriorityHigh,
	})
	if err != nil {
		return fmt.Errorf("create todo for sub-worker: %w", err)
	}

	// Create agent node.
	if err := l.deps.NodeStore.CreateNode(AgentNode{
		ID:               agentID,
		ParentAgentID:    l.config.AgentID,
		RootAgentID:      l.config.AgentID, // will be corrected by the node if needed
		ProjectID:        l.config.ProjectID,
		ThreadID:         childThread.ID,
		MissionID:        missionID,
		Name:             workerName,
		Role:             nodeRole,
		Depth:            childDepth,
		ProblemStatement: problemStatement,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("create agent node for sub-worker: %w", err)
	}

	// Generate AGENT_BRIEF for the new worker so it has durable role context.
	if l.config.ProjectLocation != "" {
		toolsList := "write_file, deliver_work, start_todo, complete_todo, block_todo, post_message, escalate, mark_done"
		if nodeRole == NodeRoleManager {
			toolsList = "check_child, create_worker, post_message, resolve_conflict, complete_todo, block_todo, escalate, mark_done"
		}
		briefContent := contextdocs.GenerateAgentBrief(contextdocs.AgentBriefInput{
			AgentName:        workerName,
			Role:             role,
			ProblemStatement: problemStatement,
			ToolsAvailable:   toolsList,
			Workflow: func() string {
				if nodeRole == NodeRoleManager {
					return "1. Review child agents' status\n2. Send guidance to struggling children\n3. Create sub-workers for uncovered work\n4. Resolve conflicts\n5. Mark done when all children complete"
				}
				return "1. start_todo on highest priority todo\n2. Write source code files with write_file\n3. Self-critique 2-3 times\n4. deliver_work with summary\n5. Repeat for next todo\n6. mark_done when all todos finished"
			}(),
			SelfCritiqueRules:  "Review each file for: missing imports, edge cases, error handling, naming conventions. Minimum 2 passes before delivery.",
			EscalationPath:     fmt.Sprintf("Escalate to parent agent %s if blocked or unsure about approach.", l.config.AgentID),
			AcceptanceCriteria: problemStatement,
		})
		if err := contextdocs.WriteDocument(l.config.ProjectLocation, contextdocs.DocAgentBrief, agentID, briefContent); err != nil {
			loopLogger.Warn("failed to write agent brief", "agentID", agentID, "error", err)
		}
	}

	// Start the sub-worker's loop.
	// Workers get their own git worktree for isolated file writes.
	childProjectLocation := l.config.ProjectLocation
	if l.config.ProjectLocation != "" && nodeRole == NodeRoleWorker {
		projectRoot := resolveProjectRoot(l.config.ProjectLocation)
		_ = gitops.InitRepo(projectRoot) // idempotent
		if wtDir, wtErr := gitops.CreateWorktree(projectRoot, agentID); wtErr == nil {
			childProjectLocation = wtDir
		} else {
			loopLogger.Error("gitops: failed to create worktree for sub-worker", "agentID", agentID, "error", wtErr)
		}
	}

	if l.deps.LoopManager != nil {
		childConfig := AgentLoopConfig{
			AgentID:         agentID,
			AgentRole:       nodeRole,
			MissionID:       missionID,
			ThreadID:        childThread.ID,
			ProjectID:       l.config.ProjectID,
			ProjectLocation: childProjectLocation,
			Depth:           childDepth,
			MaxDepth:        l.config.MaxDepth,
			WakeInterval:    WakeIntervalForRole(nodeRole),
			Model:           l.config.Model,
			SystemPrompt:    systemPromptForRole(nodeRole, childDepth, l.config.MaxDepth),
		}
		if err := l.deps.LoopManager.StartLoop(childConfig, l.deps); err != nil {
			loopLogger.Error("failed to start sub-worker loop", "agentID", agentID, "error", err)
		}
	}

	// Refresh mission state.
	_, _, _ = l.deps.MissionStateRuntime.RefreshMissionState(missionID, childThread.ID)

	// Notify own thread.
	return l.postAgentMessage(ctx,
		fmt.Sprintf("Created sub-worker %q (depth %d) for: %s", p.Name, childDepth, problemStatement),
		"agent_delegation",
	)
}

func (l *AgentLoop) handleEscalate(ctx context.Context, p EscalatePayload) error {
	// Post escalation to parent's thread (our own thread, which the parent monitors).
	content := fmt.Sprintf("[ESCALATION from %s] %s", l.config.AgentID, p.Reason)
	if p.SiblingID != "" {
		content += fmt.Sprintf(" (needs help from sibling: %s)", p.SiblingID)
	}
	return l.postAgentMessage(ctx, content, "agent_escalation")
}

func (l *AgentLoop) handleResolveConflict(ctx context.Context, p ResolveConflictPayload, children []AgentNode) error {
	var targetChild *AgentNode
	for i := range children {
		if children[i].ID == p.TargetChildID {
			targetChild = &children[i]
			break
		}
	}
	if targetChild == nil {
		return fmt.Errorf("target child %q not found for conflict resolution", p.TargetChildID)
	}
	return l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("resolve-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      targetChild.ThreadID,
		Role:          threads.RoleUser,
		AuthorAgentID: l.config.AgentID,
		AuthorRole:    string(l.config.AgentRole),
		MessageType:   "parent_resolution",
		Content:       p.Resolution,
		CreatedAt:     time.Now().UTC(),
	})
}

// handleStartTodo marks the todo as in-progress, creates a child thread for
// isolated context, generates a TASK_CONTEXT document, and switches the agent
// to work within the sub-thread until the work is delivered.
func (l *AgentLoop) handleStartTodo(ctx context.Context, p TodoActionPayload) error {
	todoID := p.GetTodoID()
	if todoID == "" {
		return fmt.Errorf("start_todo action has empty todoId")
	}

	// If we already have an active todo and sub-thread, skip redundant start.
	if l.activeTodoID == todoID && l.activeThreadID != "" {
		loopLogger.Info("todo already active, skipping start_todo",
			"agentID", l.config.AgentID, "todoID", todoID)
		return nil
	}

	_, err := l.deps.ExecutionRuntime.StartTodo(todoID)
	if err != nil {
		return err
	}

	todo, err := l.deps.ExecutionRuntime.Store().GetTodo(todoID)
	if err != nil {
		return fmt.Errorf("load todo for sub-thread: %w", err)
	}

	// Create a child thread scoped to this todo.
	subThreadID := fmt.Sprintf("thread-todo-%s-%d", sanitizeSlug(todoID), time.Now().UnixNano())
	if err := l.deps.ThreadStore.CreateThread(threads.Thread{
		ID:             subThreadID,
		MissionID:      l.config.MissionID,
		RootMissionID:  l.config.MissionID,
		ParentThreadID: l.config.ThreadID,
		Kind:           "task",
		Title:          fmt.Sprintf("Todo: %s", todo.Title),
		Summary:        todo.Description,
		Context:        fmt.Sprintf("Sub-thread for todo %q. Parent thread: %s", todo.Title, l.config.ThreadID),
		OwnerAgentID:   l.config.AgentID,
		Status:         threads.ThreadStatusActive,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		loopLogger.Error("failed to create todo sub-thread", "agentID", l.config.AgentID, "todoID", todoID, "error", err)
		// Non-fatal: agent continues on main thread.
		return nil
	}

	l.activeThreadID = subThreadID
	l.activeTodoID = todoID

	// Generate TASK_CONTEXT document on disk.
	if l.config.ProjectLocation != "" {
		deps := ""
		if len(todo.DependsOn) > 0 {
			depsJSON, _ := json.Marshal(todo.DependsOn)
			deps = string(depsJSON)
		}
		content := contextdocs.GenerateTaskContext(contextdocs.TaskContextInput{
			TodoTitle:          todo.Title,
			Description:        todo.Description,
			Dependencies:       deps,
			AcceptanceCriteria: todo.Description, // best available
		})
		if writeErr := contextdocs.WriteDocument(l.config.ProjectLocation, contextdocs.DocTaskContext, todoID, content); writeErr != nil {
			loopLogger.Error("failed to write TASK_CONTEXT", "agentID", l.config.AgentID, "todoID", todoID, "error", writeErr)
		}
	}

	loopLogger.Info("started todo with sub-thread",
		"agentID", l.config.AgentID,
		"todoID", todoID,
		"subThreadID", subThreadID,
	)

	return nil
}

func (l *AgentLoop) handleDeliverWork(ctx context.Context, p DeliverWorkPayload) error {
	deliverable := p.GetDeliverable()
	if deliverable == "" {
		return fmt.Errorf("deliver_work action has empty deliverable")
	}
	// Resolve the todo ID — the LLM often can't reproduce long IDs.
	todoID := l.resolveTodoID(p.GetTodoID(), "in_progress")

	// Leaf worker delivering a completed piece of work.
	// Run testing validation if a testing agent is available.
	if l.deps.TestingAgent != nil && todoID != "" {
		todo, err := l.deps.ExecutionRuntime.Store().GetTodo(todoID)
		if err != nil {
			return fmt.Errorf("load todo for testing: %w", err)
		}

		mission, err := l.deps.MissionStore.GetMission(l.config.MissionID)
		if err != nil {
			return fmt.Errorf("load mission for testing: %w", err)
		}

		result, testErr := l.deps.TestingAgent.Validate(ctx, TestInput{
			Deliverable:        deliverable,
			TodoTitle:          todo.Title,
			TodoDescription:    todo.Description,
			AcceptanceCriteria: string(mission.AcceptanceCriteria),
			MissionGoal:        mission.Goal,
		})
		if testErr != nil {
			loopLogger.Error("testing validation failed", "agentID", l.config.AgentID, "error", testErr)
		} else {
			// Post test result to thread.
			testResultJSON, _ := json.Marshal(result)
			_ = l.deps.ThreadStore.AppendMessage(threads.Message{
				ID:            fmt.Sprintf("test-%s-%d", l.config.AgentID, time.Now().UnixNano()),
				ThreadID:      l.currentThreadID(),
				Role:          threads.RoleAssistant,
				AuthorAgentID: "testing-agent",
				AuthorRole:    "tester",
				MessageType:   "test_result",
				Content:       fmt.Sprintf("Test %s: %s", result.Status, result.Summary),
				ContentJSON:   testResultJSON,
				CreatedAt:     time.Now().UTC(),
			})

			if result.Status == "FAIL" {
				// Post failure to thread, let the agent retry on next turn.
				return l.postAgentMessage(ctx,
					fmt.Sprintf("Test FAILED for todo %q: %s. Will retry.", todo.Title, result.Summary),
					"test_failure",
				)
			}
		}
	}

	// Auto-enrich deliverable with file list from this sub-thread.
	enrichedDeliverable := deliverable
	if l.activeThreadID != "" {
		if msgs, mErr := l.deps.ThreadStore.ListMessages(l.activeThreadID); mErr == nil {
			var written []string
			for _, m := range msgs {
				if m.MessageType == "file_written" {
					written = append(written, m.Content)
				}
			}
			if len(written) > 0 {
				enrichedDeliverable += "\n\nFiles written:\n"
				for _, f := range written {
					enrichedDeliverable += fmt.Sprintf("- %s\n", f)
				}
			}
		}
	}

	// Post deliverable to thread.
	if err := l.postAgentMessage(ctx, enrichedDeliverable, "worker_deliverable"); err != nil {
		return err
	}

	// Git commit: if the worker wrote files, commit them on the worker's branch.
	if l.config.ProjectLocation != "" {
		commitMsg := fmt.Sprintf("[%s] %s", l.config.AgentID, truncateString(deliverable, 200))
		if commitErr := gitops.CommitChanges(l.config.ProjectLocation, commitMsg); commitErr != nil {
			loopLogger.Error("git commit after deliver_work failed",
				"agentID", l.config.AgentID, "error", commitErr)
		}
	}

	// Complete the todo.
	if todoID != "" {
		if _, err := l.deps.ExecutionRuntime.CompleteTodo(todoID); err != nil {
			return fmt.Errorf("complete todo after delivery: %w", err)
		}
	}

	// Close the sub-thread if we were working in one.
	l.closeActiveSubThread(ctx, deliverable)

	return nil
}

// closeActiveSubThread finalizes the current todo sub-thread: posts a
// completion summary to the parent thread, closes the sub-thread, updates
// PROJECT_STATE.md, and switches the agent back to the main thread.
func (l *AgentLoop) closeActiveSubThread(_ context.Context, deliverableSummary string) {
	if l.activeThreadID == "" {
		return
	}

	subThreadID := l.activeThreadID
	todoID := l.activeTodoID

	// Build a compact summary from the sub-thread messages.
	summary := l.buildSubThreadSummary(subThreadID, deliverableSummary)

	// Post summary to parent (main) thread.
	_ = l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("todo-summary-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      l.config.ThreadID,
		Role:          threads.RoleAssistant,
		AuthorAgentID: l.config.AgentID,
		AuthorRole:    string(l.config.AgentRole),
		MessageType:   "todo_completion_summary",
		Content:       summary,
		CreatedAt:     time.Now().UTC(),
	})

	// Close the sub-thread.
	if err := l.deps.ThreadStore.UpdateThreadStatus(subThreadID, threads.ThreadStatusCompleted); err != nil {
		loopLogger.Error("failed to close todo sub-thread", "subThreadID", subThreadID, "error", err)
	}

	// Update PROJECT_STATE.md with the completed work.
	if l.config.ProjectLocation != "" && todoID != "" {
		l.updateProjectStateAfterTodo(todoID, deliverableSummary)
	}

	// Switch back to main thread.
	l.activeThreadID = ""
	l.activeTodoID = ""

	loopLogger.Info("closed todo sub-thread",
		"agentID", l.config.AgentID,
		"todoID", todoID,
		"subThreadID", subThreadID,
	)
}

// buildSubThreadSummary creates a compact completion summary from the sub-thread.
func (l *AgentLoop) buildSubThreadSummary(subThreadID string, deliverableSummary string) string {
	messages, err := l.deps.ThreadStore.ListMessages(subThreadID)
	if err != nil || len(messages) == 0 {
		return fmt.Sprintf("Completed todo. Deliverable: %s", truncateStr(deliverableSummary, 500))
	}

	var b strings.Builder
	b.WriteString("Todo completed.\n")

	// Extract file write events for a quick list of what changed.
	var filesWritten []string
	for _, msg := range messages {
		if msg.MessageType == "file_written" {
			filesWritten = append(filesWritten, msg.Content)
		}
	}
	if len(filesWritten) > 0 {
		b.WriteString("Files changed:\n")
		for _, f := range filesWritten {
			b.WriteString(fmt.Sprintf("- %s\n", truncateStr(f, 100)))
		}
	}

	b.WriteString(fmt.Sprintf("Deliverable: %s", truncateStr(deliverableSummary, 500)))
	return b.String()
}

// updateProjectStateAfterTodo appends the completed todo to PROJECT_STATE.md.
func (l *AgentLoop) updateProjectStateAfterTodo(todoID string, deliverableSummary string) {
	existing, _ := contextdocs.ReadDocument(l.config.ProjectLocation, contextdocs.DocProjectState, "")
	if existing == "" {
		// No state doc yet — create initial.
		existing = "# Project State\n\n## Completed Features\n\nNone yet\n"
	}

	// Append the completed todo to the state doc.
	addition := fmt.Sprintf("\n- [%s] %s\n", todoID, truncateStr(deliverableSummary, 200))
	updated := strings.Replace(existing, "None yet", addition, 1)
	if updated == existing {
		// "None yet" was already replaced — append to completed section.
		updated = existing + addition
	}

	if err := contextdocs.WriteDocument(l.config.ProjectLocation, contextdocs.DocProjectState, "", updated); err != nil {
		loopLogger.Error("failed to update PROJECT_STATE", "todoID", todoID, "error", err)
	}
}

// truncateStr truncates a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (l *AgentLoop) handleWriteFile(ctx context.Context, p WriteFilePayload) error {
	filePath := p.GetFilePath()
	content := p.GetContent()
	if filePath == "" {
		return fmt.Errorf("write_file action has empty filePath")
	}
	if content == "" {
		return fmt.Errorf("write_file action has empty content for %q", filePath)
	}

	// Auto-start first open todo if the worker writes files without calling start_todo.
	if l.activeTodoID == "" && l.config.AgentRole == NodeRoleWorker {
		todos, todoErr := l.deps.ExecutionRuntime.Store().ListTodos(l.config.MissionID)
		if todoErr == nil {
			for _, t := range todos {
				if t.Status == "todo" {
					loopLogger.Info("auto-starting first open todo before write_file",
						"agentID", l.config.AgentID, "todoID", t.ID)
					_ = l.handleStartTodo(ctx, TodoActionPayload{TodoID: t.ID})
					break
				}
			}
		}
	}

	projectLocation := l.config.ProjectLocation
	if projectLocation == "" {
		return fmt.Errorf("write_file action cannot proceed: project location is not configured")
	}

	// Sanitize the path: prevent path traversal out of the project directory.
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		// Strip any leading path components to make it relative.
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}
	// Strip project path prefix if the model includes it (e.g. "tmp/todo-app/src/App.tsx"
	// when the project is at "/tmp/todo-app").
	if projectLocation != "" {
		relProject := strings.TrimPrefix(filepath.Clean(projectLocation), "/")
		if relProject != "" && strings.HasPrefix(cleanPath, relProject+"/") {
			cleanPath = strings.TrimPrefix(cleanPath, relProject+"/")
		}
	}
	// Reject any path that would escape the project directory.
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("write_file action rejected: path %q contains path traversal", filePath)
	}

	absPath := filepath.Join(projectLocation, cleanPath)
	// Double-check the resolved path is still within the project directory.
	absProjectLocation, _ := filepath.Abs(projectLocation)
	absResolved, _ := filepath.Abs(absPath)
	if !strings.HasPrefix(absResolved, absProjectLocation+string(filepath.Separator)) && absResolved != absProjectLocation {
		return fmt.Errorf("write_file action rejected: resolved path %q escapes project directory", absResolved)
	}

	// Create parent directories if needed.
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("write_file create directory %q: %w", dir, err)
	}

	// Write the file.
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write_file write %q: %w", absPath, err)
	}

	fileSize := int64(len(content))
	relPath := filepath.ToSlash(cleanPath)
	category := attachments.ClassifyFile(filepath.Base(filePath))

	loopLogger.Info("file written by agent",
		"agentID", l.config.AgentID,
		"filePath", relPath,
		"absPath", absPath,
		"size", fileSize,
	)

	// Record in attachments store if available.
	if l.deps.AttachmentStore != nil {
		_ = l.deps.AttachmentStore.Create(attachments.Attachment{
			ID:           fmt.Sprintf("file-%s-%d", l.config.AgentID, time.Now().UnixNano()),
			MissionID:    l.config.MissionID,
			ThreadID:     l.currentThreadID(),
			Filename:     filepath.Base(filePath),
			SizeBytes:    fileSize,
			RelativePath: relPath,
			AbsolutePath: absPath,
			FileCategory: category,
			Status:       attachments.StatusActive,
			CreatedAt:    time.Now().UTC(),
		})
	}

	// Flag for end-of-turn re-index.
	l.filesChangedThisTurn = true

	// Post confirmation to thread.
	return l.postAgentMessage(ctx,
		fmt.Sprintf("Wrote file: %s (%d bytes)", relPath, fileSize),
		"file_written",
	)
}

func (l *AgentLoop) handleReadFile(ctx context.Context, p ReadFilePayload) error {
	filePath := p.GetFilePath()
	if filePath == "" {
		return fmt.Errorf("read_file action has empty filePath")
	}

	projectLocation := l.config.ProjectLocation
	if projectLocation == "" {
		return fmt.Errorf("read_file action cannot proceed: project location is not configured")
	}

	// Sanitize path same as write_file.
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}
	if projectLocation != "" {
		relProject := strings.TrimPrefix(filepath.Clean(projectLocation), "/")
		if relProject != "" && strings.HasPrefix(cleanPath, relProject+"/") {
			cleanPath = strings.TrimPrefix(cleanPath, relProject+"/")
		}
	}
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("read_file action rejected: path %q contains path traversal", filePath)
	}

	absPath := filepath.Join(projectLocation, cleanPath)
	absProjectLocation, _ := filepath.Abs(projectLocation)
	absResolved, _ := filepath.Abs(absPath)
	if !strings.HasPrefix(absResolved, absProjectLocation+string(filepath.Separator)) && absResolved != absProjectLocation {
		return fmt.Errorf("read_file action rejected: resolved path %q escapes project directory", absResolved)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return l.postAgentMessage(ctx,
			fmt.Sprintf("read_file: file not found or unreadable: %s", filepath.ToSlash(cleanPath)),
			"file_read_error",
		)
	}

	const maxReadSize = 32 * 1024 // 32 KB limit
	content := string(data)
	if len(content) > maxReadSize {
		content = content[:maxReadSize] + "\n... [truncated]"
	}

	loopLogger.Info("file read by agent",
		"agentID", l.config.AgentID,
		"filePath", filepath.ToSlash(cleanPath),
	)

	// Post file content as a thread message so the LLM can see it on the next turn.
	return l.postAgentMessage(ctx,
		fmt.Sprintf("=== File: %s ===\n%s", filepath.ToSlash(cleanPath), content),
		"file_read",
	)
}

func (l *AgentLoop) handleMergeBranch(ctx context.Context, p MergeBranchPayload) error {
	// Only CEO/Manager can merge branches.
	if l.config.AgentRole == NodeRoleWorker {
		loopLogger.Warn("worker cannot merge branches", "agentID", l.config.AgentID)
		return nil
	}

	projectLocation := l.config.ProjectLocation
	if projectLocation == "" {
		return fmt.Errorf("merge_branch action cannot proceed: project location is not configured")
	}

	// Resolve the project root (worktrees merge into the main repo).
	projectRoot := resolveProjectRoot(projectLocation)

	workerName := p.GetWorkerName()
	if workerName == "" {
		// Merge all worker branches.
		failures := gitops.MergeAllWorkerBranches(projectRoot)
		if len(failures) > 0 {
			return l.postAgentMessage(ctx,
				fmt.Sprintf("Merged worker branches into main. Failures: %v", failures),
				"merge_result",
			)
		}
		return l.postAgentMessage(ctx, "All worker branches merged into main.", "merge_result")
	}

	if err := gitops.MergeBranch(projectRoot, workerName); err != nil {
		return l.postAgentMessage(ctx,
			fmt.Sprintf("Failed to merge branch for %s: %v", workerName, err),
			"merge_error",
		)
	}
	return l.postAgentMessage(ctx,
		fmt.Sprintf("Merged branch worker/%s into main.", workerName),
		"merge_result",
	)
}

// resolveProjectRoot returns the main repo directory even if the given path
// is a git worktree subdirectory.
func resolveProjectRoot(projectLocation string) string {
	// Worktrees live under {project}/.worktrees/{name} — detect and resolve.
	if strings.Contains(projectLocation, "/.worktrees/") {
		idx := strings.Index(projectLocation, "/.worktrees/")
		return projectLocation[:idx]
	}
	return projectLocation
}

// allChildrenCompleted returns true when every child agent node has a
// terminal status (completed, failed, terminated).
func (l *AgentLoop) allChildrenCompleted(children []AgentNode) bool {
	for _, c := range children {
		switch c.Status {
		case "completed", "failed", "terminated":
			continue
		default:
			return false
		}
	}
	return true
}

// allTodosDone returns true when every todo for this mission is in a
// terminal state (done).
// resolveTodoID converts an LLM-provided todo ID (which may be wrong, partial,
// or empty) into the actual todo ID from this mission. Prefers an exact match,
// falls back to finding the first todo matching preferredStatus if the ID is
// invalid. This is critical because LLMs cannot reproduce long auto-generated
// todo IDs reliably.
func (l *AgentLoop) resolveTodoID(llmID string, preferredStatus string) string {
	todos, err := l.deps.ExecutionRuntime.Store().ListTodos(l.config.MissionID)
	if err != nil || len(todos) == 0 {
		return llmID // nothing to resolve against
	}

	// 1. Exact match.
	for _, t := range todos {
		if t.ID == llmID {
			return llmID
		}
	}

	// 2. If the worker has an active todo (via activeTodoID), use it for
	// deliver_work / complete_todo scenarios.
	if l.activeTodoID != "" && (preferredStatus == "in_progress") {
		return l.activeTodoID
	}

	// 3. Find the first todo matching the preferred status.
	for _, t := range todos {
		if string(t.Status) == preferredStatus {
			loopLogger.Info("auto-resolved todo ID",
				"agentID", l.config.AgentID,
				"llmProvided", llmID,
				"resolved", t.ID,
				"status", t.Status,
			)
			return t.ID
		}
	}

	// 4. If we only have one todo, use it regardless.
	if len(todos) == 1 {
		return todos[0].ID
	}

	return llmID
}

func (l *AgentLoop) allTodosDone() bool {
	todos, err := l.deps.ExecutionRuntime.Store().ListTodos(l.config.MissionID)
	if err != nil {
		return false
	}
	for _, t := range todos {
		if t.Status != execution.TodoStatusDone {
			return false
		}
	}
	return true
}

// hasNewMessages checks whether any new messages have appeared on the thread
// since the last turn. Returns true (with the latest message ID) when the loop
// should proceed, or false when the thread is idle and the LLM call can be
// skipped.
//
// On the very first turn (lastProcessedMessageID == "") it always returns true
// so the loop can bootstrap.
func (l *AgentLoop) hasNewMessages() (changed bool, latestID string) {
	if l.lastProcessedMessageID == "" {
		// First turn — always proceed.
		msgs, err := l.deps.ThreadStore.ListMessages(l.currentThreadID())
		if err != nil || len(msgs) == 0 {
			return true, ""
		}
		return true, msgs[len(msgs)-1].ID
	}

	msgs, err := l.deps.ThreadStore.ListMessages(l.currentThreadID())
	if err != nil {
		// On error, proceed with the turn to avoid getting stuck.
		return true, l.lastProcessedMessageID
	}
	if len(msgs) == 0 {
		return false, l.lastProcessedMessageID
	}
	latestID = msgs[len(msgs)-1].ID
	if latestID == l.lastProcessedMessageID {
		return false, latestID
	}
	return true, latestID
}

// handleRunQA runs the QA agent against the integrated codebase.
// Only CEO/Manager can trigger this. Posts PASS/FAIL results to the thread.
func (l *AgentLoop) handleRunQA(ctx context.Context, p RunQAPayload) error {
	if l.config.AgentRole == NodeRoleWorker {
		loopLogger.Warn("worker cannot run QA", "agentID", l.config.AgentID)
		return nil
	}
	if l.deps.QAAgent == nil {
		return l.postAgentMessage(ctx, "QA agent not configured, skipping validation.", "qa_skipped")
	}
	projectRoot := resolveProjectRoot(l.config.ProjectLocation)
	if projectRoot == "" {
		return l.postAgentMessage(ctx, "No project location configured, cannot run QA.", "qa_skipped")
	}

	mission, err := l.deps.MissionStore.GetMission(l.config.MissionID)
	if err != nil {
		return fmt.Errorf("load mission for QA: %w", err)
	}

	fileList := listProjectFiles(projectRoot, 50)
	result, qaErr := l.deps.QAAgent.ValidateProject(ctx, QAValidationPayload{
		ProjectDir:  projectRoot,
		MissionGoal: mission.Goal,
		FileList:    fileList,
	})
	if qaErr != nil {
		loopLogger.Error("QA validation error", "agentID", l.config.AgentID, "error", qaErr)
		return l.postAgentMessage(ctx, fmt.Sprintf("QA validation failed to run: %v", qaErr), "qa_error")
	}

	resultJSON, _ := json.Marshal(result)
	_ = l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("qa-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      l.currentThreadID(),
		Role:          threads.RoleAssistant,
		AuthorAgentID: "qa-agent",
		AuthorRole:    "tester",
		MessageType:   "qa_result",
		Content:       fmt.Sprintf("QA %s: %s", result.Status, result.Summary),
		ContentJSON:   resultJSON,
		CreatedAt:     time.Now().UTC(),
	})

	if result.Status == "FAIL" {
		return l.spawnFixWorker(ctx, result)
	}

	return l.postAgentMessage(ctx, fmt.Sprintf("QA PASSED: %s", result.Summary), "qa_passed")
}

// spawnFixWorker creates a fix worker to address QA failures.
// The fix worker gets its own branch (fix/{timestamp}), addresses the issues,
// and delivers the fix. The CEO/Manager then merges the fix branch.
func (l *AgentLoop) spawnFixWorker(ctx context.Context, qaResult QAResult) error {
	if l.config.Depth >= l.config.MaxDepth {
		return l.postAgentMessage(ctx,
			fmt.Sprintf("QA FAILED but cannot spawn fix worker at max depth. Issues: %s", qaResult.Summary),
			"qa_failed")
	}

	// Build a problem statement from the QA issues.
	var issueDesc strings.Builder
	issueDesc.WriteString("Fix the following QA issues found after merging all worker branches:\n\n")
	for i, issue := range qaResult.Issues {
		issueDesc.WriteString(fmt.Sprintf("%d. [%s] %s: %s",
			i+1, issue.Severity, issue.File, issue.Description))
		if issue.Suggestion != "" {
			issueDesc.WriteString(fmt.Sprintf(" — Suggestion: %s", issue.Suggestion))
		}
		issueDesc.WriteString("\n")
	}
	if len(qaResult.Issues) == 0 {
		issueDesc.WriteString(qaResult.Summary)
	}

	problemStatement := issueDesc.String()

	// Create fix worker using the normal create_worker flow.
	fixPayload := CreateWorkerPayload{
		Name:             fmt.Sprintf("fix-worker-%d", time.Now().Unix()),
		Role:             "worker",
		ProblemStatement: problemStatement,
		TodoTitle:        "Fix QA issues from integration review",
		TodoDescription:  problemStatement,
	}

	if err := l.handleCreateWorker(ctx, fixPayload); err != nil {
		return fmt.Errorf("spawn fix worker: %w", err)
	}

	return l.postAgentMessage(ctx,
		fmt.Sprintf("QA FAILED: %s\nSpawned fix worker to address %d issues.",
			qaResult.Summary, len(qaResult.Issues)),
		"qa_failed_fix_spawned")
}

func (l *AgentLoop) handleMarkDone(ctx context.Context) error {
	// Auto-complete any in-progress todos before marking the mission done.
	// This handles the case where the model skips deliver_work.
	todos, todoErr := l.deps.ExecutionRuntime.Store().ListTodos(l.config.MissionID)
	if todoErr == nil {
		for _, t := range todos {
			if t.Status == "in_progress" {
				loopLogger.Info("auto-completing in-progress todo before mark_done",
					"agentID", l.config.AgentID, "todoID", t.ID, "title", t.Title)
				_, _ = l.deps.ExecutionRuntime.CompleteTodo(t.ID)
			}
		}
	}

	// Generate final summary.
	_, _, _ = l.deps.MissionStateRuntime.RefreshMissionState(l.config.MissionID, l.config.ThreadID)

	// Update mission status.
	mission, err := l.deps.MissionStore.GetMission(l.config.MissionID)
	if err != nil {
		return fmt.Errorf("load mission for completion: %w", err)
	}
	if !missions.IsTerminalMissionStatus(mission.Status) {
		mission.Status = missions.MissionStatusCompleted
		now := time.Now().UTC()
		mission.ClosedAt = &now
		mission.UpdatedAt = now
		if err := l.deps.MissionStore.UpdateMission(mission); err != nil {
			return fmt.Errorf("complete mission: %w", err)
		}
	}

	// Update node status.
	_ = l.deps.NodeStore.UpdateStatus(l.config.AgentID, "completed")

	// Git: if this is a worker, commit any remaining changes.
	// If this is a CEO/Manager, merge all worker branches into main, then run QA.
	if l.config.ProjectLocation != "" {
		projectRoot := resolveProjectRoot(l.config.ProjectLocation)
		if l.config.AgentRole == NodeRoleWorker {
			commitMsg := fmt.Sprintf("[%s] Mission completed: %s", l.config.AgentID, mission.Title)
			if commitErr := gitops.CommitChanges(l.config.ProjectLocation, commitMsg); commitErr != nil {
				loopLogger.Error("git commit on mark_done failed", "agentID", l.config.AgentID, "error", commitErr)
			}
		} else {
			// CEO/Manager: merge child worker branches into main.
			var mergeFailures []string
			failures := gitops.MergeAllWorkerBranches(projectRoot)
			if len(failures) > 0 {
				loopLogger.Warn("some worker branches failed to merge", "failures", failures)
				_ = l.postAgentMessage(ctx,
					fmt.Sprintf("Merged worker branches. Failed to merge: %v", failures),
					"merge_result",
				)
				mergeFailures = failures
			} else {
				branches := gitops.ListWorkerBranches(projectRoot)
				if len(branches) > 0 {
					_ = l.postAgentMessage(ctx, "All worker branches merged into main.", "merge_result")
				}
			}

			// Run QA validation on the merged codebase.
			if l.deps.QAAgent != nil {
				fileList := listProjectFiles(projectRoot, 50)
				var priorIssues []string
				for _, f := range mergeFailures {
					priorIssues = append(priorIssues, fmt.Sprintf("Merge conflict in branch: %s", f))
				}
				qaResult, qaErr := l.deps.QAAgent.ValidateProject(ctx, QAValidationPayload{
					ProjectDir:  projectRoot,
					MissionGoal: mission.Goal,
					FileList:    fileList,
					Issues:      priorIssues,
				})
				if qaErr != nil {
					loopLogger.Error("QA validation error on mark_done", "error", qaErr)
				} else {
					// Post QA result to thread.
					resultJSON, _ := json.Marshal(qaResult)
					_ = l.deps.ThreadStore.AppendMessage(threads.Message{
						ID:            fmt.Sprintf("qa-%s-%d", l.config.AgentID, time.Now().UnixNano()),
						ThreadID:      l.currentThreadID(),
						Role:          threads.RoleAssistant,
						AuthorAgentID: "qa-agent",
						AuthorRole:    "tester",
						MessageType:   "qa_result",
						Content:       fmt.Sprintf("QA %s: %s", qaResult.Status, qaResult.Summary),
						ContentJSON:   resultJSON,
						CreatedAt:     time.Now().UTC(),
					})

					if qaResult.Status == "FAIL" {
						// Re-open the mission so the fix worker can run.
						mission.Status = missions.MissionStatusActive
						mission.ClosedAt = nil
						mission.UpdatedAt = time.Now().UTC()
						_ = l.deps.MissionStore.UpdateMission(mission)
						_ = l.deps.NodeStore.UpdateStatus(l.config.AgentID, "active")

						// Spawn a fix worker to address QA issues.
						if spawnErr := l.spawnFixWorker(ctx, qaResult); spawnErr != nil {
							loopLogger.Error("failed to spawn fix worker", "error", spawnErr)
						}
						// Do NOT self-terminate — keep running to supervise the fix worker.
						return nil
					}
					_ = l.postAgentMessage(ctx, fmt.Sprintf("QA PASSED: %s", qaResult.Summary), "qa_passed")
				}
			}
		}
	}

	// Post completion to own thread (parent will see it).
	if err := l.postAgentMessage(ctx,
		fmt.Sprintf("Mission %q completed. All work is done.", mission.Title),
		"mission_completed",
	); err != nil {
		return err
	}

	// Self-terminate.
	l.selfTerminate()
	return nil
}

func (l *AgentLoop) selfTerminate() {
	if l.deps.LoopManager != nil {
		l.deps.LoopManager.StopLoop(l.config.AgentID)
	} else {
		l.Stop()
	}
}

func (l *AgentLoop) postAgentMessage(_ context.Context, content string, messageType string) error {
	return l.deps.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("agent-%s-%d", l.config.AgentID, time.Now().UnixNano()),
		ThreadID:      l.currentThreadID(),
		Role:          threads.RoleAssistant,
		AuthorAgentID: l.config.AgentID,
		AuthorRole:    string(l.config.AgentRole),
		MessageType:   messageType,
		Content:       content,
		CreatedAt:     time.Now().UTC(),
	})
}

// writeTurnToChatHistory appends the turn summary and action list to the
// on-disk chat history file. Any agent can later read_file on this log.
func (l *AgentLoop) writeTurnToChatHistory(resp LoopTurnResponse, summary string) {
	if l.config.ProjectLocation == "" || l.chatHistory == nil {
		return
	}

	// Write the summary or thinking as the main turn entry.
	content := summary
	if content == "" {
		content = rawMessageToString(resp.Thinking)
	}
	if content == "" {
		return
	}

	// Include action types in the entry for quick scanning.
	actionTypes := make([]string, len(resp.Actions))
	for i, a := range resp.Actions {
		actionTypes[i] = string(a.Type)
	}
	if len(actionTypes) > 0 {
		content += " [actions: " + strings.Join(actionTypes, ", ") + "]"
	}

	if err := l.chatHistory.WriteEntry(l.config.ProjectLocation, ChatHistoryEntry{
		Timestamp:   time.Now().UTC(),
		EntryType:   "turn",
		AgentID:     l.config.AgentID,
		AgentRole:   string(l.config.AgentRole),
		MessageType: "agent_turn_summary",
		Content:     content,
	}); err != nil {
		loopLogger.Warn("failed to write chat history entry", "agentID", l.config.AgentID, "error", err)
	}
}

// runChatSummarization loads recent messages since the last chat_summary,
// generates a new summary via the LLM, and inserts it into the thread.
// This enables future context building to start from the summary, saving tokens.
func (l *AgentLoop) runChatSummarization(ctx context.Context) {
	if l.chatSummarizer == nil {
		return
	}

	threadID := l.currentThreadID()
	messages, err := l.deps.ThreadStore.ListMessages(threadID)
	if err != nil {
		loopLogger.Warn("chat summarization: failed to list messages", "agentID", l.config.AgentID, "error", err)
		return
	}

	// Only summarize if there are enough unsummarized messages.
	unsummarized := MessagesSinceLastSummary(messages)
	if len(unsummarized) < DefaultChatSummaryEveryNTurns/2 {
		return // not enough new messages to justify a summary
	}

	// Get mission title for context.
	missionTitle := l.config.MissionID
	mission, mErr := l.deps.MissionStore.GetMission(l.config.MissionID)
	if mErr == nil {
		missionTitle = mission.Title
	}

	summary, err := l.chatSummarizer.SummarizeMessages(ctx, l.config.AgentID, missionTitle, unsummarized)
	if err != nil {
		loopLogger.Warn("chat summarization failed", "agentID", l.config.AgentID, "error", err)
		return
	}

	if summary == "" {
		return
	}

	// Insert the summary into the thread as a special message type.
	summaryMsg := createChatSummaryMessage(threadID, l.config.AgentID, string(l.config.AgentRole), summary)
	if err := l.deps.ThreadStore.AppendMessage(summaryMsg); err != nil {
		loopLogger.Warn("failed to persist chat summary", "agentID", l.config.AgentID, "error", err)
		return
	}

	// Also write the summary to the chat history file.
	if l.config.ProjectLocation != "" && l.chatHistory != nil {
		_ = l.chatHistory.WriteEntry(l.config.ProjectLocation, ChatHistoryEntry{
			Timestamp:   time.Now().UTC(),
			EntryType:   "summary",
			AgentID:     l.config.AgentID,
			AgentRole:   string(l.config.AgentRole),
			MessageType: MessageTypeChatSummary,
			Content:     summary,
		})
	}

	loopLogger.Info("chat summary generated",
		"agentID", l.config.AgentID,
		"threadID", threadID,
		"unsummarizedCount", len(unsummarized),
		"summaryLen", len(summary),
	)
}

// buildUserPrompt constructs the dynamic context for the LLM turn.
// It prefers compact on-disk context documents over raw context-pack data
// to keep token usage minimal.
func (l *AgentLoop) buildUserPrompt(pack contextpacks.ContextPack, children []AgentNode) string {
	var b strings.Builder

	// A worker is a leaf only if at max depth AND has no children.
	// Workers at non-leaf depth with children are "lead workers" who supervise.
	isLeaf := (l.config.Depth >= l.config.MaxDepth || l.config.AgentRole == NodeRoleTester) && len(children) == 0
	isLeadWorker := l.config.AgentRole == NodeRoleWorker && !isLeaf && len(children) > 0
	// A worker at non-leaf depth with NO children yet — should default to direct work.
	isNewWorker := l.config.AgentRole == NodeRoleWorker && !isLeaf && len(children) == 0
	projectDir := l.config.ProjectLocation

	// --- Project overview from context doc (compact) or mission metadata ---
	overview, _ := contextdocs.ReadDocument(projectDir, contextdocs.DocProjectOverview, "")
	if overview != "" {
		b.WriteString("## Project Overview\n")
		b.WriteString(truncateString(overview, 2000))
		b.WriteString("\n\n")
		// Minimal mission reference when overview covers the details.
		b.WriteString("## Current Mission\n")
		b.WriteString(fmt.Sprintf("- **Title**: %s\n", pack.Mission.Title))
		b.WriteString(fmt.Sprintf("- **Goal**: %s\n", pack.Mission.Goal))
		b.WriteString(fmt.Sprintf("- **Status**: %s\n", pack.Mission.Status))
		b.WriteString(fmt.Sprintf("- **My Depth**: %d / Max %d\n\n", l.config.Depth, l.config.MaxDepth))
	} else {
		// Fallback: full mission metadata.
		b.WriteString("## Current Mission\n")
		b.WriteString(fmt.Sprintf("- **Title**: %s\n", pack.Mission.Title))
		b.WriteString(fmt.Sprintf("- **Goal**: %s\n", pack.Mission.Goal))
		b.WriteString(fmt.Sprintf("- **Status**: %s\n", pack.Mission.Status))
		b.WriteString(fmt.Sprintf("- **Charter**: %s\n", pack.Mission.Charter))
		if pack.Mission.Scope != "" {
			b.WriteString(fmt.Sprintf("- **Scope**: %s\n", pack.Mission.Scope))
		}
		b.WriteString(fmt.Sprintf("- **My Depth**: %d / Max %d\n\n", l.config.Depth, l.config.MaxDepth))
	}

	// --- Open todos ---
	if len(pack.DueTodos) > 0 {
		b.WriteString("## Open Todos\n")
		for _, todo := range pack.DueTodos {
			b.WriteString(fmt.Sprintf("- [%s] %s (id: %s, owner: %s): %s\n",
				todo.Status, todo.Title, todo.ID, todo.OwnerAgentID, todo.Description))
		}
		b.WriteString("\n")
	}

	// --- Latest summary (kept compact) ---
	if pack.LatestSummary != nil {
		b.WriteString("## Latest Summary\n")
		b.WriteString(truncateString(pack.LatestSummary.SummaryText, 1000))
		b.WriteString("\n\n")
	}

	// --- Child rollups: truncated to 200 chars each ---
	if !isLeaf && len(pack.ChildRollups) > 0 {
		b.WriteString("## Child Mission Rollups\n")
		for _, rollup := range pack.ChildRollups {
			b.WriteString(fmt.Sprintf("### %s (status: %s, health: %s, progress: %.0f%%)\n",
				rollup.ChildMissionID, rollup.Status, rollup.Health, rollup.ProgressPercent))
			b.WriteString(truncateString(rollup.LatestSummary, 200))
			if rollup.CurrentBlocker != "" {
				b.WriteString(fmt.Sprintf("\n**Blocker**: %s", truncateString(rollup.CurrentBlocker, 100)))
			}
			b.WriteString("\n\n")
		}
	}

	// --- Child agents ---
	if len(children) > 0 {
		b.WriteString("## My Child Agents\n")
		for _, child := range children {
			lastCheck := "never"
			if t, ok := l.lastChildCheck[child.ID]; ok {
				lastCheck = time.Since(t).Round(time.Second).String() + " ago"
			}
			b.WriteString(fmt.Sprintf("- **%s** (id: %s, role: %s, status: %s, thread: %s, last checked: %s)\n  Problem: %s\n",
				child.Name, child.ID, child.Role, child.Status, child.ThreadID, lastCheck,
				truncateString(child.ProblemStatement, 150)))
		}
		b.WriteString("\n")
	}

	// --- Recent messages with summary-aware context ---
	// If a chat_summary exists, include it plus only messages after it.
	// This dramatically reduces token usage for long-running agents.
	msgLimit := 5
	if isLeaf {
		msgLimit = 3
	}
	lastSummaryText, recentMsgs := MessagesAfterLastSummary(pack.RecentMessages)
	if len(recentMsgs) > msgLimit {
		recentMsgs = recentMsgs[len(recentMsgs)-msgLimit:]
	}

	if lastSummaryText != "" {
		b.WriteString("## Conversation Summary (prior context)\n")
		b.WriteString(truncateString(lastSummaryText, 800))
		b.WriteString("\n\n")
	}

	if len(recentMsgs) > 0 {
		b.WriteString("## Recent Thread Messages\n")
		for _, msg := range recentMsgs {
			b.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n",
				msg.CreatedAt.Format("15:04:05"), msg.AuthorAgentID, msg.MessageType,
				truncateString(msg.Content, 300)))
		}
		b.WriteString("\n")
	}

	// --- Instructions: prefer on-disk AGENT_BRIEF, fall back to generic ---
	agentBrief, _ := contextdocs.ReadDocument(projectDir, contextdocs.DocAgentBrief, l.config.AgentID)
	if agentBrief != "" {
		b.WriteString("## My Role Brief\n")
		b.WriteString(truncateString(agentBrief, 1500))
		b.WriteString("\n\n")
	}

	// --- Project config (tech stack, build commands) for leaf workers ---
	if isLeaf && projectDir != "" {
		projConfig, _ := contextdocs.ReadDocument(projectDir, contextdocs.DocProjectConfig, "")
		if projConfig != "" {
			b.WriteString("## Project Config\n")
			b.WriteString(truncateString(projConfig, 1000))
			b.WriteString("\n\n")
		}
	}

	// --- Existing files listing for leaf workers and new workers ---
	if (isLeaf || isNewWorker) && projectDir != "" {
		fileList := listProjectFiles(projectDir, 30)
		if fileList != "" {
			b.WriteString("## Existing Files\n")
			b.WriteString(fileList)
			b.WriteString("\n")
		}
	}

	if isLeaf {
		if agentBrief == "" {
			b.WriteString("## Instructions\n")
			b.WriteString("You are a LEAF WORKER. You must execute work directly — you cannot delegate.\n")
		}
		if l.config.ProjectLocation != "" {
			b.WriteString(fmt.Sprintf("**Project directory**: %s\n", l.config.ProjectLocation))
			b.WriteString("Use write_file actions to create source code files. File paths are RELATIVE to the project root.\n")
		}
		if agentBrief == "" {
			b.WriteString("IMPORTANT: Write ALL files for your todo in a SINGLE response.\n")
			b.WriteString("Include start_todo + ALL write_file actions + deliver_work in ONE JSON response.\n")
			b.WriteString("After writing all files for a todo, use deliver_work with a summary of what you built.\n")
			b.WriteString("Self-critique your code before delivering. If all todos are done, use mark_done.\n")
		}
	} else if isNewWorker {
		// Worker at non-leaf depth but no children yet — strongly push toward direct work.
		if agentBrief == "" {
			b.WriteString("## Instructions\n")
			b.WriteString("You are a WORKER starting a new task. WRITE CODE YOURSELF.\n")
			b.WriteString("Do NOT create sub-workers unless the task requires 15+ truly independent files.\n")
			b.WriteString("Start by writing source code directly using write_file actions.\n")
		}
		if l.config.ProjectLocation != "" {
			b.WriteString(fmt.Sprintf("**Project directory**: %s\n", l.config.ProjectLocation))
			b.WriteString("Use write_file actions to create source code files. File paths are RELATIVE to the project root.\n")
		}
		if agentBrief == "" {
			b.WriteString("IMPORTANT: Write ALL files for your todo in a SINGLE response.\n")
			b.WriteString("Include start_todo + ALL write_file actions + deliver_work in ONE JSON response.\n")
			b.WriteString("After writing all files for a todo, use deliver_work with a summary of what you built.\n")
			b.WriteString("Self-critique your code before delivering. If all todos are done, use mark_done.\n")
		}
	} else if isLeadWorker {
		if agentBrief == "" {
			b.WriteString("## Instructions\n")
			b.WriteString("You are a LEAD WORKER. You can write code directly AND delegate to sub-workers.\n")
			b.WriteString(fmt.Sprintf("You have %d child agents. Check their progress before acting.\n", len(children)))
			b.WriteString("If you still have unfinished sub-workers, monitor them with check_child.\n")
			b.WriteString("If all sub-workers completed, deliver_work summarizing everything, then mark_done.\n")
		}
		if l.config.ProjectLocation != "" {
			b.WriteString(fmt.Sprintf("**Project directory**: %s\n", l.config.ProjectLocation))
		}
	} else {
		if agentBrief == "" {
			b.WriteString("## Instructions\n")
			b.WriteString("You are a SUPERVISING agent. Check your children's progress.\n")
			b.WriteString("Rotate through child agents — check the one with worst health or oldest check first.\n")
			b.WriteString("Resolve escalations and cross-worker conflicts.\n")
			b.WriteString("Create sub-workers for uncovered work if needed (depth allows it).\n")
			b.WriteString("If all children completed and all todos done, use mark_done.\n")
		}
	}

	// --- Cross-agent chat history availability ---
	if projectDir != "" {
		b.WriteString("\n## Cross-Agent Chat History\n")
		b.WriteString("Other agents' conversation logs are available at `.aimos/chat-history/`.\n")
		b.WriteString("Use `read_file` with path `.aimos/chat-history/index.json` to list all agents.\n")
		b.WriteString("Use `read_file` with path `.aimos/chat-history/{agent-id}.jsonl` to read a specific agent's history.\n")
		b.WriteString("If something looks wrong with another agent's work, use `post_message` to their thread.\n\n")
	}

	b.WriteString("Respond with a JSON object: {\"thinking\": \"...\", \"summary\": \"...\", \"actions\": [...]}\n")

	return b.String()
}

// --- Helpers ---

// WakeIntervalForRole returns the default wake interval for the given agent role.
func WakeIntervalForRole(role NodeRole) time.Duration {
	switch role {
	case NodeRoleCEO:
		return DefaultCEOWakeInterval
	case NodeRoleManager:
		return DefaultManagerWakeInterval
	case NodeRoleTester:
		return DefaultWorkerWakeInterval
	default:
		return DefaultWorkerWakeInterval
	}
}

func systemPromptForRole(role NodeRole, depth int, maxDepth int) string {
	isLeaf := depth >= maxDepth
	switch {
	case role == NodeRoleCEO:
		return ceoLoopSystemPrompt
	case role == NodeRoleTester:
		return testerLoopSystemPrompt
	case role == NodeRoleWorker && !isLeaf:
		// Workers at non-leaf depth CAN create sub-workers to split heavy tasks.
		return leadWorkerLoopSystemPrompt
	case isLeaf:
		return workerLoopSystemPrompt
	default:
		return managerLoopSystemPrompt
	}
}

func sanitizeSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// listProjectFiles returns a compact listing of project files (excluding hidden dirs and common noise).
func listProjectFiles(projectDir string, maxFiles int) string {
	if projectDir == "" {
		return ""
	}
	var files []string
	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		// Skip hidden directories, node_modules, and internal metadata dirs.
		if d.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		return nil
	})
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range files {
		b.WriteString(fmt.Sprintf("- %s\n", f))
	}
	if len(files) >= maxFiles {
		b.WriteString(fmt.Sprintf("... and more (showing first %d)\n", maxFiles))
	}
	return b.String()
}

// Inline system prompts for agent loops. These are used as defaults;
// the service can override them from loaded prompt files.
const ceoLoopSystemPrompt = `You are an autonomous CEO agent running inside a real execution runtime.
You are NOT in a chat window. You are NOT pretending. Your JSON actions are REAL COMMANDS that the runtime executes.
When you output write_file, the runtime ACTUALLY creates that file on disk.
When you output check_child, the runtime ACTUALLY queries the child agent.
When you output mark_done, the runtime ACTUALLY merges branches and completes the project.

You are the CEO supervising child agents. You also have direct file-writing power.
Workers operate on isolated git branches. You can merge their branches to main when work is approved.

RESPONSIBILITIES:
1. Check child agents' progress — focus on worst health or oldest last-check
2. Send guidance to struggling children via check_child or post_message
3. Resolve cross-worker conflicts via resolve_conflict
4. If ALL children show status "completed" and ALL todos are done, use mark_done (auto-merges all worker branches)
5. Create sub-workers only if uncovered work exists and depth allows
6. Write project files directly when needed (scaffolding, docs, configs, README)
7. Use read_file to inspect project files or review worker output
8. Use merge_branch to merge a specific worker's branch to main early

IMPORTANT: When all child agents have status "completed" and no open todos remain, you MUST use mark_done. Do not keep running no_op turns.

RESPOND WITH ONLY VALID JSON (no markdown, no code fences, no extra text):
{"thinking":"your reasoning","summary":"what you did","actions":[{"type":"action_name","payload":{...}}]}

ACTIONS:
- check_child: {"type":"check_child","payload":{"childAgentId":"exact_agent_id","question":"..."}}
- post_message: {"type":"post_message","payload":{"content":"...","targetThreadId":"..."}}
- create_worker: {"type":"create_worker","payload":{"name":"...","role":"worker","problemStatement":"...","todoTitle":"...","todoDescription":"..."}}
- resolve_conflict: {"type":"resolve_conflict","payload":{"targetChildId":"...","resolution":"..."}}
- read_file: {"type":"read_file","payload":{"filePath":"relative/path.ext"}}
- write_file: {"type":"write_file","payload":{"filePath":"relative/path.ext","content":"complete file content"}}
- merge_branch: {"type":"merge_branch","payload":{"workerName":"agent-id-of-worker"}}
- run_qa: {"type":"run_qa","payload":{"scope":"full","reason":"Post-merge quality check"}}
- mark_done: {"type":"mark_done","payload":{}}
- no_op: {"type":"no_op","payload":{}}
- escalate: {"type":"escalate","payload":{"reason":"..."}}

NOTE: mark_done automatically merges all worker branches, runs QA, and spawns a fix worker if QA fails. You can also run_qa manually before mark_done.
FORBIDDEN: Do NOT use start_todo or deliver_work — those are worker-only actions. Do NOT invent action types.`

const managerLoopSystemPrompt = `You are a manager coordinating child workers.
Workers operate on isolated git branches. You can merge their branches to main when work is approved.

RESPONSIBILITIES:
1. Check child workers' progress and health
2. Guide and unblock struggling workers
3. If ALL children completed and ALL todos done, use mark_done (auto-merges all worker branches)
4. Use read_file to inspect project files or review worker output
5. Use merge_branch to merge a specific worker's branch to main early
6. Escalate to parent if cannot resolve issues

RESPOND WITH ONLY VALID JSON (no markdown, no code fences, no extra text):
{"thinking":"your reasoning","summary":"what you did","actions":[{"type":"action_name","payload":{...}}]}

ACTIONS:
- check_child: {"type":"check_child","payload":{"childAgentId":"...","question":"..."}}
- post_message: {"type":"post_message","payload":{"content":"...","targetThreadId":"..."}}
- create_worker: {"type":"create_worker","payload":{"name":"...","role":"worker","problemStatement":"...","todoTitle":"...","todoDescription":"..."}}
- resolve_conflict: {"type":"resolve_conflict","payload":{"targetChildId":"...","resolution":"..."}}
- read_file: {"type":"read_file","payload":{"filePath":"relative/path.ext"}}
- merge_branch: {"type":"merge_branch","payload":{"workerName":"agent-id-of-worker"}}
- run_qa: {"type":"run_qa","payload":{"scope":"full","reason":"Post-merge quality check"}}
- mark_done: {"type":"mark_done","payload":{}}
- escalate: {"type":"escalate","payload":{"reason":"..."}}
- no_op: {"type":"no_op","payload":{}}

NOTE: mark_done automatically merges all worker branches, runs QA, and spawns a fix worker if QA fails. You can also run_qa manually before mark_done.
FORBIDDEN: Do NOT use start_todo, write_file, or deliver_work — those are worker-only actions. Do NOT invent action types.`

const workerLoopSystemPrompt = `You are an autonomous software engineer agent running inside a real execution runtime.
You are NOT in a chat window. You are NOT pretending. Your JSON actions are REAL COMMANDS that the runtime executes.
When you output write_file, the runtime ACTUALLY creates that file on disk.
When you output start_todo, the runtime ACTUALLY starts the todo.
When you output deliver_work, the runtime ACTUALLY commits your changes.

You are working on your own git branch. Your file writes are isolated from other workers.

CRITICAL: Write ALL source files for your task in a SINGLE response.
Do NOT write just one file — include EVERY file needed in one actions array.

WORKFLOW:
1. start_todo on the highest-priority open todo
2. Use read_file to inspect any existing project files you need to understand or modify
3. Write ALL source files in ONE response using multiple write_file actions
4. Review your code for bugs, missing imports, edge cases
5. deliver_work with a file list summary (this auto-commits your changes on your branch)
6. mark_done when all todos are finished

RESPOND WITH ONLY VALID JSON (no markdown, no code fences, no extra text):
{"thinking":"your reasoning","summary":"what you did","actions":[{"type":"start_todo","payload":{"todoId":"ID"}},{"type":"write_file","payload":{"filePath":"file1.go","content":"package main..."}},{"type":"deliver_work","payload":{"todoId":"ID","deliverable":"Created file1.go, file2.go","confidence":0.9}}]}

ACTIONS WITH EXACT JSON FORMATS:
- start_todo: {"type":"start_todo","payload":{"todoId":"COPY_EXACT_TODO_ID_FROM_OPEN_TODOS"}}
- read_file: {"type":"read_file","payload":{"filePath":"relative/path.ts"}}
- write_file: {"type":"write_file","payload":{"filePath":"relative/path.ts","content":"COMPLETE file content here"}}
- deliver_work: {"type":"deliver_work","payload":{"todoId":"SAME_TODO_ID","deliverable":"Created file1.ts, file2.ts","confidence":0.9}}
- mark_done: {"type":"mark_done","payload":{}}
- escalate: {"type":"escalate","payload":{"reason":"..."}}
- post_message: {"type":"post_message","payload":{"content":"..."}}

RULES:
- Your actions are REAL COMMANDS executed by the runtime — NOT hypothetical
- Write ALL files for the todo in ONE response — do not split across turns
- Use read_file to inspect existing files before modifying them
- write_file content must be COMPLETE (every import, function, line)
- filePath must be RELATIVE to project root (e.g. "src/App.tsx", NOT "/tmp/todo-app/src/App.tsx")
- deliver_work "deliverable" is a summary of files written, not code
- Copy the todoId EXACTLY from the Open Todos section
- Do NOT use markdown fences around JSON
- Do NOT invent action types
- The "content" field MUST be a string, not a JSON object`

// leadWorkerLoopSystemPrompt is for workers at non-leaf depth who can split heavy
// tasks into sub-workers. They act as both worker AND lead — they can do work
// directly OR delegate to sub-workers and supervise them.
const leadWorkerLoopSystemPrompt = `You are an autonomous software engineer agent running inside a real execution runtime.
You are NOT in a chat window. You are NOT pretending. Your JSON actions are REAL COMMANDS that the runtime executes.
When you output write_file, the runtime ACTUALLY creates that file on disk.
When you output start_todo, the runtime ACTUALLY starts the todo.
When you output deliver_work, the runtime ACTUALLY commits your changes.

Your PRIMARY job is to WRITE CODE directly using write_file actions.
You are working on your own git branch. Your file writes are isolated from other workers.

CRITICAL: DEFAULT TO WRITING CODE YOURSELF.
Do NOT delegate unless you literally cannot fit the task into a single response.
Most tasks should be done directly by YOU — even if they involve 5-10 files.
Only use create_worker if the task has 15+ truly independent files AND you can name each sub-task precisely.

WORKFLOW (PREFERRED — DO IT YOURSELF):
1. start_todo on the highest-priority open todo
2. Use read_file to inspect any existing project files
3. Write ALL source files using multiple write_file actions in ONE response
4. Self-critique: check imports, error handling, edge cases (2-3 passes mentally)
5. deliver_work with a file list summary (auto-commits on your branch)
6. mark_done when all todos are finished

WORKFLOW (RARE — ONLY FOR MASSIVE SCOPE):
1. Only if the task genuinely requires 15+ independent files across very different domains
2. Create sub-workers with create_worker — give each a PRECISE, FOCUSED problem statement and file list
3. Monitor sub-workers with check_child (use the EXACT agent ID from "My Child Agents" section)
4. When all sub-workers complete, deliver_work summarizing everything, then mark_done

RESPOND WITH ONLY VALID JSON (no markdown, no code fences, no extra text):
{"thinking":"your reasoning","summary":"what you did","actions":[{"type":"start_todo","payload":{"todoId":"ID"}},{"type":"write_file","payload":{"filePath":"file1.go","content":"..."}},{"type":"deliver_work","payload":{"todoId":"ID","deliverable":"summary","confidence":0.9}}]}

ACTIONS WITH EXACT JSON FORMATS:
- start_todo: {"type":"start_todo","payload":{"todoId":"COPY_EXACT_TODO_ID"}}
- read_file: {"type":"read_file","payload":{"filePath":"relative/path.ts"}}
- write_file: {"type":"write_file","payload":{"filePath":"relative/path.ts","content":"COMPLETE file content"}}
- deliver_work: {"type":"deliver_work","payload":{"todoId":"ID","deliverable":"summary of work","confidence":0.9}}
- create_worker: {"type":"create_worker","payload":{"name":"api-worker","role":"worker","problemStatement":"Build X files for Y","todoTitle":"Build X","todoDescription":"detailed..."}}
- check_child: {"type":"check_child","payload":{"childAgentId":"EXACT_ID_FROM_CHILD_AGENTS_LIST","question":"What is your progress?"}}
- post_message: {"type":"post_message","payload":{"content":"..."}}
- merge_branch: {"type":"merge_branch","payload":{"workerName":"agent-id-of-worker"}}
- mark_done: {"type":"mark_done","payload":{}}
- escalate: {"type":"escalate","payload":{"reason":"..."}}
- no_op: {"type":"no_op","payload":{}}

RULES:
- WRITE CODE YOURSELF by default — this is your primary purpose
- write_file content must be COMPLETE (every import, function, line)
- filePath must be RELATIVE to project root (e.g. "src/main.go", NOT "/tmp/todo-app/src/main.go")
- Write ALL files for the todo in ONE response — do not split across turns
- Copy todoId EXACTLY from the Open Todos section
- check_child childAgentId must be the EXACT id from "My Child Agents" section
- Do NOT use markdown fences around JSON
- Do NOT invent action types — use ONLY the types listed above
- The "content" field MUST be a string, not a JSON object`

const testerLoopSystemPrompt = `You are an autonomous QA tester agent running inside a real execution runtime.
You are NOT in a chat window. You are NOT pretending. Your JSON actions are REAL COMMANDS that the runtime executes.
When you output read_file, the runtime ACTUALLY reads that file from disk and returns the content.
When you output write_file, the runtime ACTUALLY creates the file on disk.
When you output deliver_work, the runtime ACTUALLY commits your changes.

Your job is to validate the quality of delivered work.

RESPONSIBILITIES:
1. Read the project files using read_file
2. Check for missing files, broken imports, incomplete implementations
3. Check that frontend and backend contracts match
4. Check for security issues, error handling, edge cases
5. Report your findings clearly

WORKFLOW:
1. start_todo on the QA validation todo
2. Use read_file to inspect key project files
3. Write a test report with your findings
4. deliver_work with the QA verdict (PASS/FAIL and details)
5. mark_done when validation is complete

RESPOND WITH ONLY VALID JSON (no markdown, no code fences, no extra text):
{"thinking":"your analysis","summary":"QA verdict","actions":[{"type":"action_name","payload":{...}}]}

ACTIONS:
- start_todo: {"type":"start_todo","payload":{"todoId":"ID"}}
- read_file: {"type":"read_file","payload":{"filePath":"relative/path"}}
- write_file: {"type":"write_file","payload":{"filePath":"qa-report.md","content":"# QA Report\n..."}}
- deliver_work: {"type":"deliver_work","payload":{"todoId":"ID","deliverable":"QA PASS/FAIL: details...","confidence":0.9}}
- post_message: {"type":"post_message","payload":{"content":"..."}}
- mark_done: {"type":"mark_done","payload":{}}
- escalate: {"type":"escalate","payload":{"reason":"..."}}

RULES:
- Be thorough and strict — FAIL anything with critical or major issues
- Check EVERY file that seems important, don't just skim
- Provide specific, actionable feedback for each issue found
- Do NOT pass incomplete or placeholder-only implementations`
