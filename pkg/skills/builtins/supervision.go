package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/skills"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// HandleCheckChild posts a status-check message to a child agent's thread.
func HandleCheckChild(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		ChildAgentID string `json:"child_agent_id"`
		Question     string `json:"question"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse check_child args: %w", err)
	}
	if env.NodeStore == nil {
		return "no node store available", nil
	}
	children, _ := env.NodeStore.ListChildren(env.AgentID)
	if len(children) == 0 {
		return "No child agents exist yet. Create sub-workers with create_worker or do the work directly.", nil
	}

	var target *skills.NodeInfo
	for i := range children {
		if children[i].ID == p.ChildAgentID {
			target = &children[i]
			break
		}
	}
	// Fallback: pick first active child.
	if target == nil {
		for i := range children {
			if children[i].Status == "active" || children[i].Status == "busy" {
				target = &children[i]
				break
			}
		}
	}
	if target == nil {
		return "No active children to check.", nil
	}

	question := p.Question
	if question == "" {
		question = fmt.Sprintf("Status check from %s: What is your current progress?", env.AgentID)
	}
	err := env.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("check-%s-%d", env.AgentID, time.Now().UnixNano()),
		ThreadID:      target.ThreadID,
		Role:          threads.RoleUser,
		AuthorAgentID: env.AgentID,
		AuthorRole:    env.AgentRole,
		MessageType:   "parent_check",
		Content:       question,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Status check sent to child %s (thread %s)", target.ID, target.ThreadID), nil
}

// HandleResolveConflict sends a resolution message to a child agent's thread.
func HandleResolveConflict(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		TargetChildID string `json:"target_child_id"`
		Resolution    string `json:"resolution"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse resolve_conflict args: %w", err)
	}
	if env.NodeStore == nil {
		return "", fmt.Errorf("no node store available")
	}
	children, _ := env.NodeStore.ListChildren(env.AgentID)
	var target *skills.NodeInfo
	for i := range children {
		if children[i].ID == p.TargetChildID {
			target = &children[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("target child %q not found for conflict resolution", p.TargetChildID)
	}
	err := env.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("resolve-%s-%d", env.AgentID, time.Now().UnixNano()),
		ThreadID:      target.ThreadID,
		Role:          threads.RoleUser,
		AuthorAgentID: env.AgentID,
		AuthorRole:    env.AgentRole,
		MessageType:   "parent_resolution",
		Content:       p.Resolution,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Resolution sent to child %s", target.ID), nil
}

// HandleCreateWorker creates a new child agent with a mission and todo.
func HandleCreateWorker(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Name             string `json:"name"`
		Role             string `json:"role"`
		ProblemStatement string `json:"problem_statement"`
		TodoTitle        string `json:"todo_title"`
		TodoDescription  string `json:"todo_description"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse create_worker args: %w", err)
	}
	if env.Depth >= env.MaxDepth {
		return "Cannot create sub-worker: at maximum depth.", nil
	}
	if p.ProblemStatement == "" {
		return "", fmt.Errorf("create_worker requires a problem_statement")
	}
	if env.LoopManager == nil {
		return "", fmt.Errorf("loop manager not available")
	}

	workerName := p.Name
	if workerName == "" {
		workerName = sanitizeSlug(p.ProblemStatement)
		if workerName == "" {
			workerName = fmt.Sprintf("sub-worker-%d", time.Now().UnixNano())
		}
	}

	role := p.Role
	if role == "" {
		role = "worker"
	}

	childDepth := env.Depth + 1
	missionID := fmt.Sprintf("sub-%s-%d", env.MissionID, time.Now().UnixNano())
	threadID := fmt.Sprintf("thread-%s", missionID)
	agentID := fmt.Sprintf("agent-%s-%d", sanitizeSlug(workerName), time.Now().UnixNano())

	// Create child mission via the mission runtime.
	if env.MissionRuntime != nil {
		if err := createChildMissionAndTodo(ctx, env, skills.ChildLoopConfig{
			AgentID:   agentID,
			AgentRole: role,
			MissionID: missionID,
			ThreadID:  threadID,
			ProjectID: env.ProjectID,
			Depth:     childDepth,
			MaxDepth:  env.MaxDepth,
			Model:     env.Model,
		}, p.ProblemStatement, p.TodoTitle, p.TodoDescription, workerName); err != nil {
			return "", err
		}
	}

	// Start the child loop.
	if err := env.LoopManager.StartChildLoop(ctx, skills.ChildLoopConfig{
		AgentID:         agentID,
		AgentRole:       role,
		MissionID:       missionID,
		ThreadID:        threadID,
		ProjectID:       env.ProjectID,
		ProjectLocation: env.ProjectDir,
		Depth:           childDepth,
		MaxDepth:        env.MaxDepth,
		Model:           env.Model,
	}); err != nil {
		return "", fmt.Errorf("start child loop: %w", err)
	}

	return fmt.Sprintf("Created sub-worker %q (depth %d) for: %s", workerName, childDepth, p.ProblemStatement), nil
}

// HandleMergeBranch merges a worker's git branch into the main branch.
func HandleMergeBranch(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		WorkerName string `json:"worker_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse merge_branch args: %w", err)
	}
	if env.ProjectDir == "" {
		return "", fmt.Errorf("merge_branch: project location not configured")
	}
	// Merge logic deferred to the agent loop integration phase,
	// since it requires gitops package access.
	return fmt.Sprintf("merge_branch for %q: deferred to loop integration", p.WorkerName), nil
}

var checkChildSkill = &skills.Skill{
	Name:         "check_child",
	Description:  "Send a status check message to a child agent. Use to monitor delegated work.",
	AllowedRoles: []string{"CEO", "Manager"},
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"child_agent_id": map[string]any{"type": "string", "description": "ID of the child agent to check. If empty, the first active child is checked."},
			"question":       map[string]any{"type": "string", "description": "The question or status check message to send."},
		},
		"required":             []any{"child_agent_id"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleCheckChild,
}

var createWorkerSkill = &skills.Skill{
	Name:         "create_worker",
	Description:  "Create a new sub-worker agent with its own mission and todo. Use to delegate a sub-task.",
	AllowedRoles: []string{"CEO", "Manager"},
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":              map[string]any{"type": "string", "description": "Name for the new worker agent."},
			"role":              map[string]any{"type": "string", "description": "Role for the new agent: worker, manager, or tester. Defaults to worker."},
			"problem_statement": map[string]any{"type": "string", "description": "What the sub-worker should accomplish."},
			"todo_title":        map[string]any{"type": "string", "description": "Title for the initial todo item."},
			"todo_description":  map[string]any{"type": "string", "description": "Description of work for the initial todo."},
		},
		"required":             []any{"problem_statement"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleCreateWorker,
}

var resolveConflictSkill = &skills.Skill{
	Name:         "resolve_conflict",
	Description:  "Send a directive to a child agent to resolve a conflict or provide guidance.",
	AllowedRoles: []string{"CEO", "Manager"},
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target_child_id": map[string]any{"type": "string", "description": "ID of the child agent receiving the resolution."},
			"resolution":      map[string]any{"type": "string", "description": "The resolution or directive to send."},
		},
		"required":             []any{"target_child_id", "resolution"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleResolveConflict,
}

var mergeBranchSkill = &skills.Skill{
	Name:         "merge_branch",
	Description:  "Merge a worker's git branch into the main branch.",
	AllowedRoles: []string{"CEO", "Manager"},
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"worker_name": map[string]any{"type": "string", "description": "Name of the worker whose branch to merge. Empty merges all."},
		},
		"required":             []any{},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleMergeBranch,
}
