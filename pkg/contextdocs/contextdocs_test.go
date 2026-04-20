package contextdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadDocument(t *testing.T) {
	tmp := t.TempDir()
	content := "# Test Overview\n\nThis is a test."
	if err := WriteDocument(tmp, DocProjectOverview, "", content); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDocument(tmp, DocProjectOverview, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", got, content)
	}
}

func TestScopedDocument(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteDocument(tmp, DocAgentBrief, "agent-42", "brief content"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadDocument(tmp, DocAgentBrief, "agent-42")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "brief content" {
		t.Errorf("got %q, want brief content", got)
	}
	// Different scope ID should return empty.
	other, err := ReadDocument(tmp, DocAgentBrief, "agent-99")
	if err != nil {
		t.Fatalf("read other: %v", err)
	}
	if other != "" {
		t.Errorf("expected empty for different scope, got %q", other)
	}
}

func TestReadMissingDocument(t *testing.T) {
	tmp := t.TempDir()
	got, err := ReadDocument(tmp, DocProjectState, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for missing doc, got %q", got)
	}
}

func TestExists(t *testing.T) {
	tmp := t.TempDir()
	if Exists(tmp, DocProjectConfig, "") {
		t.Error("should not exist before write")
	}
	if err := WriteDocument(tmp, DocProjectConfig, "", "config"); err != nil {
		t.Fatal(err)
	}
	if !Exists(tmp, DocProjectConfig, "") {
		t.Error("should exist after write")
	}
}

func TestListDocuments(t *testing.T) {
	tmp := t.TempDir()
	_ = WriteDocument(tmp, DocProjectOverview, "", "overview")
	_ = WriteDocument(tmp, DocProjectState, "", "state")
	_ = WriteDocument(tmp, DocAgentBrief, "a1", "brief")
	paths, err := ListDocuments(tmp)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 docs, got %d: %v", len(paths), paths)
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") {
			t.Errorf("expected .md file, got %q", p)
		}
	}
}

func TestListDocumentsNoDir(t *testing.T) {
	tmp := t.TempDir()
	paths, err := ListDocuments(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty list, got %v", paths)
	}
}

func TestEmptyProjectDir(t *testing.T) {
	err := WriteDocument("", DocProjectOverview, "", "test")
	if err == nil {
		t.Error("expected error for empty project dir")
	}
	got, err := ReadDocument("", DocProjectOverview, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if Exists("", DocProjectOverview, "") {
		t.Error("Exists should return false for empty dir")
	}
}

func TestOverwrite(t *testing.T) {
	tmp := t.TempDir()
	_ = WriteDocument(tmp, DocProjectState, "", "v1")
	_ = WriteDocument(tmp, DocProjectState, "", "v2")
	got, _ := ReadDocument(tmp, DocProjectState, "")
	if got != "v2" {
		t.Errorf("expected v2 after overwrite, got %q", got)
	}
}

func TestGenerateProjectOverview(t *testing.T) {
	content := GenerateProjectOverview(ProjectOverviewInput{
		Vision:     "Build a world-class todo app",
		TargetUser: "Developers",
		TechStack:  "Go + React",
	})
	if !strings.Contains(content, "# Project Overview") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "Build a world-class todo app") {
		t.Error("missing vision")
	}
	if !strings.Contains(content, "Go + React") {
		t.Error("missing tech stack")
	}
	// Empty optional fields should be omitted.
	if strings.Contains(content, "## Constraints") {
		t.Error("constraints section should be omitted when empty")
	}
}

func TestGenerateAgentBrief(t *testing.T) {
	content := GenerateAgentBrief(AgentBriefInput{
		AgentName:        "frontend-worker",
		Role:             "Worker",
		ProblemStatement: "Build the login page",
		ToolsAvailable:   "write_file, deliver_work",
	})
	if !strings.Contains(content, "# Agent Brief: frontend-worker") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "Build the login page") {
		t.Error("missing problem statement")
	}
}

func TestGenerateTaskContext(t *testing.T) {
	content := GenerateTaskContext(TaskContextInput{
		TodoTitle:          "Create REST API",
		Description:        "Build CRUD endpoints for todos",
		RelevantFiles:      "- cmd/server/main.go\n- pkg/api/handler.go",
		AcceptanceCriteria: "All endpoints return JSON",
		DoNotTouch:         "- web-ui/",
	})
	if !strings.Contains(content, "# Task: Create REST API") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "web-ui/") {
		t.Error("missing do-not-touch section")
	}
}

func TestFilenameOnDisk(t *testing.T) {
	tmp := t.TempDir()
	_ = WriteDocument(tmp, DocTaskContext, "todo-123", "task data")
	expected := filepath.Join(tmp, ContextDir, "TASK_CONTEXT_todo-123.md")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file at %q, got error: %v", expected, err)
	}
}

func TestSanitizeSpecialChars(t *testing.T) {
	tmp := t.TempDir()
	scopeID := "agent/../../etc/passwd"
	_ = WriteDocument(tmp, DocAgentBrief, scopeID, "safe")
	got, _ := ReadDocument(tmp, DocAgentBrief, scopeID)
	if got != "safe" {
		t.Errorf("expected safe, got %q", got)
	}
	entries, _ := os.ReadDir(filepath.Join(tmp, ContextDir))
	for _, e := range entries {
		if strings.Contains(e.Name(), "..") || strings.Contains(e.Name(), "/") {
			t.Errorf("unsafe filename: %q", e.Name())
		}
	}
}
