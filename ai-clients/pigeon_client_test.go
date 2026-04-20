package aiclients

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestModelFolder(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gpt-5.4", "gpt-5_4"},
		{"gpt-5.3-codex", "gpt-5_3-codex"},
		{"claude-opus-4.6", "claude-opus-4_6"},
		{"qwen2.5-coder:7b", "qwen2_5-coder_7b"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		got := ModelFolder(tt.model)
		if got != tt.want {
			t.Errorf("ModelFolder(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestPigeonGenerate(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}, nil)

	// Simulate external AI agent: watch for request files and write _answered files.
	go simulateExternalAgent(t, baseDir, "gpt-5_4", "Hello from Pigeon!", "")

	result, err := client.Generate(context.Background(), "gpt-5.4", "You are helpful.", "Say hello.")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result != "Hello from Pigeon!" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestPigeonGenerateFromMessages(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}, nil)

	go simulateExternalAgent(t, baseDir, "gpt-5_4", "Response from messages!", "")

	msgs := []threads.Message{
		{Role: threads.RoleSystem, Content: "System prompt"},
		{Role: threads.RoleUser, Content: "User message"},
	}
	result, err := client.GenerateFromMessages(context.Background(), "gpt-5.4", msgs)
	if err != nil {
		t.Fatalf("GenerateFromMessages failed: %v", err)
	}
	if result != "Response from messages!" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestPigeonGenerateFromMessagesEmpty(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  1 * time.Second,
	}, nil)

	_, err := client.GenerateFromMessages(context.Background(), "gpt-5.4", []threads.Message{
		{Role: threads.RoleUser, Content: ""},
		{Role: threads.RoleUser, Content: "   "},
	})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
	if !strings.Contains(err.Error(), "no non-empty messages") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPigeonTimeout(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  200 * time.Millisecond,
	}, nil)

	// Don't simulate any external agent — let it timeout.
	_, err := client.Generate(context.Background(), "gpt-5.4", "sys", "usr")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestPigeonContextCancelled(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  30 * time.Second,
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	_, err := client.Generate(ctx, "gpt-5.4", "sys", "usr")
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Fatalf("expected context cancelled error, got: %v", err)
	}
}

func TestPigeonErrorResponse(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}, nil)

	go simulateExternalAgent(t, baseDir, "gpt-5_4", "", "rate limit exceeded")

	_, err := client.Generate(context.Background(), "gpt-5.4", "sys", "usr")
	if err == nil {
		t.Fatal("expected error from model")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("expected rate limit error, got: %v", err)
	}
}

