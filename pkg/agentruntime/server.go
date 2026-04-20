package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Sarnga/agent-platform/pkg/a2a"
)

// AgentServer hosts an agent and exposes it via HTTP for A2A communication.
type AgentServer struct {
	agentID string
	handler a2a.Handler
	server  *http.Server
	logger  *slog.Logger
	addr    string
	mu      sync.Mutex
	running bool
}

// Config holds configuration for an agent server.
type Config struct {
	AgentID string
	Addr    string // e.g. ":0" for auto-assign
	Logger  *slog.Logger
}

// NewAgentServer creates a new agent server that routes A2A messages to the handler.
func NewAgentServer(config Config, handler a2a.Handler) (*AgentServer, error) {
	if config.AgentID == "" {
		return nil, fmt.Errorf("agentID is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	if config.Addr == "" {
		config.Addr = ":0"
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &AgentServer{
		agentID: config.AgentID,
		handler: handler,
		logger:  config.Logger,
		addr:    config.Addr,
	}, nil
}

// Start begins listening for A2A messages. Returns the actual address.
func (s *AgentServer) Start(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s.addr, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /a2a/message", s.handleMessage)
	mux.HandleFunc("GET /healthz", s.handleHealth)

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", s.addr, err)
	}

	actualAddr := listener.Addr().String()
	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}
	s.running = true
	s.addr = actualAddr

	s.logger.Info("agent server starting", "agentId", s.agentID, "addr", actualAddr)

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("agent server failed", "agentId", s.agentID, "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return actualAddr, nil
}

// Stop gracefully shuts down the agent server.
func (s *AgentServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return
	}

	s.logger.Info("agent server stopping", "agentId", s.agentID)
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.server.Shutdown(shutCtx); err != nil {
		s.logger.Error("agent server shutdown error", "agentId", s.agentID, "error", err)
	}
	s.running = false
}

// Addr returns the address the server is listening on.
func (s *AgentServer) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// AgentID returns the server's agent ID.
func (s *AgentServer) AgentID() string {
	return s.agentID
}

func (s *AgentServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	var msg a2a.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, a2a.Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid message body: %v", err),
		})
		return
	}
	if err := msg.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, a2a.ErrorResponse(msg, err))
		return
	}

	s.logger.Info("received A2A message",
		"agentId", s.agentID,
		"from", msg.From,
		"type", msg.Type,
		"name", msg.Name,
		"taskId", msg.TaskID,
		"traceId", msg.TraceID,
	)

	resp, err := s.handler.Handle(r.Context(), msg)
	if err != nil {
		s.logger.Error("handler error", "agentId", s.agentID, "error", err, "messageId", msg.MessageID)
		writeJSON(w, http.StatusInternalServerError, a2a.ErrorResponse(msg, err))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *AgentServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"agentId": s.agentID,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
