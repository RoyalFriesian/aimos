package main

import (
	"context"
	"log/slog"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/knowledge"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// llmClient wraps the existing OpenAI client to implement knowledge.CompletionClient.
type llmClient struct {
	client *aiclients.OpenAIClient
}

func newLLMClient(cfg knowledge.Config) knowledge.CompletionClient {
	oc := aiclients.NewOpenAIClient(aiclients.OpenAIConfig{
		APIKey: cfg.APIKey,
	}, slog.Default())
	return &llmClient{client: oc}
}

func (l *llmClient) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	messages := []threads.Message{
		{Role: threads.RoleSystem, Content: systemPrompt},
		{Role: threads.RoleUser, Content: userPrompt},
	}
	return l.client.GenerateFromMessages(ctx, model, messages)
}
