package agents

import (
	"errors"
	"time"
)

var ErrNodeNotFound = errors.New("agent node not found")

type NodeRole string

const (
	NodeRoleCEO     NodeRole = "CEO"
	NodeRoleManager NodeRole = "Manager"
	NodeRoleWorker  NodeRole = "Worker"
	NodeRoleTester  NodeRole = "Tester"
)

// AgentNode represents an AI agent in the recursive problem-decomposition tree.
// Each node owns exactly one Thread for its conversations.
// The mindmap renders AgentNodes — not threads.
type AgentNode struct {
	ID               string    `json:"id"`
	ParentAgentID    string    `json:"parentAgentId,omitempty"`
	RootAgentID      string    `json:"rootAgentId"`
	ProjectID        string    `json:"projectId"`
	ThreadID         string    `json:"threadId"`
	MissionID        string    `json:"missionId,omitempty"`
	Name             string    `json:"name"`
	Role             NodeRole  `json:"role"`
	Depth            int       `json:"depth"`
	ProblemStatement string    `json:"problemStatement,omitempty"`
	Status           string    `json:"status"`
	Model            string    `json:"model,omitempty"`
	Paused           bool      `json:"paused"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type NodeStore interface {
	CreateNode(node AgentNode) error
	GetNode(agentID string) (AgentNode, error)
	ListByProject(projectID string) ([]AgentNode, error)
	ListChildren(parentAgentID string) ([]AgentNode, error)
	ListActive() ([]AgentNode, error)
	UpdateStatus(agentID string, status string) error
	UpdateProblemStatement(agentID string, problemStatement string) error
	UpdateModel(agentID string, model string) error
	SetPaused(agentID string, paused bool) error
	SetProjectPaused(projectID string, paused bool) error
}
