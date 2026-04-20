package ceo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/agents"
	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/contextdocs"
	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/microai"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	pgbootstrap "github.com/Sarnga/agent-platform/pkg/postgres"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

const ceoRecentMessagesLimit = 8

type Service struct {
	config              Config
	llm                 CompletionClient
	missionStore        missions.Store
	threadStore         threads.Store
	missionStateStore   missionstate.Store
	feedbackStore       feedback.Store
	attachmentStore     attachments.Store
	nodeStore           agents.NodeStore
	contextBuilder      *contextpacks.Builder
	executionRuntime    *execution.Runtime
	timerProcessor      *execution.TimerProcessor
	missionRuntime      *missions.Runtime
	missionStateRuntime *missionstate.Runtime
	delegateSelector    *delegateSelector
	loopManager         *agents.AgentLoopManager
	wakeConfig          *agents.WakeIntervalConfig
	testingAgent        *agents.TestingAgent
	qaAgent             *agents.QAAgent
	reasoner            microai.Interface
	backgroundCancel    context.CancelFunc
	cleanup             func()
	onFilesChanged      func(ctx context.Context, projectPath string)
}

type CompletionClient interface {
	Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error)
	GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error)
}

type contextEnvelope struct {
	Mode      Mode   `json:"mode,omitempty"`
	MissionID string `json:"missionId,omitempty"`
}

func NewService(config Config, llm CompletionClient, missionStore missions.Store, threadStore threads.Store, missionStateStore missionstate.Store, executionStore execution.Store, feedbackStore feedback.Store, attachmentStore attachments.Store, nodeStore agents.NodeStore) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if llm == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("completion client is required"))
	}
	if missionStore == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("mission store is required"))
	}
	if threadStore == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("thread store is required"))
	}
	if missionStateStore == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("mission state store is required"))
	}
	if executionStore == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("execution store is required"))
	}
	if feedbackStore == nil {
		return nil, logValidationError("invalid CEO service", fmt.Errorf("feedback store is required"))
	}
	missionStateRuntime, err := missionstate.NewRuntime(missionStateStore, missionStore, threadStore, executionStore)
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	observedThreadStore, err := missionstate.NewObservedThreadStore(threadStore, missionStateRuntime)
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	contextBuilder, err := contextpacks.NewBuilder(missionStore, observedThreadStore, missionStateStore, executionStore, attachmentStore)
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	missionRuntime, err := missions.NewRuntime(missionStore, observedThreadStore)
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	executionRuntime, err := execution.NewRuntime(executionStore)
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	delegateSelector, err := newDelegateSelector()
	if err != nil {
		return nil, logValidationError("invalid CEO service", err)
	}
	if nodeStore == nil {
		nodeStore = agents.NewMemoryNodeStore()
	}
	return &Service{
		config:              config,
		llm:                 llm,
		missionStore:        missionStore,
		threadStore:         observedThreadStore,
		missionStateStore:   missionStateStore,
		feedbackStore:       feedbackStore,
		attachmentStore:     attachmentStore,
		nodeStore:           nodeStore,
		contextBuilder:      contextBuilder,
		executionRuntime:    executionRuntime,
		missionRuntime:      missionRuntime,
		missionStateRuntime: missionStateRuntime,
		delegateSelector:    delegateSelector,
		loopManager:         agents.NewAgentLoopManager(),
		wakeConfig:          agents.NewWakeIntervalConfig(),
		testingAgent:        agents.NewTestingAgent(llm, config.Model),
		qaAgent:             agents.NewQAAgent(llm, config.Model),
	}, nil
}

func NewServiceFromEnv(envFile string) (*Service, error) {
	config, err := LoadConfig(envFile)
	if err != nil {
		return nil, err
	}
	postgresConfig, err := pgbootstrap.LoadConfig(envFile)
	if err != nil {
		return nil, err
	}
	stores, err := pgbootstrap.OpenStores(context.Background(), postgresConfig)
	if err != nil {
		return nil, err
	}

	routerClient := aiclients.NewRouterClient(aiclients.RouterConfig{
		OpenAI: &aiclients.OpenAIConfig{
			APIKey:          config.APIKey,
			BaseURL:         config.BaseURL,
			MaxOutputTokens: config.MaxOutputTokens,
		},
		Ollama: &aiclients.OllamaConfig{
			BaseURL: config.OllamaURL,
		},
		Pigeon: config.pigeonConfig(),
	}, logger)

	service, err := NewService(config, routerClient, stores.Missions, stores.Threads, stores.MissionState, stores.Execution, stores.Feedback, stores.Attachments, stores.Nodes)
	if err != nil {
		stores.Close()
		return nil, err
	}
	service.reasoner = microai.New(routerClient, microai.WithModel(config.EffectiveMicroModel()))
	service.cleanup = stores.Close
	if err := service.startTimerProcessor(); err != nil {
		service.Close()
		return nil, err
	}
	service.recoverActiveLoops()
	return service, nil
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	if s.loopManager != nil {
		s.loopManager.StopAll()
	}
	if s.backgroundCancel != nil {
		s.backgroundCancel()
		s.backgroundCancel = nil
	}
	if s.cleanup != nil {
		s.cleanup()
		s.cleanup = nil
	}
}

// SetOnFilesChanged registers a callback invoked once per agent turn when
// any write_file actions succeed. Used to trigger knowledge re-indexing.
func (s *Service) SetOnFilesChanged(fn func(ctx context.Context, projectPath string)) {
	s.onFilesChanged = fn
}

func (s *Service) startTimerProcessor() error {
	if s == nil || s.executionRuntime == nil {
		return nil
	}
	processor, err := execution.NewTimerProcessor(s.executionRuntime, s.handleTriggeredTimer, execution.TimerProcessorConfig{
		OnError: func(err error) {
			logger.Error("timer processor cycle failed", "error", err)
		},
	})
	if err != nil {
		return logValidationError("failed to create timer processor", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.timerProcessor = processor
	s.backgroundCancel = cancel
	go func() {
		if runErr := processor.Run(ctx); runErr != nil && ctx.Err() == nil {
			logger.Error("timer processor stopped", "error", runErr)
		}
	}()
	return nil
}

func (s *Service) handleTriggeredTimer(ctx context.Context, timer execution.Timer) error {
	messageType := "timer_triggered"
	messageContent, err := s.applyTriggeredTimerPolicy(ctx, timer)
	if err != nil {
		return err
	}
	if timer.ActionType == "escalate" {
		messageType = "timer_escalated"
	}
	threadID := timer.ThreadID
	if threadID == "" {
		mission, err := s.missionStore.GetMission(timer.MissionID)
		if err != nil {
			return err
		}
		threadID = mission.OwningThreadID
	}
	messageTime := time.Now().UTC()
	return s.threadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("timer-triggered-%d", messageTime.UnixNano()),
		ThreadID:      threadID,
		Role:          threads.RoleAssistant,
		AuthorAgentID: "timer-worker",
		AuthorRole:    "system",
		MessageType:   messageType,
		Content:       messageContent,
		ContentJSON:   timer.ActionPayload,
		Mode:          string(ModeExecutionPrep),
		CreatedAt:     messageTime,
	})
}

