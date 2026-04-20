package agentruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/a2a"
	"github.com/Sarnga/agent-platform/pkg/agents"
)

func TestAgentServer_StartAndHealth(t *testing.T) {
	server, err := NewAgentServer(Config{AgentID: "test-agent", Addr: ":0"}, a2a.HandlerFunc(func(_ context.Context, msg a2a.Message) (a2a.Response, error) {
		return a2a.SuccessResponse(msg, nil), nil
	}))
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if addr == "" {
		t.Fatal("expected non-empty address")
	}
	defer server.Stop()

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var health map[string]string
	json.NewDecoder(resp.Body).Decode(&health)
	if health["agentId"] != "test-agent" {
		t.Errorf("expected agentId=test-agent, got %q", health["agentId"])
	}
}

func TestAgentServer_HandleMessage(t *testing.T) {
	handler := a2a.HandlerFunc(func(_ context.Context, msg a2a.Message) (a2a.Response, error) {
		return a2a.SuccessResponse(msg, map[string]string{"echo": msg.Name}), nil
	})

	server, err := NewAgentServer(Config{AgentID: "echo-agent", Addr: ":0"}, handler)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer server.Stop()

	transport := a2a.NewHTTPTransport()
	msg, _ := a2a.NewMessage("test", "echo-agent", "task-1", "thread-1", "trace-1", a2a.MessageTypeCommand, "ping", nil)
	resp, err := transport.Send(context.Background(), a2a.AgentEndpoint{
		AgentID:  "echo-agent",
		HTTPAddr: "http://" + addr,
	}, msg)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
}

func TestLifecycle_StartStopAgent(t *testing.T) {
	directory := agents.NewMemoryDirectory()
	transport := a2a.NewInProcessTransport()
	lc := NewLifecycle(directory, transport, nil)

	handler := a2a.HandlerFunc(func(_ context.Context, msg a2a.Message) (a2a.Response, error) {
		return a2a.SuccessResponse(msg, nil), nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	endpoint, err := lc.StartAgent(ctx, agents.Profile{
		ID:           "lifecycle-test",
		Role:         "worker",
		Capabilities: []string{"test"},
		Status:       "idle",
	}, handler)
	if err != nil {
		t.Fatalf("start agent: %v", err)
	}
	if endpoint.HTTPAddr == "" {
		t.Fatal("expected HTTP address")
	}

	running := lc.RunningAgents()
	if len(running) != 1 || running[0] != "lifecycle-test" {
		t.Errorf("expected [lifecycle-test], got %v", running)
	}

	profile, err := directory.GetProfile("lifecycle-test")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.Status != "active" {
		t.Errorf("expected active, got %q", profile.Status)
	}

	if err := lc.StopAgent("lifecycle-test"); err != nil {
		t.Fatalf("stop agent: %v", err)
	}
	if len(lc.RunningAgents()) != 0 {
		t.Error("expected no running agents after stop")
	}
}
