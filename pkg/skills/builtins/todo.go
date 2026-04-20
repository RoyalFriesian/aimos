package builtins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/skills"
)

// HandleCompleteTodo marks a todo as done.
func HandleCompleteTodo(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	id, err := parseTodoID(args)
	if err != nil {
		return "", err
	}
	if env.ExecutionRuntime == nil {
		return "", fmt.Errorf("execution runtime not available")
	}
	todo, err := env.ExecutionRuntime.CompleteTodo(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Todo %q completed (status=%s)", todo.Title, todo.Status), nil
}

// HandleBlockTodo marks a todo as blocked with an optional reason.
func HandleBlockTodo(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		TodoID string `json:"todo_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse block_todo args: %w", err)
	}
	id := p.TodoID
	if id == "" {
		return "", fmt.Errorf("todo_id is required")
	}
	if env.ExecutionRuntime == nil {
		return "", fmt.Errorf("execution runtime not available")
	}
	todo, err := env.ExecutionRuntime.BlockTodo(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Todo %q blocked (reason: %s)", todo.Title, p.Reason), nil
}

// HandleStartTodo marks a todo as in-progress and optionally switches thread.
func HandleStartTodo(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		TodoID   string `json:"todo_id"`
		ThreadID string `json:"thread_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse start_todo args: %w", err)
	}
	if p.TodoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}
	if env.ExecutionRuntime == nil {
		return "", fmt.Errorf("execution runtime not available")
	}
	todo, err := env.ExecutionRuntime.StartTodo(p.TodoID)
	if err != nil {
		return "", err
	}

	// Optionally switch thread context.
	threadID := p.ThreadID
	if threadID == "" {
		threadID = todo.ThreadID
	}
	if threadID != "" && env.OnThreadSwitch != nil {
		env.OnThreadSwitch(threadID, todo.ID)
	}

	return fmt.Sprintf("Todo %q started (thread %s)", todo.Title, threadID), nil
}

// HandleCreateTodo creates a new mission todo.
func HandleCreateTodo(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse create_todo args: %w", err)
	}
	if p.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	if env.ExecutionRuntime == nil {
		return "", fmt.Errorf("execution runtime not available")
	}
	prio := missions.PriorityMedium
	switch p.Priority {
	case "critical":
		prio = missions.PriorityCritical
	case "high":
		prio = missions.PriorityHigh
	case "low":
		prio = missions.PriorityLow
	}

	todo, err := env.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
		MissionID:    env.MissionID,
		ThreadID:     env.ThreadID,
		Title:        p.Title,
		Description:  p.Description,
		OwnerAgentID: env.AgentID,
		Priority:     prio,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Todo %q created (id=%s)", todo.Title, todo.ID), nil
}

func parseTodoID(args json.RawMessage) (string, error) {
	var p struct {
		TodoID string `json:"todo_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse todo args: %w", err)
	}
	if p.TodoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}
	return p.TodoID, nil
}

var completeTodoSkill = &skills.Skill{
	Name:        "complete_todo",
	Description: "Mark a todo item as done.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todo_id": map[string]any{"type": "string", "description": "ID of the todo to complete."},
		},
		"required":             []any{"todo_id"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleCompleteTodo,
}

var blockTodoSkill = &skills.Skill{
	Name:        "block_todo",
	Description: "Mark a todo item as blocked with an optional reason.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todo_id": map[string]any{"type": "string", "description": "ID of the todo to block."},
			"reason":  map[string]any{"type": "string", "description": "Reason the todo is blocked."},
		},
		"required":             []any{"todo_id"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleBlockTodo,
}

var startTodoSkill = &skills.Skill{
	Name:        "start_todo",
	Description: "Start working on a todo item, marking it in-progress. Optionally switch thread context.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todo_id":   map[string]any{"type": "string", "description": "ID of the todo to start."},
			"thread_id": map[string]any{"type": "string", "description": "Thread to switch to. Defaults to the todo's thread."},
		},
		"required":             []any{"todo_id"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleStartTodo,
}

var createTodoSkill = &skills.Skill{
	Name:        "create_todo",
	Description: "Create a new mission todo item for tracking work.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":       map[string]any{"type": "string", "description": "Title of the new todo."},
			"description": map[string]any{"type": "string", "description": "Detailed description of what needs to be done."},
			"priority":    map[string]any{"type": "string", "enum": []any{"critical", "high", "medium", "low"}, "description": "Priority level. Defaults to medium."},
		},
		"required":             []any{"title"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleCreateTodo,
}