func (s *Service) applyTriggeredTimerPolicy(ctx context.Context, timer execution.Timer) (string, error) {
	// Let the escalate action block the mission.
	if timer.ActionType == "escalate" {
		mission, err := s.missionStore.GetMission(timer.MissionID)
		if err != nil {
			return "", err
		}
		if !missions.IsTerminalMissionStatus(mission.Status) {
			mission.Status = missions.MissionStatusBlocked
			mission.WaitingUntil = nil
			if err := s.missionStore.UpdateMission(mission); err != nil {
				return "", err
			}
		}
	}
	// Use AI to compose the trigger message.
	if s.reasoner != nil {
		payload := decodeObjectMap(timer.ActionPayload)
		input := fmt.Sprintf("actionType=%s missionID=%s wakeAt=%s payload=%v", timer.ActionType, timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339), payload)
		task := "Compose a concise one-sentence timer trigger notification for an agent. Include the action type, mission ID, timestamp, and any message or reason from the payload. Return ONLY the message text."
		result, err := s.reasoner.Reason(ctx, task, input)
		if err == nil {
			result = strings.TrimSpace(result)
			if result != "" {
				return result, nil
			}
		}
	}
	// Fallback: simple structured message.
	msg := fmt.Sprintf("Timer triggered: %s for mission %s at %s.", timer.ActionType, timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339))
	payload := decodeObjectMap(timer.ActionPayload)
	if reason, ok := payload["reason"].(string); ok && reason != "" {
		msg += " Reason: " + reason
	} else if message, ok := payload["message"].(string); ok && message != "" {
		msg += " Message: " + message
	}
	return msg, nil
}

func (s *Service) Respond(ctx context.Context, request Request) (ResponseEnvelope, error) {
	if err := request.Validate(); err != nil {
		return ResponseEnvelope{}, err
	}

	contextData, err := contextFromRequest(request.Context)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	effectiveMissionID, err := resolveMissionID(request, contextData)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	if request.Action != nil {
		if effectiveMissionID == "" && request.ThreadID == "" {
			return ResponseEnvelope{}, logValidationError("invalid CEO action request", errors.New("missionId or threadId is required for mission-scoped actions"))
		}
		thread, err := s.resolveConversationTarget(request, effectiveMissionID)
		if err != nil {
			return ResponseEnvelope{}, err
		}
		return s.respondToExecutionAction(request, thread)
	}
	mode, err := s.resolveMode(ctx, request)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	thread, err := s.resolveConversationTarget(request, effectiveMissionID)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	threadID := thread.ID
	if err := s.threadStore.UpdateThreadMode(threadID, string(mode)); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to update thread mode", err, "threadID", threadID, "mode", mode)
	}

	projectLocation := s.resolveProjectLocation(thread.ID)

	// Enrich context with project metadata so the logging middleware
	// can write per-project AI call logs.
	ctx = aiclients.WithLogContext(ctx, aiclients.LogContext{
		ProjectSlug: projectLocation,
		MissionID:   thread.MissionID,
		ThreadID:    threadID,
		TraceID:     traceIDOrFallback(request.TraceID, threadID),
	})

	// On transition to planning modes, generate durable context documents
	// from the conversation history so all downstream consumers (CEO,
	// agents, context packs) use compact indexed docs instead of raw chat.
	if (mode == ModeHighLevelPlan || mode == ModeRoadmap) &&
		projectLocation != "" &&
		!contextdocs.Exists(projectLocation, contextdocs.DocProjectOverview, "") {
		s.generateContextDocs(ctx, threadID, projectLocation, s.modelForRequest(request))
	}

	// When context docs exist, reduce the attachment budget since the
	// knowledge index provides project awareness more efficiently.
	attachmentBudget := 0 // 0 = use default (32KB)
	if contextdocs.Exists(projectLocation, contextdocs.DocProjectOverview, "") {
		attachmentBudget = 8192 // 8KB when context docs provide project awareness
	}
	contextPack, err := s.contextBuilder.BuildMissionPack(thread.MissionID, threadID, contextpacks.BuildOptions{
		RecentMessagesLimit:   ceoRecentMessagesLimit,
		IncludeChildRollups:   true,
		AttachmentTokenBudget: attachmentBudget,
	})
	if err != nil {
		return ResponseEnvelope{}, logValidationError("failed to build context pack", err, "threadID", threadID, "missionID", thread.MissionID)
	}
	var payload map[string]any
	var assistantMessage string
	if mode == ModeRoadmap {
		payload, assistantMessage, err = s.planRoadmap(ctx, contextPack, request.Prompt, s.modelForRequest(request))
		if err != nil {
			return ResponseEnvelope{}, err
		}
	} else {
		conversation, err := s.buildConversation(mode, request.Prompt, contextPack, request.KnowledgeSummary, projectLocation)
		if err != nil {
			return ResponseEnvelope{}, err
		}

		rawResponse, err := s.llm.GenerateFromMessages(ctx, s.modelForRequest(request), conversation)
		if err != nil {
			return ResponseEnvelope{}, err
		}
		payload, assistantMessage, err = buildResponsePayload(mode, rawResponse, s.modelForRequest(request))
		if err != nil {
			return ResponseEnvelope{}, err
		}
	}

	// Execute any file-writing actions the CEO included in its response.
	if projectLocation != "" {
		if actions, ok := payload["actions"].([]ceoActionItem); ok && len(actions) > 0 {
			written := s.executeCEOFileActions(actions, projectLocation, thread)
			if written > 0 {
				logger.Info("CEO wrote files directly", "count", written, "threadID", threadID, "projectLocation", projectLocation)
				if s.onFilesChanged != nil {
					s.onFilesChanged(ctx, projectLocation)
				}
			}
		}
	}

	now := time.Now().UTC()
	userMessageID := fmt.Sprintf("user-%d", now.UnixNano())
	if err := s.threadStore.AppendMessage(threads.Message{
		ID:            userMessageID,
		ThreadID:      threadID,
		Role:          threads.RoleUser,
		AuthorAgentID: "user",
		AuthorRole:    "client",
		MessageType:   "client_message",
		Content:       request.Prompt,
		ContentJSON:   request.Context,
		Mode:          string(mode),
		CreatedAt:     now,
	}); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to append user message", err, "threadID", threadID)
	}
	assistantTime := time.Now().UTC()
	responseID := fmt.Sprintf("assistant-%d", assistantTime.UnixNano())
	responseEnvelope, err := NewResponseEnvelope(
		responseID,
		threadID,
		traceIDOrFallback(request.TraceID, threadID),
		mode,
		payload,
		defaultRatingPrompt(),
	)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	responseEnvelope.CreatedAt = assistantTime
	if err := s.threadStore.AppendMessage(threads.Message{
		ID:               responseID,
		ThreadID:         threadID,
		Role:             threads.RoleAssistant,
		AuthorAgentID:    "ceo",
		AuthorRole:       "ceo",
		MessageType:      "ceo_message",
		Content:          assistantMessage,
		ContentJSON:      responseEnvelope.Payload,
		Mode:             string(mode),
		ReplyToMessageID: userMessageID,
		CreatedAt:        assistantTime,
	}); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to append assistant message", err, "threadID", threadID)
	}

	return responseEnvelope, nil
}

func (s *Service) resolveConversationTarget(request Request, missionID string) (threads.Thread, error) {
	if missionID != "" {
		return s.resolveMissionTarget(missionID, request.ThreadID)
	}
	threadID := threadIDOrFallback(request.ThreadID)
	customTitle := ""
	if len(request.Context) > 0 {
		var ctxMap map[string]interface{}
		if json.Unmarshal(request.Context, &ctxMap) == nil {
			if title, ok := ctxMap["customTitle"].(string); ok {
				customTitle = title
			}
		}
	}
	return s.ensureConversationGraph(threadID, customTitle)
}

