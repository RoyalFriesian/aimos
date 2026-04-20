package aiclients

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FileLogger writes structured JSONL records to per-project log files.
//
// Directory layout:
//
//	{baseDir}/{project-slug}/ai-calls.jsonl   — per-project log
//	{baseDir}/_default/ai-calls.jsonl         — calls without a project
type FileLogger struct {
	baseDir string
	mu      sync.Mutex
	files   map[string]*os.File // slug → open file handle
}

// AICallRecord is a single JSONL line written by the file logger.
type AICallRecord struct {
	Timestamp   string         `json:"timestamp"`
	Project     string         `json:"project,omitempty"`
	MissionID   string         `json:"missionId,omitempty"`
	ThreadID    string         `json:"threadId,omitempty"`
	TraceID     string         `json:"traceId,omitempty"`
	Direction   string         `json:"direction"` // "request" or "response"
	Method      string         `json:"method"`
	Model       string         `json:"model"`
	TokensEst   int            `json:"tokensEst"`
	DurationMs  int64          `json:"durationMs,omitempty"`
	Error       string         `json:"error,omitempty"`
	Input       any            `json:"input,omitempty"`
	Output      string         `json:"output,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

// NewFileLogger creates a file logger that writes under baseDir.
// It creates the directory if it does not exist.
func NewFileLogger(baseDir string) (*FileLogger, error) {
	if baseDir == "" {
		baseDir = "logs/ai-calls"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log base dir: %w", err)
	}
	return &FileLogger{
		baseDir: baseDir,
		files:   make(map[string]*os.File),
	}, nil
}

// Write appends one JSONL record for the given project.
func (fl *FileLogger) Write(record AICallRecord) error {
	slug := sanitizeSlug(record.Project)
	if slug == "" {
		slug = "_default"
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal log record: %w", err)
	}
	data = append(data, '\n')

	fl.mu.Lock()
	defer fl.mu.Unlock()

	f, err := fl.openFile(slug)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// Close flushes and closes all open file handles.
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	var firstErr error
	for slug, f := range fl.files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(fl.files, slug)
	}
	return firstErr
}

// openFile returns an existing or newly opened *os.File for the slug.
// Caller must hold fl.mu.
func (fl *FileLogger) openFile(slug string) (*os.File, error) {
	if f, ok := fl.files[slug]; ok {
		return f, nil
	}
	dir := filepath.Join(fl.baseDir, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create project log dir: %w", err)
	}
	path := filepath.Join(dir, "ai-calls.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}
	fl.files[slug] = f
	return f, nil
}

// Now is a replaceable clock for testing.
var Now = func() time.Time { return time.Now().UTC() }

var slugRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeSlug(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Use the last path component as the slug if a path is provided.
	s = filepath.Base(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.ToLower(s)
}
