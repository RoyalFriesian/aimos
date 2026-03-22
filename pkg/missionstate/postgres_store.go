package missionstate

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

func (s *PostgresStore) CreateSummary(summary Summary) error {
	if summary.ID == "" {
		return fmt.Errorf("summary id is required")
	}
	if summary.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if summary.Level == "" {
		return fmt.Errorf("summary level is required")
	}
	if summary.Kind == "" {
		return fmt.Errorf("summary kind is required")
	}
	if summary.CoverageStartRef == "" {
		return fmt.Errorf("coverage start ref is required")
	}
	if summary.CoverageEndRef == "" {
		return fmt.Errorf("coverage end ref is required")
	}
	if summary.SummaryText == "" {
		return fmt.Errorf("summary text is required")
	}
	if len(summary.KeyDecisions) == 0 {
		summary.KeyDecisions = []byte(`[]`)
	}
	if len(summary.OpenQuestions) == 0 {
		summary.OpenQuestions = []byte(`[]`)
	}
	if len(summary.Blockers) == 0 {
		summary.Blockers = []byte(`[]`)
	}
	if len(summary.NextActions) == 0 {
		summary.NextActions = []byte(`[]`)
	}
	if summary.CreatedAt.IsZero() {
		summary.CreatedAt = time.Now().UTC()
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_summaries (
			summary_id, mission_id, thread_id, summary_level, summary_kind,
			coverage_start_ref, coverage_end_ref, summary_text, key_decisions_json,
			open_questions_json, blockers_json, next_actions_json, created_at
		) VALUES (
			$1, $2, NULLIF($3,''), $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13
		)
		ON CONFLICT (summary_id) DO NOTHING
	`, summary.ID, summary.MissionID, summary.ThreadID, summary.Level, summary.Kind,
		summary.CoverageStartRef, summary.CoverageEndRef, summary.SummaryText, summary.KeyDecisions,
		summary.OpenQuestions, summary.Blockers, summary.NextActions, summary.CreatedAt)
	return err
}

func (s *PostgresStore) GetLatestSummary(missionID string) (Summary, error) {
	var summary Summary
	err := s.pool.QueryRow(context.Background(), `
		SELECT summary_id, mission_id, COALESCE(thread_id, ''), summary_level, summary_kind,
			coverage_start_ref, coverage_end_ref, summary_text, key_decisions_json,
			open_questions_json, blockers_json, next_actions_json, created_at
		FROM mission_summaries
		WHERE mission_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, missionID).Scan(
		&summary.ID, &summary.MissionID, &summary.ThreadID, &summary.Level, &summary.Kind,
		&summary.CoverageStartRef, &summary.CoverageEndRef, &summary.SummaryText, &summary.KeyDecisions,
		&summary.OpenQuestions, &summary.Blockers, &summary.NextActions, &summary.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, ErrSummaryNotFound
	}
	return summary, err
}

