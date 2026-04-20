package agents

import (
	"context"

	"github.com/Sarnga/agent-platform/pkg/skills"
)

// skillNodeStoreAdapter wraps the agents.NodeStore to satisfy skills.NodeStore.
type skillNodeStoreAdapter struct {
	store NodeStore
}

func (a *skillNodeStoreAdapter) GetNode(agentID string) (skills.NodeInfo, error) {
	node, err := a.store.GetNode(agentID)
	if err != nil {
		return skills.NodeInfo{}, err
	}
	return agentNodeToSkillNodeInfo(node), nil
}

func (a *skillNodeStoreAdapter) ListChildren(parentAgentID string) ([]skills.NodeInfo, error) {
	nodes, err := a.store.ListChildren(parentAgentID)
	if err != nil {
		return nil, err
	}
	out := make([]skills.NodeInfo, len(nodes))
	for i := range nodes {
		out[i] = agentNodeToSkillNodeInfo(nodes[i])
	}
	return out, nil
}

func (a *skillNodeStoreAdapter) UpdateStatus(agentID string, status string) error {
	return a.store.UpdateStatus(agentID, status)
}

func agentNodeToSkillNodeInfo(n AgentNode) skills.NodeInfo {
	return skills.NodeInfo{
		ID:               n.ID,
		ParentAgentID:    n.ParentAgentID,
		RootAgentID:      n.RootAgentID,
		ProjectID:        n.ProjectID,
		ThreadID:         n.ThreadID,
		MissionID:        n.MissionID,
		Name:             n.Name,
		Role:             string(n.Role),
		Depth:            n.Depth,
		ProblemStatement: n.ProblemStatement,
		Status:           n.Status,
		Model:            n.Model,
		Paused:           n.Paused,
	}
}

// skillLoopManagerAdapter wraps the agents.AgentLoopManager to satisfy skills.LoopManager.
type skillLoopManagerAdapter struct {
	mgr  *AgentLoopManager
	deps AgentLoopDeps
	cfg  AgentLoopConfig
}

func (a *skillLoopManagerAdapter) StartChildLoop(ctx context.Context, config skills.ChildLoopConfig) error {
	if a.mgr == nil {
		return nil
	}
	role := NodeRole(config.AgentRole)
	childConfig := AgentLoopConfig{
		AgentID:         config.AgentID,
		AgentRole:       role,
		MissionID:       config.MissionID,
		ThreadID:        config.ThreadID,
		ProjectID:       config.ProjectID,
		ProjectLocation: config.ProjectLocation,
		Depth:           config.Depth,
		MaxDepth:        config.MaxDepth,
		WakeInterval:    WakeIntervalForRole(role),
		Model:           config.Model,
		SystemPrompt:    config.SystemPrompt,
	}
	if childConfig.SystemPrompt == "" {
		childConfig.SystemPrompt = systemPromptForRole(role, config.Depth, config.MaxDepth)
	}
	return a.mgr.StartLoop(childConfig, a.deps)
}

func (a *skillLoopManagerAdapter) StopLoop(agentID string) {
	if a.mgr != nil {
		a.mgr.StopLoop(agentID)
	}
}

// skillTestingAdapter wraps the agents.TestingAgent to satisfy skills.TestingValidator.
type skillTestingAdapter struct {
	agent *TestingAgent
}

func (a *skillTestingAdapter) Validate(ctx context.Context, input skills.TestInput) (skills.TestResult, error) {
	if a.agent == nil {
		return skills.TestResult{Status: "pass", Summary: "no testing agent"}, nil
	}
	result, err := a.agent.Validate(ctx, TestInput{
		Deliverable:        input.Deliverable,
		TodoTitle:          input.TodoTitle,
		TodoDescription:    input.TodoDescription,
		AcceptanceCriteria: input.AcceptanceCriteria,
		MissionGoal:        input.MissionGoal,
	})
	if err != nil {
		return skills.TestResult{}, err
	}
	return skills.TestResult{
		Status:  result.Status,
		Summary: result.Summary,
		Issues:  result.Issues,
	}, nil
}

// skillQAAdapter wraps the agents.QAAgent to satisfy skills.QAValidator.
type skillQAAdapter struct {
	agent *QAAgent
}

func (a *skillQAAdapter) ValidateProject(ctx context.Context, input skills.QAInput) (skills.QAResult, error) {
	if a.agent == nil {
		return skills.QAResult{Status: "pass", Summary: "no QA agent"}, nil
	}
	result, err := a.agent.ValidateProject(ctx, QAValidationPayload{
		ProjectDir:  input.ProjectDir,
		MissionGoal: input.MissionGoal,
	})
	if err != nil {
		return skills.QAResult{}, err
	}
	return skills.QAResult{
		Status:  result.Status,
		Summary: result.Summary,
	}, nil
}

// wrapNodeStore returns a skills.NodeStore adapter for the agent loop's NodeStore.
func (l *AgentLoop) wrapNodeStore() skills.NodeStore {
	if l.deps.NodeStore == nil {
		return nil
	}
	return &skillNodeStoreAdapter{store: l.deps.NodeStore}
}

// wrapLoopManager returns a skills.LoopManager adapter for the agent loop manager.
func (l *AgentLoop) wrapLoopManager() skills.LoopManager {
	if l.deps.LoopManager == nil {
		return nil
	}
	return &skillLoopManagerAdapter{
		mgr:  l.deps.LoopManager,
		deps: l.deps,
		cfg:  l.config,
	}
}

// wrapTestingAgent returns a skills.TestingValidator adapter for the testing agent.
func (l *AgentLoop) wrapTestingAgent() skills.TestingValidator {
	if l.deps.TestingAgent == nil {
		return nil
	}
	return &skillTestingAdapter{agent: l.deps.TestingAgent}
}

// wrapQAAgent returns a skills.QAValidator adapter for the QA agent.
func (l *AgentLoop) wrapQAAgent() skills.QAValidator {
	if l.deps.QAAgent == nil {
		return nil
	}
	return &skillQAAdapter{agent: l.deps.QAAgent}
}
