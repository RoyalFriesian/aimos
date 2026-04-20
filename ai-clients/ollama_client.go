package aiclients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

const (
	defaultOllamaBaseURL = "http://localhost:11434"
	ollamaChatPath       = "/api/chat"
)

// OllamaConfig holds configuration for the Ollama client.
type OllamaConfig struct {
	BaseURL string // e.g. "http://localhost:11434"
}

// OllamaClient implements CompletionClient using Ollama's native chat API.
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewOllamaClient(config OllamaConfig, logger *slog.Logger) *OllamaClient {
	base := config.BaseURL
	if base == "" {
		base = defaultOllamaBaseURL
	}
	base = strings.TrimRight(base, "/")
	if logger == nil {
		logger = slog.Default()
	}
	return &OllamaClient{
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		logger: logger,
	}
}

// ollamaChatRequest is the request body for Ollama's /api/chat endpoint.
type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Format   string              `json:"format,omitempty"` // "json" to force JSON output
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse is the response body from Ollama's /api/chat endpoint (non-streaming).
type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

func (c *OllamaClient) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	messages := []threads.Message{
		{Role: threads.RoleSystem, Content: systemPrompt},
		{Role: threads.RoleUser, Content: userPrompt},
	}
	return c.GenerateFromMessages(ctx, model, messages)
}

func (c *OllamaClient) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	chatMsgs := make([]ollamaChatMessage, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		chatMsgs = append(chatMsgs, ollamaChatMessage{
			Role:    ollamaRole(msg.Role),
			Content: content,
		})
	}
	if len(chatMsgs) == 0 {
		return "", errors.New("no non-empty messages provided")
	}

	reqBody := ollamaChatRequest{
		Model:    model,
		Messages: chatMsgs,
		Stream:   false,
		Format:   "json",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	url := c.baseURL + ollamaChatPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("ollama request", "model", model, "messages", len(chatMsgs))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		wrapped := fmt.Errorf("ollama request failed: %w", err)
		c.logger.Error("ollama request failed", "error", wrapped, "model", model)
		return "", wrapped
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		wrapped := fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBytes))
		c.logger.Error("ollama request failed", "error", wrapped, "model", model, "status", resp.StatusCode)
		return "", wrapped
	}

	var chatResp ollamaChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal ollama response: %w", err)
	}

	content := strings.TrimSpace(chatResp.Message.Content)
	if content == "" {
		return "", errors.New("empty response content returned from Ollama")
	}

	c.logger.Debug("ollama response", "model", model, "length", len(content))
	return content, nil
}

// Ping checks if the Ollama server is reachable.
func (c *OllamaClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", c.baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}

// ollamaTagsResponse is the response from Ollama's /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []ollamaModelInfo `json:"models"`
}

type ollamaModelInfo struct {
	Name string `json:"name"`
}

// ListModels returns the model names available in the local Ollama instance.
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decode ollama tags: %w", err)
	}
	models := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}
	return models, nil
}

func ollamaRole(role threads.Role) string {
	switch role {
	case threads.RoleSystem:
		return "system"
	case threads.RoleAssistant:
		return "assistant"
	default:
		return "user"
	}
}

// GenerateWithTools is not supported by Ollama — returns an error.
func (o *OllamaClient) GenerateWithTools(_ context.Context, _ string, _ string, _ string, _ []responses.ToolUnionParam, _ ToolExecutor, _ int) (ToolCallResult, error) {
	return ToolCallResult{}, fmt.Errorf("ollama does not support function-calling tools")
}
