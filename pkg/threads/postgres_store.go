package threads

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (s *PostgresStore) CreateThread(thread Thread) error {
	if thread.ID == "" {
		return fmt.Errorf("thread id is required")
	}
	if thread.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if thread.Title == "" {
		return fmt.Errorf("thread title is required")
	}
	if thread.Summary == "" {
		thread.Summary = thread.Title
	}
	if thread.Context == "" {
		thread.Context = thread.Summary
	}
	if thread.Status == "" {
		thread.Status = ThreadStatusActive
	}
	if thread.CreatedAt.IsZero() {
		thread.CreatedAt = time.Now().UTC()
	}
	if thread.UpdatedAt.IsZero() {
		thread.UpdatedAt = thread.CreatedAt
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO threads (
			thread_id, mission_id, root_mission_id, parent_thread_id, thread_kind, title,
			summary, context, status, current_mode, owner_agent_id, waiting_until,
			last_activity_at, created_at, updated_at
		) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (thread_id) DO NOTHING
	`, thread.ID, thread.MissionID, thread.RootMissionID, thread.ParentThreadID, thread.Kind, thread.Title,
		thread.Summary, thread.Context, string(thread.Status), thread.CurrentMode, thread.OwnerAgentID, thread.WaitingUntil,
		thread.UpdatedAt, thread.CreatedAt, thread.UpdatedAt)
	return err
}

func (s *PostgresStore) GetThread(threadID string) (Thread, error) {
	var thread Thread
	row := s.pool.QueryRow(context.Background(), `
		SELECT thread_id, mission_id, root_mission_id, COALESCE(parent_thread_id, ''), thread_kind, title,
			summary, context, owner_agent_id, current_mode, status, waiting_until, created_at, updated_at
		FROM threads
		WHERE thread_id = $1
	`, threadID)
	err := row.Scan(
		&thread.ID, &thread.MissionID, &thread.RootMissionID, &thread.ParentThreadID, &thread.Kind, &thread.Title,
		&thread.Summary, &thread.Context, &thread.OwnerAgentID, &thread.CurrentMode, &thread.Status,
		&thread.WaitingUntil, &thread.CreatedAt, &thread.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrThreadNotFound
	}
	return thread, err
}

func (s *PostgresStore) ListByMission(missionID string) ([]Thread, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT thread_id, mission_id, root_mission_id, COALESCE(parent_thread_id, ''), thread_kind, title,
			summary, context, owner_agent_id, current_mode, status, waiting_until, created_at, updated_at
		FROM threads
		WHERE mission_id = $1
		ORDER BY created_at ASC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threadsForMission []Thread
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(
			&thread.ID, &thread.MissionID, &thread.RootMissionID, &thread.ParentThreadID, &thread.Kind, &thread.Title,
			&thread.Summary, &thread.Context, &thread.OwnerAgentID, &thread.CurrentMode, &thread.Status,
			&thread.WaitingUntil, &thread.CreatedAt, &thread.UpdatedAt,
		); err != nil {
			return nil, err
		}
		threadsForMission = append(threadsForMission, thread)
	}
	return threadsForMission, rows.Err()
}

func (s *PostgresStore) SearchReusableThreads(query string, limit int) ([]ReusableThreadMatch, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT thread_id, mission_id, root_mission_id, COALESCE(parent_thread_id, ''), thread_kind, title,
			summary, context, owner_agent_id, current_mode, status, waiting_until, created_at, updated_at
		FROM threads
		WHERE status = ANY($1)
		ORDER BY updated_at DESC
	`, []string{string(ThreadStatusCompleted), string(ThreadStatusFinished), string(ThreadStatusSuperseded)})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []Thread
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(
			&thread.ID, &thread.MissionID, &thread.RootMissionID, &thread.ParentThreadID, &thread.Kind, &thread.Title,
			&thread.Summary, &thread.Context, &thread.OwnerAgentID, &thread.CurrentMode, &thread.Status,
			&thread.WaitingUntil, &thread.CreatedAt, &thread.UpdatedAt,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return findReusableThreadMatches(candidates, query, limit), nil
}

func (s *PostgresStore) AppendMessage(message Message) error {
	if message.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if message.Role == "" {
		return fmt.Errorf("message role is required")
	}
	if message.Content == "" {
		return fmt.Errorf("message content is required")
	}
	if message.ID == "" {
		message.ID = fmt.Sprintf("msg-%d", time.Now().UTC().UnixNano())
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	if message.AuthorAgentID == "" {
		message.AuthorAgentID = defaultAuthorAgentID(message.Role)
	}
	if message.AuthorRole == "" {
		message.AuthorRole = string(message.Role)
	}
	if message.MessageType == "" {
		message.MessageType = defaultMessageType(message.Role)
	}
	if len(message.ContentJSON) == 0 {
		message.ContentJSON = []byte(`{}`)
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO thread_messages (
			message_id, thread_id, mission_id, root_mission_id, author_agent_id, author_role,
			message_type, content_text, content_json, reply_to_message_id, created_at
		)
		SELECT $1, t.thread_id, t.mission_id, t.root_mission_id, $2, $3, $4, $5, $6, NULLIF($7,''), $8
		FROM threads t
		WHERE t.thread_id = $9
	`, message.ID, message.AuthorAgentID, message.AuthorRole, message.MessageType, message.Content, message.ContentJSON, message.ReplyToMessageID, message.CreatedAt, message.ThreadID)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(context.Background(), `
		UPDATE threads
		SET last_activity_at = $2, updated_at = $2
		WHERE thread_id = $1
	`, message.ThreadID, message.CreatedAt)
	return err
}

func (s *PostgresStore) ListMessages(threadID string) ([]Message, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT message_id, thread_id, author_agent_id, author_role, message_type,
			content_text, content_json, COALESCE(reply_to_message_id, ''), created_at
		FROM thread_messages
		WHERE thread_id = $1
		ORDER BY created_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(
			&message.ID, &message.ThreadID, &message.AuthorAgentID, &message.AuthorRole,
			&message.MessageType, &message.Content, &message.ContentJSON, &message.ReplyToMessageID, &message.CreatedAt,
		); err != nil {
			return nil, err
		}
		message.Role = persistedMessageRole(message.AuthorRole, message.MessageType)
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		if _, err := s.GetThread(threadID); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func persistedMessageRole(authorRole string, messageType string) Role {
	switch Role(authorRole) {
	case RoleSystem, RoleUser, RoleAssistant:
		return Role(authorRole)
	}
	switch authorRole {
	case "client":
		return RoleUser
	case "ceo":
		return RoleAssistant
	case "system":
		return RoleSystem
	}
	if messageType == "client_message" || messageType == "client_action_request" {
		return RoleUser
	}
	if messageType == "ceo_message" || messageType == "ceo_action_result" || messageType == "timer_triggered" || messageType == "timer_escalated" {
		return RoleAssistant
	}
	return RoleAssistant
}

func (s *PostgresStore) UpdateThreadMode(threadID string, mode string) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE threads
		SET current_mode = $2, updated_at = NOW()
		WHERE thread_id = $1
	`, threadID, mode)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateThreadOwner(threadID string, ownerAgentID string) error {
	if ownerAgentID == "" {
		return fmt.Errorf("owner agent id is required")
	}

	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE threads
		SET owner_agent_id = $2, updated_at = NOW()
		WHERE thread_id = $1
	`, threadID, ownerAgentID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func defaultAuthorAgentID(role Role) string {
	switch role {
	case RoleAssistant:
		return "ceo"
	case RoleSystem:
		return "system"
	default:
		return "user"
	}
}

func defaultMessageType(role Role) string {
	switch role {
	case RoleAssistant:
		return "ceo_message"
	case RoleSystem:
		return "system_post"
	default:
		return "client_message"
	}
}


func (s *PostgresStore) ListRootThreads() ([]Thread, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT thread_id, mission_id, root_mission_id, COALESCE(parent_thread_id, ''), thread_kind, title,
			summary, context, owner_agent_id, current_mode, status, waiting_until, created_at, updated_at
		FROM threads
		WHERE parent_thread_id IS NULL OR parent_thread_id = ''
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.ID, &t.MissionID, &t.RootMissionID, &t.ParentThreadID, &t.Kind, &t.Title,
			&t.Summary, &t.Context, &t.OwnerAgentID, &t.CurrentMode, &t.Status, &t.WaitingUntil,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, rows.Err()
}
