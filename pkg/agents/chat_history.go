package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const chatHistoryDir = ".aimos/chat-history"

// ChatHistoryEntry is a single line in the JSONL chat log.
type ChatHistoryEntry struct {
	Timestamp   time.Time `json:"ts"`
	EntryType   string    `json:"type"` // "turn", "summary", "action", "system"
	AgentID     string    `json:"agent"`
	AgentRole   string    `json:"role"`
	MessageType string    `json:"msgType,omitempty"`
	Content     string    `json:"content"`
}

// ChatHistoryIndex tracks all agents whose history is logged.
type ChatHistoryIndex struct {
	Agents []ChatHistoryAgentEntry `json:"agents"`
}

// ChatHistoryAgentEntry describes one agent's chat log file.
type ChatHistoryAgentEntry struct {
	AgentID   string    `json:"id"`
	Role      string    `json:"role"`
	ThreadID  string    `json:"threadId"`
	MissionID string    `json:"missionId"`
	File      string    `json:"file"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ChatHistoryWriter writes structured conversation logs to project files.
// It is safe for concurrent use from multiple goroutines.
type ChatHistoryWriter struct {
	mu sync.Mutex
}

// NewChatHistoryWriter creates a new writer.
func NewChatHistoryWriter() *ChatHistoryWriter {
	return &ChatHistoryWriter{}
}

// WriteEntry appends a single JSONL entry to the agent's chat log file.
func (w *ChatHistoryWriter) WriteEntry(projectDir string, entry ChatHistoryEntry) error {
	if projectDir == "" {
		return nil // no project dir, skip silently
	}

	dir := filepath.Join(projectDir, chatHistoryDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chat history dir: %w", err)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal chat history entry: %w", err)
	}
	data = append(data, '\n')

	logPath := filepath.Join(dir, sanitizeFilename(entry.AgentID)+".jsonl")

	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open chat log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write chat log entry: %w", err)
	}

	return nil
}

// UpdateIndex updates (or creates) the agents index file so any agent can
// discover all available chat histories.
func (w *ChatHistoryWriter) UpdateIndex(projectDir string, agent ChatHistoryAgentEntry) error {
	if projectDir == "" {
		return nil
	}

	dir := filepath.Join(projectDir, chatHistoryDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chat history dir: %w", err)
	}

	indexPath := filepath.Join(dir, "index.json")

	w.mu.Lock()
	defer w.mu.Unlock()

	var index ChatHistoryIndex
	if data, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(data, &index) // best-effort load
	}

	// Upsert agent entry.
	found := false
	for i, existing := range index.Agents {
		if existing.AgentID == agent.AgentID {
			index.Agents[i] = agent
			found = true
			break
		}
	}
	if !found {
		index.Agents = append(index.Agents, agent)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	return os.WriteFile(indexPath, data, 0o644)
}

// sanitizeFilename removes characters that are unsafe in file names.
func sanitizeFilename(name string) string {
	var out []byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
}