func (s *Service) resolveMissionTarget(missionID string, requestedThreadID string) (threads.Thread, error) {
	mission, err := s.missionStore.GetMission(missionID)
	if err != nil {
		if errors.Is(err, missions.ErrMissionNotFound) {
			return threads.Thread{}, logValidationError("mission target not found", err, "missionID", missionID)
		}
		return threads.Thread{}, logValidationError("failed to fetch mission target", err, "missionID", missionID)
	}

	targetThreadID := requestedThreadID
	if targetThreadID == "" {
		targetThreadID = mission.OwningThreadID
	}
	if targetThreadID == "" {
		return threads.Thread{}, logValidationError("mission target is missing owning thread", fmt.Errorf("mission %q does not have an owning thread", missionID), "missionID", missionID)
	}

	thread, err := s.threadStore.GetThread(targetThreadID)
	if err != nil {
		if errors.Is(err, threads.ErrThreadNotFound) {
			return threads.Thread{}, logValidationError("mission target thread not found", err, "missionID", missionID, "threadID", targetThreadID)
		}
		return threads.Thread{}, logValidationError("failed to fetch mission target thread", err, "missionID", missionID, "threadID", targetThreadID)
	}
	if thread.MissionID != missionID {
		return threads.Thread{}, logValidationError("mission target thread mismatch", fmt.Errorf("thread %q belongs to mission %q, not %q", thread.ID, thread.MissionID, missionID), "missionID", missionID, "threadID", targetThreadID)
	}
	return thread, nil
}

func (s *Service) ensureConversationGraph(threadID string, customTitle string) (threads.Thread, error) {
	thread, err := s.threadStore.GetThread(threadID)
	if err == nil {
		if thread.MissionID == "" {
			return threads.Thread{}, logValidationError("thread is missing mission linkage", fmt.Errorf("thread %q does not reference a mission", threadID), "threadID", threadID)
		}
		if _, missionErr := s.missionStore.GetMission(thread.MissionID); missionErr == nil {
			// Thread and mission exist — ensure the CEO agent node is also present
			// (it may be missing if the in-memory node store was reset by a server restart).
			s.ensureCEOAgentNode(threadID, customTitle)
			return thread, nil
		} else if errors.Is(missionErr, missions.ErrMissionNotFound) {
			return threads.Thread{}, logValidationError("thread references missing mission", missionErr, "threadID", threadID, "missionID", thread.MissionID)
		} else {
			return threads.Thread{}, logValidationError("failed to fetch mission for thread", missionErr, "threadID", threadID, "missionID", thread.MissionID)
		}
	}
	if !errors.Is(err, threads.ErrThreadNotFound) {
		return threads.Thread{}, logValidationError("failed to fetch thread", err, "threadID", threadID)
	}

	if _, _, _, err := s.missionRuntime.CreateProgramWithRootMission(missions.RootMissionInput{
		ProgramID:      fallbackProgramID(threadID),
		ClientID:       "client-pending",
		ProgramTitle:   fallbackProgramTitle(threadID, customTitle),
		MissionID:      threadID,
		ThreadID:       threadID,
		OwnerAgentID:   "ceo",
		OwnerRole:      "ceo",
		MissionType:    "conversation",
		ThreadKind:     "strategy",
		MissionTitle:   fallbackMissionTitle(threadID, customTitle),
		Charter:        "Maintain the CEO conversation lane until orchestration provisions a richer mission structure.",
		Goal:           "Handle the current CEO conversation with durable mission state.",
		Scope:          "Single fallback CEO conversation thread.",
		AuthorityLevel: "thread",
		ThreadTitle:    fallbackThreadTitle(threadID, customTitle),
		ThreadSummary:  "CEO conversation thread",
		ThreadContext:  "Auto-created fallback root mission and owning thread so the CEO can build mission-scoped context before dedicated orchestration takes over.",
	}); err != nil {
		return threads.Thread{}, logValidationError("failed to bootstrap fallback mission graph", err, "threadID", threadID)
	}

	// Create root CEO agent node for the mindmap
	s.ensureCEOAgentNode(threadID, customTitle)

	thread, err = s.threadStore.GetThread(threadID)
	if err != nil {
		return threads.Thread{}, logValidationError("failed to fetch bootstrapped thread", err, "threadID", threadID)
	}
	return thread, nil
}

func (s *Service) buildConversation(mode Mode, prompt string, pack contextpacks.ContextPack, knowledgeSummary string, projectLocation string) ([]threads.Message, error) {
	systemPrompt, err := loadSystemPrompt(mode)
	if err != nil {
		return nil, err
	}

	hasContextDocs := contextdocs.Exists(projectLocation, contextdocs.DocProjectOverview, "")

	// When durable context docs exist, reduce the recent messages window
	// because project state is captured in docs instead of chat history.
	recentMessages := pack.RecentMessages
	if hasContextDocs && len(recentMessages) > 4 {
		recentMessages = recentMessages[len(recentMessages)-4:]
	}

	conversation := make([]threads.Message, 0, len(recentMessages)+4)
	conversation = append(conversation,
		threads.Message{Role: threads.RoleSystem, Content: systemPrompt},
		threads.Message{Role: threads.RoleSystem, Content: formatContextPack(pack, projectLocation)},
	)
	if knowledgeSummary != "" {
		// When context docs exist, the knowledge summary is supplementary.
		// Truncate it more aggressively to keep total context compact.
		if hasContextDocs && len(knowledgeSummary) > 4000 {
			knowledgeSummary = knowledgeSummary[:4000] + "\n... [truncated — context docs provide primary project context]"
		}
		conversation = append(conversation, threads.Message{
			Role:    threads.RoleSystem,
			Content: knowledgeSummary,
		})
	}
	conversation = append(conversation, recentMessages...)
	userMsg := threads.Message{Role: threads.RoleUser, Content: prompt, Mode: string(mode)}
	if len(pack.ImageDataURLs) > 0 {
		userMsg.ImageDataURLs = pack.ImageDataURLs
	}
	conversation = append(conversation, userMsg)
	return conversation, nil
}

// ensureCEOAgentNode creates the root CEO agent node for the mindmap if it does not
// already exist. This is safe to call on every request so the CEO node survives
// in-memory store resets caused by server restarts.
func (s *Service) ensureCEOAgentNode(threadID string, customTitle string) {
	ceoNodeID := fmt.Sprintf("agent-ceo-%s", threadID)
	if _, err := s.nodeStore.GetNode(ceoNodeID); err == nil {
		return // already exists
	}
	title := fallbackMissionTitle(threadID, customTitle)
	if err := s.nodeStore.CreateNode(agents.AgentNode{
		ID:               ceoNodeID,
		RootAgentID:      ceoNodeID,
		ProjectID:        threadID,
		ThreadID:         threadID,
		MissionID:        threadID,
		Name:             "CEO Agent",
		Role:             agents.NodeRoleCEO,
		ProblemStatement: title,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}); err != nil {
		logger.Error("failed to create CEO agent node", "error", err, "threadID", threadID)
	}
}

// recoverActiveLoops scans for active agent nodes and restarts their autonomous loops.
// Called during service startup to recover loops lost after a server restart.
// When config.CleanStartup is set, all active nodes are marked terminated instead.
func (s *Service) recoverActiveLoops() {
	if s.loopManager == nil || s.nodeStore == nil {
		return
	}
	activeNodes, err := s.nodeStore.ListActive()
	if err != nil {
		logger.Error("failed to list active nodes for loop recovery", "error", err)
		return
	}
	if s.config.CleanStartup {
		for _, node := range activeNodes {
			if err := s.nodeStore.UpdateStatus(node.ID, "terminated"); err != nil {
				logger.Error("failed to terminate stale node on clean startup", "agentID", node.ID, "error", err)
			}
		}
		if len(activeNodes) > 0 {
			logger.Info("clean startup: terminated stale agent nodes", "count", len(activeNodes))
		}
		return
	}
	recovered := 0
	for _, node := range activeNodes {
		if s.loopManager.IsRunning(node.ID) {
			continue
		}
		s.startAgentLoop(node)
		recovered++
	}
	if recovered > 0 {
		logger.Info("recovered active agent loops on startup", "count", recovered)
	}
}

