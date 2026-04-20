package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/skills"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// HandleMarkDone signals that the agent has finished its mission.
func HandleMarkDone(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse mark_done args: %w", err)
	}

	// Post a completion message.
	if env.ThreadStore != nil {
		_ = env.ThreadStore.AppendMessage(threads.Message{
			ID:            fmt.Sprintf("done-%s-%d", env.AgentID, time.Now().UnixNano()),
			ThreadID:      env.ThreadID,
			Role:          threads.RoleAssistant,
			AuthorAgentID: env.AgentID,
			AuthorRole:    env.AgentRole,
			MessageType:   "mark_done",
			Content:       p.Summary,
			CreatedAt:     time.Now().UTC(),
		})
	}

	// Update mission status to completed.
	if env.MissionStore != nil {
		m, err := env.MissionStore.GetMission(env.MissionID)
		if err == nil {
			m.Status = missions.MissionStatusCompleted
			_ = env.MissionStore.UpdateMission(m)
		}
	}

	// Signal the loop to stop.
	if env.OnSelfTerminate != nil {
		env.OnSelfTerminate()
	}
	return fmt.Sprintf("Agent %s marked done: %s", env.AgentID, p.Summary), nil
}

// HandleDeliverWork delivers a work artifact and optionally triggers testing.
func HandleDeliverWork(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Deliverable        string `json:"deliverable"`
		TodoTitle          string `json:"todo_title"`
		TodoDescription    string `json:"todo_description"`
		AcceptanceCriteria string `json:"acceptance_criteria"`
		MissionGoal        string `json:"mission_goal"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse deliver_work args: %w", err)
	}
	if p.Deliverable == "" {
		return "", fmt.Errorf("deliverable is required")
	}

	// Notify that files might have changed.
	if env.OnFilesChanged != nil {
		env.OnFilesChanged(ctx, env.ProjectDir)
	}

	// Invoke testing agent if available.
	if env.Testing != nil {
		result, err := env.Testing.Validate(ctx, skills.TestInput{
			Deliverable:        p.Deliverable,
			TodoTitle:          p.TodoTitle,
			TodoDescription:    p.TodoDescription,
			AcceptanceCriteria: p.AcceptanceCriteria,
			MissionGoal:        p.MissionGoal,
		})
		if err != nil {
			return "", fmt.Errorf("testing validation failed: %w", err)
		}
		if result.Status != "pass" && result.Status != "PASS" {
			return fmt.Sprintf("Delivery REJECTED by testing: %s\nIssues: %v", result.Summary, result.Issues), nil
		}
	}

	// Post delivery message.
	if env.ThreadStore != nil {
		_ = env.ThreadStore.AppendMessage(threads.Message{
			ID:            fmt.Sprintf("deliver-%s-%d", env.AgentID, time.Now().UnixNano()),
			ThreadID:      env.ThreadID,
			Role:          threads.RoleAssistant,
			AuthorAgentID: env.AgentID,
			AuthorRole:    env.AgentRole,
			MessageType:   "deliver_work",
			Content:       p.Deliverable,
			CreatedAt:     time.Now().UTC(),
		})
	}
	return "Work delivered successfully", nil
}

// HandleUpdateSummary refreshes the mission summary and parent rollup.
func HandleUpdateSummary(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		SummaryText string `json:"summary_text"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse update_summary args: %w", err)
	}

	if env.MissionStateRuntime == nil {
		return "mission state runtime not available, summary skipped", nil
	}
	summary, err := env.MissionStateRuntime.RefreshMissionSummary(env.MissionID, env.ThreadID)
	if err != nil {
		return "", fmt.Errorf("refresh summary: %w", err)
	}
	// Also refresh parent rollup if applicable.
	_, _ = env.MissionStateRuntime.PublishParentRollup(env.MissionID, env.ThreadID)

	return fmt.Sprintf("Summary refreshed: %s", truncateStr(summary.SummaryText, 200)), nil
}

