package ceo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"
)

type Mode string

const (
	ModeDiscovery     Mode = "discovery"
	ModeAlignment     Mode = "alignment"
	ModeHighLevelPlan Mode = "high_level_plan"
	ModeRoadmap       Mode = "roadmap"
	ModeExecutionPrep Mode = "execution_prep"
	ModeReview        Mode = "review"
)

var (
	modeRegistryMu sync.RWMutex
	modeRegistry   = map[Mode]struct{}{}
	logger         = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
)

func init() {
	if err := ConfigureModes(DefaultModes()); err != nil {
		logger.Error("failed to initialize CEO mode registry", "error", err)
	}
}

func DefaultModes() []Mode {
	return []Mode{
		ModeDiscovery,
		ModeAlignment,
		ModeHighLevelPlan,
		ModeRoadmap,
		ModeExecutionPrep,
		ModeReview,
	}
}

func SetLogger(customLogger *slog.Logger) {
	if customLogger == nil {
		return
	}
	logger = customLogger
}

func ConfigureModes(modes []Mode) error {
	registry, err := buildModeRegistry(modes)
	if err != nil {
		logger.Error("failed to configure CEO modes", "error", err, "modes", modes)
		return err
	}

	modeRegistryMu.Lock()
	modeRegistry = registry
	modeRegistryMu.Unlock()
	return nil
}

func RegisterModes(modes ...Mode) error {
	modeRegistryMu.RLock()
	currentModes := make([]Mode, 0, len(modeRegistry)+len(modes))
	for mode := range modeRegistry {
		currentModes = append(currentModes, mode)
	}
	modeRegistryMu.RUnlock()

	currentModes = append(currentModes, modes...)
	return ConfigureModes(currentModes)
}

func AllowedModes() []Mode {
	modeRegistryMu.RLock()
	defer modeRegistryMu.RUnlock()

	modes := make([]Mode, 0, len(modeRegistry))
	for mode := range modeRegistry {
		modes = append(modes, mode)
	}
	sort.Slice(modes, func(i, j int) bool {
		return modes[i] < modes[j]
	})
	return modes
}

func buildModeRegistry(modes []Mode) (map[Mode]struct{}, error) {
	if len(modes) == 0 {
		return nil, errors.New("at least one CEO mode must be configured")
	}

	registry := make(map[Mode]struct{}, len(modes))
	for _, mode := range modes {
		if mode == "" {
			return nil, errors.New("CEO mode cannot be empty")
		}
		registry[mode] = struct{}{}
	}
	return registry, nil
}

func logValidationError(message string, err error, attrs ...any) error {
	logger.Error(message, append([]any{"error", err}, attrs...)...)
	return err
}

type Request struct {
	Prompt           string          `json:"prompt"`
	Model            string          `json:"model,omitempty"`
	MissionID        string          `json:"missionId,omitempty"`
	Action           *ActionRequest  `json:"action,omitempty"`
	Context          json.RawMessage `json:"context,omitempty"`
	ThreadID         string          `json:"threadId,omitempty"`
	TraceID          string          `json:"traceId,omitempty"`
	KnowledgeSummary string          `json:"-"` // Server-injected codebase context; never from client JSON
}

type ActionType string

const (
	ActionCreateTodo    ActionType = "create_todo"
	ActionAssignTodo    ActionType = "assign_todo"
	ActionBlockTodo     ActionType = "block_todo"
	ActionCompleteTodo  ActionType = "complete_todo"
	ActionScheduleTimer ActionType = "schedule_timer"
	ActionCancelTimer   ActionType = "cancel_timer"
	ActionReindex       ActionType = "reindex"
)

