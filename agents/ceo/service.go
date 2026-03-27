package ceo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/feedback"
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
	contextBuilder      *contextpacks.Builder
	executionRuntime    *execution.Runtime
	timerProcessor      *execution.TimerProcessor
	missionRuntime      *missions.Runtime
	missionStateRuntime *missionstate.Runtime
	delegateSelector    *delegateSelector
	backgroundCancel    context.CancelFunc
	cleanup             func()
}

type CompletionClient interface {
	Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error)
	GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error)
}

type contextEnvelope struct {
	Mode      Mode   `json:"mode,omitempty"`
	MissionID string `json:"missionId,omitempty"`
}

func NewService(config Config, llm CompletionClient, missionStore missions.Store, threadStore threads.Store, missionStateStore missionstate.Store, executionStore execution.Store, feedbackStore feedback.Store, attachmentStore attachments.Store) (*Service, error) {
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
	return &Service{
		config:              config,
		llm:                 llm,
		missionStore:        missionStore,
		threadStore:         observedThreadStore,
		missionStateStore:   missionStateStore,
		feedbackStore:       feedbackStore,
		attachmentStore:     attachmentStore,
		contextBuilder:      contextBuilder,
		executionRuntime:    executionRuntime,
		missionRuntime:      missionRuntime,
		missionStateRuntime: missionStateRuntime,
		delegateSelector:    delegateSelector,
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

	service, err := NewService(config, aiclients.NewOpenAIClient(aiclients.OpenAIConfig{
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
	}, logger), stores.Missions, stores.Threads, stores.MissionState, stores.Execution, stores.Feedback, stores.Attachments)
	if err != nil {
		stores.Close()
		return nil, err
	}
	service.cleanup = stores.Close
	if err := service.startTimerProcessor(); err != nil {
		service.Close()
		return nil, err
	}
	return service, nil
}

func (s *Service) Close() {
	if s == nil {
		return
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

func (s *Service) applyTriggeredTimerPolicy(_ context.Context, timer execution.Timer) (string, error) {
	payload := decodeObjectMap(timer.ActionPayload)
	switch timer.ActionType {
	case "escalate":
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
		return formatTimerEscalationMessage(timer, payload), nil
	case "follow_up", "status_check":
		return formatTimerTriggerMessage(timer, payload), nil
	default:
		return formatTimerTriggerMessage(timer, payload), nil
	}
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

	contextPack, err := s.contextBuilder.BuildMissionPack(thread.MissionID, threadID, contextpacks.BuildOptions{
		RecentMessagesLimit: ceoRecentMessagesLimit,
		IncludeChildRollups: true,
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
		conversation, err := s.buildConversation(mode, request.Prompt, contextPack)
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

	thread, err = s.threadStore.GetThread(threadID)
	if err != nil {
		return threads.Thread{}, logValidationError("failed to fetch bootstrapped thread", err, "threadID", threadID)
	}
	return thread, nil
}

func (s *Service) buildConversation(mode Mode, prompt string, pack contextpacks.ContextPack) ([]threads.Message, error) {
	systemPrompt, err := loadSystemPrompt(mode)
	if err != nil {
		return nil, err
	}

	conversation := make([]threads.Message, 0, len(pack.RecentMessages)+3)
	conversation = append(conversation,
		threads.Message{Role: threads.RoleSystem, Content: systemPrompt},
		threads.Message{Role: threads.RoleSystem, Content: formatContextPack(pack)},
	)
	conversation = append(conversation, pack.RecentMessages...)
	userMsg := threads.Message{Role: threads.RoleUser, Content: prompt, Mode: string(mode)}
	if len(pack.ImageDataURLs) > 0 {
		userMsg.ImageDataURLs = pack.ImageDataURLs
	}
	conversation = append(conversation, userMsg)
	return conversation, nil
}

func formatContextPack(pack contextpacks.ContextPack) string {
	var builder strings.Builder
	builder.WriteString("Use this durable mission context before answering. Prefer the mission state and child rollups over replaying older transcript details.\n\n")
	builder.WriteString("Mission:\n")
	builder.WriteString(fmt.Sprintf("- ID: %s\n", pack.Mission.ID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", pack.Mission.Title))
	builder.WriteString(fmt.Sprintf("- Type: %s\n", pack.Mission.MissionType))
	builder.WriteString(fmt.Sprintf("- Goal: %s\n", pack.Mission.Goal))
	builder.WriteString(fmt.Sprintf("- Scope: %s\n", pack.Mission.Scope))
	builder.WriteString(fmt.Sprintf("- Authority: %s\n", pack.Mission.AuthorityLevel))
	builder.WriteString(fmt.Sprintf("- Status: %s\n", pack.Mission.Status))
	builder.WriteString(fmt.Sprintf("- Progress: %.0f%%\n", pack.Mission.ProgressPercent))
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
			builder.WriteString(fmt.Sprintf("- Child Mission: %s | Status: %s | Progress: %.0f%% | Health: %s | Blocker: %s | Overdue: %s | Execution: %s | Summary: %s\n",
				rollup.ChildMissionID,
				rollup.Status,
				rollup.ProgressPercent,
				rollup.Health,
				emptyFallback(rollup.CurrentBlocker, "none"),
				formatOverdueFlags(rollup.OverdueFlags),
				formatExecutionSummary(rollup.ExecutionSummary),
				rollup.LatestSummary,
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
	return builder.String()
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

func formatTimerTriggerMessage(timer execution.Timer, payload map[string]any) string {
	message := strings.TrimSpace(stringValue(payload, "message"))
	if message != "" {
		return fmt.Sprintf("Timer triggered: %s for mission %s at %s. %s", timer.ActionType, timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339), trimSentence(message)+".")
	}
	reason := strings.TrimSpace(stringValue(payload, "reason"))
	if reason != "" {
		return fmt.Sprintf("Timer triggered: %s for mission %s at %s. Reason: %s.", timer.ActionType, timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339), trimSentence(reason))
	}
	return fmt.Sprintf("Timer triggered: %s for mission %s at %s.", timer.ActionType, timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339))
}

func formatTimerEscalationMessage(timer execution.Timer, payload map[string]any) string {
	reason := strings.TrimSpace(stringValue(payload, "reason"))
	if reason != "" {
		return fmt.Sprintf("Timer escalation triggered for mission %s at %s. Reason: %s.", timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339), trimSentence(reason))
	}
	message := strings.TrimSpace(stringValue(payload, "message"))
	if message != "" {
		return fmt.Sprintf("Timer escalation triggered for mission %s at %s. %s", timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339), trimSentence(message)+".")
	}
	return fmt.Sprintf("Timer escalation triggered for mission %s at %s.", timer.MissionID, timer.WakeAt.UTC().Format(time.RFC3339))
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

func trimSentence(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ".")
	return trimmed
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
	missionThreads, err := s.threadStore.ListByMission(rootThread.MissionID)
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
