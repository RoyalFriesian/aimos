package missions

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

func (s *PostgresStore) CreateProgram(program Program) error {
	if program.ID == "" {
		return fmt.Errorf("program id is required")
	}
	if program.ClientID == "" {
		return fmt.Errorf("client id is required")
	}
	if program.Title == "" {
		return fmt.Errorf("program title is required")
	}
	if program.Status == "" {
		program.Status = ProgramStatusDrafted
	}
	if program.CreatedAt.IsZero() {
		program.CreatedAt = time.Now().UTC()
	}
	if program.UpdatedAt.IsZero() {
		program.UpdatedAt = program.CreatedAt
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO programs (program_id, client_id, title, root_mission_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, $6, $7)
		ON CONFLICT (program_id) DO NOTHING
	`, program.ID, program.ClientID, program.Title, program.RootMissionID, string(program.Status), program.CreatedAt, program.UpdatedAt)
	return err
}

func (s *PostgresStore) GetProgram(programID string) (Program, error) {
	var program Program
	err := s.pool.QueryRow(context.Background(), `
		SELECT program_id, client_id, title, COALESCE(root_mission_id, ''), status, created_at, updated_at
		FROM programs
		WHERE program_id = $1
	`, programID).Scan(&program.ID, &program.ClientID, &program.Title, &program.RootMissionID, &program.Status, &program.CreatedAt, &program.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Program{}, ErrProgramNotFound
	}
	return program, err
}

func (s *PostgresStore) UpdateProgram(program Program) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE programs
		SET client_id = $2, title = $3, root_mission_id = NULLIF($4,''), status = $5, updated_at = $6
		WHERE program_id = $1
	`, program.ID, program.ClientID, program.Title, program.RootMissionID, string(program.Status), time.Now().UTC())
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrProgramNotFound
	}
	return nil
}

