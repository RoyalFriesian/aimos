package knowledge

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Config holds all configuration for the knowledge indexing pipeline.
type Config struct {
	BaseDir          string  // Root directory for knowledge base storage
	Model            string  // LLM model for summarization
	APIKey           string  // OpenAI API key
	TargetTokens     int     // Max tokens for master context (~80K)
	CompressionRatio float64 // Target compression per level (0.10 = 10%)
	Concurrency      int     // Worker pool size for L1 summarization
	AgentFileLimit   int     // Max files per agent assignment
	AgentTokenBudget int     // Target raw token budget per L1 agent
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseDir:          filepath.Join(homeDir(), ".aimos-knowledge"),
		Model:            "gpt-4o-mini",
		TargetTokens:     80000,
		CompressionRatio: 0.10,
		Concurrency:      5,
		AgentFileLimit:   5,
		AgentTokenBudget: 50000,
	}
}

// ConfigFromEnv builds a Config from environment variables, falling back to defaults.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("KNOWLEDGE_BASE_DIR"); v != "" {
		cfg.BaseDir = v
	}
	if v := os.Getenv("KNOWLEDGE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("KNOWLEDGE_TARGET_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TargetTokens = n
		}
	}
	if v := os.Getenv("KNOWLEDGE_COMPRESSION_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f < 1 {
			cfg.CompressionRatio = f
		}
	}
	if v := os.Getenv("KNOWLEDGE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Concurrency = n
		}
	}

	return cfg
}

// RepoDir returns the knowledge base directory for a specific repo.
func (c Config) RepoDir(repoID string) string {
	return filepath.Join(c.BaseDir, repoID)
}

// Validate checks that required configuration fields are present.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return errMissingAPIKey
	}
	return nil
}

var errMissingAPIKey = &configError{"OPENAI_API_KEY is required"}

type configError struct{ msg string }

func (e *configError) Error() string { return e.msg }

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}
