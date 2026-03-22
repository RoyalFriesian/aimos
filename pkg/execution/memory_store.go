package execution

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

type MemoryStore struct {
	mu      sync.RWMutex
	todos   map[string]Todo
	timers  map[string]Timer
	byScope map[string][]string
	byWake  map[string][]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		todos:   map[string]Todo{},
		timers:  map[string]Timer{},
		byScope: map[string][]string{},
		byWake:  map[string][]string{},
	}
}

func (s *MemoryStore) CreateTodo(todo Todo) error {
	if todo.ID == "" {
		return fmt.Errorf("todo id is required")
	}
	if todo.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if todo.Title == "" {
		return fmt.Errorf("todo title is required")
	}
	if todo.Description == "" {
		return fmt.Errorf("todo description is required")
	}
	if todo.OwnerAgentID == "" {
		return fmt.Errorf("todo owner agent id is required")
	}
	if todo.Status == "" {
		todo.Status = TodoStatusTodo
	}
	if todo.Priority == "" {
		todo.Priority = missions.PriorityMedium
	}
	if len(todo.DependsOn) == 0 {
		todo.DependsOn = []byte(`[]`)
	}
	if len(todo.ArtifactPaths) == 0 {
		todo.ArtifactPaths = []byte(`[]`)
	}
	now := time.Now().UTC()
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = now
	}
	todo.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.todos[todo.ID]; exists {
		return nil
	}
	s.todos[todo.ID] = todo
	s.byScope[todo.MissionID] = append(s.byScope[todo.MissionID], todo.ID)
	return nil
}

func (s *MemoryStore) GetTodo(todoID string) (Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	todo, exists := s.todos[todoID]
	if !exists {
		return Todo{}, ErrTodoNotFound
	}
	return todo, nil
}

func (s *MemoryStore) ListTodos(missionID string) ([]Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	todoIDs := s.byScope[missionID]
	todos := make([]Todo, 0, len(todoIDs))
	for _, todoID := range todoIDs {
		todos = append(todos, s.todos[todoID])
	}
	sort.SliceStable(todos, func(i, j int) bool {
		if todos[i].Priority == todos[j].Priority {
			return todos[i].CreatedAt.Before(todos[j].CreatedAt)
		}
		return priorityRank(todos[i].Priority) < priorityRank(todos[j].Priority)
	})
	return todos, nil
}

func (s *MemoryStore) UpdateTodo(todo Todo) error {
	if todo.ID == "" {
		return fmt.Errorf("todo id is required")
	}
	if len(todo.DependsOn) == 0 {
		todo.DependsOn = []byte(`[]`)
	}
	if len(todo.ArtifactPaths) == 0 {
		todo.ArtifactPaths = []byte(`[]`)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.todos[todo.ID]
	if !exists {
		return ErrTodoNotFound
	}
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = existing.CreatedAt
	}
	todo.UpdatedAt = time.Now().UTC()
	s.todos[todo.ID] = todo
	return nil
}

func (s *MemoryStore) ListDueTodos(dueBefore time.Time, limit int) ([]Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates := make([]Todo, 0, len(s.todos))
	for _, todo := range s.todos {
		if todo.DueAt == nil {
			continue
		}
		if todo.Status == TodoStatusDone {
			continue
		}
		if todo.DueAt.After(dueBefore) {
			continue
		}
		candidates = append(candidates, todo)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].DueAt.Equal(*candidates[j].DueAt) {
			return priorityRank(candidates[i].Priority) < priorityRank(candidates[j].Priority)
		}
		return candidates[i].DueAt.Before(*candidates[j].DueAt)
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (s *MemoryStore) CreateTimer(timer Timer) error {
	if timer.ID == "" {
		return fmt.Errorf("timer id is required")
	}
	if timer.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if timer.SetByAgentID == "" {
		return fmt.Errorf("set by agent id is required")
	}
	if timer.WakeAt.IsZero() {
		return fmt.Errorf("wake at is required")
	}
	if timer.ActionType == "" {
		return fmt.Errorf("action type is required")
	}
	if len(timer.ActionPayload) == 0 {
		timer.ActionPayload = []byte(`{}`)
	}
	if timer.Status == "" {
		timer.Status = TimerStatusScheduled
	}
	if timer.CreatedAt.IsZero() {
		timer.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.timers[timer.ID]; exists {
		return nil
	}
	s.timers[timer.ID] = timer
	s.byWake[timer.MissionID] = append(s.byWake[timer.MissionID], timer.ID)
	return nil
}

func (s *MemoryStore) GetTimer(timerID string) (Timer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	timer, exists := s.timers[timerID]
	if !exists {
		return Timer{}, ErrTimerNotFound
	}
	return timer, nil
}

func (s *MemoryStore) ListTimers(missionID string) ([]Timer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	timerIDs := s.byWake[missionID]
	timers := make([]Timer, 0, len(timerIDs))
	for _, timerID := range timerIDs {
		timers = append(timers, s.timers[timerID])
	}
	sort.SliceStable(timers, func(i, j int) bool {
		return timers[i].WakeAt.Before(timers[j].WakeAt)
	})
	return timers, nil
}

func (s *MemoryStore) UpdateTimer(timer Timer) error {
	if timer.ID == "" {
		return fmt.Errorf("timer id is required")
	}
	if len(timer.ActionPayload) == 0 {
		timer.ActionPayload = []byte(`{}`)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.timers[timer.ID]
	if !exists {
		return ErrTimerNotFound
	}
	if timer.CreatedAt.IsZero() {
		timer.CreatedAt = existing.CreatedAt
	}
	s.timers[timer.ID] = timer
	return nil
}

func (s *MemoryStore) ListDueTimers(wakeBefore time.Time, limit int) ([]Timer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates := make([]Timer, 0, len(s.timers))
	for _, timer := range s.timers {
		if timer.Status != TimerStatusScheduled {
			continue
		}
		if timer.WakeAt.After(wakeBefore) {
			continue
		}
		candidates = append(candidates, timer)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].WakeAt.Before(candidates[j].WakeAt)
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (s *MemoryStore) ClaimDueTimers(wakeBefore time.Time, limit int) ([]Timer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := make([]Timer, 0, len(s.timers))
	for _, timer := range s.timers {
		if timer.Status != TimerStatusScheduled {
			continue
		}
		if timer.WakeAt.After(wakeBefore) {
			continue
		}
		candidates = append(candidates, timer)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].WakeAt.Before(candidates[j].WakeAt)
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	claimedAt := time.Now().UTC()
	claimed := make([]Timer, 0, len(candidates))
	for _, timer := range candidates {
		timer.Status = TimerStatusTriggered
		timer.TriggeredAt = &claimedAt
		s.timers[timer.ID] = timer
		claimed = append(claimed, timer)
	}
	return claimed, nil
}

func priorityRank(priority missions.Priority) int {
	switch priority {
	case missions.PriorityCritical:
		return 0
	case missions.PriorityHigh:
		return 1
	case missions.PriorityMedium:
		return 2
	default:
		return 3
	}
}

var _ Store = (*MemoryStore)(nil)