func (s *PostgresStore) CreateMission(mission Mission) error {
	if mission.ID == "" {
		return fmt.Errorf("mission id is required")
	}
	if mission.ProgramID == "" {
		return fmt.Errorf("program id is required")
	}
	if mission.Title == "" {
		return fmt.Errorf("mission title is required")
	}
	if mission.OwnerAgentID == "" {
		return fmt.Errorf("owner agent id is required")
	}
	if mission.OwnerRole == "" {
		return fmt.Errorf("owner role is required")
	}
	if mission.MissionType == "" {
		return fmt.Errorf("mission type is required")
	}
	if mission.Charter == "" {
		return fmt.Errorf("mission charter is required")
	}
	if mission.Goal == "" {
		return fmt.Errorf("mission goal is required")
	}
	if mission.Scope == "" {
		return fmt.Errorf("mission scope is required")
	}
	if mission.AuthorityLevel == "" {
		return fmt.Errorf("authority level is required")
	}
	if mission.Status == "" {
		mission.Status = MissionStatusDrafted
	}
	if len(mission.ReuseTrace) == 0 {
		mission.ReuseTrace = []byte(`[]`)
	}
	if mission.Priority == "" {
		mission.Priority = PriorityMedium
	}
	if mission.RiskLevel == "" {
		mission.RiskLevel = PriorityMedium
	}
	if len(mission.Constraints) == 0 {
		mission.Constraints = []byte(`{}`)
	}
	if len(mission.AcceptanceCriteria) == 0 {
		mission.AcceptanceCriteria = []byte(`[]`)
	}
	if len(mission.DelegationPolicy) == 0 {
		mission.DelegationPolicy = []byte(`{}`)
	}
	if mission.CreatedAt.IsZero() {
		mission.CreatedAt = time.Now().UTC()
	}
	if mission.UpdatedAt.IsZero() {
		mission.UpdatedAt = mission.CreatedAt
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO missions (
			mission_id, program_id, parent_mission_id, root_mission_id, owning_thread_id, owner_agent_id,
			owner_role, mission_type, title, charter, goal, scope, reuse_trace_json, constraints_json,
			acceptance_criteria_json, authority_level, delegation_policy_json, status, priority,
			risk_level, progress_percent, waiting_until, created_at, updated_at, closed_at
		) VALUES (
			$1,$2,NULLIF($3,''),$4,NULLIF($5,''),$6,
			$7,$8,$9,$10,$11,$12,$13,$14,
			$15,$16,$17,$18,$19,
			$20,$21,$22,$23,$24,$25
		)
		ON CONFLICT (mission_id) DO NOTHING
	`, mission.ID, mission.ProgramID, mission.ParentMissionID, mission.RootMissionID, mission.OwningThreadID, mission.OwnerAgentID,
		mission.OwnerRole, mission.MissionType, mission.Title, mission.Charter, mission.Goal, mission.Scope, mission.ReuseTrace, mission.Constraints,
		mission.AcceptanceCriteria, mission.AuthorityLevel, mission.DelegationPolicy, string(mission.Status), string(mission.Priority),
		string(mission.RiskLevel), mission.ProgressPercent, mission.WaitingUntil, mission.CreatedAt, mission.UpdatedAt, mission.ClosedAt)
	return err
}

func (s *PostgresStore) GetMission(missionID string) (Mission, error) {
	var mission Mission
	err := s.pool.QueryRow(context.Background(), `
		SELECT mission_id, program_id, COALESCE(parent_mission_id, ''), root_mission_id, COALESCE(owning_thread_id, ''),
			owner_agent_id, owner_role, mission_type, title, charter, goal, scope, reuse_trace_json, constraints_json,
			acceptance_criteria_json, authority_level, delegation_policy_json, status, priority, risk_level,
			progress_percent, waiting_until, created_at, updated_at, closed_at
		FROM missions
		WHERE mission_id = $1
	`, missionID).Scan(
		&mission.ID, &mission.ProgramID, &mission.ParentMissionID, &mission.RootMissionID, &mission.OwningThreadID,
		&mission.OwnerAgentID, &mission.OwnerRole, &mission.MissionType, &mission.Title, &mission.Charter,
		&mission.Goal, &mission.Scope, &mission.ReuseTrace, &mission.Constraints, &mission.AcceptanceCriteria, &mission.AuthorityLevel,
		&mission.DelegationPolicy, &mission.Status, &mission.Priority, &mission.RiskLevel, &mission.ProgressPercent,
		&mission.WaitingUntil, &mission.CreatedAt, &mission.UpdatedAt, &mission.ClosedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Mission{}, ErrMissionNotFound
	}
	return mission, err
}

func (s *PostgresStore) UpdateMission(mission Mission) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE missions
		SET program_id = $2,
			parent_mission_id = NULLIF($3,''),
			root_mission_id = $4,
			owning_thread_id = NULLIF($5,''),
			owner_agent_id = $6,
			owner_role = $7,
			mission_type = $8,
			title = $9,
			charter = $10,
			goal = $11,
			scope = $12,
			reuse_trace_json = $13,
			constraints_json = $14,
			acceptance_criteria_json = $15,
			authority_level = $16,
			delegation_policy_json = $17,
			status = $18,
			priority = $19,
			risk_level = $20,
			progress_percent = $21,
			waiting_until = $22,
			updated_at = $23,
			closed_at = $24
		WHERE mission_id = $1
	`, mission.ID, mission.ProgramID, mission.ParentMissionID, mission.RootMissionID, mission.OwningThreadID,
		mission.OwnerAgentID, mission.OwnerRole, mission.MissionType, mission.Title, mission.Charter, mission.Goal,
		mission.Scope, defaultArrayJSON(mission.ReuseTrace), defaultObjectJSON(mission.Constraints), defaultArrayJSON(mission.AcceptanceCriteria), mission.AuthorityLevel,
		defaultObjectJSON(mission.DelegationPolicy), string(mission.Status), string(mission.Priority), string(mission.RiskLevel),
		mission.ProgressPercent, mission.WaitingUntil, time.Now().UTC(), mission.ClosedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrMissionNotFound
	}
	return nil
}

