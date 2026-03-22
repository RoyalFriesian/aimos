package missionstate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

type Runtime struct {
	store     Store
	missions  missions.Store
	threads   threads.Store
	execution execution.Store
}

type executionRollupSnapshot struct {
	TotalTodos       int        `json:"totalTodos"`
	OpenTodos        int        `json:"openTodos"`
	InProgressTodos  int        `json:"inProgressTodos"`
	BlockedTodos     int        `json:"blockedTodos"`
	DoneTodos        int        `json:"doneTodos"`
	DueTodos         int        `json:"dueTodos"`
	ScheduledTimers  int        `json:"scheduledTimers"`
	DueTimers        int        `json:"dueTimers"`
	NextTimerAt      *time.Time `json:"nextTimerAt,omitempty"`
	BlockedTodoID    string     `json:"-"`
	BlockedTodoTitle string     `json:"-"`
}

func NewRuntime(store Store, missionStore missions.Store, threadStore threads.Store, executionStore execution.Store) (*Runtime, error) {
	if store == nil {
		return nil, fmt.Errorf("mission state store is required")
	}
	if missionStore == nil {
		return nil, fmt.Errorf("mission store is required")
	}
	if threadStore == nil {
		return nil, fmt.Errorf("thread store is required")
	}
	if executionStore == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return &Runtime{store: store, missions: missionStore, threads: threadStore, execution: executionStore}, nil
}

func (r *Runtime) RefreshMissionSummary(missionID string, threadID string) (Summary, error) {
	mission, thread, messages, err := r.loadMissionContext(missionID, threadID)
	if err != nil {
		return Summary{}, err
	}

	coverageStartRef := thread.ID
	coverageEndRef := thread.ID
	if len(messages) > 0 {
		coverageStartRef = messages[0].ID
		coverageEndRef = messages[len(messages)-1].ID
	}

	latestMessage := ""
	if len(messages) > 0 {
		latestMessage = strings.TrimSpace(messages[len(messages)-1].Content)
	}
	summaryText := buildSummaryText(mission, thread, latestMessage)
	keyDecisions := jsonListFromStrings([]string{fmt.Sprintf("Delegate owner: %s", mission.OwnerAgentID)})
	blockers := jsonListFromStrings(extractBlockers(mission, latestMessage))
	nextActions := jsonListFromStrings(defaultNextActions(mission))
	summary := Summary{
		ID:               fmt.Sprintf("summary-%s-%d", mission.ID, time.Now().UTC().UnixNano()),
		MissionID:        mission.ID,
		ThreadID:         thread.ID,
		Level:            "mission",
		Kind:             "rolling",
		CoverageStartRef: coverageStartRef,
		CoverageEndRef:   coverageEndRef,
		SummaryText:      summaryText,
		KeyDecisions:     keyDecisions,
		Blockers:         blockers,
		NextActions:      nextActions,
		OpenQuestions:    json.RawMessage(`[]`),
		CreatedAt:        time.Now().UTC(),
	}
	if err := r.store.CreateSummary(summary); err != nil {
		return Summary{}, err
	}
	return r.store.GetLatestSummary(mission.ID)
}

