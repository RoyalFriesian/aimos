package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) CreateTodo(todo Todo) error {
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
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = time.Now().UTC()
	}
	if todo.UpdatedAt.IsZero() {
		todo.UpdatedAt = todo.CreatedAt
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_todos (
			todo_id, mission_id, thread_id, title, description, owner_agent_id,
			status, priority, due_at, depends_on_json, artifact_paths_json, created_at, updated_at
		) VALUES (
			$1, $2, NULLIF($3,''), $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (todo_id) DO NOTHING
	`, todo.ID, todo.MissionID, todo.ThreadID, todo.Title, todo.Description, todo.OwnerAgentID,
		string(todo.Status), string(todo.Priority), todo.DueAt, todo.DependsOn, todo.ArtifactPaths, todo.CreatedAt, todo.UpdatedAt)
	return err
}

func (s *PostgresStore) GetTodo(todoID string) (Todo, error) {
	var todo Todo
	err := s.pool.QueryRow(context.Background(), `
		SELECT todo_id, mission_id, COALESCE(thread_id, ''), title, description, owner_agent_id,
			status, priority, due_at, depends_on_json, artifact_paths_json, created_at, updated_at
		FROM mission_todos
		WHERE todo_id = $1
	`, todoID).Scan(&todo.ID, &todo.MissionID, &todo.ThreadID, &todo.Title, &todo.Description, &todo.OwnerAgentID,
		&todo.Status, &todo.Priority, &todo.DueAt, &todo.DependsOn, &todo.ArtifactPaths, &todo.CreatedAt, &todo.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Todo{}, ErrTodoNotFound
	}
	return todo, err
}

func (s *PostgresStore) ListTodos(missionID string) ([]Todo, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT todo_id, mission_id, COALESCE(thread_id, ''), title, description, owner_agent_id,
			status, priority, due_at, depends_on_json, artifact_paths_json, created_at, updated_at
		FROM mission_todos
		WHERE mission_id = $1
		ORDER BY CASE priority
			WHEN 'critical' THEN 0
			WHEN 'high' THEN 1
			WHEN 'medium' THEN 2
			ELSE 3 END,
			created_at ASC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.MissionID, &todo.ThreadID, &todo.Title, &todo.Description, &todo.OwnerAgentID,
			&todo.Status, &todo.Priority, &todo.DueAt, &todo.DependsOn, &todo.ArtifactPaths, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	return todos, rows.Err()
}

func (s *PostgresStore) UpdateTodo(todo Todo) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE mission_todos
		SET mission_id = $2,
			thread_id = NULLIF($3,''),
			title = $4,
			description = $5,
			owner_agent_id = $6,
			status = $7,
			priority = $8,
			due_at = $9,
			depends_on_json = $10,
			artifact_paths_json = $11,
			updated_at = $12
		WHERE todo_id = $1
	`, todo.ID, todo.MissionID, todo.ThreadID, todo.Title, todo.Description, todo.OwnerAgentID,
		string(todo.Status), string(todo.Priority), todo.DueAt, defaultArrayJSON(todo.DependsOn), defaultArrayJSON(todo.ArtifactPaths), time.Now().UTC())
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTodoNotFound
	}
	return nil
}

func (s *PostgresStore) ListDueTodos(dueBefore time.Time, limit int) ([]Todo, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT todo_id, mission_id, COALESCE(thread_id, ''), title, description, owner_agent_id,
			status, priority, due_at, depends_on_json, artifact_paths_json, created_at, updated_at
		FROM mission_todos
		WHERE due_at IS NOT NULL
			AND due_at <= $1
			AND status <> 'done'
		ORDER BY due_at ASC
		LIMIT $2
	`, dueBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.MissionID, &todo.ThreadID, &todo.Title, &todo.Description, &todo.OwnerAgentID,
			&todo.Status, &todo.Priority, &todo.DueAt, &todo.DependsOn, &todo.ArtifactPaths, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	return todos, rows.Err()
}

func (s *PostgresStore) CreateTimer(timer Timer) error {
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
	if timer.Status == "" {
		timer.Status = TimerStatusScheduled
	}
	if len(timer.ActionPayload) == 0 {
		timer.ActionPayload = []byte(`{}`)
	}
	if timer.CreatedAt.IsZero() {
		timer.CreatedAt = time.Now().UTC()
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_timers (
			timer_id, mission_id, thread_id, set_by_agent_id, wake_at,
			action_type, action_payload_json, status, created_at, triggered_at
		) VALUES (
			$1, $2, NULLIF($3,''), $4, $5,
			$6, $7, $8, $9, $10
		)
		ON CONFLICT (timer_id) DO NOTHING
	`, timer.ID, timer.MissionID, timer.ThreadID, timer.SetByAgentID, timer.WakeAt,
		timer.ActionType, timer.ActionPayload, string(timer.Status), timer.CreatedAt, timer.TriggeredAt)
	return err
}

