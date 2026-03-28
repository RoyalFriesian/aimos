package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/Sarnga/agent-platform/agents/ceo"
	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/app"
	"github.com/Sarnga/agent-platform/pkg/knowledge"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// llmAdapter bridges the ai-clients OpenAI client to the knowledge CompletionClient interface.
type llmAdapter struct{ client *aiclients.OpenAIClient }

func (a *llmAdapter) Generate(ctx context.Context, model, sys, usr string) (string, error) {
	msgs := []threads.Message{
		{Role: threads.RoleSystem, Content: sys},
		{Role: threads.RoleUser, Content: usr},
	}
	return a.client.GenerateFromMessages(ctx, model, msgs)
}

func main() {
	service, err := ceo.NewServiceFromEnv("")
	if err != nil {
		log.Fatalf("create CEO service: %v", err)
	}
	defer service.Close()

	var opts []app.ServerOption

	// Wire up knowledge service if API key is available
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		knowledgeModel := os.Getenv("KNOWLEDGE_MODEL")
		if knowledgeModel == "" {
			knowledgeModel = "gpt-4o-mini"
		}
		cfg := knowledge.DefaultConfig()
		cfg.APIKey = apiKey
		cfg.Model = knowledgeModel
		if v := os.Getenv("KNOWLEDGE_INDEX_MODEL"); v != "" {
			cfg.IndexModel = v
		}
		if v := os.Getenv("KNOWLEDGE_QUERY_MODEL"); v != "" {
			cfg.QueryModel = v
		}
		if v := os.Getenv("KNOWLEDGE_REASONING_MODEL"); v != "" {
			cfg.ReasoningModel = v
		}
		cfg.Concurrency = 3

		oc := aiclients.NewOpenAIClient(aiclients.OpenAIConfig{APIKey: apiKey}, slog.Default())
		llm := &llmAdapter{client: oc}
		ks := app.NewKnowledgeService(llm, cfg)
		opts = append(opts, app.WithKnowledge(ks))
		log.Printf("Knowledge service enabled (model=%s, index=%s, query=%s, reasoning=%s)",
			knowledgeModel, cfg.GetIndexModel(), cfg.GetQueryModel(), cfg.GetReasoningModel())
	}

	server, err := app.NewServer(service, opts...)
	if err != nil {
		log.Fatalf("create HTTP server: %v", err)
	}

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("CEO API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve HTTP API: %v", err)
	}
}