func (r *Runtime) PublishParentRollup(missionID string, threadID string) (Rollup, error) {
	mission, _, _, err := r.loadMissionContext(missionID, threadID)
	if err != nil {
		return Rollup{}, err
	}
	if mission.ParentMissionID == "" {
		return Rollup{}, ErrRollupNotFound
	}
	summary, err := r.store.GetLatestSummary(mission.ID)
	if err != nil {
		if err == ErrSummaryNotFound {
			summary, err = r.RefreshMissionSummary(mission.ID, threadID)
		}
		if err != nil {
			return Rollup{}, err
		}
	}

	asOf := time.Now().UTC()
	executionSnapshot, err := r.buildExecutionSnapshot(mission.ID, asOf)
	if err != nil {
		return Rollup{}, err
	}
	nextExpectedUpdate := deriveNextExpectedUpdate(asOf, executionSnapshot)
	overdueFlags, err := buildOverdueFlagsFromSnapshot(executionSnapshot)
	if err != nil {
		return Rollup{}, err
	}
	executionSummary, err := json.Marshal(executionSnapshot)
	if err != nil {
		return Rollup{}, err
	}
	rollup := Rollup{
		ID:                   fmt.Sprintf("rollup-%s-%s", mission.ParentMissionID, mission.ID),
		ParentMissionID:      mission.ParentMissionID,
		ChildMissionID:       mission.ID,
		Status:               mission.Status,
		ProgressPercent:      deriveRollupProgress(mission.ProgressPercent, executionSnapshot),
		Health:               deriveHealth(mission.Status, executionSnapshot),
		CurrentBlocker:       deriveCurrentBlocker(summary, executionSnapshot),
		LatestSummary:        summary.SummaryText,
		NextExpectedUpdateAt: &nextExpectedUpdate,
		OverdueFlags:         overdueFlags,
		ExecutionSummary:     executionSummary,
		UpdatedAt:            asOf,
	}
	if err := r.store.UpsertRollup(rollup); err != nil {
		return Rollup{}, err
	}
	return r.store.GetRollup(mission.ParentMissionID, mission.ID)
}

func (r *Runtime) RefreshMissionState(missionID string, threadID string) (Summary, *Rollup, error) {
	summary, err := r.RefreshMissionSummary(missionID, threadID)
	if err != nil {
		return Summary{}, nil, err
	}
	mission, err := r.missions.GetMission(missionID)
	if err != nil {
		return Summary{}, nil, err
	}
	if mission.ParentMissionID == "" {
		return summary, nil, nil
	}
	rollup, err := r.PublishParentRollup(missionID, threadID)
	if err != nil {
		return Summary{}, nil, err
	}
	return summary, &rollup, nil
}

func (r *Runtime) loadMissionContext(missionID string, threadID string) (missions.Mission, threads.Thread, []threads.Message, error) {
	mission, err := r.missions.GetMission(missionID)
	if err != nil {
		return missions.Mission{}, threads.Thread{}, nil, err
	}
	if threadID == "" {
		threadID = mission.OwningThreadID
	}
	thread, err := r.threads.GetThread(threadID)
	if err != nil {
		return missions.Mission{}, threads.Thread{}, nil, err
	}
	messages, err := r.threads.ListMessages(thread.ID)
	if err != nil {
		if err == threads.ErrThreadNotFound {
			return missions.Mission{}, threads.Thread{}, nil, err
		}
		return mission, thread, nil, err
	}
	return mission, thread, messages, nil
}

func buildSummaryText(mission missions.Mission, thread threads.Thread, latestMessage string) string {
	parts := []string{fmt.Sprintf("%s is currently %s under %s ownership.", mission.Title, mission.Status, mission.OwnerAgentID)}
	if strings.TrimSpace(mission.Goal) != "" {
		parts = append(parts, fmt.Sprintf("Goal: %s.", strings.TrimSpace(mission.Goal)))
	}
	if strings.TrimSpace(latestMessage) != "" {
		parts = append(parts, fmt.Sprintf("Latest thread update: %s.", trimSentence(latestMessage)))
	} else if strings.TrimSpace(thread.Summary) != "" {
		parts = append(parts, fmt.Sprintf("Thread focus: %s.", trimSentence(thread.Summary)))
	}
	return strings.Join(parts, " ")
}

func deriveHealth(status missions.MissionStatus, snapshot executionRollupSnapshot) string {
	switch status {
	case missions.MissionStatusBlocked, missions.MissionStatusFailed:
		return "red"
	case missions.MissionStatusWaiting, missions.MissionStatusReview:
		return "yellow"
	default:
		if snapshot.BlockedTodos > 0 {
			return "red"
		}
		if snapshot.DueTodos > 0 || snapshot.DueTimers > 0 {
			return "yellow"
		}
		return "green"
	}
}