// buildLoopDeps constructs the shared dependency bundle for agent loops from the service's stores.
func (s *Service) buildLoopDeps() agents.AgentLoopDeps {
	return agents.AgentLoopDeps{
		LLM:                 s.llm,
		NodeStore:           s.nodeStore,
		ThreadStore:         s.threadStore,
		MissionStore:        s.missionStore,
		ExecutionRuntime:    s.executionRuntime,
		MissionRuntime:      s.missionRuntime,
		MissionStateRuntime: s.missionStateRuntime,
		ContextBuilder:      s.contextBuilder,
		LoopManager:         s.loopManager,
		TestingAgent:        s.testingAgent,
		QAAgent:             s.qaAgent,
		AttachmentStore:     s.attachmentStore,
		OnFilesChanged:      s.onFilesChanged,
		Reasoner:            s.reasoner,
		WakeConfig:          s.wakeConfig,
	}
}

// startAgentLoop starts an autonomous loop for the given agent node if it is not already running.
func (s *Service) startAgentLoop(node agents.AgentNode) {
	s.startAgentLoopWithLocation(node, "")
}

// startAgentLoopWithLocation starts an agent loop with an explicit project
// location. When projectLocationOverride is empty the default resolution is used.
func (s *Service) startAgentLoopWithLocation(node agents.AgentNode, projectLocationOverride string) {
	if s.loopManager == nil || s.loopManager.IsRunning(node.ID) {
		return
	}
	// Use per-node model if set, otherwise fall back to the service default.
	model := node.Model
	if model == "" {
		model = s.config.Model
	}
	projectLocation := projectLocationOverride
	if projectLocation == "" {
		projectLocation = s.resolveProjectLocation(node.ProjectID)
	}
	config := agents.AgentLoopConfig{
		AgentID:         node.ID,
		AgentRole:       node.Role,
		MissionID:       node.MissionID,
		ThreadID:        node.ThreadID,
		ProjectID:       node.ProjectID,
		ProjectLocation: projectLocation,
		Depth:           node.Depth,
		MaxDepth:        agents.DefaultMaxDepth,
		WakeInterval:    agents.WakeIntervalForRole(node.Role),
		Model:           model,
	}
	if err := s.loopManager.StartLoop(config, s.buildLoopDeps()); err != nil {
		logger.Error("failed to start agent loop", "agentID", node.ID, "error", err)
		return
	}
	// If the node was paused in the database, pause the loop immediately.
	if node.Paused {
		s.loopManager.PauseLoop(node.ID)
	}
}

// resolveProjectLocation extracts the project filesystem path from thread messages
// using AI classification. Falls back to structural scanning if AI is unavailable.
func (s *Service) resolveProjectLocation(projectID string) string {
	if projectID == "" || s.threadStore == nil {
		return defaultProjectLocation(projectID)
	}
	messages, err := s.threadStore.ListMessages(projectID)
	if err != nil {
		return defaultProjectLocation(projectID)
	}
	// Collect recent user messages for AI extraction.
	var candidates []string
	for i := len(messages) - 1; i >= 0 && len(candidates) < 5; i-- {
		msg := messages[i]
		if msg.Role != threads.RoleUser {
			continue
		}
		snippet := ""
		if len(msg.ContentJSON) > 0 {
			snippet += string(msg.ContentJSON) + "\n"
		}
		if msg.Content != "" {
			snippet += msg.Content
		}
		if strings.TrimSpace(snippet) != "" {
			candidates = append(candidates, snippet)
		}
	}
	if len(candidates) == 0 {
		return defaultProjectLocation(projectID)
	}
	if s.reasoner != nil {
		task := "Extract the project filesystem path from the following user messages. Look for fields like projectDir, projectPath, projectLocation, project_dir, project_path, project_location in JSON, or 'Location: /path' in text. Return ONLY the absolute path, nothing else. If no path is found, return NONE."
		input := strings.Join(candidates, "\n---\n")
		result, err := s.reasoner.Reason(context.Background(), task, input)
		if err == nil {
			result = strings.TrimSpace(strings.Trim(result, "\"'` \n"))
			if result != "" && result != "NONE" && strings.HasPrefix(result, "/") {
				return result
			}
		}
	}
	// Keyword fallback: scan for known patterns when no reasoner is available.
	for _, candidate := range candidates {
		// Try JSON fields.
		var obj map[string]any
		if json.Unmarshal([]byte(candidate), &obj) == nil {
			for _, key := range []string{"projectDir", "projectPath", "projectLocation", "project_dir", "project_path", "project_location"} {
				if v, ok := obj[key].(string); ok && strings.HasPrefix(v, "/") {
					return v
				}
			}
		}
		// Try "Location: /path" text pattern.
		for _, line := range strings.Split(candidate, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Location:") {
				path := strings.TrimSpace(strings.TrimPrefix(line, "Location:"))
				if strings.HasPrefix(path, "/") {
					return path
				}
			}
		}
	}
	return defaultProjectLocation(projectID)
}

// defaultProjectLocation returns a fallback project directory when no explicit
// location was provided by the client. Creates the directory if it does not exist.
func defaultProjectLocation(projectID string) string {
	if projectID == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, "aimos-projects", projectID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Error("failed to create default project directory", "path", dir, "error", err)
		return ""
	}
	return dir
}

