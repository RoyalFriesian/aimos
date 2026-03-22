package ceo

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

const (
	defaultEnvFile = ".env"
	defaultModel   = "gpt-5.4"
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
}

func LoadConfig(envFile string) (Config, error) {
	if envFile == "" {
		envFile = defaultEnvFile
	}

	if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
		logger.Error("failed to load CEO env file", "error", err, "envFile", envFile)
		return Config{}, fmt.Errorf("load env file: %w", err)
	}

	config := Config{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   getenvDefault("OPENAI_MODEL", defaultModel),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
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
