// Package gitops provides Git operations for the agent platform.
// Each worker gets an isolated git worktree so multiple agents can
// write concurrently without stepping on each other.
package gitops

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var logger = slog.Default()

// InitRepo initialises a git repository at projectDir if one does not
// already exist. It creates an initial empty commit on the "main" branch
// so that worktrees can be added immediately.
func InitRepo(projectDir string) error {
	if projectDir == "" {
		return fmt.Errorf("gitops: project directory is empty")
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("gitops: create project dir: %w", err)
	}
	// Already a git repo?
	if _, err := os.Stat(filepath.Join(projectDir, ".git")); err == nil {
		return nil
	}
	if err := run(projectDir, "git", "init", "-b", "main"); err != nil {
		return fmt.Errorf("gitops: git init: %w", err)
	}
	if err := run(projectDir, "git", "config", "user.email", "aimos@agent.local"); err != nil {
		return fmt.Errorf("gitops: git config email: %w", err)
	}
	if err := run(projectDir, "git", "config", "user.name", "AimOS Agent"); err != nil {
		return fmt.Errorf("gitops: git config name: %w", err)
	}
	if err := run(projectDir, "git", "commit", "--allow-empty", "-m", "Initial commit"); err != nil {
		return fmt.Errorf("gitops: initial commit: %w", err)
	}
	logger.Info("gitops: initialized repo", "dir", projectDir)
	return nil
}

// WorktreeDir returns the filesystem path for a worker's worktree.
func WorktreeDir(projectDir, workerName string) string {
	return filepath.Join(projectDir, ".worktrees", sanitize(workerName))
}

// BranchName returns the git branch name for a worker.
func BranchName(workerName string) string {
	return "worker/" + sanitize(workerName)
}

// CreateWorktree creates a new git branch and linked worktree for a worker.
// Returns the absolute path of the worktree directory. If the worktree
// already exists it returns the existing path without error.
func CreateWorktree(projectDir, workerName string) (string, error) {
	wtDir := WorktreeDir(projectDir, workerName)
	branch := BranchName(workerName)

	// Already exists?
	if _, err := os.Stat(wtDir); err == nil {
		return wtDir, nil
	}

	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		return "", fmt.Errorf("gitops: create worktree parent: %w", err)
	}
	if err := run(projectDir, "git", "worktree", "add", "-b", branch, wtDir, "main"); err != nil {
		return "", fmt.Errorf("gitops: create worktree %q: %w", branch, err)
	}
	// Inherit git config in worktree.
	_ = run(wtDir, "git", "config", "user.email", "aimos@agent.local")
	_ = run(wtDir, "git", "config", "user.name", "AimOS Agent")

	logger.Info("gitops: created worktree", "branch", branch, "dir", wtDir)
	return wtDir, nil
}

// CommitChanges stages all changes in the given directory (which may be
// a worktree or the main repo) and creates a commit. Returns nil if there
// is nothing to commit.
func CommitChanges(dir, message string) error {
	if message == "" {
		message = "Agent work delivery"
	}
	if err := run(dir, "git", "add", "-A"); err != nil {
		return fmt.Errorf("gitops: git add: %w", err)
	}
	// Check if there is anything to commit.
	out, err := runOutput(dir, "git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("gitops: git status: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		logger.Info("gitops: nothing to commit", "dir", dir)
		return nil
	}
	if err := run(dir, "git", "commit", "-m", message); err != nil {
		return fmt.Errorf("gitops: git commit: %w", err)
	}
	logger.Info("gitops: committed changes", "dir", dir, "message", message)
	return nil
}

// MergeBranch merges the named worker branch into main inside projectDir.
// Returns an error if there are conflicts (the caller should handle it).
func MergeBranch(projectDir, workerName string) error {
	branch := BranchName(workerName)

	// Make sure we're on main.
	if err := run(projectDir, "git", "checkout", "main"); err != nil {
		return fmt.Errorf("gitops: checkout main: %w", err)
	}
	if err := run(projectDir, "git", "merge", "--no-ff", "-m",
		fmt.Sprintf("Merge %s into main", branch), branch); err != nil {
		return fmt.Errorf("gitops: merge %s: %w", branch, err)
	}
	logger.Info("gitops: merged branch", "branch", branch, "into", "main")
	return nil
}

// MergeAllWorkerBranches merges every worker/* branch into main.
// Returns a list of branches that failed to merge.
func MergeAllWorkerBranches(projectDir string) []string {
	out, err := runOutput(projectDir, "git", "branch", "--list", "worker/*")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var failures []string
	for _, line := range strings.Split(out, "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		worker := strings.TrimPrefix(branch, "worker/")
		if mergeErr := MergeBranch(projectDir, worker); mergeErr != nil {
			logger.Error("gitops: merge failed", "branch", branch, "error", mergeErr)
			failures = append(failures, branch)
			// Abort any partial merge so the next one starts clean.
			_ = run(projectDir, "git", "merge", "--abort")
		}
	}
	return failures
}

// RemoveWorktree removes a worker's worktree and prunes it from git.
func RemoveWorktree(projectDir, workerName string) error {
	wtDir := WorktreeDir(projectDir, workerName)
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		return nil
	}
	if err := run(projectDir, "git", "worktree", "remove", "--force", wtDir); err != nil {
		return fmt.Errorf("gitops: remove worktree: %w", err)
	}
	logger.Info("gitops: removed worktree", "dir", wtDir)
	return nil
}

// ListWorkerBranches returns all worker/* branch names.
func ListWorkerBranches(projectDir string) []string {
	out, err := runOutput(projectDir, "git", "branch", "--list", "worker/*")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		branch = strings.TrimSpace(branch)
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	return branches
}

// --- helpers ---

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}

func runOutput(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func sanitize(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, slug)
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return slug
}
