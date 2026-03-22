package threads

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrThreadNotFound = errors.New("thread not found")

type ThreadStatus string

const (
	ThreadStatusCreated    ThreadStatus = "created"
	ThreadStatusActive     ThreadStatus = "active"
	ThreadStatusWaiting    ThreadStatus = "waiting"
	ThreadStatusBlocked    ThreadStatus = "blocked"
	ThreadStatusCompleted  ThreadStatus = "completed"
	ThreadStatusFinished   ThreadStatus = "finished"
	ThreadStatusSuperseded ThreadStatus = "superseded"
	ThreadStatusFailed     ThreadStatus = "failed"
	ThreadStatusCancelled  ThreadStatus = "cancelled"
)

func IsTerminalThreadStatus(status ThreadStatus) bool {
	switch status {
	case ThreadStatusCompleted, ThreadStatusFinished, ThreadStatusSuperseded, ThreadStatusFailed, ThreadStatusCancelled:
		return true
	default:
		return false
	}
}

func IsReusableThreadStatus(status ThreadStatus) bool {
	switch status {
	case ThreadStatusCompleted, ThreadStatusFinished, ThreadStatusSuperseded:
		return true
	default:
		return false
	}
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Thread struct {
	ID             string
	MissionID      string
	RootMissionID  string
	ParentThreadID string
	Kind           string
	Title          string
	Summary        string
	Context        string
	OwnerAgentID   string
	CurrentMode    string
	Status         ThreadStatus
	WaitingUntil   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Message struct {
	ID               string
	ThreadID         string
	Role             Role
	AuthorAgentID    string
	AuthorRole       string
	MessageType      string
	Content          string
	ContentJSON      json.RawMessage
	Mode             string
	ReplyToMessageID string
	CreatedAt        time.Time
}

type ReusableThreadMatch struct {
	Thread       Thread
	Score        float64
	MatchedTerms []string
}

type Store interface {
	CreateThread(thread Thread) error
	GetThread(threadID string) (Thread, error)
	ListByMission(missionID string) ([]Thread, error)
	ListRootThreads() ([]Thread, error)
	SearchReusableThreads(query string, limit int) ([]ReusableThreadMatch, error)
	AppendMessage(message Message) error
	ListMessages(threadID string) ([]Message, error)
	UpdateThreadMode(threadID string, mode string) error
	UpdateThreadOwner(threadID string, ownerAgentID string) error
}