func formatContextPack(pack contextpacks.ContextPack, projectLocation string) string {
	var builder strings.Builder
	builder.WriteString("Use this durable mission context before answering. Prefer the mission state and child rollups over replaying older transcript details.\n\n")

	// When PROJECT_OVERVIEW exists, use it as compact project context and
	// emit only minimal mission metadata. Otherwise, use full mission fields.
	overview, _ := contextdocs.ReadDocument(projectLocation, contextdocs.DocProjectOverview, "")
	if overview != "" {
		builder.WriteString("Project Overview:\n")
		if len(overview) > 2000 {
			overview = overview[:2000] + "\n... [truncated]"
		}
		builder.WriteString(overview)
		builder.WriteString("\n")
		builder.WriteString("Mission:\n")
		builder.WriteString(fmt.Sprintf("- ID: %s\n", pack.Mission.ID))
		builder.WriteString(fmt.Sprintf("- Title: %s\n", pack.Mission.Title))
		builder.WriteString(fmt.Sprintf("- Status: %s\n", pack.Mission.Status))
		builder.WriteString(fmt.Sprintf("- Progress: %.0f%%\n", pack.Mission.ProgressPercent))
	} else {
		builder.WriteString("Mission:\n")
		builder.WriteString(fmt.Sprintf("- ID: %s\n", pack.Mission.ID))
		builder.WriteString(fmt.Sprintf("- Title: %s\n", pack.Mission.Title))
		builder.WriteString(fmt.Sprintf("- Type: %s\n", pack.Mission.MissionType))
		builder.WriteString(fmt.Sprintf("- Goal: %s\n", pack.Mission.Goal))
		builder.WriteString(fmt.Sprintf("- Scope: %s\n", pack.Mission.Scope))
		builder.WriteString(fmt.Sprintf("- Authority: %s\n", pack.Mission.AuthorityLevel))
		builder.WriteString(fmt.Sprintf("- Status: %s\n", pack.Mission.Status))
		builder.WriteString(fmt.Sprintf("- Progress: %.0f%%\n", pack.Mission.ProgressPercent))
	}

	builder.WriteString("Thread:\n")
	builder.WriteString(fmt.Sprintf("- Title: %s\n", pack.Thread.Title))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", pack.Thread.Summary))
	builder.WriteString(fmt.Sprintf("- Context: %s\n", pack.Thread.Context))

	if pack.LatestSummary != nil {
		builder.WriteString("Latest Summary:\n")
		builder.WriteString(pack.LatestSummary.SummaryText)
		builder.WriteString("\n")
	} else {
		builder.WriteString("Latest Summary:\n- none yet\n")
	}

	if len(pack.ChildRollups) > 0 {
		builder.WriteString("Child Rollups:\n")
		for _, rollup := range pack.ChildRollups {
			summary := rollup.LatestSummary
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			builder.WriteString(fmt.Sprintf("- Child Mission: %s | Status: %s | Progress: %.0f%% | Health: %s | Blocker: %s | Overdue: %s | Execution: %s | Summary: %s\n",
				rollup.ChildMissionID,
				rollup.Status,
				rollup.ProgressPercent,
				rollup.Health,
				emptyFallback(rollup.CurrentBlocker, "none"),
				formatOverdueFlags(rollup.OverdueFlags),
				formatExecutionSummary(rollup.ExecutionSummary),
				summary,
			))
		}
	} else {
		builder.WriteString("Child Rollups:\n- none yet\n")
	}

	if len(pack.DueTodos) > 0 {
		builder.WriteString("Due Todos:\n")
		for _, todo := range pack.DueTodos {
			builder.WriteString(fmt.Sprintf("- %s | Status: %s | Priority: %s | Owner: %s | Due: %s\n",
				todo.Title,
				todo.Status,
				todo.Priority,
				todo.OwnerAgentID,
				formatDueAt(todo.DueAt),
			))
		}
	} else {
		builder.WriteString("Due Todos:\n- none due\n")
	}

	if len(pack.DueTimers) > 0 {
		builder.WriteString("Due Timers:\n")
		for _, timer := range pack.DueTimers {
			builder.WriteString(fmt.Sprintf("- %s | Wake: %s | Set By: %s | Status: %s\n",
				timer.ActionType,
				timer.WakeAt.UTC().Format(time.RFC3339),
				timer.SetByAgentID,
				timer.Status,
			))
		}
	} else {
		builder.WriteString("Due Timers:\n- none due\n")
	}

	if len(pack.AttachmentContents) > 0 {
		totalTokens := 0
		for _, ac := range pack.AttachmentContents {
			totalTokens += ac.Tokens
		}
		builder.WriteString(fmt.Sprintf("Project Attachments (%d files, ~%d tokens):\n", len(pack.AttachmentContents), totalTokens))
		for _, ac := range pack.AttachmentContents {
			truncNote := ""
			if ac.Truncated {
				truncNote = " [truncated at token budget]"
			}
			builder.WriteString(fmt.Sprintf("--- %s (%s, ~%d tokens)%s ---\n", ac.Filename, ac.Category, ac.Tokens, truncNote))
			builder.WriteString(ac.Content)
			if len(ac.Content) > 0 && ac.Content[len(ac.Content)-1] != '\n' {
				builder.WriteString("\n")
			}
			builder.WriteString("--- end ---\n")
		}
		// Note image attachments that are not injected as text.
		imageCount := 0
		for _, att := range pack.Attachments {
			if att.FileCategory == "image" {
				imageCount++
			}
		}
		if imageCount > 0 {
			builder.WriteString(fmt.Sprintf("Image Attachments: %d (included via multimodal input when supported)\n", imageCount))
		}
	} else if len(pack.Attachments) > 0 {
		builder.WriteString(fmt.Sprintf("Project Attachments: %d registered (no text-injectable files)\n", len(pack.Attachments)))
	} else {
		builder.WriteString("Project Attachments: none\n")
	}

	builder.WriteString(fmt.Sprintf("Recent Messages Window: %d\n", len(pack.RecentMessages)))

	if projectLocation != "" {
		builder.WriteString(fmt.Sprintf("Project Directory: %s\n", projectLocation))
		builder.WriteString("You can write files to this project using write_file actions in your response.\n")
	}

	return builder.String()
}

// executeCEOFileActions processes write_file actions from a CEO LLM response.
// Returns the number of files successfully written.
func (s *Service) executeCEOFileActions(actions []ceoActionItem, projectLocation string, thread threads.Thread) int {
	if projectLocation == "" || len(actions) == 0 {
		return 0
	}
	written := 0
	for _, action := range actions {
		if action.Type != "write_file" {
			continue
		}
		var p agents.WriteFilePayload
		if err := json.Unmarshal(action.Payload, &p); err != nil {
			logger.Error("CEO write_file: failed to decode payload", "error", err)
			continue
		}
		filePath := p.GetFilePath()
		content := p.GetContent()
		if filePath == "" || content == "" {
			continue
		}

		// Sanitize path: prevent traversal out of project directory.
		cleanPath := filepath.Clean(filePath)
		if filepath.IsAbs(cleanPath) {
			cleanPath = strings.TrimPrefix(cleanPath, "/")
		}
		// Strip project path prefix if the model includes it.
		relProject := strings.TrimPrefix(filepath.Clean(projectLocation), "/")
		if relProject != "" && strings.HasPrefix(cleanPath, relProject+"/") {
			cleanPath = strings.TrimPrefix(cleanPath, relProject+"/")
		}
		if strings.Contains(cleanPath, "..") {
			logger.Warn("CEO write_file rejected: path traversal", "filePath", filePath)
			continue
		}
		absPath := filepath.Join(projectLocation, cleanPath)
		absProjectLocation, _ := filepath.Abs(projectLocation)
		absResolved, _ := filepath.Abs(absPath)
		if !strings.HasPrefix(absResolved, absProjectLocation+string(filepath.Separator)) && absResolved != absProjectLocation {
			logger.Warn("CEO write_file rejected: escapes project dir", "filePath", filePath)
			continue
		}

		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			logger.Error("CEO write_file mkdir failed", "dir", dir, "error", err)
			continue
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			logger.Error("CEO write_file failed", "path", absPath, "error", err)
			continue
		}
		relPath := filepath.ToSlash(cleanPath)
		logger.Info("CEO wrote file", "filePath", relPath, "absPath", absPath, "size", len(content))

		// Record in attachments store if available.
		if s.attachmentStore != nil {
			_ = s.attachmentStore.Create(attachments.Attachment{
				ID:           fmt.Sprintf("ceo-file-%d", time.Now().UnixNano()),
				MissionID:    thread.MissionID,
				ThreadID:     thread.ID,
				Filename:     filepath.Base(filePath),
				SizeBytes:    int64(len(content)),
				RelativePath: relPath,
				AbsolutePath: absPath,
				FileCategory: attachments.ClassifyFile(filepath.Base(filePath)),
				Status:       attachments.StatusActive,
				CreatedAt:    time.Now().UTC(),
			})
		}
		written++
	}
	return written
}

