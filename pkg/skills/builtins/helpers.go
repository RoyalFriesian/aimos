package builtins

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/skills"
)

var slugRegexp = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeSlug converts a string to a lowercase slug suitable for IDs.
func sanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// truncateStr truncates a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// createChildMissionAndTodo creates a child mission and its initial todo.
func createChildMissionAndTodo(
	ctx context.Context,
	env *skills.Env,
	cfg skills.ChildLoopConfig,
	problemStatement, todoTitle, todoDescription, workerName string,
) error {
	if env.MissionRuntime == nil {
		return nil
	}
	_, _, err := env.MissionRuntime.CreateChildMission(missions.ChildMissionInput{
		MissionID:       cfg.MissionID,
		ParentMissionID: env.MissionID,
		ThreadID:        cfg.ThreadID,
		OwnerAgentID:    cfg.AgentID,
		OwnerRole:       cfg.AgentRole,
		MissionTitle:    workerName,
		Goal:            problemStatement,
		ThreadTitle:     problemStatement,
	})
	if err != nil {
		return fmt.Errorf("create child mission: %w", err)
	}

	if env.ExecutionRuntime != nil && todoTitle != "" {
		title := todoTitle
		if title == "" {
			title = problemStatement
		}
		_, err := env.ExecutionRuntime.CreateTodo(execution.CreateTodoInput{
			MissionID:    cfg.MissionID,
			ThreadID:     cfg.ThreadID,
			Title:        title,
			Description:  todoDescription,
			OwnerAgentID: cfg.AgentID,
		})
		if err != nil {
			return fmt.Errorf("create initial todo: %w", err)
		}
	}

	return nil
}
