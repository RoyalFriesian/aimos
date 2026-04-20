package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Transport defines how A2A messages are delivered between agents.
type Transport interface {
	Send(ctx context.Context, target AgentEndpoint, msg Message) (Response, error)
}

// HTTPTransport sends A2A messages via HTTP POST.
type HTTPTransport struct {
	client *http.Client
}

// NewHTTPTransport creates a transport that sends messages over HTTP.
func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Send delivers a message to the target agent's HTTP endpoint.
func (t *HTTPTransport) Send(ctx context.Context, target AgentEndpoint, msg Message) (Response, error) {
	if target.HTTPAddr == "" {
		return Response{}, fmt.Errorf("target agent %q has no HTTP address", target.AgentID)
	}
	if err := msg.Validate(); err != nil {
		return Response{}, fmt.Errorf("invalid A2A message: %w", err)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return Response{}, fmt.Errorf("marshal A2A message: %w", err)
	}

	url := target.HTTPAddr + "/a2a/message"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create A2A request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-A2A-From", msg.From)
	req.Header.Set("X-A2A-Task-ID", msg.TaskID)
	req.Header.Set("X-A2A-Thread-ID", msg.ThreadID)
	req.Header.Set("X-A2A-Trace-ID", msg.TraceID)

	resp, err := t.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send A2A message to %s: %w", target.AgentID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read A2A response from %s: %w", target.AgentID, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("A2A message to %s returned status %d: %s", target.AgentID, resp.StatusCode, string(respBody))
	}

	var a2aResp Response
	if err := json.Unmarshal(respBody, &a2aResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal A2A response from %s: %w", target.AgentID, err)
	}
	return a2aResp, nil
}

// InProcessTransport delivers messages to handlers registered in the same process.
// Used for testing and single-process deployments.
type InProcessTransport struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// Handler processes incoming A2A messages.
type Handler interface {
	Handle(ctx context.Context, msg Message) (Response, error)
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(ctx context.Context, msg Message) (Response, error)

func (f HandlerFunc) Handle(ctx context.Context, msg Message) (Response, error) {
	return f(ctx, msg)
}

// NewInProcessTransport creates a transport for same-process communication.
func NewInProcessTransport() *InProcessTransport {
	return &InProcessTransport{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler for the given agent ID.
func (t *InProcessTransport) Register(agentID string, handler Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[agentID] = handler
}

// Send delivers a message to the target agent's registered handler.
func (t *InProcessTransport) Send(ctx context.Context, target AgentEndpoint, msg Message) (Response, error) {
	if err := msg.Validate(); err != nil {
		return Response{}, fmt.Errorf("invalid A2A message: %w", err)
	}

	t.mu.RLock()
	handler, ok := t.handlers[target.AgentID]
	t.mu.RUnlock()

	if !ok {
		return Response{}, fmt.Errorf("no in-process handler registered for agent %q", target.AgentID)
	}
	return handler.Handle(ctx, msg)
}
