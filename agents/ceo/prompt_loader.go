package ceo

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

//go:embed prompts/*.json
var promptFS embed.FS

type promptConfig struct {
	Mode         Mode   `json:"mode"`
	SystemPrompt string `json:"systemPrompt"`
}

func loadSystemPrompt(mode Mode) (string, error) {
	fileName := path.Join("prompts", sanitizeModeFileName(mode)+".json")
	content, err := promptFS.ReadFile(fileName)
	if err != nil {
		return "", logValidationError("missing CEO system prompt config", fmt.Errorf("read %s: %w", fileName, err), "mode", mode)
	}

	var cfg promptConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return "", logValidationError("invalid CEO system prompt config", fmt.Errorf("unmarshal %s: %w", fileName, err), "mode", mode)
	}
	if cfg.Mode == "" {
		return "", logValidationError("invalid CEO system prompt config", fmt.Errorf("mode is required in %s", fileName), "mode", mode)
	}
	if cfg.Mode != mode {
		return "", logValidationError("mismatched CEO system prompt config", fmt.Errorf("config mode %q does not match requested mode %q", cfg.Mode, mode), "mode", mode)
	}
	if strings.TrimSpace(cfg.SystemPrompt) == "" {
		return "", logValidationError("invalid CEO system prompt config", fmt.Errorf("systemPrompt is required in %s", fileName), "mode", mode)
	}

	return cfg.SystemPrompt, nil
}

func sanitizeModeFileName(mode Mode) string {
	value := strings.TrimSpace(string(mode))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}
