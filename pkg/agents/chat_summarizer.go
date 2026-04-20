package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

const (
	// DefaultChatSummaryEveryNTurns controls how often the agent loop
	// generates a conversation summary. The summary is inserted into the
	// thread as a chat_summary message and written to the chat history file.
	DefaultChatSummaryEveryNTurns = 10

	// MessageTypeChatSummary is the message type used for periodic
	// conversation summaries. Context building can look for this to decide
	// where to start replaying messages.
	MessageTypeChatSummary = "chat_summary"

	chatSummarySystemPrompt = `You are a concise conversation summarizer for an AI agent system.
Given a sequence of agent messages from a work thread, produce a SHORT summary paragraph.

Focus on:
1. What was accomplished (files written, todos completed, decisions made)
2. Current state (what is in progress, what is blocked)
3. Key decisions or direction changes
4. Any open problems or blockers

Rules:
- Maximum 200 words
- No filler or preamble — start with the facts
- Use past tense for completed items, present tense for current state
- Include file names and action types when relevant
- This summary will replace the raw messages in future LLM context, so it must be self-contained`
)

// ChatSummarizer generates periodic conversation summaries using the LLM.
type ChatSummarizer struct {
	llm   LoopCompletionClient
	model string
}

// NewChatSummarizer creates a summarizer that uses the given LLM client.
func NewChatSummarizer(llm LoopCompletionClient, model string) *ChatSummarizer {
	return &ChatSummarizer{llm: llm, model: model}
}

// SummarizeMessages generates a concise summary of a batch of messages.
func (s *ChatSummarizer) SummarizeMessages(ctx context.Context, agentID string, missionTitle string, messages []threads.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Agent: %s\nMission: %s\n", agentID, missionTitle))
	b.WriteString(fmt.Sprintf("Messages to summarize (%d):\n\n", len(messages)))

	for _, msg := range messages {
		// Truncate individual messages to keep the summarization prompt compact.
		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		b.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n",
			msg.CreatedAt.Format("15:04:05"),
			msg.AuthorAgentID,
			msg.MessageType,
			content,
		))
	}

	summary, err := s.llm.Generate(ctx, s.model, chatSummarySystemPrompt, b.String())
	if err != nil {
		return "", fmt.Errorf("LLM summarize: %w", err)
	}

	// Clean up: the LLM might return quotes or markdown; strip them.
	summary = strings.TrimSpace(summary)
	summary = strings.Trim(summary, "\"")

	return summary, nil
}

// MessagesAfterLastSummary splits a message list into the latest summary
// and the messages that came after it. If no summary exists, the full
// list is returned with an empty summary string.
func MessagesAfterLastSummary(messages []threads.Message) (lastSummary string, recent []threads.Message) {
	lastSummaryIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].MessageType == MessageTypeChatSummary {
			lastSummaryIdx = i
			break
		}
	}

	if lastSummaryIdx < 0 {
		return "", messages
	}

	return messages[lastSummaryIdx].Content, messages[lastSummaryIdx+1:]
}

// MessagesSinceLastSummary returns only the messages after the most recent
// chat_summary. Returns all messages if no summary exists.
func MessagesSinceLastSummary(messages []threads.Message) []threads.Message {
	_, recent := MessagesAfterLastSummary(messages)
	return recent
}

// NeedsSummarization returns true when enough messages have accumulated
// since the last chat_summary to warrant a new one.
func NeedsSummarization(messages []threads.Message, threshold int) bool {
	since := MessagesSinceLastSummary(messages)
	return len(since) >= threshold
}

// createChatSummaryMessage builds a Message suitable for storing the summary
// in the thread.
func createChatSummaryMessage(threadID, agentID, agentRole, summary string) threads.Message {
	return threads.Message{
		ID:            fmt.Sprintf("summary-%s-%d", agentID, time.Now().UnixNano()),
		ThreadID:      threadID,
		Role:          threads.RoleAssistant,
		AuthorAgentID: agentID,
		AuthorRole:    agentRole,
		MessageType:   MessageTypeChatSummary,
		Content:       summary,
		CreatedAt:     time.Now().UTC(),
	}
}
