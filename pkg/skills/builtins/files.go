package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/skills"
)

// HandleWriteFile writes content to a file within the project directory.
func HandleWriteFile(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse write_file args: %w", err)
	}
	if p.Path == "" || p.Content == "" {
		return "", fmt.Errorf("path and content are required")
	}

	root := resolveProjectRoot(env)
	if root == "" {
		return "", fmt.Errorf("write_file: project directory not configured")
	}

	fullPath := filepath.Join(root, p.Path)
	if err := validatePathSecurity(fullPath, root); err != nil {
		return "", err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(p.Content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", fullPath, err)
	}

	if env.OnFilesChanged != nil {
		env.OnFilesChanged(ctx, root)
	}

	return fmt.Sprintf("File written: %s (%d bytes)", p.Path, len(p.Content)), nil
}

// HandleReadFile reads a file from the project directory and returns its content.
func HandleReadFile(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse read_file args: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	root := resolveProjectRoot(env)
	if root == "" {
		return "", fmt.Errorf("read_file: project directory not configured")
	}

	fullPath := filepath.Join(root, p.Path)
	if err := validatePathSecurity(fullPath, root); err != nil {
		return "", err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", fullPath, err)
	}

	content := string(data)
	const maxLen = 100_000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... [truncated]"
	}
	return content, nil
}

// HandleRunQA runs the QA agent against the project.
func HandleRunQA(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	if env.QA == nil {
		return "QA agent not available", nil
	}
	root := resolveProjectRoot(env)
	result, err := env.QA.ValidateProject(ctx, skills.QAInput{
		ProjectDir:  root,
		MissionGoal: "",
		AgentID:     env.AgentID,
	})
	if err != nil {
		return "", fmt.Errorf("run_qa: %w", err)
	}
	return fmt.Sprintf("QA %s: %s", result.Status, result.Summary), nil
}

func resolveProjectRoot(env *skills.Env) string {
	if env.ProjectDir != "" {
		return env.ProjectDir
	}
	return ""
}

func validatePathSecurity(fullPath, root string) error {
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return fmt.Errorf("path traversal blocked: %s escapes project root", fullPath)
	}
	return nil
}

var writeFileSkill = &skills.Skill{
	Name:        "write_file",
	Description: "Write content to a file in the project directory.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Relative file path within the project."},
			"content": map[string]any{"type": "string", "description": "Full file content to write."},
		},
		"required":             []any{"path", "content"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleWriteFile,
}

var readFileSkill = &skills.Skill{
	Name:        "read_file",
	Description: "Read a file from the project directory and return its content.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Relative file path within the project."},
		},
		"required":             []any{"path"},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleReadFile,
}

var runQASkill = &skills.Skill{
	Name:         "run_qa",
	Description:  "Run QA validation against the project. Available to managers and above.",
	AllowedRoles: []string{"CEO", "Manager"},
	Parameters: map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []any{},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleRunQA,
}
