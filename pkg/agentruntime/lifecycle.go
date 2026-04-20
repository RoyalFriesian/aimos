package agentruntime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Sarnga/agent-platform/pkg/a2a"
	"github.com/Sarnga/agent-platform/pkg/agents"
)

// Lifecycle manages starting, registering, and stopping agents in the runtime.
type Lifecycle struct {
	directory agents.Directory
	transport a2a.Transport
	logger    *slog.Logger
	mu        sync.Mutex
	servers   map[string]*AgentServer
}

// NewLifecycle creates a lifecycle manager.
func NewLifecycle(directory agents.Directory, transport a2a.Transport, logger *slog.Logger) *Lifecycle {
	if logger == nil {
		logger = slog.Default()
	}
	return &Lifecycle{
		directory: directory,
		transport: transport,
		logger:    logger,
		servers:   make(map[string]*AgentServer),
	}
}

// StartAgent boots an agent, registers it in the directory, and returns its endpoint.
func (l *Lifecycle) StartAgent(ctx context.Context, profile agents.Profile, handler a2a.Handler) (a2a.AgentEndpoint, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.servers[profile.ID]; exists {
		return a2a.AgentEndpoint{}, fmt.Errorf("agent %q is already running", profile.ID)
	}

	server, err := NewAgentServer(Config{
		AgentID: profile.ID,
		Addr:    ":0",
		Logger:  l.logger,
	}, handler)
	if err != nil {
		return a2a.AgentEndpoint{}, fmt.Errorf("create agent server %q: %w", profile.ID, err)
	}

	addr, err := server.Start(ctx)
	if err != nil {
		return a2a.AgentEndpoint{}, fmt.Errorf("start agent server %q: %w", profile.ID, err)
	}

	endpoint := a2a.AgentEndpoint{
		AgentID:  profile.ID,
		HTTPAddr: "http://" + addr,
	}

	profile.Status = "active"
	if err := l.directory.Register(profile); err != nil {
		server.Stop()
		return a2a.AgentEndpoint{}, fmt.Errorf("register agent %q: %w", profile.ID, err)
	}

	l.servers[profile.ID] = server
	l.logger.Info("agent started", "agentId", profile.ID, "addr", addr, "role", profile.Role)
	return endpoint, nil
}

// StopAgent shuts down a running agent and marks it as idle in the directory.
func (l *Lifecycle) StopAgent(agentID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	server, exists := l.servers[agentID]
	if !exists {
		return fmt.Errorf("agent %q is not running", agentID)
	}

	server.Stop()
	delete(l.servers, agentID)
	_ = l.directory.UpdateStatus(agentID, "idle")
	l.logger.Info("agent stopped", "agentId", agentID)
	return nil
}

// StopAll shuts down all running agents.
func (l *Lifecycle) StopAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for id, server := range l.servers {
		server.Stop()
		_ = l.directory.UpdateStatus(id, "idle")
		l.logger.Info("agent stopped", "agentId", id)
	}
	l.servers = make(map[string]*AgentServer)
}

// RunningAgents returns the IDs of all running agents.
func (l *Lifecycle) RunningAgents() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	ids := make([]string, 0, len(l.servers))
	for id := range l.servers {
		ids = append(ids, id)
	}
	return ids
}

// SendToAgent sends a message to an agent through the configured transport.
func (l *Lifecycle) SendToAgent(ctx context.Context, endpoint a2a.AgentEndpoint, msg a2a.Message) (a2a.Response, error) {
	return l.transport.Send(ctx, endpoint, msg)
}
