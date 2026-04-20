package aiclients

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

const ollamaModelPrefix = "ollama/"

// RouterClient routes LLM requests to different providers based on model name.
// Models prefixed with "pigeon/" are sent to the Pigeon file-relay client.
// Models prefixed with "ollama/" are sent to the Ollama client (prefix stripped).
// All other models go to the OpenAI client.
//
// Every resolved provider is automatically wrapped with logging and token-budget
// middlewares so callers get transparent observability and context compaction.
type RouterClient struct {
	openai       *OpenAIClient
	ollama       *OllamaClient
	pigeon       *PigeonClient
	logger       *slog.Logger
	budgetConfig *BudgetConfig
	fileLogger   *FileLogger
}

// RouterConfig holds configuration for the multi-provider router.
type RouterConfig struct {
	OpenAI *OpenAIConfig
	Ollama *OllamaConfig
	Pigeon *PigeonConfig
}

func NewRouterClient(config RouterConfig, logger *slog.Logger) *RouterClient {
	if logger == nil {
		logger = slog.Default()
	}
	logDir := os.Getenv("AI_LOG_DIR")
	if logDir == "" {
		logDir = "logs/ai-calls"
	}
	fl, err := NewFileLogger(logDir)
	if err != nil {
		logger.Warn("failed to create AI file logger, file logging disabled", "error", err, "dir", logDir)
	}
	r := &RouterClient{
		logger:       logger,
		budgetConfig: NewBudgetConfig(),
		fileLogger:   fl,
	}
	if config.OpenAI != nil {
		r.openai = NewOpenAIClient(*config.OpenAI, logger)
	}
	if config.Ollama != nil {
		r.ollama = NewOllamaClient(*config.Ollama, logger)
	}
	if config.Pigeon != nil {
		r.pigeon = NewPigeonClient(*config.Pigeon, logger)
	}
	return r
}

// BudgetConfig returns the live token-budget configuration.
// The returned pointer is safe for concurrent read/write and changes
// take effect on the next LLM call without a server restart.
func (r *RouterClient) BudgetConfig() *BudgetConfig { return r.budgetConfig }

func (r *RouterClient) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	provider, resolvedModel := r.resolveProvider(model)
	return provider.Generate(ctx, resolvedModel, systemPrompt, userPrompt)
}

func (r *RouterClient) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	provider, resolvedModel := r.resolveProvider(model)
	return provider.GenerateFromMessages(ctx, resolvedModel, messages)
}

func (r *RouterClient) GenerateWithTools(ctx context.Context, model string, systemPrompt string, userPrompt string, tools []responses.ToolUnionParam, executor ToolExecutor, maxRounds int) (ToolCallResult, error) {
	provider, resolvedModel := r.resolveProvider(model)
	return provider.GenerateWithTools(ctx, resolvedModel, systemPrompt, userPrompt, tools, executor, maxRounds)
}

// CompletionProvider is the interface both OpenAI and Ollama clients satisfy.
type CompletionProvider interface {
	Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error)
	GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error)
	GenerateWithTools(ctx context.Context, model string, systemPrompt string, userPrompt string, tools []responses.ToolUnionParam, executor ToolExecutor, maxRounds int) (ToolCallResult, error)
}

func (r *RouterClient) resolveProvider(model string) (CompletionProvider, string) {
	var raw CompletionProvider
	var resolved string

	if strings.HasPrefix(model, pigeonModelPrefix) {
		resolved = strings.TrimPrefix(model, pigeonModelPrefix)
		if r.pigeon != nil {
			r.logger.Debug("routing to pigeon", "model", resolved)
			raw = r.pigeon
		} else {
			r.logger.Warn("model prefixed pigeon/ but Pigeon not configured, falling back to OpenAI", "model", model)
		}
	}
	if raw == nil && strings.HasPrefix(model, ollamaModelPrefix) {
		resolved = strings.TrimPrefix(model, ollamaModelPrefix)
		if r.ollama != nil {
			r.logger.Debug("routing to ollama", "model", resolved)
			raw = r.ollama
		} else {
			r.logger.Warn("model prefixed ollama/ but Ollama not configured, falling back to OpenAI", "model", model)
		}
	}
	if raw == nil {
		resolved = model
		if r.openai != nil {
			raw = r.openai
		} else {
			raw = &failClient{}
		}
	}

	// Wrap with middlewares: token budget first, then logging on the outside
	// so logs reflect pre-budget and post-budget state.
	wrapped := CompletionProvider(NewTokenBudgetMiddleware(raw, r.budgetConfig, r.logger))
	lm := NewLoggingMiddleware(wrapped, r.logger)
	if r.fileLogger != nil {
		lm.SetFileLogger(r.fileLogger)
	}
	return lm, resolved
}

// OllamaAvailable returns true if the Ollama provider is configured and reachable.
func (r *RouterClient) OllamaAvailable(ctx context.Context) bool {
	if r.ollama == nil {
		return false
	}
	return r.ollama.Ping(ctx) == nil
}

// ListOllamaModels returns the model names available from the Ollama provider.
func (r *RouterClient) ListOllamaModels(ctx context.Context) ([]string, error) {
	if r.ollama == nil {
		return nil, fmt.Errorf("ollama provider not configured")
	}
	return r.ollama.ListModels(ctx)
}

// failClient is returned when no provider is available.
type failClient struct{}

func (f *failClient) Generate(_ context.Context, model string, _ string, _ string) (string, error) {
	return "", fmt.Errorf("no LLM provider configured for model %q", model)
}

func (f *failClient) GenerateFromMessages(_ context.Context, model string, _ []threads.Message) (string, error) {
	return "", fmt.Errorf("no LLM provider configured for model %q", model)
}

func (f *failClient) GenerateWithTools(_ context.Context, model string, _ string, _ string, _ []responses.ToolUnionParam, _ ToolExecutor, _ int) (ToolCallResult, error) {
	return ToolCallResult{}, fmt.Errorf("no LLM provider configured for model %q", model)
}
