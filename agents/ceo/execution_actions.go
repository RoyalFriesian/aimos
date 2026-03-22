package ceo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

type createTodoActionPayload struct {
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	OwnerAgentID  string            `json:"ownerAgentId,omitempty"`
	Priority      missions.Priority `json:"priority,omitempty"`
	DueAt         *time.Time        `json:"dueAt,omitempty"`
	DependsOn     []string          `json:"dependsOn,omitempty"`
	ArtifactPaths []string          `json:"artifactPaths,omitempty"`
}

type assignTodoActionPayload struct {
	TodoID       string `json:"todoId"`
	OwnerAgentID string `json:"ownerAgentId"`
}

type todoStateActionPayload struct {
	TodoID string `json:"todoId"`
}

type scheduleTimerActionPayload struct {
	SetByAgentID  string          `json:"setByAgentId,omitempty"`
	WakeAt        time.Time       `json:"wakeAt"`
	ActionType    string          `json:"actionType"`
	ActionPayload json.RawMessage `json:"actionPayload,omitempty"`
}

type cancelTimerActionPayload struct {
	TimerID string `json:"timerId"`
}

func (s *Service) respondToExecutionAction(request Request, thread threads.Thread) (ResponseEnvelope, error) {
	if err := s.threadStore.UpdateThreadMode(thread.ID, string(ModeExecutionPrep)); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to update thread mode", err, "threadID", thread.ID, "mode", ModeExecutionPrep)
	}
	requestTime := time.Now().UTC()
	requestMessageID := fmt.Sprintf("client-action-%d", requestTime.UnixNano())
	if err := s.threadStore.AppendMessage(threads.Message{
		ID:            requestMessageID,
		ThreadID:      thread.ID,
		Role:          threads.RoleUser,
		AuthorAgentID: "user",
		AuthorRole:    "client",
		MessageType:   "client_action_request",
		Content:       formatActionRequestMessage(*request.Action),
		ContentJSON:   actionRequestPayload(*request.Action),
		Mode:          string(ModeExecutionPrep),
		CreatedAt:     requestTime,
	}); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to append action request message", err, "threadID", thread.ID)
	}

	payload, assistantMessage, err := s.executeMissionAction(*request.Action, thread)
	if err != nil {
		return ResponseEnvelope{}, err
	}

	assistantTime := time.Now().UTC()
	responseID := fmt.Sprintf("assistant-action-%d", assistantTime.UnixNano())
	responseEnvelope, err := NewResponseEnvelope(
		responseID,
		thread.ID,
		traceIDOrFallback(request.TraceID, thread.ID),
		ModeExecutionPrep,
		payload,
		defaultRatingPrompt(),
	)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	responseEnvelope.CreatedAt = assistantTime
	if err := s.threadStore.AppendMessage(threads.Message{
		ID:               responseID,
		ThreadID:         thread.ID,
		Role:             threads.RoleAssistant,
		AuthorAgentID:    "ceo",
		AuthorRole:       "ceo",
		MessageType:      "ceo_action_result",
		Content:          assistantMessage,
		ContentJSON:      responseEnvelope.Payload,
		Mode:             string(ModeExecutionPrep),
		ReplyToMessageID: requestMessageID,
		CreatedAt:        assistantTime,
	}); err != nil {
		return ResponseEnvelope{}, logValidationError("failed to append action result message", err, "threadID", thread.ID)
	}

	return responseEnvelope, nil
}

func formatActionRequestMessage(action ActionRequest) string {
	return fmt.Sprintf("Requested CEO action %q.", action.Type)
}

