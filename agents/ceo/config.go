package ceo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/joho/godotenv"
)

const (
	defaultEnvFile    = ".env"
	defaultModel      = "gpt-5.4"
	DefaultChildModel = "gpt-5-mini"
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	// Child model override (OpenAI model name, no prefix needed).
	ChildModel string // e.g. "gpt-5.4-nano"
	// Max output tokens per LLM call (0 = provider default).
	MaxOutputTokens int64
	// Ollama settings
	OllamaURL        string // e.g. "http://localhost:11434"
	OllamaChildModel string // e.g. "qwen2.5-coder:7b" (stored as "ollama/qwen2.5-coder:7b" in agent nodes)
	// Pigeon settings — file-based AI relay protocol.
	PigeonEnabled bool   // Set PIGEON_ENABLED=true to activate.
	PigeonBaseDir string // Root directory for request/response files (env: PIGEON_BASE_DIR).
	// MicroModel is the tiny fast model for classification and normalization.
	MicroModel string // e.g. "gpt-4.1-nano"
	// CleanStartup marks all persisted active agents as terminated on server
	// boot instead of recovering their loops. Useful for dev/testing.
	CleanStartup bool
}

func LoadConfig(envFile string) (Config, error) {
	if envFile == "" {
		envFile = defaultEnvFile
	}

	if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
		logger.Error("failed to load CEO env file", "error", err, "envFile", envFile)
		return Config{}, fmt.Errorf("load env file: %w", err)
	}

	maxTokens, _ := strconv.ParseInt(os.Getenv("MAX_OUTPUT_TOKENS"), 10, 64)

	config := Config{
		APIKey:           os.Getenv("OPENAI_API_KEY"),
		Model:            getenvDefault("OPENAI_MODEL", defaultModel),
		BaseURL:          os.Getenv("OPENAI_BASE_URL"),
		ChildModel:       os.Getenv("CHILD_MODEL"),
		MaxOutputTokens:  maxTokens,
		OllamaURL:        getenvDefault("OLLAMA_URL", "http://localhost:11434"),
		OllamaChildModel: os.Getenv("OLLAMA_CHILD_MODEL"), // e.g. "qwen2.5-coder:7b"
		PigeonEnabled:    os.Getenv("PIGEON_ENABLED") == "true" || os.Getenv("PIGEON_ENABLED") == "1",
		PigeonBaseDir:    os.Getenv("PIGEON_BASE_DIR"),
		MicroModel:       os.Getenv("MICRO_MODEL"),
		CleanStartup:     os.Getenv("CLEAN_STARTUP") == "true" || os.Getenv("CLEAN_STARTUP") == "1",
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if c.APIKey == "" {
		return logValidationError("invalid CEO config", fmt.Errorf("OPENAI_API_KEY is required"))
	}
	if c.Model == "" {
		return logValidationError("invalid CEO config", fmt.Errorf("OPENAI_MODEL is required"))
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// EffectiveChildModel returns the child model to assign to new worker agents.
// Priority: CHILD_MODEL env > OLLAMA_CHILD_MODEL env > DefaultChildModel constant.
func (c Config) EffectiveChildModel() string {
	if c.ChildModel != "" {
		return c.ChildModel
	}
	if c.OllamaChildModel != "" {
		return "ollama/" + c.OllamaChildModel
	}
	return DefaultChildModel
}

const DefaultMicroModel = "gpt-4.1-nano"

// EffectiveMicroModel returns the model for tiny, fast classification calls.
// Priority: MICRO_MODEL env > DefaultMicroModel constant.
func (c Config) EffectiveMicroModel() string {
	if c.MicroModel != "" {
		return c.MicroModel
	}
	return DefaultMicroModel
}

const DefaultPigeonBaseDir = "aimos-ai-requests"

// pigeonConfig returns a PigeonConfig pointer when Pigeon is enabled, or nil otherwise.
func (c Config) pigeonConfig() *aiclients.PigeonConfig {
	if !c.PigeonEnabled {
		return nil
	}
	baseDir := c.PigeonBaseDir
	if baseDir == "" {
		// Default: sibling directory to the workspace root.
		home, err := os.UserHomeDir()
		if err == nil {
			baseDir = filepath.Join(home, "go", "src", "github.com", DefaultPigeonBaseDir)
		} else {
			baseDir = DefaultPigeonBaseDir
		}
	}
	return &aiclients.PigeonConfig{BaseDir: baseDir}
}
