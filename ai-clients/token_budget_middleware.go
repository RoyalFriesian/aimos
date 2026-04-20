package aiclients

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/openai/openai-go/v3/responses"
)

const summarizeSystemPrompt = `You are a context compressor. Given a conversation, produce a SHORT, self-contained summary that preserves all essential facts, decisions, and instructions.

Rules:
- Be extremely concise — target the word budget given below
- Preserve: key decisions, specific values, file names, technical details, constraints
- Remove: greetings, filler, repetition, verbose explanations
- Write in present tense for current state, past tense for completed items
- The summary replaces the original messages — it must be self-contained`

// TokenBudgetMiddleware checks estimated token count before each LLM call.
// If the input exceeds the configured threshold, it summarizes the context
// using a cheap LLM call to fit within the target budget.
type TokenBudgetMiddleware struct {
	inner  CompletionProvider
	config *BudgetConfig
	logger *slog.Logger
}

// NewTokenBudgetMiddleware wraps a provider with token budget enforcement.
func NewTokenBudgetMiddleware(inner CompletionProvider, config *BudgetConfig, logger *slog.Logger) *TokenBudgetMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &TokenBudgetMiddleware{inner: inner, config: config, logger: logger}
}

func (m *TokenBudgetMiddleware) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	if !m.config.Enabled() {
		return m.inner.Generate(ctx, model, systemPrompt, userPrompt)
	}

	totalTokens := estimateTokens(systemPrompt) + estimateTokens(userPrompt)
	threshold := int(m.config.Threshold())

	if totalTokens <= threshold {
		return m.inner.Generate(ctx, model, systemPrompt, userPrompt)
	}

	m.logger.Info("token_budget.summarizing",
		"model", model,
		"estimated_tokens", totalTokens,
		"threshold", threshold,
		"target", m.config.Target(),
	)

	// Summarize the user prompt (system prompt is kept intact as it defines behavior).
	summarized, err := m.summarizeText(ctx, model, userPrompt)
	if err != nil {
		m.logger.Warn("token_budget.summarize_failed, proceeding with original",
			"error", err.Error(),
		)
		return m.inner.Generate(ctx, model, systemPrompt, userPrompt)
	}

	m.logger.Info("token_budget.summarized",
		"original_tokens", estimateTokens(userPrompt),
		"summarized_tokens", estimateTokens(summarized),
	)

	return m.inner.Generate(ctx, model, systemPrompt, summarized)
}

func (m *TokenBudgetMiddleware) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	if !m.config.Enabled() {
		return m.inner.GenerateFromMessages(ctx, model, messages)
	}

	totalTokens := EstimateMessagesTokens(messages)
	threshold := int(m.config.Threshold())

	if totalTokens <= threshold {
		return m.inner.GenerateFromMessages(ctx, model, messages)
	}

	m.logger.Info("token_budget.summarizing_messages",
		"model", model,
		"message_count", len(messages),
		"estimated_tokens", totalTokens,
		"threshold", threshold,
		"target", m.config.Target(),
	)

	compacted, err := m.compactMessages(ctx, model, messages)
	if err != nil {
		m.logger.Warn("token_budget.summarize_failed, proceeding with original",
			"error", err.Error(),
		)
		return m.inner.GenerateFromMessages(ctx, model, messages)
	}

	newTokens := EstimateMessagesTokens(compacted)
	m.logger.Info("token_budget.summarized_messages",
		"original_tokens", totalTokens,
		"summarized_tokens", newTokens,
		"original_messages", len(messages),
		"summarized_messages", len(compacted),
	)

	return m.inner.GenerateFromMessages(ctx, model, compacted)
}

func (m *TokenBudgetMiddleware) GenerateWithTools(ctx context.Context, model string, systemPrompt string, userPrompt string, tools []responses.ToolUnionParam, executor ToolExecutor, maxRounds int) (ToolCallResult, error) {
	if !m.config.Enabled() {
		return m.inner.GenerateWithTools(ctx, model, systemPrompt, userPrompt, tools, executor, maxRounds)
	}

	totalTokens := estimateTokens(systemPrompt) + estimateTokens(userPrompt)
	threshold := int(m.config.Threshold())

	if totalTokens <= threshold {
		return m.inner.GenerateWithTools(ctx, model, systemPrompt, userPrompt, tools, executor, maxRounds)
	}

	m.logger.Info("token_budget.summarizing_tools",
		"model", model,
		"estimated_tokens", totalTokens,
		"threshold", threshold,
	)

	summarized, err := m.summarizeText(ctx, model, userPrompt)
	if err != nil {
		m.logger.Warn("token_budget.summarize_failed, proceeding with original",
			"error", err.Error(),
		)
		return m.inner.GenerateWithTools(ctx, model, systemPrompt, userPrompt, tools, executor, maxRounds)
	}

	return m.inner.GenerateWithTools(ctx, model, systemPrompt, summarized, tools, executor, maxRounds)
}

// compactMessages separates system messages from conversation messages,
// summarizes the conversation portion, and returns a reduced message list.
func (m *TokenBudgetMiddleware) compactMessages(ctx context.Context, model string, messages []threads.Message) ([]threads.Message, error) {
	var systemMsgs []threads.Message
	var conversationMsgs []threads.Message

	for _, msg := range messages {
		if msg.Role == threads.RoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else {
			conversationMsgs = append(conversationMsgs, msg)
		}
	}

	// If there's nothing to summarize beyond system messages, return as-is.
	if len(conversationMsgs) == 0 {
		return messages, nil
	}

	// Build the conversation text to summarize.
	var b strings.Builder
	for _, msg := range conversationMsgs {
		b.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}

	summarized, err := m.summarizeText(ctx, model, b.String())
	if err != nil {
		return nil, err
	}

	// Reconstruct: keep system messages + one summarized user message.
	result := make([]threads.Message, 0, len(systemMsgs)+1)
	result = append(result, systemMsgs...)
	result = append(result, threads.Message{
		Role:    threads.RoleUser,
		Content: summarized,
	})

	return result, nil
}

// summarizeText calls the LLM to compress text within the target token budget.
func (m *TokenBudgetMiddleware) summarizeText(ctx context.Context, model string, text string) (string, error) {
	targetTokens := m.config.Target()
	// Approximate words from tokens (~0.75 words per token).
	targetWords := int(float64(targetTokens) * 0.75)

	prompt := fmt.Sprintf("Compress the following into at most %d words (roughly %d tokens):\n\n%s",
		targetWords, targetTokens, text)

	return m.inner.Generate(ctx, model, summarizeSystemPrompt, prompt)
}
