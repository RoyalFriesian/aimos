package ceo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// ListOpenAIModels returns the model IDs visible to the configured OpenAI API key.
func (s *Service) ListOpenAIModels(ctx context.Context) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("ceo service is nil")
	}

	options := []option.RequestOption{}
	if s.config.APIKey != "" {
		options = append(options, option.WithAPIKey(s.config.APIKey))
	}
	if s.config.BaseURL != "" {
		options = append(options, option.WithBaseURL(s.config.BaseURL))
	}

	client := openai.NewClient(options...)
	pager := client.Models.ListAutoPaging(ctx)

	seen := map[string]struct{}{}
	models := make([]string, 0, 128)
	for pager.Next() {
		id := strings.TrimSpace(pager.Current().ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("list openai models: %w", err)
	}

	sort.Strings(models)
	return models, nil
}
