package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// QAResult holds the output of a QA agent validation run.
type QAResult struct {
	Status  string    `json:"status"` // "PASS" or "FAIL"
	Summary string    `json:"summary"`
	Issues  []QAIssue `json:"issues,omitempty"`
}

// QAIssue describes a single quality problem found by the QA agent.
type QAIssue struct {
	File        string `json:"file"`
	Severity    string `json:"severity"` // "critical", "major", "minor"
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// QAAgent validates the integrated codebase after merge via an LLM call.
// Unlike TestingAgent (which validates individual deliverables), QAAgent
// inspects the full merged project for integration issues, missing files,
// broken imports, and overall quality.
type QAAgent struct {
	llm   LoopCompletionClient
	model string
}

// NewQAAgent creates a QA agent.
func NewQAAgent(llm LoopCompletionClient, model string) *QAAgent {
	if model == "" {
		model = "gpt-4.1"
	}
	return &QAAgent{llm: llm, model: model}
}

// ValidateProject runs a full-project quality check on the merged codebase.
func (q *QAAgent) ValidateProject(ctx context.Context, input QAValidationPayload) (QAResult, error) {
	if q.llm == nil {
		return QAResult{Status: "PASS", Summary: "No LLM configured, auto-passing."}, nil
	}

	// Read a sample of key files to give the QA agent real content to review.
	fileSamples := q.sampleProjectFiles(input.ProjectDir, 15, 4096)

	userPrompt := fmt.Sprintf(`## Project Quality Review

**Mission Goal**: %s

### Project Files
%s

### File Contents (sample)
%s`,
		input.MissionGoal,
		input.FileList,
		fileSamples,
	)

	if len(input.Issues) > 0 {
		userPrompt += fmt.Sprintf("\n\n### Known Issues from Merge\n%s",
			strings.Join(input.Issues, "\n"))
	}

	userPrompt += `

---

Perform a thorough quality review. Check for:
1. Missing files that should exist based on imports/references
2. Broken imports or references between files
3. Inconsistent APIs (frontend calling endpoints that backend doesn't expose)
4. Missing error handling, security issues
5. Files that are too incomplete or placeholder-only
6. Missing package.json dependencies, go.mod entries, etc.
7. Integration issues between components (frontend/backend contract mismatches)

Respond with a JSON object:
{"status": "PASS" or "FAIL", "summary": "brief overall assessment", "issues": [{"file": "path", "severity": "critical|major|minor", "description": "what's wrong", "suggestion": "how to fix"}]}`

	raw, err := q.llm.Generate(ctx, q.model, qaSystemPrompt, userPrompt)
	if err != nil {
		return QAResult{}, fmt.Errorf("QA agent LLM call: %w", err)
	}

	var result QAResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// Try to extract JSON from markdown fences.
		cleaned := extractJSONFromQA(raw)
		if err2 := json.Unmarshal([]byte(cleaned), &result); err2 != nil {
			return QAResult{
				Status:  "FAIL",
				Summary: fmt.Sprintf("Could not parse QA result: %s", truncateString(raw, 500)),
			}, nil
		}
	}
	return result, nil
}

// sampleProjectFiles reads the first maxBytes from up to maxFiles project files.
func (q *QAAgent) sampleProjectFiles(projectDir string, maxFiles int, maxBytesPerFile int) string {
	if projectDir == "" {
		return "(no project directory)"
	}

	var samples []string
	count := 0

	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// Only read source files.
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".html", ".css", ".json", ".yaml", ".yml", ".toml", ".md", ".sql", ".sh":
			// ok
		default:
			return nil
		}

		rel, _ := filepath.Rel(projectDir, path)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if len(content) > maxBytesPerFile {
			content = content[:maxBytesPerFile] + "\n... (truncated)"
		}
		samples = append(samples, fmt.Sprintf("### %s\n```\n%s\n```", filepath.ToSlash(rel), content))
		count++
		if count >= maxFiles {
			return filepath.SkipAll
		}
		return nil
	})

	if len(samples) == 0 {
		return "(no source files found)"
	}
	return strings.Join(samples, "\n\n")
}

// extractJSONFromQA tries to pull JSON from a markdown fenced code block.
func extractJSONFromQA(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	// Find first { and last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return strings.TrimSpace(s)
}

const qaSystemPrompt = `You are an independent QA (Quality Assurance) agent. Your job is to validate the INTEGRATED codebase after all worker branches have been merged.

You must be thorough and strict:
1. Check that all imports/references between files resolve correctly.
2. Check that frontend and backend contracts match (API endpoints, request/response shapes).
3. Check that package manifests (package.json, go.mod, etc.) include all required dependencies.
4. Check for missing files — if a file is imported but doesn't exist, that's a critical issue.
5. Check for placeholder or incomplete implementations.
6. Check for security issues (hardcoded secrets, SQL injection, XSS, etc.).
7. Check for basic correctness — does the code make logical sense?

Severity levels:
- critical: App will crash or fundamentally not work (missing files, broken imports, wrong API contracts)
- major: Significant quality issue (no error handling, security vulnerability, incomplete feature)
- minor: Style, naming, or minor improvement opportunity

You MUST respond with a JSON object:
{"status": "PASS" or "FAIL", "summary": "overall assessment", "issues": [{"file": "path", "severity": "critical|major|minor", "description": "what's wrong", "suggestion": "how to fix"}]}

Return FAIL if ANY critical or major issue exists. Return PASS only if the codebase is integration-ready.`