func (s *PostgresStore) ListSummaries(missionID string) ([]Summary, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT summary_id, mission_id, COALESCE(thread_id, ''), summary_level, summary_kind,
			coverage_start_ref, coverage_end_ref, summary_text, key_decisions_json,
			open_questions_json, blockers_json, next_actions_json, created_at
		FROM mission_summaries
		WHERE mission_id = $1
		ORDER BY created_at ASC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var summary Summary
		if err := rows.Scan(
			&summary.ID, &summary.MissionID, &summary.ThreadID, &summary.Level, &summary.Kind,
			&summary.CoverageStartRef, &summary.CoverageEndRef, &summary.SummaryText, &summary.KeyDecisions,
			&summary.OpenQuestions, &summary.Blockers, &summary.NextActions, &summary.CreatedAt,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *PostgresStore) UpsertRollup(rollup Rollup) error {
	if rollup.ID == "" {
		return fmt.Errorf("rollup id is required")
	}
	if rollup.ParentMissionID == "" {
		return fmt.Errorf("parent mission id is required")
	}
	if rollup.ChildMissionID == "" {
		return fmt.Errorf("child mission id is required")
	}
	if rollup.Status == "" {
		return fmt.Errorf("rollup status is required")
	}
	if rollup.Health == "" {
		return fmt.Errorf("rollup health is required")
	}
	if rollup.LatestSummary == "" {
		return fmt.Errorf("latest summary is required")
	}
	if len(rollup.OverdueFlags) == 0 {
		rollup.OverdueFlags = []byte(`[]`)
	}
	if len(rollup.ExecutionSummary) == 0 {
		rollup.ExecutionSummary = []byte(`{}`)
	}
	if rollup.UpdatedAt.IsZero() {
		rollup.UpdatedAt = time.Now().UTC()
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_rollups (
			rollup_id, parent_mission_id, child_mission_id, status, progress_percent,
			health, current_blocker, latest_summary, next_expected_update_at,
			overdue_flags_json, execution_summary_json, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12
		)
		ON CONFLICT (parent_mission_id, child_mission_id)
		DO UPDATE SET
			rollup_id = EXCLUDED.rollup_id,
			status = EXCLUDED.status,
			progress_percent = EXCLUDED.progress_percent,
			health = EXCLUDED.health,
			current_blocker = EXCLUDED.current_blocker,
			latest_summary = EXCLUDED.latest_summary,
			next_expected_update_at = EXCLUDED.next_expected_update_at,
			overdue_flags_json = EXCLUDED.overdue_flags_json,
			execution_summary_json = EXCLUDED.execution_summary_json,
			updated_at = EXCLUDED.updated_at
	`, rollup.ID, rollup.ParentMissionID, rollup.ChildMissionID, string(rollup.Status), rollup.ProgressPercent,
		rollup.Health, rollup.CurrentBlocker, rollup.LatestSummary, rollup.NextExpectedUpdateAt,
		rollup.OverdueFlags, rollup.ExecutionSummary, rollup.UpdatedAt)
	return err
}

func (s *PostgresStore) GetRollup(parentMissionID string, childMissionID string) (Rollup, error) {
	var rollup Rollup
	err := s.pool.QueryRow(context.Background(), `
		SELECT rollup_id, parent_mission_id, child_mission_id, status, progress_percent,
			health, current_blocker, latest_summary, next_expected_update_at,
			overdue_flags_json, execution_summary_json, updated_at
		FROM mission_rollups
		WHERE parent_mission_id = $1 AND child_mission_id = $2
	`, parentMissionID, childMissionID).Scan(
		&rollup.ID, &rollup.ParentMissionID, &rollup.ChildMissionID, &rollup.Status, &rollup.ProgressPercent,
		&rollup.Health, &rollup.CurrentBlocker, &rollup.LatestSummary, &rollup.NextExpectedUpdateAt,
		&rollup.OverdueFlags, &rollup.ExecutionSummary, &rollup.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rollup{}, ErrRollupNotFound
	}
	return rollup, err
}

func (s *PostgresStore) ListRollups(parentMissionID string) ([]Rollup, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT rollup_id, parent_mission_id, child_mission_id, status, progress_percent,
			health, current_blocker, latest_summary, next_expected_update_at,
			overdue_flags_json, execution_summary_json, updated_at
		FROM mission_rollups
		WHERE parent_mission_id = $1
		ORDER BY updated_at DESC
	`, parentMissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rollups []Rollup
	for rows.Next() {
		var rollup Rollup
		if err := rows.Scan(
			&rollup.ID, &rollup.ParentMissionID, &rollup.ChildMissionID, &rollup.Status, &rollup.ProgressPercent,
			&rollup.Health, &rollup.CurrentBlocker, &rollup.LatestSummary, &rollup.NextExpectedUpdateAt,
			&rollup.OverdueFlags, &rollup.ExecutionSummary, &rollup.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rollups = append(rollups, rollup)
	}
	return rollups, rows.Err()
}

var _ Store = (*PostgresStore)(nil)
var _ = missions.MissionStatusActive
