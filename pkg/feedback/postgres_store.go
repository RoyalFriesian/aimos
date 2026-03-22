package feedback

import (
	"context"
	"errors"
	"fmt"

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

func (s *PostgresStore) CreateFeedback(record Record) error {
	if record.ID == "" {
		return fmt.Errorf("feedback id is required")
	}
	if record.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if record.ResponseID == "" {
		return fmt.Errorf("response id is required")
	}
	if record.AnalysisStatus == "" {
		record.AnalysisStatus = AnalysisStatusRaw
	}
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO ceo_feedback (
			feedback_id, mission_id, thread_id, response_id, client_message_id, task_id, trace_id,
			rating, reason, categories_json, client_message, ceo_response, mode,
			artifact_paths_json, todo_refs_json, context_summary, evidence_refs_json,
			enriched_by_feedback_agent, analysis_status, created_at
		) VALUES (
			$1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),
			$8,$9,$10,$11,$12,$13,
			$14,$15,$16,$17,
			$18,$19,$20
		)
		ON CONFLICT (feedback_id) DO NOTHING
	`, record.ID, record.MissionID, record.ThreadID, record.ResponseID, record.ClientMessageID, record.TaskID, record.TraceID,
		record.Rating, record.Reason, record.Categories, record.ClientMessage, record.CEOResponse, record.Mode,
		record.ArtifactPaths, record.TodoRefs, record.ContextSummary, record.EvidenceRefs,
		record.EnrichedByFeedbackAgent, record.AnalysisStatus, record.CreatedAt)
	return err
}

func (s *PostgresStore) GetFeedback(feedbackID string) (Record, error) {
	row := s.pool.QueryRow(context.Background(), `
		SELECT feedback_id, mission_id, thread_id, response_id, COALESCE(client_message_id, ''),
			COALESCE(task_id, ''), COALESCE(trace_id, ''), rating, reason, categories_json,
			client_message, ceo_response, mode, artifact_paths_json, todo_refs_json,
			context_summary, evidence_refs_json, enriched_by_feedback_agent, analysis_status, created_at
		FROM ceo_feedback
		WHERE feedback_id = $1
	`, feedbackID)
	var record Record
	err := row.Scan(
		&record.ID, &record.MissionID, &record.ThreadID, &record.ResponseID, &record.ClientMessageID,
		&record.TaskID, &record.TraceID, &record.Rating, &record.Reason, &record.Categories,
		&record.ClientMessage, &record.CEOResponse, &record.Mode, &record.ArtifactPaths, &record.TodoRefs,
		&record.ContextSummary, &record.EvidenceRefs, &record.EnrichedByFeedbackAgent, &record.AnalysisStatus, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrFeedbackNotFound
	}
	return record, err
}

func (s *PostgresStore) ListByThread(threadID string) ([]Record, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT feedback_id, mission_id, thread_id, response_id, COALESCE(client_message_id, ''),
			COALESCE(task_id, ''), COALESCE(trace_id, ''), rating, reason, categories_json,
			client_message, ceo_response, mode, artifact_paths_json, todo_refs_json,
			context_summary, evidence_refs_json, enriched_by_feedback_agent, analysis_status, created_at
		FROM ceo_feedback
		WHERE thread_id = $1
		ORDER BY created_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.ID, &record.MissionID, &record.ThreadID, &record.ResponseID, &record.ClientMessageID,
			&record.TaskID, &record.TraceID, &record.Rating, &record.Reason, &record.Categories,
			&record.ClientMessage, &record.CEOResponse, &record.Mode, &record.ArtifactPaths, &record.TodoRefs,
			&record.ContextSummary, &record.EvidenceRefs, &record.EnrichedByFeedbackAgent, &record.AnalysisStatus, &record.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