func TestPigeonMessagesArrayFallback(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}, nil)

	// Simulate an external agent that responds with messages array instead of top-level content.
	go simulateExternalAgentMessagesFormat(t, baseDir, "gpt-5_4", "Hello from messages array!")

	result, err := client.Generate(context.Background(), "gpt-5.4", "sys", "usr")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result != "Hello from messages array!" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestPigeonDirectoryCreation(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "nested", "path")
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  200 * time.Millisecond,
	}, nil)

	// Will timeout, but should still create the directory.
	_, _ = client.Generate(context.Background(), "gpt-5.4", "sys", "usr")

	modelDir := filepath.Join(baseDir, "gpt-5_4")
	info, err := os.Stat(modelDir)
	if err != nil {
		t.Fatalf("model directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestPigeonRequestFileFormat(t *testing.T) {
	baseDir := t.TempDir()
	client := NewPigeonClient(PigeonConfig{
		BaseDir:      baseDir,
		PollInterval: 50 * time.Millisecond,
		PollTimeout:  200 * time.Millisecond,
	}, nil)

	// Let it timeout — we just want to verify the request file contents.
	_, _ = client.Generate(context.Background(), "gpt-5.4", "You are a CEO.", "Plan the product.")

	modelDir := filepath.Join(baseDir, "gpt-5_4")
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		t.Fatalf("read model dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one request file")
	}

	// Read the first request file and verify structure.
	data, err := os.ReadFile(filepath.Join(modelDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var req PigeonRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("invalid request JSON: %v", err)
	}
	if req.Model != "gpt-5.4" {
		t.Fatalf("expected model gpt-5.4, got %s", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "You are a CEO." {
		t.Fatalf("unexpected system message: %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Content != "Plan the product." {
		t.Fatalf("unexpected user message: %+v", req.Messages[1])
	}
	if req.ID == "" {
		t.Fatal("request ID is empty")
	}
	if req.CreatedAt == "" {
		t.Fatal("created_at is empty")
	}
	if !strings.HasPrefix(entries[0].Name(), "request_") {
		t.Fatalf("file name does not start with request_: %s", entries[0].Name())
	}
}

func TestPigeonToolCallingNotSupported(t *testing.T) {
	client := NewPigeonClient(PigeonConfig{BaseDir: t.TempDir()}, nil)
	_, err := client.GenerateWithTools(context.Background(), "gpt-5.4", "sys", "usr", nil, nil, 10)
	if err == nil {
		t.Fatal("expected tool calling error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPigeonRouterRouting(t *testing.T) {
	baseDir := t.TempDir()
	router := NewRouterClient(RouterConfig{
		Pigeon: &PigeonConfig{
			BaseDir:      baseDir,
			PollInterval: 50 * time.Millisecond,
			PollTimeout:  5 * time.Second,
		},
	}, nil)

	go simulateExternalAgent(t, baseDir, "gpt-5_4", "Routed through pigeon!", "")

	result, err := router.Generate(context.Background(), "pigeon/gpt-5.4", "sys", "hello")
	if err != nil {
		t.Fatalf("routed Generate failed: %v", err)
	}
	if result != "Routed through pigeon!" {
		t.Fatalf("unexpected result: %s", result)
	}
}

// simulateExternalAgent watches a model directory for request files and writes
// answered files after a short delay. This simulates the external AI agent.
func simulateExternalAgent(t *testing.T, baseDir, modelFolder, content, errMsg string) {
	t.Helper()
	modelDir := filepath.Join(baseDir, modelFolder)

	// Wait for the directory to appear.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(modelDir); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Watch for request files.
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(modelDir)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, "request_") || strings.Contains(name, "_answered") {
				continue
			}
			// Found a request file — read it, build response, write _answered file.
			reqPath := filepath.Join(modelDir, name)
			data, err := os.ReadFile(reqPath)
			if err != nil {
				continue
			}
			var req PigeonRequest
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}

			resp := PigeonResponse{
				ID:         req.ID,
				Model:      req.Model,
				Content:    content,
				Error:      errMsg,
				AnsweredAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			respData, _ := json.MarshalIndent(resp, "", "  ")
			answeredPath := filepath.Join(modelDir, req.ID+"_answered.json")
			if writeErr := os.WriteFile(answeredPath, respData, 0o644); writeErr != nil {
				t.Logf("simulate agent: failed to write answered file: %v", writeErr)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// simulateExternalAgentMessagesFormat simulates an external agent that responds
// with the answer inside a messages array (no top-level content field), matching
// the format used by some real external agents.
func simulateExternalAgentMessagesFormat(t *testing.T, baseDir, modelFolder, content string) {
	t.Helper()
	modelDir := filepath.Join(baseDir, modelFolder)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(modelDir); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(modelDir)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, "request_") || strings.Contains(name, "_answered") {
				continue
			}
			reqPath := filepath.Join(modelDir, name)
			data, err := os.ReadFile(reqPath)
			if err != nil {
				continue
			}
			var req PigeonRequest
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}

			// Build response in messages-array format (no top-level content).
			msgs := append(req.Messages, PigeonMessage{Role: "assistant", Content: content})
			resp := PigeonResponse{
				ID:         req.ID,
				Model:      req.Model,
				Messages:   msgs,
				AnsweredAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			respData, _ := json.MarshalIndent(resp, "", "  ")
			answeredPath := filepath.Join(modelDir, req.ID+"_answered.json")
			if writeErr := os.WriteFile(answeredPath, respData, 0o644); writeErr != nil {
				t.Logf("simulate agent: failed to write answered file: %v", writeErr)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