type ActionRequest struct {
	Type    ActionType      `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type RatingPrompt struct {
	Enabled  bool   `json:"enabled"`
	Question string `json:"question,omitempty"`
	Scale    []int  `json:"scale,omitempty"`
}

type ResponseEnvelope struct {
	ResponseID   string          `json:"responseId"`
	ThreadID     string          `json:"threadId"`
	TraceID      string          `json:"traceId"`
	Mode         Mode            `json:"mode"`
	Payload      json.RawMessage `json:"payload"`
	RatingPrompt RatingPrompt    `json:"ratingPrompt"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type FeedbackSubmission struct {
	ThreadID   string    `json:"threadId"`
	ResponseID string    `json:"responseId"`
	TraceID    string    `json:"traceId,omitempty"`
	Rating     int       `json:"rating"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (m Mode) Validate() error {
	modeRegistryMu.RLock()
	_, ok := modeRegistry[m]
	modeRegistryMu.RUnlock()
	if ok {
		return nil
	}
	return logValidationError("invalid CEO mode", fmt.Errorf("invalid CEO mode %q", m), "mode", m, "allowedModes", AllowedModes())
}

func (r Request) Validate() error {
	if r.Action != nil {
		return r.Action.Validate()
	}
	if r.Prompt == "" {
		return logValidationError("invalid CEO request", errors.New("prompt is required"))
	}
	return nil
}

func (a ActionRequest) Validate() error {
	switch a.Type {
	case ActionCreateTodo, ActionAssignTodo, ActionBlockTodo, ActionCompleteTodo, ActionScheduleTimer, ActionCancelTimer:
		return nil
	case "":
		return logValidationError("invalid CEO action request", errors.New("action type is required"))
	default:
		return logValidationError("invalid CEO action request", fmt.Errorf("unsupported action type %q", a.Type), "actionType", a.Type)
	}
}

func (p RatingPrompt) Validate() error {
	if !p.Enabled {
		return nil
	}
	if p.Question == "" {
		return logValidationError("invalid CEO rating prompt", errors.New("rating question is required when rating prompt is enabled"))
	}
	if len(p.Scale) == 0 {
		return logValidationError("invalid CEO rating prompt", errors.New("rating scale is required when rating prompt is enabled"))
	}
	return nil
}

func (e ResponseEnvelope) Validate() error {
	if e.ResponseID == "" {
		return logValidationError("invalid CEO response envelope", errors.New("response id is required"))
	}
	if e.ThreadID == "" {
		return logValidationError("invalid CEO response envelope", errors.New("thread id is required"))
	}
	if e.TraceID == "" {
		return logValidationError("invalid CEO response envelope", errors.New("trace id is required"))
	}
	if err := e.Mode.Validate(); err != nil {
		return err
	}
	if len(e.Payload) == 0 {
		return logValidationError("invalid CEO response envelope", errors.New("payload is required"), "mode", e.Mode)
	}
	if err := e.RatingPrompt.Validate(); err != nil {
		return err
	}
	if e.CreatedAt.IsZero() {
		return logValidationError("invalid CEO response envelope", errors.New("created at is required"), "mode", e.Mode)
	}
	return nil
}

func (f FeedbackSubmission) Validate() error {
	if f.ThreadID == "" {
		return logValidationError("invalid CEO feedback submission", errors.New("thread id is required"))
	}
	if f.ResponseID == "" {
		return logValidationError("invalid CEO feedback submission", errors.New("response id is required"))
	}
	if f.Rating < 1 || f.Rating > 5 {
		return logValidationError("invalid CEO feedback submission", errors.New("rating must be between 1 and 5"), "rating", f.Rating)
	}
	if f.Rating < 4 && f.Reason == "" {
		return logValidationError("invalid CEO feedback submission", errors.New("reason is required when rating is below 4"), "rating", f.Rating)
	}
	if f.CreatedAt.IsZero() {
		return logValidationError("invalid CEO feedback submission", errors.New("created at is required"), "rating", f.Rating)
	}
	return nil
}

func NewResponseEnvelope(responseID, threadID, traceID string, mode Mode, payload any, ratingPrompt RatingPrompt) (ResponseEnvelope, error) {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		wrapped := fmt.Errorf("marshal payload: %w", err)
		logger.Error("failed to marshal CEO response payload", "error", wrapped, "mode", mode)
		return ResponseEnvelope{}, wrapped
	}

	envelope := ResponseEnvelope{
		ResponseID:   responseID,
		ThreadID:     threadID,
		TraceID:      traceID,
		Mode:         mode,
		Payload:      encodedPayload,
		RatingPrompt: ratingPrompt,
		CreatedAt:    time.Now().UTC(),
	}
	if err := envelope.Validate(); err != nil {
		return ResponseEnvelope{}, err
	}
	return envelope, nil
}