// generateContextDocs creates PROJECT_OVERVIEW, PROJECT_CONFIG, and initial
// PROJECT_STATE documents from the conversation history. Called once when the
// CEO transitions to a planning mode and docs do not yet exist.
func (s *Service) generateContextDocs(ctx context.Context, threadID, projectLocation, model string) {
	messages, err := s.threadStore.ListMessages(threadID)
	if err != nil {
		logger.Error("generateContextDocs: failed to list messages", "error", err, "threadID", threadID)
		return
	}
	if len(messages) == 0 {
		return
	}

	transcript := formatConversationTranscript(messages)

	// Generate PROJECT_OVERVIEW.md
	overviewJSON, err := s.llm.Generate(ctx, model, contextdocs.PromptSummarizeOverview, transcript)
	if err != nil {
		logger.Error("generateContextDocs: overview LLM call failed", "error", err)
	} else if overview, parseErr := parseOverviewJSON(overviewJSON); parseErr != nil {
		logger.Error("generateContextDocs: overview JSON parse failed", "error", parseErr, "raw", truncateForLog(overviewJSON, 200))
	} else {
		content := contextdocs.GenerateProjectOverview(overview)
		if writeErr := contextdocs.WriteDocument(projectLocation, contextdocs.DocProjectOverview, "", content); writeErr != nil {
			logger.Error("generateContextDocs: failed to write overview", "error", writeErr)
		}
	}

	// Generate PROJECT_CONFIG.md
	configJSON, err := s.llm.Generate(ctx, model, contextdocs.PromptSummarizeConfig, transcript)
	if err != nil {
		logger.Error("generateContextDocs: config LLM call failed", "error", err)
	} else if config, parseErr := parseConfigJSON(configJSON); parseErr != nil {
		logger.Error("generateContextDocs: config JSON parse failed", "error", parseErr, "raw", truncateForLog(configJSON, 200))
	} else {
		content := contextdocs.GenerateProjectConfig(config)
		if writeErr := contextdocs.WriteDocument(projectLocation, contextdocs.DocProjectConfig, "", content); writeErr != nil {
			logger.Error("generateContextDocs: failed to write config", "error", writeErr)
		}
	}

	// Generate initial PROJECT_STATE.md
	stateContent := contextdocs.GenerateProjectState(contextdocs.ProjectStateInput{
		CompletedFeatures:  "None yet",
		InProgressFeatures: "Project planning in progress",
		KnownIssues:        "None",
		FileTreeSummary:    "Project has not been scaffolded yet",
	})
	if writeErr := contextdocs.WriteDocument(projectLocation, contextdocs.DocProjectState, "", stateContent); writeErr != nil {
		logger.Error("generateContextDocs: failed to write state", "error", writeErr)
	}

	// Trigger re-index so the knowledge service picks up the new docs.
	if s.onFilesChanged != nil {
		s.onFilesChanged(ctx, projectLocation)
	}

	logger.Info("generateContextDocs: context documents created", "projectLocation", projectLocation, "threadID", threadID)
}

// formatConversationTranscript converts thread messages into a readable
// transcript suitable for sending to the LLM as a user prompt.
func formatConversationTranscript(messages []threads.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Role == threads.RoleSystem {
			continue
		}
		label := "User"
		if msg.Role == threads.RoleAssistant {
			label = "CEO"
		}
		content := msg.Content
		// Cap individual messages to avoid blowing up context for very long threads.
		if len(content) > 3000 {
			content = content[:3000] + "... [truncated]"
		}
		b.WriteString(fmt.Sprintf("[%s]: %s\n\n", label, content))
	}
	return b.String()
}

// parseOverviewJSON parses the LLM response into a ProjectOverviewInput.
func parseOverviewJSON(raw string) (contextdocs.ProjectOverviewInput, error) {
	cleaned := stripJSONFences(raw)
	var result struct {
		Vision          string `json:"vision"`
		TargetUser      string `json:"targetUser"`
		KeyFeatures     string `json:"keyFeatures"`
		TechStack       string `json:"techStack"`
		Constraints     string `json:"constraints"`
		SuccessCriteria string `json:"successCriteria"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return contextdocs.ProjectOverviewInput{}, fmt.Errorf("parse overview: %w", err)
	}
	return contextdocs.ProjectOverviewInput{
		Vision:          result.Vision,
		TargetUser:      result.TargetUser,
		KeyFeatures:     result.KeyFeatures,
		TechStack:       result.TechStack,
		Constraints:     result.Constraints,
		SuccessCriteria: result.SuccessCriteria,
	}, nil
}

// parseConfigJSON parses the LLM response into a ProjectConfigInput.
func parseConfigJSON(raw string) (contextdocs.ProjectConfigInput, error) {
	cleaned := stripJSONFences(raw)
	var result struct {
		ProjectDirectory  string `json:"projectDirectory"`
		LanguageFramework string `json:"languageFramework"`
		FileConventions   string `json:"fileConventions"`
		BuildAndRun       string `json:"buildAndRun"`
		Dependencies      string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return contextdocs.ProjectConfigInput{}, fmt.Errorf("parse config: %w", err)
	}
	return contextdocs.ProjectConfigInput{
		ProjectDirectory:  result.ProjectDirectory,
		LanguageFramework: result.LanguageFramework,
		FileConventions:   result.FileConventions,
		BuildAndRun:       result.BuildAndRun,
		Dependencies:      result.Dependencies,
	}, nil
}

// stripJSONFences removes markdown code fences that LLMs sometimes wrap around JSON.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

// truncateForLog truncates a string for safe inclusion in log messages.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *Service) resolveMode(ctx context.Context, request Request) (Mode, error) {
	if mode, ok, err := modeFromContext(request.Context); err != nil {
		return "", err
	} else if ok {
		return mode, nil
	}

	selectionPrompt := fmt.Sprintf(
		"Choose the best CEO mode for this request. Allowed modes: %s. Return only the mode string.",
		strings.Join(modeStrings(AllowedModes()), ", "),
	)
	selection, err := s.llm.Generate(ctx, s.config.Model, selectionPrompt, request.Prompt)
	if err != nil {
		return "", err
	}
	mode := Mode(strings.TrimSpace(selection))
	if err := mode.Validate(); err != nil {
		return "", logValidationError("openai returned invalid CEO mode", err, "rawMode", selection)
	}
	return mode, nil
}

func modeFromContext(raw json.RawMessage) (Mode, bool, error) {
	envelope, err := contextFromRequest(raw)
	if err != nil {
		return "", false, err
	}
	if envelope.Mode == "" {
		return "", false, nil
	}
	if err := envelope.Mode.Validate(); err != nil {
		return "", false, err
	}
	return envelope.Mode, true, nil
}

func contextFromRequest(raw json.RawMessage) (contextEnvelope, error) {
	if len(raw) == 0 {
		return contextEnvelope{}, nil
	}

	var envelope contextEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return contextEnvelope{}, logValidationError("invalid CEO request context", fmt.Errorf("unmarshal context: %w", err))
	}
	return envelope, nil
}

func resolveMissionID(request Request, contextData contextEnvelope) (string, error) {
	if request.MissionID != "" && contextData.MissionID != "" && request.MissionID != contextData.MissionID {
		return "", logValidationError(
			"mission id mismatch between request envelope and context",
			fmt.Errorf("request missionId %q does not match context missionId %q", request.MissionID, contextData.MissionID),
			"requestMissionID", request.MissionID,
			"contextMissionID", contextData.MissionID,
		)
	}
	if request.MissionID != "" {
		return request.MissionID, nil
	}
	return contextData.MissionID, nil
}

func modeStrings(modes []Mode) []string {
	values := make([]string, 0, len(modes))
	for _, mode := range modes {
		values = append(values, string(mode))
	}
	return values
}

func defaultRatingPrompt() RatingPrompt {
	return RatingPrompt{
		Enabled:  true,
		Question: "How would you rate this response?",
		Scale:    []int{1, 2, 3, 4, 5},
	}
}

func (s *Service) modelForRequest(request Request) string {
	if request.Model != "" {
		return request.Model
	}
	return s.config.Model
}

func threadIDOrFallback(threadID string) string {
	if threadID != "" {
		return threadID
	}
	return "thread-pending"
}

func traceIDOrFallback(traceID string, threadID string) string {
	if traceID != "" {
		return traceID
	}
	if threadID != "" {
		return threadID
	}
	return "trace-pending"
}

func fallbackProgramID(threadID string) string {
	return "program-" + threadID
}

func fallbackProgramTitle(threadID string, customTitle string) string {
	if customTitle != "" {
		return customTitle
	}
	return "Program " + threadID
}

func fallbackMissionTitle(threadID string, customTitle string) string {
	if customTitle != "" {
		return customTitle
	}
	return "CEO mission " + threadID
}

func fallbackThreadTitle(threadID string, customTitle string) string {
	if customTitle != "" {
		return customTitle
	}
	return "CEO thread " + threadID
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatDueAt(dueAt *time.Time) string {
	if dueAt == nil {
		return "unscheduled"
	}
	return dueAt.UTC().Format(time.RFC3339)
}

func formatOverdueFlags(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "none"
	}
	var flags []string
	if err := json.Unmarshal(raw, &flags); err != nil || len(flags) == 0 {
		return "none"
	}
	return strings.Join(flags, ", ")
}

func formatExecutionSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "none"
	}
	var summary struct {
		TotalTodos      int        `json:"totalTodos"`
		OpenTodos       int        `json:"openTodos"`
		InProgressTodos int        `json:"inProgressTodos"`
		BlockedTodos    int        `json:"blockedTodos"`
		DoneTodos       int        `json:"doneTodos"`
		DueTodos        int        `json:"dueTodos"`
		ScheduledTimers int        `json:"scheduledTimers"`
		DueTimers       int        `json:"dueTimers"`
		NextTimerAt     *time.Time `json:"nextTimerAt,omitempty"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		return "none"
	}
	parts := []string{fmt.Sprintf("todos total=%d open=%d in_progress=%d blocked=%d done=%d due=%d", summary.TotalTodos, summary.OpenTodos, summary.InProgressTodos, summary.BlockedTodos, summary.DoneTodos, summary.DueTodos)}
	parts = append(parts, fmt.Sprintf("timers scheduled=%d due=%d", summary.ScheduledTimers, summary.DueTimers))
	if summary.NextTimerAt != nil {
		parts = append(parts, fmt.Sprintf("next_timer=%s", summary.NextTimerAt.UTC().Format(time.RFC3339)))
	}
	return strings.Join(parts, "; ")
}

