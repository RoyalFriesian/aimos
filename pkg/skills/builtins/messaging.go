package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/skills"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// HandlePostMessage posts a message to a target thread.
func HandlePostMessage(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		TargetThreadID string `json:"target_thread_id"`
		Content        string `json:"content"`
		ChildAgentID   string `json:"child_agent_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse post_message args: %w", err)
	}
	if p.Content == "" {
		return "no content provided, skipping", nil
	}

	targetThread := p.TargetThreadID
	if targetThread == "" && p.ChildAgentID != "" && env.NodeStore != nil {
		children, _ := env.NodeStore.ListChildren(env.AgentID)
		for _, ch := range children {
			if ch.ID == p.ChildAgentID {
				targetThread = ch.ThreadID
				break
			}
		}
	}
	if targetThread == "" {
		targetThread = env.ThreadID
	}

	err := env.ThreadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("agent-%s-%d", env.AgentID, time.Now().UnixNano()),
		ThreadID:      targetThread,
		Role:          threads.RoleAssistant,
		AuthorAgentID: env.AgentID,
		AuthorRole:    env.AgentRole,
		MessageType:   "agent_message",
		Content:       p.Content,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Message posted to thread %s", targetThread), nil
}

var postMessageSkill = &skills.Skill{
	Name:        "post_message",
	Description: "Post a message to a thread. Use to communicate with other agents or the parent thread.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content":          map[string]any{"type": "string", "description": "The message content to post"},
			"target_thread_id": map[string]any{"type": "string", "description": "Thread ID to post to. Defaults to current thread if empty."},
			"child_agent_id":   map[string]any{"type": "string", "description": "Child agent ID to resolve thread from, if target_thread_id is not known."},
		},
		"required":             []any{"content"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandlePostMessage,
}