func (s *PostgresStore) GetTimer(timerID string) (Timer, error) {
	var timer Timer
	err := s.pool.QueryRow(context.Background(), `
		SELECT timer_id, mission_id, COALESCE(thread_id, ''), set_by_agent_id, wake_at,
			action_type, action_payload_json, status, created_at, triggered_at
		FROM mission_timers
		WHERE timer_id = $1
	`, timerID).Scan(&timer.ID, &timer.MissionID, &timer.ThreadID, &timer.SetByAgentID, &timer.WakeAt,
		&timer.ActionType, &timer.ActionPayload, &timer.Status, &timer.CreatedAt, &timer.TriggeredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Timer{}, ErrTimerNotFound
	}
	return timer, err
}

func (s *PostgresStore) ListTimers(missionID string) ([]Timer, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT timer_id, mission_id, COALESCE(thread_id, ''), set_by_agent_id, wake_at,
			action_type, action_payload_json, status, created_at, triggered_at
		FROM mission_timers
		WHERE mission_id = $1
		ORDER BY wake_at ASC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timers []Timer
	for rows.Next() {
		var timer Timer
		if err := rows.Scan(&timer.ID, &timer.MissionID, &timer.ThreadID, &timer.SetByAgentID, &timer.WakeAt,
			&timer.ActionType, &timer.ActionPayload, &timer.Status, &timer.CreatedAt, &timer.TriggeredAt); err != nil {
			return nil, err
		}
		timers = append(timers, timer)
	}
	return timers, rows.Err()
}

func (s *PostgresStore) UpdateTimer(timer Timer) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE mission_timers
		SET mission_id = $2,
			thread_id = NULLIF($3,''),
			set_by_agent_id = $4,
			wake_at = $5,
			action_type = $6,
			action_payload_json = $7,
			status = $8,
			triggered_at = $9
		WHERE timer_id = $1
	`, timer.ID, timer.MissionID, timer.ThreadID, timer.SetByAgentID, timer.WakeAt,
		timer.ActionType, defaultObjectJSON(timer.ActionPayload), string(timer.Status), timer.TriggeredAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTimerNotFound
	}
	return nil
}

func (s *PostgresStore) ListDueTimers(wakeBefore time.Time, limit int) ([]Timer, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT timer_id, mission_id, COALESCE(thread_id, ''), set_by_agent_id, wake_at,
			action_type, action_payload_json, status, created_at, triggered_at
		FROM mission_timers
		WHERE wake_at <= $1
			AND status = 'scheduled'
		ORDER BY wake_at ASC
		LIMIT $2
	`, wakeBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timers []Timer
	for rows.Next() {
		var timer Timer
		if err := rows.Scan(&timer.ID, &timer.MissionID, &timer.ThreadID, &timer.SetByAgentID, &timer.WakeAt,
			&timer.ActionType, &timer.ActionPayload, &timer.Status, &timer.CreatedAt, &timer.TriggeredAt); err != nil {
			return nil, err
		}
		timers = append(timers, timer)
	}
	return timers, rows.Err()
}

func (s *PostgresStore) ClaimDueTimers(wakeBefore time.Time, limit int) ([]Timer, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		WITH due AS (
			SELECT timer_id
			FROM mission_timers
			WHERE wake_at <= $1
				AND status = 'scheduled'
			ORDER BY wake_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		), claimed AS (
			UPDATE mission_timers mt
			SET status = 'triggered',
				triggered_at = NOW()
			FROM due
			WHERE mt.timer_id = due.timer_id
			RETURNING mt.timer_id, mt.mission_id, COALESCE(mt.thread_id, '') AS thread_id, mt.set_by_agent_id,
				mt.wake_at, mt.action_type, mt.action_payload_json, mt.status, mt.created_at, mt.triggered_at
		)
		SELECT timer_id, mission_id, thread_id, set_by_agent_id, wake_at,
			action_type, action_payload_json, status, created_at, triggered_at
		FROM claimed
		ORDER BY wake_at ASC
	`, wakeBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timers []Timer
	for rows.Next() {
		var timer Timer
		if err := rows.Scan(&timer.ID, &timer.MissionID, &timer.ThreadID, &timer.SetByAgentID, &timer.WakeAt,
			&timer.ActionType, &timer.ActionPayload, &timer.Status, &timer.CreatedAt, &timer.TriggeredAt); err != nil {
			return nil, err
		}
		timers = append(timers, timer)
	}
	return timers, rows.Err()
}

func defaultArrayJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	return raw
}

func defaultObjectJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

var _ Store = (*PostgresStore)(nil)