func (s *Service) GenerateProjectName(ctx context.Context, prompt string) (string, error) {
	systemPrompt := "You are a naming assistant. Read the user's project description and output a descriptive title for the project, using a maximum of 20 words. Do NOT include ANY conversation, punctuation, quotes, context, or explanations. ONLY respond with the words themselves."
	title, err := s.llm.Generate(ctx, s.config.Model, systemPrompt, prompt)
	if err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	return title, nil
}

func (s *Service) ListRootThreads(ctx context.Context) ([]threads.Thread, error) {
	return s.threadStore.ListRootThreads()
}

func (s *Service) LoadProject(ctx context.Context, threadID string) ([]threads.Thread, map[string][]threads.Message, error) {
	rootThread, err := s.threadStore.GetThread(threadID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get root thread: %w", err)
	}
	missionThreads, err := s.threadStore.ListByRootMission(rootThread.MissionID)
	if err != nil {
		return nil, nil, err
	}
	msgsMap := make(map[string][]threads.Message)
	for _, t := range missionThreads {
		msgs, _ := s.threadStore.ListMessages(t.ID)
		msgsMap[t.ID] = msgs
	}
	return missionThreads, msgsMap, nil
}

func (s *Service) LoadProjectAgents(ctx context.Context, projectID string) ([]agents.AgentNode, error) {
	// Ensure the CEO agent node exists even after an in-memory store reset (server restart).
	s.ensureCEOAgentNode(projectID, "")
	return s.nodeStore.ListByProject(projectID)
}

