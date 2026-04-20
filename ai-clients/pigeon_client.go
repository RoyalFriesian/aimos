package aiclients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

const (
	pigeonModelPrefix      = "pigeon/"
	defaultPigeonPollMs    = 500
	defaultPigeonTimeoutMs = 300_000 // 5 minutes
)

// PigeonConfig holds configuration for the file-based Pigeon AI relay.
type PigeonConfig struct {
	// BaseDir is the root directory where model folders and request files are written.
	// Example: /Users/you/go/src/github.com/aimos-ai-requests
	BaseDir string
	// PollInterval is how often to check for the _answered file (default 500ms).
	PollInterval time.Duration
	// PollTimeout is the maximum wait for an answer before returning an error (default 5m).
	PollTimeout time.Duration
}

// PigeonMessage is one message in a Pigeon request payload.
type PigeonMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PigeonRequest is the JSON payload written to the request file.
type PigeonRequest struct {
	ID        string          `json:"id"`
	Model     string          `json:"model"`
	Messages  []PigeonMessage `json:"messages"`
	CreatedAt string          `json:"created_at"`
}

// PigeonResponse is the JSON payload expected in the _answered file.
// Supports two formats:
//   - Top-level "content" field (preferred)
//   - Messages array with the last assistant message as the response
type PigeonResponse struct {
	ID         string          `json:"id"`
	Model      string          `json:"model"`
	Content    string          `json:"content"`
	Messages   []PigeonMessage `json:"messages,omitempty"`
	Error      string          `json:"error,omitempty"`
	AnsweredAt string          `json:"answered_at"`
}

// PigeonClient implements CompletionProvider by writing request JSON files
// and polling for _answered response files. An external AI agent running on
// the system is expected to read the request, call the real model, and write
// the answered file back.
type PigeonClient struct {
	baseDir      string
	pollInterval time.Duration
	pollTimeout  time.Duration
	logger       *slog.Logger
}

// NewPigeonClient creates a new file-based Pigeon AI relay client.
func NewPigeonClient(config PigeonConfig, logger *slog.Logger) *PigeonClient {
	if logger == nil {
		logger = slog.Default()
	}
	poll := config.PollInterval
	if poll <= 0 {
		poll = time.Duration(defaultPigeonPollMs) * time.Millisecond
	}
	timeout := config.PollTimeout
	if timeout <= 0 {
		timeout = time.Duration(defaultPigeonTimeoutMs) * time.Millisecond
	}
	return &PigeonClient{
		baseDir:      config.BaseDir,
		pollInterval: poll,
		pollTimeout:  timeout,
		logger:       logger,
	}
}

// Generate sends a system+user prompt pair via the Pigeon file relay.
func (c *PigeonClient) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	msgs := []PigeonMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return c.sendAndPoll(ctx, model, msgs)
}

// GenerateFromMessages sends a conversation via the Pigeon file relay.
func (c *PigeonClient) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	pigeonMsgs := make([]PigeonMessage, 0, len(messages))
	for _, m := range messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		pigeonMsgs = append(pigeonMsgs, PigeonMessage{
			Role:    string(m.Role),
			Content: content,
		})
	}
	if len(pigeonMsgs) == 0 {
		return "", fmt.Errorf("pigeon: no non-empty messages provided")
	}
	return c.sendAndPoll(ctx, model, pigeonMsgs)
}

// GenerateWithTools is not supported by Pigeon — the external agent handles
// raw request/response only, not multi-round tool calling.
func (c *PigeonClient) GenerateWithTools(_ context.Context, model string, _ string, _ string, _ []responses.ToolUnionParam, _ ToolExecutor, _ int) (ToolCallResult, error) {
	return ToolCallResult{}, fmt.Errorf("pigeon: tool calling is not supported for model %q — use OpenAI provider for tool-calling models", model)
}

// ModelFolder converts a model name to a filesystem-safe folder name.
// Dots become underscores, colons become underscores.
// Example: "gpt-5.4" → "gpt-5_4", "gpt-5.3-codex" → "gpt-5_3-codex"
func ModelFolder(model string) string {
	r := strings.NewReplacer(".", "_", ":", "_")
	return r.Replace(model)
}

// sendAndPoll writes the request file, then polls until the _answered file appears or timeout.
func (c *PigeonClient) sendAndPoll(ctx context.Context, model string, messages []PigeonMessage) (string, error) {
	folder := filepath.Join(c.baseDir, ModelFolder(model))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fmt.Errorf("pigeon: create model directory %s: %w", folder, err)
	}

	now := time.Now().UTC()
	randomPart := rand.Intn(9000) + 1000 // 4-digit: 1000..9999
	requestID := fmt.Sprintf("request_%d_%d", randomPart, now.UnixNano())

	reqPayload := PigeonRequest{
		ID:        requestID,
		Model:     model,
		Messages:  messages,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(reqPayload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("pigeon: marshal request: %w", err)
	}

	requestFile := filepath.Join(folder, requestID+".json")
	if err := os.WriteFile(requestFile, data, 0o644); err != nil {
		return "", fmt.Errorf("pigeon: write request file %s: %w", requestFile, err)
	}

	c.logger.Info("pigeon request written", "model", model, "file", requestFile)

	// Poll for the _answered file.
	answeredFile := filepath.Join(folder, requestID+"_answered.json")
	deadline := time.Now().Add(c.pollTimeout)
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("pigeon: context cancelled while waiting for answer: %w", ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				return "", fmt.Errorf("pigeon: timeout after %s waiting for answered file %s", c.pollTimeout, answeredFile)
			}
			respData, err := os.ReadFile(answeredFile)
			if err != nil {
				if os.IsNotExist(err) {
					continue // not ready yet
				}
				return "", fmt.Errorf("pigeon: read answered file: %w", err)
			}
			// Parse the response.
			var resp PigeonResponse
			if err := json.Unmarshal(respData, &resp); err != nil {
				return "", fmt.Errorf("pigeon: parse answered file %s: %w", answeredFile, err)
			}
			if resp.Error != "" {
				return "", fmt.Errorf("pigeon: model returned error: %s", resp.Error)
			}
			content := strings.TrimSpace(resp.Content)
			// Fallback: extract from last assistant message in messages array.
			if content == "" {
				for i := len(resp.Messages) - 1; i >= 0; i-- {
					if resp.Messages[i].Role == "assistant" {
						content = strings.TrimSpace(resp.Messages[i].Content)
						break
					}
				}
			}
			if content == "" {
				return "", fmt.Errorf("pigeon: answered file %s has empty content", answeredFile)
			}
			c.logger.Info("pigeon response received", "model", model, "file", answeredFile)
			return content, nil
		}
	}
}