func extractBlockers(mission missions.Mission, latestMessage string) []string {
	if mission.Status == missions.MissionStatusBlocked {
		if strings.TrimSpace(latestMessage) != "" {
			return []string{trimSentence(latestMessage)}
		}
		return []string{"Mission is currently blocked."}
	}
	return []string{}
}

func defaultNextActions(mission missions.Mission) []string {
	if mission.Status == missions.MissionStatusBlocked {
		return []string{"Resolve active blocker", "Publish updated mission status"}
	}
	return []string{"Continue delegated execution", "Publish next mission update"}
}

func jsonListFromStrings(values []string) json.RawMessage {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return encoded
}

func firstJSONArrayValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
		return ""
	}
	return values[0]
}

func trimSentence(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ".")
	return trimmed
}

func (r *Runtime) buildExecutionSnapshot(missionID string, asOf time.Time) (executionRollupSnapshot, error) {
	todos, err := r.execution.ListTodos(missionID)
	if err != nil {
		return executionRollupSnapshot{}, err
	}
	timers, err := r.execution.ListTimers(missionID)
	if err != nil {
		return executionRollupSnapshot{}, err
	}
	snapshot := executionRollupSnapshot{}
	for _, todo := range todos {
		snapshot.TotalTodos++
		switch todo.Status {
		case execution.TodoStatusDone:
			snapshot.DoneTodos++
		case execution.TodoStatusInProgress:
			snapshot.OpenTodos++
			snapshot.InProgressTodos++
		case execution.TodoStatusBlocked:
			snapshot.OpenTodos++
			snapshot.BlockedTodos++
			if snapshot.BlockedTodoTitle == "" {
				snapshot.BlockedTodoID = todo.ID
				snapshot.BlockedTodoTitle = todo.Title
			}
		default:
			snapshot.OpenTodos++
		}
		if todo.DueAt != nil && !todo.DueAt.After(asOf) && todo.Status != execution.TodoStatusDone {
			snapshot.DueTodos++
		}
	}
	for _, timer := range timers {
		if timer.Status == execution.TimerStatusScheduled {
			snapshot.ScheduledTimers++
			if snapshot.NextTimerAt == nil || timer.WakeAt.Before(*snapshot.NextTimerAt) {
				wakeAt := timer.WakeAt
				snapshot.NextTimerAt = &wakeAt
			}
			if !timer.WakeAt.After(asOf) {
				snapshot.DueTimers++
			}
		}
	}
	return snapshot, nil
}

func buildOverdueFlagsFromSnapshot(snapshot executionRollupSnapshot) (json.RawMessage, error) {
	flags := make([]string, 0, 2)
	if snapshot.DueTodos > 0 {
		flags = append(flags, "todo_due")
	}
	if snapshot.DueTimers > 0 {
		flags = append(flags, "timer_due")
	}
	encoded, err := json.Marshal(flags)
	if err != nil {
		return json.RawMessage(`[]`), nil
	}
	return encoded, nil
}

func deriveRollupProgress(baseProgress float64, snapshot executionRollupSnapshot) float64 {
	if snapshot.TotalTodos == 0 {
		return baseProgress
	}
	return (float64(snapshot.DoneTodos) / float64(snapshot.TotalTodos)) * 100
}

func deriveCurrentBlocker(summary Summary, snapshot executionRollupSnapshot) string {
	if blocker := firstJSONArrayValue(summary.Blockers); blocker != "" {
		return blocker
	}
	if snapshot.BlockedTodoTitle != "" {
		return fmt.Sprintf("Blocked todo: %s", snapshot.BlockedTodoTitle)
	}
	return ""
}

func deriveNextExpectedUpdate(asOf time.Time, snapshot executionRollupSnapshot) time.Time {
	if snapshot.NextTimerAt != nil {
		return snapshot.NextTimerAt.UTC()
	}
	return asOf.Add(24 * time.Hour)
}
