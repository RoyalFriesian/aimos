package aiclients

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

// LoggingMiddleware wraps a CompletionProvider and logs every request/response.
type LoggingMiddleware struct {
	inner      CompletionProvider
	logger     *slog.Logger
	fileLogger *FileLogger
}

// NewLoggingMiddleware creates a logging wrapper around the given provider.
func NewLoggingMiddleware(inner CompletionProvider, logger *slog.Logger) *LoggingMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingMiddleware{inner: inner, logger: logger}
}

// SetFileLogger attaches a file-based JSONL logger. When set, every AI call
// is written to a per-project log file in addition to the slog output.
func (m *LoggingMiddleware) SetFileLogger(fl *FileLogger) {
	m.fileLogger = fl
}

func (m *LoggingMiddleware) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	start := time.Now()
	lc := LogContextFromCtx(ctx)
	totalEst := estimateTokens(systemPrompt) + estimateTokens(userPrompt)

	m.logger.Info("llm.request",
		"method", "Generate",
		"model", model,
		"project", lc.ProjectSlug,
		"system_tokens_est", estimateTokens(systemPrompt),
		"user_tokens_est", estimateTokens(userPrompt),
		"total_tokens_est", totalEst,
		"system_len", len(systemPrompt),
		"user_len", len(userPrompt),
	)
	m.writeFile(AICallRecord{
		Timestamp: Now().Format(time.RFC3339Nano),
		Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
		Direction: "request", Method: "Generate", Model: model, TokensEst: totalEst,
		Input:   map[string]string{"system": systemPrompt, "user": userPrompt},
		Details: map[string]any{"system_len": len(systemPrompt), "user_len": len(userPrompt)},
	})

	resp, err := m.inner.Generate(ctx, model, systemPrompt, userPrompt)

	elapsed := time.Since(start)
	if err != nil {
		m.logger.Error("llm.response",
			"method", "Generate",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"error", err.Error(),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "Generate", Model: model, DurationMs: elapsed.Milliseconds(),
			Error: err.Error(),
		})
	} else {
		respEst := estimateTokens(resp)
		m.logger.Info("llm.response",
			"method", "Generate",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"response_tokens_est", respEst,
			"response_len", len(resp),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "Generate", Model: model, TokensEst: respEst,
			DurationMs: elapsed.Milliseconds(),
			Output:     resp,
			Details:    map[string]any{"response_len": len(resp)},
		})
	}
	return resp, err
}

func (m *LoggingMiddleware) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	start := time.Now()
	lc := LogContextFromCtx(ctx)

	var totalChars int
	roles := make(map[string]int)
	for _, msg := range messages {
		totalChars += len(msg.Content)
		roles[string(msg.Role)]++
	}
	totalEst := estimateTokensFromCount(totalChars)

	m.logger.Info("llm.request",
		"method", "GenerateFromMessages",
		"model", model,
		"project", lc.ProjectSlug,
		"message_count", len(messages),
		"total_tokens_est", totalEst,
		"total_chars", totalChars,
		"roles", roles,
	)
	m.writeFile(AICallRecord{
		Timestamp: Now().Format(time.RFC3339Nano),
		Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
		Direction: "request", Method: "GenerateFromMessages", Model: model, TokensEst: totalEst,
		Input:   serializeMessages(messages),
		Details: map[string]any{"message_count": len(messages), "roles": roles},
	})

	resp, err := m.inner.GenerateFromMessages(ctx, model, messages)

	elapsed := time.Since(start)
	if err != nil {
		m.logger.Error("llm.response",
			"method", "GenerateFromMessages",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"error", err.Error(),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "GenerateFromMessages", Model: model, DurationMs: elapsed.Milliseconds(),
			Error: err.Error(),
		})
	} else {
		respEst := estimateTokens(resp)
		m.logger.Info("llm.response",
			"method", "GenerateFromMessages",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"response_tokens_est", respEst,
			"response_len", len(resp),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "GenerateFromMessages", Model: model, TokensEst: respEst,
			DurationMs: elapsed.Milliseconds(),
			Output:     resp,
			Details:    map[string]any{"response_len": len(resp)},
		})
	}
	return resp, err
}

