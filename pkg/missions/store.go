package missions

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrProgramNotFound    = errors.New("program not found")
	ErrMissionNotFound    = errors.New("mission not found")
	ErrAssignmentNotFound = errors.New("mission assignment not found")
)

type ProgramStatus string

const (
	ProgramStatusDrafted    ProgramStatus = "drafted"
	ProgramStatusActive     ProgramStatus = "active"
	ProgramStatusWaiting    ProgramStatus = "waiting"
	ProgramStatusBlocked    ProgramStatus = "blocked"
	ProgramStatusCompleted  ProgramStatus = "completed"
	ProgramStatusFinished   ProgramStatus = "finished"
	ProgramStatusSuperseded ProgramStatus = "superseded"
	ProgramStatusFailed     ProgramStatus = "failed"
	ProgramStatusCancelled  ProgramStatus = "cancelled"
)

type MissionStatus string

const (
	MissionStatusDrafted    MissionStatus = "drafted"
	MissionStatusActive     MissionStatus = "active"
	MissionStatusWaiting    MissionStatus = "waiting"
	MissionStatusBlocked    MissionStatus = "blocked"
	MissionStatusReview     MissionStatus = "review"
	MissionStatusCompleted  MissionStatus = "completed"
	MissionStatusFinished   MissionStatus = "finished"
	MissionStatusSuperseded MissionStatus = "superseded"
	MissionStatusFailed     MissionStatus = "failed"
	MissionStatusCancelled  MissionStatus = "cancelled"
)

func IsTerminalMissionStatus(status MissionStatus) bool {
	switch status {
	case MissionStatusCompleted, MissionStatusFinished, MissionStatusSuperseded, MissionStatusFailed, MissionStatusCancelled:
		return true
	default:
		return false
	}
}

func IsReusableMissionStatus(status MissionStatus) bool {
	switch status {
	case MissionStatusCompleted, MissionStatusFinished, MissionStatusSuperseded:
		return true
	default:
		return false
	}
}

type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

type Program struct {
	ID            string
	ClientID      string
	Title         string
	RootMissionID string
	Status        ProgramStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Mission struct {
	ID                 string
	ProgramID          string
	ParentMissionID    string
	RootMissionID      string
	OwningThreadID     string
	OwnerAgentID       string
	OwnerRole          string
	MissionType        string
	Title              string
	Charter            string
	Goal               string
	Scope              string
	ReuseTrace         json.RawMessage
	Constraints        json.RawMessage
	AcceptanceCriteria json.RawMessage
	AuthorityLevel     string
	DelegationPolicy   json.RawMessage
	Status             MissionStatus
	Priority           Priority
	RiskLevel          Priority
	ProgressPercent    float64
	WaitingUntil       *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ClosedAt           *time.Time
}

type Assignment struct {
	ID                 string
	MissionID          string
	AgentID            string
	AgentRole          string
	AuthorityScope     json.RawMessage
	ReportingToAgentID string
	AssignedAt         time.Time
	RevokedAt          *time.Time
}

type ReuseTraceRef struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Reason     string `json:"reason,omitempty"`
}

type ReusableMissionMatch struct {
	Mission      Mission
	Score        float64
	MatchedTerms []string
}

type ProgramStore interface {
	CreateProgram(program Program) error
	GetProgram(programID string) (Program, error)
	UpdateProgram(program Program) error
}

type Store interface {
	ProgramStore
	CreateMission(mission Mission) error
	GetMission(missionID string) (Mission, error)
	UpdateMission(mission Mission) error
	ListChildMissions(parentMissionID string) ([]Mission, error)
	SearchReusableMissions(query string, limit int) ([]ReusableMissionMatch, error)
	AssignMission(assignment Assignment) error
	ListAssignments(missionID string) ([]Assignment, error)
}
