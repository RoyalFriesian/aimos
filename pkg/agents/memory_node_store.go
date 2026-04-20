package agents

import (
	"fmt"
	"sync"
	"time"
)

type MemoryNodeStore struct {
	mu    sync.RWMutex
	nodes map[string]AgentNode
}

func NewMemoryNodeStore() *MemoryNodeStore {
	return &MemoryNodeStore{nodes: make(map[string]AgentNode)}
}

func (s *MemoryNodeStore) CreateNode(node AgentNode) error {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.nodes[node.ID]; exists {
		return fmt.Errorf("agent node %q already exists", node.ID)
	}
	s.nodes[node.ID] = node
	return nil
}

func (s *MemoryNodeStore) GetNode(agentID string) (AgentNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[agentID]
	if !ok {
		return AgentNode{}, ErrNodeNotFound
	}
	return node, nil
}

func (s *MemoryNodeStore) ListByProject(projectID string) ([]AgentNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []AgentNode
	for _, node := range s.nodes {
		if node.ProjectID == projectID {
			result = append(result, node)
		}
	}
	return result, nil
}

func (s *MemoryNodeStore) ListChildren(parentAgentID string) ([]AgentNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []AgentNode
	for _, node := range s.nodes {
		if node.ParentAgentID == parentAgentID {
			result = append(result, node)
		}
	}
	return result, nil
}

func (s *MemoryNodeStore) ListActive() ([]AgentNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []AgentNode
	for _, node := range s.nodes {
		if node.Status == "active" {
			result = append(result, node)
		}
	}
	return result, nil
}

func (s *MemoryNodeStore) UpdateStatus(agentID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[agentID]
	if !ok {
		return ErrNodeNotFound
	}
	node.Status = status
	node.UpdatedAt = time.Now().UTC()
	s.nodes[agentID] = node
	return nil
}

func (s *MemoryNodeStore) UpdateProblemStatement(agentID string, problemStatement string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[agentID]
	if !ok {
		return ErrNodeNotFound
	}
	node.ProblemStatement = problemStatement
	node.UpdatedAt = time.Now().UTC()
	s.nodes[agentID] = node
	return nil
}

func (s *MemoryNodeStore) UpdateModel(agentID string, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[agentID]
	if !ok {
		return ErrNodeNotFound
	}
	node.Model = model
	node.UpdatedAt = time.Now().UTC()
	s.nodes[agentID] = node
	return nil
}

func (s *MemoryNodeStore) SetPaused(agentID string, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[agentID]
	if !ok {
		return ErrNodeNotFound
	}
	node.Paused = paused
	node.UpdatedAt = time.Now().UTC()
	s.nodes[agentID] = node
	return nil
}

func (s *MemoryNodeStore) SetProjectPaused(projectID string, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for id, node := range s.nodes {
		if node.ProjectID == projectID && node.Status == "active" {
			node.Paused = paused
			node.UpdatedAt = now
			s.nodes[id] = node
		}
	}
	return nil
}

var _ NodeStore = (*MemoryNodeStore)(nil)
