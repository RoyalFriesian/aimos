package agents

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresNodeStore struct {
	pool *pgxpool.Pool
}

func NewPostgresNodeStore(pool *pgxpool.Pool) (*PostgresNodeStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresNodeStore{pool: pool}, nil
}

func (s *PostgresNodeStore) CreateNode(node AgentNode) error {
	if node.ID == "" {
		return fmt.Errorf("agent node id is required")
	}
	if node.ThreadID == "" {
		return fmt.Errorf("agent node thread id is required")
	}
	if node.ProjectID == "" {
		return fmt.Errorf("agent node project id is required")
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now().UTC()
	}
	if node.UpdatedAt.IsZero() {
		node.UpdatedAt = node.CreatedAt
	}
	if node.Status == "" {
		node.Status = "active"
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO agent_nodes (
			agent_id, parent_agent_id, root_agent_id, project_id, thread_id, mission_id,
			name, role, depth, problem_statement, status, model, paused, created_at, updated_at
		) VALUES (
			$1, NULLIF($2,''), $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (agent_id) DO NOTHING
	`, node.ID, node.ParentAgentID, node.RootAgentID, node.ProjectID, node.ThreadID, node.MissionID,
		node.Name, string(node.Role), node.Depth, node.ProblemStatement, node.Status, node.Model, node.Paused, node.CreatedAt, node.UpdatedAt)
	return err
}

func (s *PostgresNodeStore) GetNode(agentID string) (AgentNode, error) {
	var node AgentNode
	var role string
	err := s.pool.QueryRow(context.Background(), `
		SELECT agent_id, COALESCE(parent_agent_id, ''), root_agent_id, project_id, thread_id, mission_id,
			name, role, depth, problem_statement, status, COALESCE(model, ''), COALESCE(paused, false), created_at, updated_at
		FROM agent_nodes
		WHERE agent_id = $1
	`, agentID).Scan(&node.ID, &node.ParentAgentID, &node.RootAgentID, &node.ProjectID, &node.ThreadID, &node.MissionID,
		&node.Name, &role, &node.Depth, &node.ProblemStatement, &node.Status, &node.Model, &node.Paused, &node.CreatedAt, &node.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentNode{}, ErrNodeNotFound
	}
	node.Role = NodeRole(role)
	return node, err
}

func (s *PostgresNodeStore) ListByProject(projectID string) ([]AgentNode, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT agent_id, COALESCE(parent_agent_id, ''), root_agent_id, project_id, thread_id, mission_id,
			name, role, depth, problem_statement, status, COALESCE(model, ''), COALESCE(paused, false), created_at, updated_at
		FROM agent_nodes
		WHERE project_id = $1
		ORDER BY depth ASC, created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (s *PostgresNodeStore) ListChildren(parentAgentID string) ([]AgentNode, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT agent_id, COALESCE(parent_agent_id, ''), root_agent_id, project_id, thread_id, mission_id,
			name, role, depth, problem_statement, status, COALESCE(model, ''), COALESCE(paused, false), created_at, updated_at
		FROM agent_nodes
		WHERE parent_agent_id = $1
		ORDER BY created_at ASC
	`, parentAgentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (s *PostgresNodeStore) ListActive() ([]AgentNode, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT agent_id, COALESCE(parent_agent_id, ''), root_agent_id, project_id, thread_id, mission_id,
			name, role, depth, problem_statement, status, COALESCE(model, ''), COALESCE(paused, false), created_at, updated_at
		FROM agent_nodes
		WHERE status = 'active'
		ORDER BY depth ASC, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (s *PostgresNodeStore) UpdateStatus(agentID string, status string) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE agent_nodes SET status = $1, updated_at = $2 WHERE agent_id = $3
	`, status, time.Now().UTC(), agentID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (s *PostgresNodeStore) UpdateProblemStatement(agentID string, problemStatement string) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE agent_nodes SET problem_statement = $1, updated_at = $2 WHERE agent_id = $3
	`, problemStatement, time.Now().UTC(), agentID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (s *PostgresNodeStore) UpdateModel(agentID string, model string) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE agent_nodes SET model = $1, updated_at = $2 WHERE agent_id = $3
	`, model, time.Now().UTC(), agentID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (s *PostgresNodeStore) SetPaused(agentID string, paused bool) error {
	commandTag, err := s.pool.Exec(context.Background(), `
		UPDATE agent_nodes SET paused = $1, updated_at = $2 WHERE agent_id = $3
	`, paused, time.Now().UTC(), agentID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (s *PostgresNodeStore) SetProjectPaused(projectID string, paused bool) error {
	_, err := s.pool.Exec(context.Background(), `
		UPDATE agent_nodes SET paused = $1, updated_at = $2 WHERE project_id = $3 AND status = 'active'
	`, paused, time.Now().UTC(), projectID)
	return err
}

func scanNodes(rows pgx.Rows) ([]AgentNode, error) {
	var nodes []AgentNode
	for rows.Next() {
		var node AgentNode
		var role string
		if err := rows.Scan(&node.ID, &node.ParentAgentID, &node.RootAgentID, &node.ProjectID, &node.ThreadID, &node.MissionID,
			&node.Name, &role, &node.Depth, &node.ProblemStatement, &node.Status, &node.Model, &node.Paused, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, err
		}
		node.Role = NodeRole(role)
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

var _ NodeStore = (*PostgresNodeStore)(nil)