func actionRequestPayload(action ActionRequest) json.RawMessage {
	payload, err := json.Marshal(map[string]any{
		"type":    action.Type,
		"payload": decodeObjectMap(action.Payload),
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func (s *Service) executeMissionAction(action ActionRequest, thread threads.Thread) (map[string]any, string, error) {
	switch action.Type {
	case ActionCreateTodo:
		var input createTodoActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		ownerAgentID := emptyFallback(input.OwnerAgentID, thread.OwnerAgentID)
		if strings.TrimSpace(ownerAgentID) == "" {
			ownerAgentID = "ceo"
		}
		todo, err := s.executionRuntime.CreateTodo(execution.CreateTodoInput{
			MissionID:     thread.MissionID,
			ThreadID:      thread.ID,
			Title:         input.Title,
			Description:   input.Description,
			OwnerAgentID:  ownerAgentID,
			Priority:      input.Priority,
			DueAt:         input.DueAt,
			DependsOn:     input.DependsOn,
			ArtifactPaths: input.ArtifactPaths,
		})
		if err != nil {
			return nil, "", logValidationError("failed to create mission todo", err, "missionID", thread.MissionID, "threadID", thread.ID)
		}
		message := fmt.Sprintf("Created todo %q for mission %s and assigned it to %s.", todo.Title, thread.MissionID, todo.OwnerAgentID)
		return executionActionPayload(action.Type, thread, message, map[string]any{"todo": todoToPayload(todo)}), message, nil
	case ActionAssignTodo:
		var input assignTodoActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		if err := s.ensureTodoMissionScope(input.TodoID, thread.MissionID); err != nil {
			return nil, "", err
		}
		todo, err := s.executionRuntime.AssignTodo(input.TodoID, input.OwnerAgentID)
		if err != nil {
			return nil, "", logValidationError("failed to assign mission todo", err, "missionID", thread.MissionID, "todoID", input.TodoID)
		}
		message := fmt.Sprintf("Assigned todo %q to %s for mission %s.", todo.Title, todo.OwnerAgentID, thread.MissionID)
		return executionActionPayload(action.Type, thread, message, map[string]any{"todo": todoToPayload(todo)}), message, nil
	case ActionBlockTodo:
		var input todoStateActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		if err := s.ensureTodoMissionScope(input.TodoID, thread.MissionID); err != nil {
			return nil, "", err
		}
		todo, err := s.executionRuntime.BlockTodo(input.TodoID)
		if err != nil {
			return nil, "", logValidationError("failed to block mission todo", err, "missionID", thread.MissionID, "todoID", input.TodoID)
		}
		message := fmt.Sprintf("Marked todo %q as blocked for mission %s.", todo.Title, thread.MissionID)
		return executionActionPayload(action.Type, thread, message, map[string]any{"todo": todoToPayload(todo)}), message, nil
	case ActionCompleteTodo:
		var input todoStateActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		if err := s.ensureTodoMissionScope(input.TodoID, thread.MissionID); err != nil {
			return nil, "", err
		}
		todo, err := s.executionRuntime.CompleteTodo(input.TodoID)
		if err != nil {
			return nil, "", logValidationError("failed to complete mission todo", err, "missionID", thread.MissionID, "todoID", input.TodoID)
		}
		message := fmt.Sprintf("Marked todo %q as done for mission %s.", todo.Title, thread.MissionID)
		return executionActionPayload(action.Type, thread, message, map[string]any{"todo": todoToPayload(todo)}), message, nil
	case ActionScheduleTimer:
		var input scheduleTimerActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		setByAgentID := emptyFallback(input.SetByAgentID, thread.OwnerAgentID)
		if strings.TrimSpace(setByAgentID) == "" {
			setByAgentID = "ceo"
		}
		timer, err := s.executionRuntime.ScheduleTimer(execution.ScheduleTimerInput{
			MissionID:     thread.MissionID,
			ThreadID:      thread.ID,
			SetByAgentID:  setByAgentID,
			WakeAt:        input.WakeAt,
			ActionType:    input.ActionType,
			ActionPayload: input.ActionPayload,
		})
		if err != nil {
			return nil, "", logValidationError("failed to schedule mission timer", err, "missionID", thread.MissionID, "threadID", thread.ID)
		}
		message := fmt.Sprintf("Scheduled timer %q for mission %s at %s.", timer.ActionType, thread.MissionID, timer.WakeAt.Format(time.RFC3339))
		return executionActionPayload(action.Type, thread, message, map[string]any{"timer": timerToPayload(timer)}), message, nil
	case ActionCancelTimer:
		var input cancelTimerActionPayload
		if err := decodeActionPayload(action.Payload, &input); err != nil {
			return nil, "", err
		}
		if err := s.ensureTimerMissionScope(input.TimerID, thread.MissionID); err != nil {
			return nil, "", err
		}
		timer, err := s.executionRuntime.CancelTimer(input.TimerID)
		if err != nil {
			return nil, "", logValidationError("failed to cancel mission timer", err, "missionID", thread.MissionID, "timerID", input.TimerID)
		}
		message := fmt.Sprintf("Cancelled timer %q for mission %s.", timer.ActionType, thread.MissionID)
		return executionActionPayload(action.Type, thread, message, map[string]any{"timer": timerToPayload(timer)}), message, nil
	default:
		return nil, "", logValidationError("unsupported CEO execution action", fmt.Errorf("unsupported action type %q", action.Type), "actionType", action.Type)
	}
}

func executionActionPayload(actionType ActionType, thread threads.Thread, message string, result map[string]any) map[string]any {
	payload := map[string]any{
		"message":   message,
		"mode":      ModeExecutionPrep,
		"action":    map[string]any{"type": actionType},
		"missionId": thread.MissionID,
		"threadId":  thread.ID,
	}
	for key, value := range result {
		payload[key] = value
	}
	return payload
}

func (s *Service) ensureTodoMissionScope(todoID string, missionID string) error {
	todo, err := s.executionRuntime.Store().GetTodo(todoID)
	if err != nil {
		return logValidationError("failed to load mission todo", err, "missionID", missionID, "todoID", todoID)
	}
	if todo.MissionID != missionID {
		return logValidationError("todo mission scope mismatch", fmt.Errorf("todo %q belongs to mission %q, not %q", todoID, todo.MissionID, missionID), "missionID", missionID, "todoID", todoID)
	}
	return nil
}

func (s *Service) ensureTimerMissionScope(timerID string, missionID string) error {
	timer, err := s.executionRuntime.Store().GetTimer(timerID)
	if err != nil {
		return logValidationError("failed to load mission timer", err, "missionID", missionID, "timerID", timerID)
	}
	if timer.MissionID != missionID {
		return logValidationError("timer mission scope mismatch", fmt.Errorf("timer %q belongs to mission %q, not %q", timerID, timer.MissionID, missionID), "missionID", missionID, "timerID", timerID)
	}
	return nil
}

func decodeActionPayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return logValidationError("invalid CEO action payload", fmt.Errorf("unmarshal action payload: %w", err))
	}
	return nil
}

func todoToPayload(todo execution.Todo) map[string]any {
	return map[string]any{
		"id":            todo.ID,
		"missionId":     todo.MissionID,
		"threadId":      todo.ThreadID,
		"title":         todo.Title,
		"description":   todo.Description,
		"ownerAgentId":  todo.OwnerAgentID,
		"status":        todo.Status,
		"priority":      todo.Priority,
		"dueAt":         todo.DueAt,
		"dependsOn":     decodeStringArray(todo.DependsOn),
		"artifactPaths": decodeStringArray(todo.ArtifactPaths),
		"createdAt":     todo.CreatedAt,
		"updatedAt":     todo.UpdatedAt,
	}
}

func timerToPayload(timer execution.Timer) map[string]any {
	return map[string]any{
		"id":            timer.ID,
		"missionId":     timer.MissionID,
		"threadId":      timer.ThreadID,
		"setByAgentId":  timer.SetByAgentID,
		"wakeAt":        timer.WakeAt,
		"actionType":    timer.ActionType,
		"actionPayload": decodeObjectMap(timer.ActionPayload),
		"status":        timer.Status,
		"createdAt":     timer.CreatedAt,
		"triggeredAt":   timer.TriggeredAt,
	}
}

func decodeStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	return values
}