func (m *LoggingMiddleware) GenerateWithTools(ctx context.Context, model string, systemPrompt string, userPrompt string, tools []responses.ToolUnionParam, executor ToolExecutor, maxRounds int) (ToolCallResult, error) {
	start := time.Now()
	lc := LogContextFromCtx(ctx)

	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		if t.OfFunction != nil {
			toolNames = append(toolNames, t.OfFunction.Name)
		}
	}
	totalEst := estimateTokens(systemPrompt) + estimateTokens(userPrompt)

	m.logger.Info("llm.request",
		"method", "GenerateWithTools",
		"model", model,
		"project", lc.ProjectSlug,
		"system_tokens_est", estimateTokens(systemPrompt),
		"user_tokens_est", estimateTokens(userPrompt),
		"total_tokens_est", totalEst,
		"tools", strings.Join(toolNames, ","),
		"max_rounds", maxRounds,
	)
	m.writeFile(AICallRecord{
		Timestamp: Now().Format(time.RFC3339Nano),
		Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
		Direction: "request", Method: "GenerateWithTools", Model: model, TokensEst: totalEst,
		Input:   map[string]any{"system": systemPrompt, "user": userPrompt, "tools": toolNames},
		Details: map[string]any{"max_rounds": maxRounds},
	})

	result, err := m.inner.GenerateWithTools(ctx, model, systemPrompt, userPrompt, tools, executor, maxRounds)

	elapsed := time.Since(start)
	if err != nil {
		m.logger.Error("llm.response",
			"method", "GenerateWithTools",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"error", err.Error(),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "GenerateWithTools", Model: model, DurationMs: elapsed.Milliseconds(),
			Error: err.Error(),
		})
	} else {
		toolCallSummary := make([]string, 0, len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			toolCallSummary = append(toolCallSummary, tc.Name)
		}
		respEst := estimateTokens(result.Text)

		m.logger.Info("llm.response",
			"method", "GenerateWithTools",
			"model", model,
			"project", lc.ProjectSlug,
			"duration_ms", elapsed.Milliseconds(),
			"response_tokens_est", respEst,
			"response_len", len(result.Text),
			"tool_calls_executed", strings.Join(toolCallSummary, ","),
			"tool_call_count", len(result.ToolCalls),
		)
		m.writeFile(AICallRecord{
			Timestamp: Now().Format(time.RFC3339Nano),
			Project:   lc.ProjectSlug, MissionID: lc.MissionID, ThreadID: lc.ThreadID, TraceID: lc.TraceID,
			Direction: "response", Method: "GenerateWithTools", Model: model, TokensEst: respEst,
			DurationMs: elapsed.Milliseconds(),
			Output:     result.Text,
			Details:    map[string]any{"tool_calls": toolCallSummary, "response_len": len(result.Text)},
		})
	}
	return result, err
}

// serializeMessages converts thread messages into a JSON-friendly slice for file logging.
func serializeMessages(messages []threads.Message) []map[string]string {
	out := make([]map[string]string, len(messages))
	for i, msg := range messages {
		out[i] = map[string]string{
			"role":    string(msg.Role),
			"content": msg.Content,
		}
	}
	return out
}

// estimateTokens returns a rough token estimate using ~4 chars per token.
func estimateTokens(text string) int {
	return estimateTokensFromCount(len(text))
}

func estimateTokensFromCount(charCount int) int {
	if charCount == 0 {
		return 0
	}
	return (charCount + 3) / 4 // ceil(charCount / 4)
}

// EstimateMessagesTokens returns the estimated total tokens for a slice of messages.
func EstimateMessagesTokens(messages []threads.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content)
	}
	return total
}

// LogRequestDetail writes a debug-level log with the full content of each message.
// Useful for deep debugging but off by default (only at DEBUG level).
func (m *LoggingMiddleware) LogRequestDetail(model string, messages []threads.Message) {
	if !m.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	for i, msg := range messages {
		content := msg.Content
		if len(content) > 2000 {
			content = content[:2000] + "...[truncated]"
		}
		m.logger.Debug("llm.request.detail",
			"model", model,
			"index", i,
			"role", string(msg.Role),
			"tokens_est", estimateTokens(msg.Content),
			"content", content,
		)
	}
}

// LogResponseDetail writes a debug-level log with the full response content.
func (m *LoggingMiddleware) LogResponseDetail(model string, response string) {
	if !m.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	content := response
	if len(content) > 2000 {
		content = content[:2000] + "...[truncated]"
	}
	m.logger.Debug("llm.response.detail",
		"model", model,
		"content", content,
	)
}

// writeFile appends a record to the file logger if one is attached.
func (m *LoggingMiddleware) writeFile(record AICallRecord) {
	if m.fileLogger == nil {
		return
	}
	if err := m.fileLogger.Write(record); err != nil {
		m.logger.Warn("failed to write AI call log file", "error", err)
	}
}
