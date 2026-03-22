package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

type Runtime struct {
	store Store
}

type CreateTodoInput struct {
	MissionID     string
	ThreadID      string
	Title         string
	Description   string
	OwnerAgentID  string
	Priority      missions.Priority
	DueAt         *time.Time
	DependsOn     []string
	ArtifactPaths []string
}

type ScheduleTimerInput struct {
	MissionID     string
	ThreadID      string
	SetByAgentID  string
	WakeAt        time.Time
	ActionType    string
	ActionPayload json.RawMessage
}

func NewRuntime(store Store) (*Runtime, error) {
	if store == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return &Runtime{store: store}, nil
}

func (r *Runtime) Store() Store {
	return r.store
}

func (r *Runtime) CreateTodo(input CreateTodoInput) (Todo, error) {
	todo := Todo{
		ID:            fmt.Sprintf("todo-%s-%d", input.MissionID, time.Now().UTC().UnixNano()),
		MissionID:     input.MissionID,
		ThreadID:      input.ThreadID,
		Title:         input.Title,
		Description:   input.Description,
		OwnerAgentID:  input.OwnerAgentID,
		Status:        TodoStatusTodo,
		Priority:      defaultPriority(input.Priority),
		DueAt:         input.DueAt,
		DependsOn:     marshalStringList(input.DependsOn),
		ArtifactPaths: marshalStringList(input.ArtifactPaths),
	}
	if err := r.store.CreateTodo(todo); err != nil {
		return Todo{}, err
	}
	return r.store.GetTodo(todo.ID)
}

func (r *Runtime) StartTodo(todoID string) (Todo, error) {
	return r.updateTodoStatus(todoID, TodoStatusInProgress)
}

func (r *Runtime) BlockTodo(todoID string) (Todo, error) {
	return r.updateTodoStatus(todoID, TodoStatusBlocked)
}

func (r *Runtime) CompleteTodo(todoID string) (Todo, error) {
	return r.updateTodoStatus(todoID, TodoStatusDone)
}

func (r *Runtime) AssignTodo(todoID string, ownerAgentID string) (Todo, error) {
	if ownerAgentID == "" {
		return Todo{}, fmt.Errorf("owner agent id is required")
	}
	todo, err := r.store.GetTodo(todoID)
	if err != nil {
		return Todo{}, err
	}
	todo.OwnerAgentID = ownerAgentID
	if err := r.store.UpdateTodo(todo); err != nil {
		return Todo{}, err
	}
	return r.store.GetTodo(todoID)
}

func (r *Runtime) UpdateTodoPriority(todoID string, priority missions.Priority) (Todo, error) {
	todo, err := r.store.GetTodo(todoID)
	if err != nil {
		return Todo{}, err
	}
	todo.Priority = defaultPriority(priority)
	if err := r.store.UpdateTodo(todo); err != nil {
		return Todo{}, err
	}
	return r.store.GetTodo(todoID)
}

func (r *Runtime) ListOpenTodos(missionID string) ([]Todo, error) {
	todos, err := r.store.ListTodos(missionID)
	if err != nil {
		return nil, err
	}
	open := make([]Todo, 0, len(todos))
	for _, todo := range todos {
		if todo.Status == TodoStatusDone {
			continue
		}
		open = append(open, todo)
	}
	return open, nil
}

func (r *Runtime) ScheduleTimer(input ScheduleTimerInput) (Timer, error) {
	timer := Timer{
		ID:            fmt.Sprintf("timer-%s-%d", input.MissionID, time.Now().UTC().UnixNano()),
		MissionID:     input.MissionID,
		ThreadID:      input.ThreadID,
		SetByAgentID:  input.SetByAgentID,
		WakeAt:        input.WakeAt,
		ActionType:    input.ActionType,
		ActionPayload: defaultObjectJSON(input.ActionPayload),
		Status:        TimerStatusScheduled,
	}
	if err := r.store.CreateTimer(timer); err != nil {
		return Timer{}, err
	}
	return r.store.GetTimer(timer.ID)
}

func (r *Runtime) TriggerTimer(timerID string) (Timer, error) {
	timer, err := r.store.GetTimer(timerID)
	if err != nil {
		return Timer{}, err
	}
	now := time.Now().UTC()
	timer.Status = TimerStatusTriggered
	timer.TriggeredAt = &now
	if err := r.store.UpdateTimer(timer); err != nil {
		return Timer{}, err
	}
	return r.store.GetTimer(timerID)
}

