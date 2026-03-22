package execution

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

var (
	ErrTodoNotFound  = errors.New("mission todo not found")
	ErrTimerNotFound = errors.New("mission timer not found")
)

type TodoStatus string

const (
	TodoStatusTodo       TodoStatus = "todo"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusBlocked    TodoStatus = "blocked"
	TodoStatusDone       TodoStatus = "done"
)

type TimerStatus string

const (
	TimerStatusScheduled TimerStatus = "scheduled"
	TimerStatusTriggered TimerStatus = "triggered"
	TimerStatusCancelled TimerStatus = "cancelled"
	TimerStatusFailed    TimerStatus = "failed"
)

type Todo struct {
	ID            string
	MissionID     string
	ThreadID      string
	Title         string
	Description   string
	OwnerAgentID  string
	Status        TodoStatus
	Priority      missions.Priority
	DueAt         *time.Time
	DependsOn     json.RawMessage
	ArtifactPaths json.RawMessage
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Timer struct {
	ID            string
	MissionID     string
	ThreadID      string
	SetByAgentID  string
	WakeAt        time.Time
	ActionType    string
	ActionPayload json.RawMessage
	Status        TimerStatus
	CreatedAt     time.Time
	TriggeredAt   *time.Time
}

type TodoStore interface {
	CreateTodo(todo Todo) error
	GetTodo(todoID string) (Todo, error)
	ListTodos(missionID string) ([]Todo, error)
	UpdateTodo(todo Todo) error
	ListDueTodos(dueBefore time.Time, limit int) ([]Todo, error)
}

type TimerStore interface {
	CreateTimer(timer Timer) error
	GetTimer(timerID string) (Timer, error)
	ListTimers(missionID string) ([]Timer, error)
	UpdateTimer(timer Timer) error
	ListDueTimers(wakeBefore time.Time, limit int) ([]Timer, error)
	ClaimDueTimers(wakeBefore time.Time, limit int) ([]Timer, error)
}

type Store interface {
	TodoStore
	TimerStore
}
