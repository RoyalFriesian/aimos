package missionstate

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

var (
	ErrSummaryNotFound = errors.New("mission summary not found")
	ErrRollupNotFound  = errors.New("mission rollup not found")
)

type Summary struct {
	ID               string
	MissionID        string
	ThreadID         string
	Level            string
	Kind             string
	CoverageStartRef string
	CoverageEndRef   string
	SummaryText      string
	KeyDecisions     json.RawMessage
	OpenQuestions    json.RawMessage
	Blockers         json.RawMessage
	NextActions      json.RawMessage
	CreatedAt        time.Time
}

type Rollup struct {
	ID                   string
	ParentMissionID      string
	ChildMissionID       string
	Status               missions.MissionStatus
	ProgressPercent      float64
	Health               string
	CurrentBlocker       string
	LatestSummary        string
	NextExpectedUpdateAt *time.Time
	OverdueFlags         json.RawMessage
	ExecutionSummary     json.RawMessage
	UpdatedAt            time.Time
}

type Store interface {
	CreateSummary(summary Summary) error
	GetLatestSummary(missionID string) (Summary, error)
	ListSummaries(missionID string) ([]Summary, error)
	UpsertRollup(rollup Rollup) error
	GetRollup(parentMissionID string, childMissionID string) (Rollup, error)
	ListRollups(parentMissionID string) ([]Rollup, error)
}