func (r *Runtime) FailTimer(timerID string) (Timer, error) {
	timer, err := r.store.GetTimer(timerID)
	if err != nil {
		return Timer{}, err
	}
	timer.Status = TimerStatusFailed
	if err := r.store.UpdateTimer(timer); err != nil {
		return Timer{}, err
	}
	return r.store.GetTimer(timerID)
}

func (r *Runtime) CancelTimer(timerID string) (Timer, error) {
	timer, err := r.store.GetTimer(timerID)
	if err != nil {
		return Timer{}, err
	}
	timer.Status = TimerStatusCancelled
	if err := r.store.UpdateTimer(timer); err != nil {
		return Timer{}, err
	}
	return r.store.GetTimer(timerID)
}

func (r *Runtime) ListDueTimers(now time.Time, limit int) ([]Timer, error) {
	return r.store.ListDueTimers(now, limit)
}

func (r *Runtime) ClaimDueTimers(now time.Time, limit int) ([]Timer, error) {
	return r.store.ClaimDueTimers(now, limit)
}

func (r *Runtime) ListDueTodos(now time.Time, limit int) ([]Todo, error) {
	return r.store.ListDueTodos(now, limit)
}

func (r *Runtime) updateTodoStatus(todoID string, status TodoStatus) (Todo, error) {
	todo, err := r.store.GetTodo(todoID)
	if err != nil {
		return Todo{}, err
	}
	todo.Status = status
	if err := r.store.UpdateTodo(todo); err != nil {
		return Todo{}, err
	}
	return r.store.GetTodo(todoID)
}

func marshalStringList(values []string) json.RawMessage {
	if len(values) == 0 {
		return []byte(`[]`)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return []byte(`[]`)
	}
	return encoded
}

func defaultPriority(priority missions.Priority) missions.Priority {
	if priority == "" {
		return missions.PriorityMedium
	}
	return priority
}

type TriggeredTimerHandler func(ctx context.Context, timer Timer) error

type TimerProcessor struct {
	runtime      *Runtime
	handler      TriggeredTimerHandler
	pollInterval time.Duration
	batchLimit   int
	now          func() time.Time
	onError      func(error)
}

type TimerProcessorConfig struct {
	PollInterval time.Duration
	BatchLimit   int
	Now          func() time.Time
	OnError      func(error)
}

func NewTimerProcessor(runtime *Runtime, handler TriggeredTimerHandler, config TimerProcessorConfig) (*TimerProcessor, error) {
	if runtime == nil {
		return nil, fmt.Errorf("execution runtime is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("triggered timer handler is required")
	}
	processor := &TimerProcessor{
		runtime:      runtime,
		handler:      handler,
		pollInterval: config.PollInterval,
		batchLimit:   config.BatchLimit,
		now:          config.Now,
		onError:      config.OnError,
	}
	if processor.pollInterval <= 0 {
		processor.pollInterval = 5 * time.Second
	}
	if processor.batchLimit <= 0 {
		processor.batchLimit = 32
	}
	if processor.now == nil {
		processor.now = func() time.Time { return time.Now().UTC() }
	}
	return processor, nil
}

func (p *TimerProcessor) Run(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("timer processor is required")
	}
	if err := p.ProcessOnce(ctx); err != nil {
		p.reportError(err)
	}
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.ProcessOnce(ctx); err != nil {
				p.reportError(err)
			}
		}
	}
}

func (p *TimerProcessor) ProcessOnce(ctx context.Context) error {
	claimed, err := p.runtime.ClaimDueTimers(p.now(), p.batchLimit)
	if err != nil {
		return err
	}
	var runErr error
	for _, timer := range claimed {
		if err := p.handler(ctx, timer); err != nil {
			if _, failErr := p.runtime.FailTimer(timer.ID); failErr != nil {
				runErr = errors.Join(runErr, fmt.Errorf("handle timer %s: %w (mark failed: %v)", timer.ID, err, failErr))
				continue
			}
			runErr = errors.Join(runErr, fmt.Errorf("handle timer %s: %w", timer.ID, err))
		}
	}
	return runErr
}

func (p *TimerProcessor) reportError(err error) {
	if err == nil {
		return
	}
	if p.onError != nil {
		p.onError(err)
	}
}