func (s *Service) RenameProject(ctx context.Context, threadID string, newName string) error {
	// First fetch the thread to ensure it exists and get the mission ID
	thread, err := s.threadStore.GetThread(threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// Update the thread title
	if err := s.threadStore.UpdateThreadTitle(threadID, newName); err != nil {
		return fmt.Errorf("failed to update thread title: %w", err)
	}

	// Wait, the project root is both a thread and a mission. We should update the mission too.
	if thread.MissionID != "" {
		mission, err := s.missionStore.GetMission(thread.MissionID)
		if err == nil {
			mission.Title = newName
			// Keep other fields intact
			_ = s.missionStore.UpdateMission(mission)
		}
	}

	// Append a system message indicating the rename
	sysMsg := threads.Message{
		ID:            "msg_" + threadID + "_rename_" + newName,
		ThreadID:      threadID,
		Role:          threads.RoleSystem,
		AuthorAgentID: "system",
		AuthorRole:    "system",
		MessageType:   "audit_event",
		Content:       fmt.Sprintf("Project/Mission renamed to: %s", newName),
		CreatedAt:     time.Now().UTC(),
	}
	_ = s.threadStore.AppendMessage(sysMsg)

	return nil
}

// RefinePrompt takes a raw user project description and returns a refined,
// structured version using a low-cost reasoning model.
func (s *Service) RefinePrompt(ctx context.Context, rawPrompt string, model string) (string, error) {
	if strings.TrimSpace(rawPrompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	systemPrompt := `You are an expert product consultant who helps clients articulate their project vision clearly.

The user has described a project idea. Your job is to refine their description into a clear, well-structured project brief that a CEO agent can act on effectively.

Rules:
- Preserve the user's core intent and all specific details they mentioned
- Add structure: break into clear sections if needed (Goal, Key Features, Target Users, Technical Preferences, Constraints)
- Clarify ambiguous points by making reasonable assumptions (mark them as assumptions)
- Remove filler words and tighten the language
- Do NOT add features or scope the user didn't mention or imply
- Do NOT use markdown headers or bullet formatting — write in clean flowing paragraphs with line breaks between sections
- Keep it concise but comprehensive
- Write in second person ("Your project will..." or "The system should...")

Output ONLY the refined project description. No preamble, no commentary.`

	refined, err := s.llm.Generate(ctx, model, systemPrompt, rawPrompt)
	if err != nil {
		return "", fmt.Errorf("refine prompt: %w", err)
	}
	return strings.TrimSpace(refined), nil
}

// ModelGuidance analyzes the project description and available models to
// recommend which model best suits the project type.
func (s *Service) ModelGuidance(ctx context.Context, projectDescription string, availableModels []string, model string) (string, error) {
	if strings.TrimSpace(projectDescription) == "" {
		return "", fmt.Errorf("project description is required")
	}
	if model == "" {
		model = s.config.Model
	}

	modelsText := strings.Join(availableModels, ", ")

	systemPrompt := `You are an expert AI model advisor. The user is about to start a new software project with an AI CEO agent that will plan and execute the project.

Your job: Based on the project description, recommend which LLM model should power the CEO agent.

Available models: ` + modelsText + `

Provide your guidance in this exact JSON format (no markdown, no code fences):
{
  "recommended": "model-name",
  "reasoning": "2-3 sentence explanation of why this model fits the project",
  "alternatives": [
    {"model": "model-name", "note": "when to prefer this instead"}
  ],
  "projectComplexity": "low|medium|high|very_high",
  "tips": ["brief tip about model choice for this project type"]
}

Model selection heuristics:
- Complex architecture, multi-service, or enterprise projects → strongest reasoning model (gpt-5.4, o3, o4-mini)
- Standard web apps, CRUD, or well-defined scope → balanced model (gpt-4.1, gpt-4o)
- Simple scripts, prototypes, or experiments → fast cheap model (gpt-4o-mini, gpt-4.1-mini)
- Projects requiring deep code generation → code-optimized models when available
- When in doubt, recommend the strongest model the user has access to

Output ONLY valid JSON. No explanation outside the JSON.`

	guidance, err := s.llm.Generate(ctx, model, systemPrompt, projectDescription)
	if err != nil {
		return "", fmt.Errorf("model guidance: %w", err)
	}
	return strings.TrimSpace(guidance), nil
}

// PauseProject pauses all active agent loops for the given project.
func (s *Service) PauseProject(ctx context.Context, projectID string) error {
	if s.loopManager != nil {
		s.loopManager.PauseByProject(projectID)
	}
	if s.nodeStore != nil {
		return s.nodeStore.SetProjectPaused(projectID, true)
	}
	return nil
}

// ResumeProject resumes all agent loops for the given project.
func (s *Service) ResumeProject(ctx context.Context, projectID string) error {
	if s.loopManager != nil {
		s.loopManager.ResumeByProject(projectID)
	}
	if s.nodeStore != nil {
		return s.nodeStore.SetProjectPaused(projectID, false)
	}
	return nil
}

// UpdateAgentModel updates the LLM model for a specific agent node (both in DB and running loop).
func (s *Service) UpdateAgentModel(ctx context.Context, agentID string, model string) error {
	if s.nodeStore != nil {
		if err := s.nodeStore.UpdateModel(agentID, model); err != nil {
			return err
		}
	}
	if s.loopManager != nil {
		s.loopManager.UpdateLoopModel(agentID, model)
	}
	return nil
}

// GetAgentStatuses returns the loop status for all agents in a project.
func (s *Service) GetAgentStatuses(ctx context.Context, projectID string) ([]agents.LoopStatus, error) {
	if s.loopManager == nil {
		return nil, nil
	}
	allStatuses := s.loopManager.GetAllLoopStatuses()
	var filtered []agents.LoopStatus
	for _, st := range allStatuses {
		if st.AgentID != "" {
			// Filter by project: check if the loop belongs to this project
			loop := s.loopManager
			_ = loop // we already have project in the config
		}
		filtered = append(filtered, st)
	}
	// Better approach: filter by project through the node store
	if s.nodeStore != nil {
		nodes, err := s.nodeStore.ListByProject(projectID)
		if err != nil {
			return nil, err
		}
		nodeIDs := make(map[string]bool)
		for _, n := range nodes {
			nodeIDs[n.ID] = true
		}
		var result []agents.LoopStatus
		for _, st := range allStatuses {
			if nodeIDs[st.AgentID] {
				result = append(result, st)
			}
		}
		return result, nil
	}
	return allStatuses, nil
}

// LLMProviderStatus describes the current LLM provider configuration and available models.
type LLMProviderStatus struct {
	Provider     string   `json:"provider"` // "openai" or "ollama"
	OllamaOnline bool     `json:"ollamaOnline"`
	CEOModel     string   `json:"ceoModel"`
	WorkerModel  string   `json:"workerModel"`
	OllamaModels []string `json:"ollamaModels,omitempty"`
}

// GetLLMProviderStatus returns current provider state: which provider is active, available models, etc.
func (s *Service) GetLLMProviderStatus(ctx context.Context) (LLMProviderStatus, error) {
	status := LLMProviderStatus{
		Provider:    "openai",
		CEOModel:    s.config.Model,
		WorkerModel: s.config.EffectiveChildModel(),
	}

	// Detect current provider from active agent models in DB.
	if s.nodeStore != nil {
		activeNodes, err := s.nodeStore.ListActive()
		if err == nil {
			for _, n := range activeNodes {
				if n.Role == agents.NodeRoleCEO && n.Model != "" {
					status.CEOModel = n.Model
				}
				if n.Role == agents.NodeRoleWorker && n.Model != "" {
					status.WorkerModel = n.Model
				}
			}
		}
	}

	// Determine provider from CEO model prefix.
	if strings.HasPrefix(status.CEOModel, "ollama/") {
		status.Provider = "ollama"
	}

	// Check Ollama availability and list models.
	if router, ok := s.llm.(*aiclients.RouterClient); ok {
		status.OllamaOnline = router.OllamaAvailable(ctx)
		if status.OllamaOnline {
			models, err := router.ListOllamaModels(ctx)
			if err == nil {
				status.OllamaModels = models
			}
		}
	}

	return status, nil
}

// SwitchLLMProvider bulk-switches all active agent nodes to the given provider.
// provider must be "openai" or "ollama".
// ceoModel and workerModel are the models to assign (e.g. "gpt-5.4" or "ollama/qwen3:4b").
func (s *Service) SwitchLLMProvider(ctx context.Context, provider string, ceoModel string, workerModel string) error {
	if provider != "openai" && provider != "ollama" {
		return fmt.Errorf("invalid provider %q: must be openai or ollama", provider)
	}

	if s.nodeStore == nil {
		return fmt.Errorf("node store not available")
	}

	activeNodes, err := s.nodeStore.ListActive()
	if err != nil {
		return fmt.Errorf("list active nodes: %w", err)
	}

	for _, node := range activeNodes {
		var targetModel string
		if node.Role == agents.NodeRoleCEO {
			targetModel = ceoModel
		} else {
			targetModel = workerModel
		}

		if err := s.nodeStore.UpdateModel(node.ID, targetModel); err != nil {
			logger.Error("failed to update agent model", "agentID", node.ID, "model", targetModel, "error", err)
			continue
		}
		if s.loopManager != nil {
			s.loopManager.UpdateLoopModel(node.ID, targetModel)
		}
	}

	logger.Info("LLM provider switched", "provider", provider, "ceoModel", ceoModel, "workerModel", workerModel, "agents", len(activeNodes))
	return nil
}

// ListOllamaModels returns the model names available from the local Ollama instance.
func (s *Service) ListOllamaModels(ctx context.Context) ([]string, error) {
	if router, ok := s.llm.(*aiclients.RouterClient); ok {
		return router.ListOllamaModels(ctx)
	}
	return nil, fmt.Errorf("ollama not configured")
}

// GetTokenBudgetConfig returns the current token budget settings.
func (s *Service) GetTokenBudgetConfig() aiclients.BudgetSnapshot {
	if router, ok := s.llm.(*aiclients.RouterClient); ok {
		return router.BudgetConfig().Snapshot()
	}
	return aiclients.BudgetSnapshot{Enabled: false}
}

// UpdateTokenBudgetConfig applies new token budget settings at runtime.
func (s *Service) UpdateTokenBudgetConfig(enabled *bool, threshold *int64, target *int64) aiclients.BudgetSnapshot {
	if router, ok := s.llm.(*aiclients.RouterClient); ok {
		bc := router.BudgetConfig()
		if enabled != nil {
			bc.SetEnabled(*enabled)
		}
		if threshold != nil && *threshold > 0 {
			bc.SetThreshold(*threshold)
		}
		if target != nil && *target > 0 {
			bc.SetTarget(*target)
		}
		logger.Info("token budget config updated", "enabled", bc.Enabled(), "threshold", bc.Threshold(), "target", bc.Target())
		return bc.Snapshot()
	}
	return aiclients.BudgetSnapshot{Enabled: false}
}

// GetWakeIntervalConfig returns the current wake interval settings.
func (s *Service) GetWakeIntervalConfig() agents.WakeIntervalSnapshot {
	return s.wakeConfig.Snapshot()
}

// UpdateWakeIntervalConfig applies new wake interval settings at runtime.
// All running agent loops will pick up the change on their next tick.
func (s *Service) UpdateWakeIntervalConfig(ceoSec *int64, managerSec *int64, workerSec *int64) agents.WakeIntervalSnapshot {
	if ceoSec != nil && *ceoSec > 0 {
		s.wakeConfig.SetCEOSeconds(*ceoSec)
	}
	if managerSec != nil && *managerSec > 0 {
		s.wakeConfig.SetManagerSeconds(*managerSec)
	}
	if workerSec != nil && *workerSec > 0 {
		s.wakeConfig.SetWorkerSeconds(*workerSec)
	}
	snap := s.wakeConfig.Snapshot()
	logger.Info("wake interval config updated",
		"ceoSeconds", snap.CEOSeconds,
		"managerSeconds", snap.ManagerSeconds,
		"workerSeconds", snap.WorkerSeconds,
	)
	return snap
}
