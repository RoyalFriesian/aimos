package contextpacks

import (
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

const defaultRecentMessagesLimit = 8

type BuildOptions struct {
	RecentMessagesLimit int
	IncludeChildRollups bool
}

type ContextPack struct {
	Mission        missions.Mission
	Thread         threads.Thread
	LatestSummary  *missionstate.Summary
	ChildRollups   []missionstate.Rollup
	DueTodos       []execution.Todo
	DueTimers      []execution.Timer
	RecentMessages []threads.Message
}

type Builder struct {
	missions     missions.Store
	threads      threads.Store
	missionState missionstate.Store
	execution    execution.Store
}

func NewBuilder(missionStore missions.Store, threadStore threads.Store, missionStateStore missionstate.Store, executionStore execution.Store) (*Builder, error) {
	if missionStore == nil {
		return nil, fmt.Errorf("mission store is required")
	}
	if threadStore == nil {
		return nil, fmt.Errorf("thread store is required")
	}
	if missionStateStore == nil {
		return nil, fmt.Errorf("mission state store is required")
	}
	if executionStore == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return &Builder{
		missions:     missionStore,
		threads:      threadStore,
		missionState: missionStateStore,
		execution:    executionStore,
	}, nil
}

func (b *Builder) BuildRootCEOPack(rootMissionID string, options BuildOptions) (ContextPack, error) {
	return b.BuildMissionPack(rootMissionID, "", options)
}

func (b *Builder) BuildMissionPack(missionID string, threadID string, options BuildOptions) (ContextPack, error) {
	mission, err := b.missions.GetMission(missionID)
	if err != nil {
		return ContextPack{}, err
	}

	if threadID == "" {
		threadID = mission.OwningThreadID
	}
	if threadID == "" {
		return ContextPack{}, fmt.Errorf("mission %q does not have an owning thread", mission.ID)
	}

	thread, err := b.threads.GetThread(threadID)
	if err != nil {
		return ContextPack{}, err
	}

	messages, err := b.threads.ListMessages(threadID)
	if err != nil {
		return ContextPack{}, err
	}

	pack := ContextPack{
		Mission:        mission,
		Thread:         thread,
		RecentMessages: lastMessages(messages, normalizeRecentMessageLimit(options.RecentMessagesLimit)),
	}

	latestSummary, err := b.missionState.GetLatestSummary(missionID)
	if err == nil {
		pack.LatestSummary = &latestSummary
	} else if err != missionstate.ErrSummaryNotFound {
		return ContextPack{}, err
	}

	if options.IncludeChildRollups {
		rollups, err := b.missionState.ListRollups(missionID)
		if err != nil {
			return ContextPack{}, err
		}
		pack.ChildRollups = rollups
	}

	dueTodos, err := b.execution.ListDueTodos(time.Now().UTC(), 64)
	if err != nil {
		return ContextPack{}, err
	}
	pack.DueTodos = filterDueTodosForMission(dueTodos, missionID)

	dueTimers, err := b.execution.ListDueTimers(time.Now().UTC(), 64)
	if err != nil {
		return ContextPack{}, err
	}
	pack.DueTimers = filterDueTimersForMission(dueTimers, missionID)

	return pack, nil
}

func normalizeRecentMessageLimit(limit int) int {
	if limit <= 0 {
		return defaultRecentMessagesLimit
	}
	return limit
}

func lastMessages(messages []threads.Message, limit int) []threads.Message {
	if len(messages) <= limit {
		copied := make([]threads.Message, len(messages))
		copy(copied, messages)
		return copied
	}
	start := len(messages) - limit
	trimmed := make([]threads.Message, limit)
	copy(trimmed, messages[start:])
	return trimmed
}

func filterDueTodosForMission(todos []execution.Todo, missionID string) []execution.Todo {
	filtered := make([]execution.Todo, 0, len(todos))
	for _, todo := range todos {
		if todo.MissionID != missionID {
			continue
		}
		filtered = append(filtered, todo)
	}
	return filtered
}

func filterDueTimersForMission(timers []execution.Timer, missionID string) []execution.Timer {
	filtered := make([]execution.Timer, 0, len(timers))
	for _, timer := range timers {
		if timer.MissionID != missionID {
			continue
		}
		filtered = append(filtered, timer)
	}
	return filtered
}