func (s *PostgresStore) ListChildMissions(parentMissionID string) ([]Mission, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT mission_id, program_id, COALESCE(parent_mission_id, ''), root_mission_id, COALESCE(owning_thread_id, ''),
			owner_agent_id, owner_role, mission_type, title, charter, goal, scope, reuse_trace_json, constraints_json,
			acceptance_criteria_json, authority_level, delegation_policy_json, status, priority, risk_level,
			progress_percent, waiting_until, created_at, updated_at, closed_at
		FROM missions
		WHERE parent_mission_id = $1
		ORDER BY created_at ASC
	`, parentMissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var missions []Mission
	for rows.Next() {
		var mission Mission
		if err := rows.Scan(
			&mission.ID, &mission.ProgramID, &mission.ParentMissionID, &mission.RootMissionID, &mission.OwningThreadID,
			&mission.OwnerAgentID, &mission.OwnerRole, &mission.MissionType, &mission.Title, &mission.Charter,
			&mission.Goal, &mission.Scope, &mission.ReuseTrace, &mission.Constraints, &mission.AcceptanceCriteria, &mission.AuthorityLevel,
			&mission.DelegationPolicy, &mission.Status, &mission.Priority, &mission.RiskLevel, &mission.ProgressPercent,
			&mission.WaitingUntil, &mission.CreatedAt, &mission.UpdatedAt, &mission.ClosedAt,
		); err != nil {
			return nil, err
		}
		missions = append(missions, mission)
	}
	return missions, rows.Err()
}

func (s *PostgresStore) SearchReusableMissions(query string, limit int) ([]ReusableMissionMatch, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT mission_id, program_id, COALESCE(parent_mission_id, ''), root_mission_id, COALESCE(owning_thread_id, ''),
			owner_agent_id, owner_role, mission_type, title, charter, goal, scope, reuse_trace_json, constraints_json,
			acceptance_criteria_json, authority_level, delegation_policy_json, status, priority, risk_level,
			progress_percent, waiting_until, created_at, updated_at, closed_at
		FROM missions
		WHERE status = ANY($1)
		ORDER BY updated_at DESC
	`, []string{string(MissionStatusCompleted), string(MissionStatusFinished), string(MissionStatusSuperseded)})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []Mission
	for rows.Next() {
		var mission Mission
		if err := rows.Scan(
			&mission.ID, &mission.ProgramID, &mission.ParentMissionID, &mission.RootMissionID, &mission.OwningThreadID,
			&mission.OwnerAgentID, &mission.OwnerRole, &mission.MissionType, &mission.Title, &mission.Charter,
			&mission.Goal, &mission.Scope, &mission.ReuseTrace, &mission.Constraints, &mission.AcceptanceCriteria, &mission.AuthorityLevel,
			&mission.DelegationPolicy, &mission.Status, &mission.Priority, &mission.RiskLevel, &mission.ProgressPercent,
			&mission.WaitingUntil, &mission.CreatedAt, &mission.UpdatedAt, &mission.ClosedAt,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, mission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return findReusableMissionMatches(candidates, query, limit), nil
}

func (s *PostgresStore) AssignMission(assignment Assignment) error {
	if assignment.ID == "" {
		return fmt.Errorf("assignment id is required")
	}
	if assignment.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if assignment.AgentID == "" {
		return fmt.Errorf("agent id is required")
	}
	if assignment.AgentRole == "" {
		return fmt.Errorf("agent role is required")
	}
	if len(assignment.AuthorityScope) == 0 {
		assignment.AuthorityScope = []byte(`{}`)
	}
	if assignment.AssignedAt.IsZero() {
		assignment.AssignedAt = time.Now().UTC()
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_assignments (
			assignment_id, mission_id, agent_id, agent_role, authority_scope_json,
			reporting_to_agent_id, assigned_at, revoked_at
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), $7, $8)
		ON CONFLICT (assignment_id) DO NOTHING
	`, assignment.ID, assignment.MissionID, assignment.AgentID, assignment.AgentRole, assignment.AuthorityScope,
		assignment.ReportingToAgentID, assignment.AssignedAt, assignment.RevokedAt)
	return err
}

func (s *PostgresStore) ListAssignments(missionID string) ([]Assignment, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT assignment_id, mission_id, agent_id, agent_role, authority_scope_json,
			COALESCE(reporting_to_agent_id, ''), assigned_at, revoked_at
		FROM mission_assignments
		WHERE mission_id = $1
		ORDER BY assigned_at ASC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []Assignment
	for rows.Next() {
		var assignment Assignment
		if err := rows.Scan(
			&assignment.ID, &assignment.MissionID, &assignment.AgentID, &assignment.AgentRole,
			&assignment.AuthorityScope, &assignment.ReportingToAgentID, &assignment.AssignedAt, &assignment.RevokedAt,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		if _, err := s.GetMission(missionID); err != nil {
			return nil, err
		}
	}
	return assignments, nil
}

func defaultObjectJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func defaultArrayJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	return raw
}