// HandleEscalate escalates an issue to the parent thread.
func HandleEscalate(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Issue      string `json:"issue"`
		Severity   string `json:"severity"`
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse escalate args: %w", err)
	}
	if p.Issue == "" {
		return "", fmt.Errorf("issue is required for escalation")
	}

	content := fmt.Sprintf("ESCALATION from %s [%s]\nIssue: %s\nSeverity: %s\nSuggestion: %s",
		env.AgentID, env.AgentRole, p.Issue, p.Severity, p.Suggestion)

	// Find parent thread.
	parentThreadID := ""
	if env.NodeStore != nil {
		node, err := env.NodeStore.GetNode(env.AgentID)
		if err == nil && node.ParentAgentID != "" {
			parent, err := env.NodeStore.GetNode(node.ParentAgentID)
			if err == nil {
				parentThreadID = parent.ThreadID
			}
		}
	}
	if parentThreadID == "" {
		parentThreadID = env.ThreadID
	}

	if env.ThreadStore != nil {
		_ = env.ThreadStore.AppendMessage(threads.Message{
			ID:            fmt.Sprintf("escalate-%s-%d", env.AgentID, time.Now().UnixNano()),
			ThreadID:      parentThreadID,
			Role:          threads.RoleAssistant,
			AuthorAgentID: env.AgentID,
			AuthorRole:    env.AgentRole,
			MessageType:   "escalation",
			Content:       content,
			CreatedAt:     time.Now().UTC(),
		})
	}
	return fmt.Sprintf("Escalation sent to thread %s", parentThreadID), nil
}

// HandleScheduleFollowup schedules a timer for a future follow-up.
func HandleScheduleFollowup(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		DelayMinutes int    `json:"delay_minutes"`
		Reason       string `json:"reason"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse schedule_followup args: %w", err)
	}
	if env.ExecutionRuntime == nil {
		return "timer scheduling not available", nil
	}
	if p.DelayMinutes <= 0 {
		p.DelayMinutes = 5
	}
	wakeAt := time.Now().UTC().Add(time.Duration(p.DelayMinutes) * time.Minute)

	payload, _ := json.Marshal(map[string]string{"reason": p.Reason})
	_, err := env.ExecutionRuntime.ScheduleTimer(execution.ScheduleTimerInput{
		MissionID:     env.MissionID,
		ThreadID:      env.ThreadID,
		SetByAgentID:  env.AgentID,
		WakeAt:        wakeAt,
		ActionType:    "followup",
		ActionPayload: payload,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Follow-up scheduled in %d minutes: %s", p.DelayMinutes, p.Reason), nil
}

var markDoneSkill = &skills.Skill{
	Name:        "mark_done",
	Description: "Signal that this agent has completed its mission and should stop.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string", "description": "Summary of work accomplished."},
		},
		"required":             []any{"summary"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleMarkDone,
}

var deliverWorkSkill = &skills.Skill{
	Name:        "deliver_work",
	Description: "Deliver a work artifact. Testing agent will validate if available.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"deliverable":         map[string]any{"type": "string", "description": "The work product to deliver."},
			"todo_title":          map[string]any{"type": "string", "description": "Title of the associated todo."},
			"todo_description":    map[string]any{"type": "string", "description": "Description of the todo for testing context."},
			"acceptance_criteria": map[string]any{"type": "string", "description": "Acceptance criteria for testing."},
			"mission_goal":        map[string]any{"type": "string", "description": "Overall mission goal for context."},
		},
		"required":             []any{"deliverable"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleDeliverWork,
}

var updateSummarySkill = &skills.Skill{
	Name:        "update_summary",
	Description: "Refresh the mission summary and parent rollup.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary_text": map[string]any{"type": "string", "description": "Optional override text for the summary."},
		},
		"required":             []any{},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleUpdateSummary,
}

var escalateSkill = &skills.Skill{
	Name:        "escalate",
	Description: "Escalate an issue to the parent agent or thread.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"issue":      map[string]any{"type": "string", "description": "Description of the issue."},
			"severity":   map[string]any{"type": "string", "enum": []any{"low", "medium", "high", "critical"}, "description": "Severity of the issue."},
			"suggestion": map[string]any{"type": "string", "description": "Suggested resolution."},
		},
		"required":             []any{"issue"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleEscalate,
}

var scheduleFollowupSkill = &skills.Skill{
	Name:        "schedule_followup",
	Description: "Schedule a timer to follow up on something later.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"delay_minutes": map[string]any{"type": "integer", "description": "Minutes until the follow-up triggers. Defaults to 5."},
			"reason":        map[string]any{"type": "string", "description": "Reason for the follow-up."},
		},
		"required":             []any{"reason"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleScheduleFollowup,
}
