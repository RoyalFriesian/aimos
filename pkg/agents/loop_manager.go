package agents

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AgentLoopManager manages the lifecycle of all active agent loops.
type AgentLoopManager struct {
	mu    sync.RWMutex
	loops map[string]*AgentLoop
}

// NewAgentLoopManager creates a new loop manager.
func NewAgentLoopManager() *AgentLoopManager {
	return &AgentLoopManager{
		loops: make(map[string]*AgentLoop),
	}
}

// StartLoop creates and starts an agent loop in a background goroutine.
func (m *AgentLoopManager) StartLoop(config AgentLoopConfig, deps AgentLoopDeps) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.loops[config.AgentID]; exists {
		return nil // already running
	}

	loop, err := NewAgentLoop(config, deps)
	if err != nil {
		return fmt.Errorf("create agent loop for %s: %w", config.AgentID, err)
	}

	m.loops[config.AgentID] = loop

	go func() {
		if runErr := loop.Run(loop.runContext()); runErr != nil {
			slog.Error("agent loop exited with error", "agentID", config.AgentID, "error", runErr)
		}
		m.mu.Lock()
		delete(m.loops, config.AgentID)
		m.mu.Unlock()
	}()

	return nil
}

// StopLoop stops a specific agent's loop.
func (m *AgentLoopManager) StopLoop(agentID string) {
	m.mu.Lock()
	loop, exists := m.loops[agentID]
	if exists {
		delete(m.loops, agentID)
	}
	m.mu.Unlock()

	if exists && loop != nil {
		loop.Stop()
		slog.Info("agent loop stopped by manager", "agentID", agentID)
	}
}

// StopAll stops all running loops.
func (m *AgentLoopManager) StopAll() {
	m.mu.Lock()
	loops := make(map[string]*AgentLoop, len(m.loops))
	for k, v := range m.loops {
		loops[k] = v
	}
	m.loops = make(map[string]*AgentLoop)
	m.mu.Unlock()

	for agentID, loop := range loops {
		loop.Stop()
		slog.Info("agent loop stopped by StopAll", "agentID", agentID)
	}
}

// IsRunning returns whether a loop is active for the given agent.
func (m *AgentLoopManager) IsRunning(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.loops[agentID]
	return exists
}

// ListRunning returns the agent IDs of all running loops.
func (m *AgentLoopManager) ListRunning() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.loops))
	for id := range m.loops {
		ids = append(ids, id)
	}
	return ids
}

// RunningCount returns the number of active loops.
func (m *AgentLoopManager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.loops)
}

// PauseLoop pauses a specific agent's loop.
func (m *AgentLoopManager) PauseLoop(agentID string) {
	m.mu.RLock()
	loop, exists := m.loops[agentID]
	m.mu.RUnlock()
	if exists && loop != nil {
		loop.Pause()
	}
}

// ResumeLoop resumes a specific agent's loop.
func (m *AgentLoopManager) ResumeLoop(agentID string) {
	m.mu.RLock()
	loop, exists := m.loops[agentID]
	m.mu.RUnlock()
	if exists && loop != nil {
		loop.Resume()
	}
}

// PauseAll pauses all running loops.
func (m *AgentLoopManager) PauseAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, loop := range m.loops {
		loop.Pause()
	}
}

// ResumeAll resumes all running loops.
func (m *AgentLoopManager) ResumeAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, loop := range m.loops {
		loop.Resume()
	}
}

// LoopStatus holds the runtime status of a single agent loop.
type LoopStatus struct {
	AgentID    string    `json:"agentId"`
	Paused     bool      `json:"paused"`
	Model      string    `json:"model"`
	NextWakeAt time.Time `json:"nextWakeAt"`
	Interval   float64   `json:"intervalSeconds"`
}

// GetLoopStatus returns the status of a specific agent's loop.
func (m *AgentLoopManager) GetLoopStatus(agentID string) (LoopStatus, bool) {
	m.mu.RLock()
	loop, exists := m.loops[agentID]
	m.mu.RUnlock()
	if !exists || loop == nil {
		return LoopStatus{}, false
	}
	return LoopStatus{
		AgentID:    agentID,
		Paused:     loop.IsPaused(),
		Model:      loop.Config().Model,
		NextWakeAt: loop.NextWakeAt(),
		Interval:   loop.effectiveInterval().Seconds(),
	}, true
}

// GetAllLoopStatuses returns the status of all running loops.
func (m *AgentLoopManager) GetAllLoopStatuses() []LoopStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]LoopStatus, 0, len(m.loops))
	for id, loop := range m.loops {
		statuses = append(statuses, LoopStatus{
			AgentID:    id,
			Paused:     loop.IsPaused(),
			Model:      loop.Config().Model,
			NextWakeAt: loop.NextWakeAt(),
			Interval:   loop.effectiveInterval().Seconds(),
		})
	}
	return statuses
}

// PauseByProject pauses all loops for agents belonging to the given project.
func (m *AgentLoopManager) PauseByProject(projectID string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, loop := range m.loops {
		if loop.Config().ProjectID == projectID {
			loop.Pause()
		}
	}
}

// ResumeByProject resumes all loops for agents belonging to the given project.
func (m *AgentLoopManager) ResumeByProject(projectID string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, loop := range m.loops {
		if loop.Config().ProjectID == projectID {
			loop.Resume()
		}
	}
}

// UpdateLoopModel updates the model for a running loop's config in-memory.
func (m *AgentLoopManager) UpdateLoopModel(agentID string, model string) bool {
	m.mu.RLock()
	loop, exists := m.loops[agentID]
	m.mu.RUnlock()
	if !exists || loop == nil {
		return false
	}
	loop.mu.Lock()
	loop.config.Model = model
	loop.mu.Unlock()
	return true
}
