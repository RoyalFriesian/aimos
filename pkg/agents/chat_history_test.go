package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestChatHistoryWriter_WriteEntry(t *testing.T) {
	dir := t.TempDir()

	w := NewChatHistoryWriter()
	entry := ChatHistoryEntry{
		Timestamp:   time.Now().UTC(),
		EntryType:   "turn",
		AgentID:     "agent-123",
		AgentRole:   "Worker",
		MessageType: "agent_turn_summary",
		Content:     "Created main.go with HTTP server setup",
	}

	if err := w.WriteEntry(dir, entry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// Verify the file was created.
	logPath := filepath.Join(dir, chatHistoryDir, "agent-123.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), "Created main.go") {
		t.Errorf("log file does not contain expected content: %s", string(data))
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Errorf("log file entry should end with newline")
	}

	// Write a second entry and verify append.
	entry2 := ChatHistoryEntry{
		Timestamp: time.Now().UTC(),
		EntryType: "turn",
		AgentID:   "agent-123",
		Content:   "Added package.json",
	}
	if err := w.WriteEntry(dir, entry2); err != nil {
		t.Fatalf("second WriteEntry failed: %v", err)
	}

	data2, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(data2)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestChatHistoryWriter_EmptyProjectDir(t *testing.T) {
	w := NewChatHistoryWriter()
	// Should silently skip with no error.
	if err := w.WriteEntry("", ChatHistoryEntry{Content: "test"}); err != nil {
		t.Errorf("expected nil error for empty project dir, got: %v", err)
	}
}

func TestChatHistoryWriter_UpdateIndex(t *testing.T) {
	dir := t.TempDir()
	w := NewChatHistoryWriter()

	// Add first agent.
	if err := w.UpdateIndex(dir, ChatHistoryAgentEntry{
		AgentID:   "agent-1",
		Role:      "Worker",
		ThreadID:  "thread-1",
		MissionID: "mission-1",
		File:      "agent-1.jsonl",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	// Add second agent.
	if err := w.UpdateIndex(dir, ChatHistoryAgentEntry{
		AgentID:   "agent-2",
		Role:      "CEO",
		ThreadID:  "thread-2",
		MissionID: "mission-1",
		File:      "agent-2.jsonl",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateIndex second agent failed: %v", err)
	}

	indexPath := filepath.Join(dir, chatHistoryDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "agent-1") || !strings.Contains(content, "agent-2") {
		t.Errorf("index should contain both agents: %s", content)
	}

	// Update existing agent (upsert).
	if err := w.UpdateIndex(dir, ChatHistoryAgentEntry{
		AgentID:   "agent-1",
		Role:      "Worker",
		ThreadID:  "thread-1-updated",
		MissionID: "mission-1",
		File:      "agent-1.jsonl",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateIndex upsert failed: %v", err)
	}

	data2, _ := os.ReadFile(indexPath)
	if strings.Count(string(data2), "agent-1") > 2 {
		// Should appear twice: once in "id" and once in "file", not duplicated as separate entries.
		t.Errorf("agent-1 should not be duplicated in index")
	}
}

func TestMessagesAfterLastSummary(t *testing.T) {
	msgs := []threads.Message{
		{ID: "1", Content: "started work", MessageType: "agent_turn_summary"},
		{ID: "2", Content: "wrote file A", MessageType: "agent_turn_summary"},
		{ID: "s1", Content: "Summary: wrote file A for backend", MessageType: MessageTypeChatSummary},
		{ID: "3", Content: "wrote file B", MessageType: "agent_turn_summary"},
		{ID: "4", Content: "delivered work", MessageType: "agent_turn_summary"},
	}

	summary, recent := MessagesAfterLastSummary(msgs)

	if summary != "Summary: wrote file A for backend" {
		t.Errorf("expected summary text, got: %q", summary)
	}
	if len(recent) != 2 {
		t.Errorf("expected 2 recent messages, got %d", len(recent))
	}
	if recent[0].ID != "3" || recent[1].ID != "4" {
		t.Errorf("wrong recent messages: %v", recent)
	}
}

func TestMessagesAfterLastSummary_NoSummary(t *testing.T) {
	msgs := []threads.Message{
		{ID: "1", Content: "hello", MessageType: "agent_turn_summary"},
		{ID: "2", Content: "world", MessageType: "agent_turn_summary"},
	}

	summary, recent := MessagesAfterLastSummary(msgs)

	if summary != "" {
		t.Errorf("expected empty summary, got: %q", summary)
	}
	if len(recent) != 2 {
		t.Errorf("expected all 2 messages, got %d", len(recent))
	}
}

func TestMessagesAfterLastSummary_MultipleSummaries(t *testing.T) {
	msgs := []threads.Message{
		{ID: "1", Content: "early work", MessageType: "agent_turn_summary"},
		{ID: "s1", Content: "First summary", MessageType: MessageTypeChatSummary},
		{ID: "2", Content: "middle work", MessageType: "agent_turn_summary"},
		{ID: "s2", Content: "Second summary", MessageType: MessageTypeChatSummary},
		{ID: "3", Content: "latest work", MessageType: "agent_turn_summary"},
	}

	summary, recent := MessagesAfterLastSummary(msgs)

	if summary != "Second summary" {
		t.Errorf("expected latest summary, got: %q", summary)
	}
	if len(recent) != 1 || recent[0].ID != "3" {
		t.Errorf("expected only messages after second summary, got: %v", recent)
	}
}

func TestNeedsSummarization(t *testing.T) {
	// 3 regular messages, threshold 5 — not ready.
	msgs := []threads.Message{
		{ID: "1", MessageType: "agent_turn_summary"},
		{ID: "2", MessageType: "agent_turn_summary"},
		{ID: "3", MessageType: "agent_turn_summary"},
	}
	if NeedsSummarization(msgs, 5) {
		t.Error("should not need summarization with only 3 messages and threshold 5")
	}

	// 5 messages after a summary, threshold 5 — ready.
	msgs2 := []threads.Message{
		{ID: "s1", MessageType: MessageTypeChatSummary},
		{ID: "1", MessageType: "agent_turn_summary"},
		{ID: "2", MessageType: "agent_turn_summary"},
		{ID: "3", MessageType: "agent_turn_summary"},
		{ID: "4", MessageType: "agent_turn_summary"},
		{ID: "5", MessageType: "agent_turn_summary"},
	}
	if !NeedsSummarization(msgs2, 5) {
		t.Error("should need summarization with 5 messages after summary")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"agent-123", "agent-123"},
		{"agent/with/slashes", "agentwithslashes"},
		{"agent with spaces", "agentwithspaces"},
		{"agent--special_chars!", "agent--special_chars"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
