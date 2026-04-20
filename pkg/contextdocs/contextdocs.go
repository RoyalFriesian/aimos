// Package contextdocs manages durable context documents that live on disk
// inside a project directory. These markdown files are automatically picked
// up by the knowledge indexer so agents can query the index instead of
// reading raw docs, keeping LLM token usage minimal.
package contextdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DocumentType identifies the kind of context document.
type DocumentType string

const (
	DocProjectOverview DocumentType = "PROJECT_OVERVIEW"
	DocProjectState    DocumentType = "PROJECT_STATE"
	DocAgentBrief      DocumentType = "AGENT_BRIEF"
	DocProjectConfig   DocumentType = "PROJECT_CONFIG"
	DocTaskContext     DocumentType = "TASK_CONTEXT"
)

// ContextDir is the subdirectory inside the project for context docs.
const ContextDir = ".aimos/context"

func filename(docType DocumentType, scopeID string) string {
	if scopeID != "" {
		return fmt.Sprintf("%s_%s.md", docType, sanitize(scopeID))
	}
	return fmt.Sprintf("%s.md", docType)
}

func docDir(projectDir string) string {
	return filepath.Join(projectDir, ContextDir)
}

// WriteDocument writes (or overwrites) a context document to disk.
func WriteDocument(projectDir string, docType DocumentType, scopeID string, content string) error {
	if projectDir == "" {
		return fmt.Errorf("contextdocs: project directory is empty")
	}
	d := docDir(projectDir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return fmt.Errorf("contextdocs: create dir %q: %w", d, err)
	}
	p := filepath.Join(d, filename(docType, scopeID))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return fmt.Errorf("contextdocs: write %q: %w", p, err)
	}
	return nil
}

// ReadDocument reads a context document from disk. Returns empty string
// and nil error when the file does not exist.
func ReadDocument(projectDir string, docType DocumentType, scopeID string) (string, error) {
	if projectDir == "" {
		return "", nil
	}
	p := filepath.Join(docDir(projectDir), filename(docType, scopeID))
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("contextdocs: read %q: %w", p, err)
	}
	return string(data), nil
}

// Exists returns true if the document is present on disk.
func Exists(projectDir string, docType DocumentType, scopeID string) bool {
	if projectDir == "" {
		return false
	}
	p := filepath.Join(docDir(projectDir), filename(docType, scopeID))
	_, err := os.Stat(p)
	return err == nil
}

// ListDocuments returns all context document paths for a project.
func ListDocuments(projectDir string) ([]string, error) {
	d := docDir(projectDir)
	entries, err := os.ReadDir(d)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("contextdocs: list %q: %w", d, err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			paths = append(paths, filepath.Join(d, e.Name()))
		}
	}
	return paths, nil
}

func writeSection(b *strings.Builder, heading, content string) {
	if content == "" {
		return
	}
	b.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", heading, content))
}

func sanitize(id string) string {
	s := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// --- Template input types ---

// ProjectOverviewInput holds fields for generating PROJECT_OVERVIEW.md.
type ProjectOverviewInput struct {
	Vision          string
	TargetUser      string
	KeyFeatures     string
	TechStack       string
	Constraints     string
	SuccessCriteria string
}

// ProjectStateInput holds fields for generating PROJECT_STATE.md.
type ProjectStateInput struct {
	CompletedFeatures  string
	InProgressFeatures string
	KnownIssues        string
	FileTreeSummary    string
}

// AgentBriefInput holds fields for generating AGENT_BRIEF_{id}.md.
type AgentBriefInput struct {
	AgentName          string
	Role               string
	ProblemStatement   string
	ToolsAvailable     string
	Workflow           string
	SelfCritiqueRules  string
	EscalationPath     string
	AcceptanceCriteria string
}

// ProjectConfigInput holds fields for generating PROJECT_CONFIG.md.
type ProjectConfigInput struct {
	ProjectDirectory  string
	LanguageFramework string
	FileConventions   string
	BuildAndRun       string
	Dependencies      string
}

// TaskContextInput holds fields for generating TASK_CONTEXT_{id}.md.
type TaskContextInput struct {
	TodoTitle          string
	Description        string
	Dependencies       string
	RelevantFiles      string
	AcceptanceCriteria string
	DoNotTouch         string
}

// --- Template generators ---

// GenerateProjectOverview creates PROJECT_OVERVIEW.md content.
func GenerateProjectOverview(p ProjectOverviewInput) string {
	var b strings.Builder
	b.WriteString("# Project Overview\n\n")
	writeSection(&b, "Vision", p.Vision)
	writeSection(&b, "Target User", p.TargetUser)
	writeSection(&b, "Key Features", p.KeyFeatures)
	writeSection(&b, "Tech Stack", p.TechStack)
	writeSection(&b, "Constraints", p.Constraints)
	writeSection(&b, "Success Criteria", p.SuccessCriteria)
	b.WriteString(fmt.Sprintf("\n_Generated: %s_\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

// GenerateProjectState creates PROJECT_STATE.md content.
func GenerateProjectState(p ProjectStateInput) string {
	var b strings.Builder
	b.WriteString("# Project State\n\n")
	writeSection(&b, "Completed Features", p.CompletedFeatures)
	writeSection(&b, "In-Progress Features", p.InProgressFeatures)
	writeSection(&b, "Known Issues", p.KnownIssues)
	writeSection(&b, "File Tree Summary", p.FileTreeSummary)
	b.WriteString(fmt.Sprintf("\n_Last updated: %s_\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

// GenerateAgentBrief creates AGENT_BRIEF_{agentID}.md content.
func GenerateAgentBrief(p AgentBriefInput) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Agent Brief: %s\n\n", p.AgentName))
	writeSection(&b, "Role", p.Role)
	writeSection(&b, "Problem Statement", p.ProblemStatement)
	writeSection(&b, "Tools Available", p.ToolsAvailable)
	writeSection(&b, "Workflow", p.Workflow)
	writeSection(&b, "Self-Critique Rules", p.SelfCritiqueRules)
	writeSection(&b, "Escalation Path", p.EscalationPath)
	writeSection(&b, "Acceptance Criteria", p.AcceptanceCriteria)
	b.WriteString(fmt.Sprintf("\n_Generated: %s_\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

// GenerateProjectConfig creates PROJECT_CONFIG.md content.
func GenerateProjectConfig(p ProjectConfigInput) string {
	var b strings.Builder
	b.WriteString("# Project Configuration\n\n")
	writeSection(&b, "Project Directory", p.ProjectDirectory)
	writeSection(&b, "Language / Framework", p.LanguageFramework)
	writeSection(&b, "File Conventions", p.FileConventions)
	writeSection(&b, "Build & Run", p.BuildAndRun)
	writeSection(&b, "Dependencies", p.Dependencies)
	b.WriteString(fmt.Sprintf("\n_Generated: %s_\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

// GenerateTaskContext creates TASK_CONTEXT_{todoID}.md content.
func GenerateTaskContext(p TaskContextInput) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Task: %s\n\n", p.TodoTitle))
	writeSection(&b, "Description", p.Description)
	writeSection(&b, "Dependencies", p.Dependencies)
	writeSection(&b, "Relevant Files", p.RelevantFiles)
	writeSection(&b, "Acceptance Criteria", p.AcceptanceCriteria)
	writeSection(&b, "Do NOT Touch", p.DoNotTouch)
	b.WriteString(fmt.Sprintf("\n_Generated: %s_\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}
