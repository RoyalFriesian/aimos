package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/knowledge"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(strings.TrimSpace(k)) == "" {
			os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
}

type llmAdapter struct{ client *aiclients.OpenAIClient }

func (a *llmAdapter) Generate(ctx context.Context, model, sys, usr string) (string, error) {
	msgs := []threads.Message{
		{Role: threads.RoleSystem, Content: sys},
		{Role: threads.RoleUser, Content: usr},
	}
	return a.client.GenerateFromMessages(ctx, model, msgs)
}

func main() {
	loadEnvFile(".env")
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	loadEnvFile(filepath.Join(repoRoot, ".env"))

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		log.Fatal("OPENAI_API_KEY required")
	}

	repo := repoRoot
	if len(os.Args) > 1 {
		repo = os.Args[1]
	}

	cfg := knowledge.DefaultConfig()
	cfg.APIKey = key
	cfg.Model = "gpt-4o-mini"

	manifest, found, err := knowledge.FindRepoByPath(cfg, repo)
	if err != nil {
		log.Fatalf("FindRepoByPath: %v", err)
	}
	if !found {
		log.Fatalf("repo %s is not indexed -- run test-index first", repo)
	}
	fmt.Printf("Found index: %s (%d files, %d levels)\n\n", manifest.Repo.ID, manifest.Repo.FileCount, manifest.Repo.LevelsCount)

	oc := aiclients.NewOpenAIClient(aiclients.OpenAIConfig{APIKey: key}, slog.Default())
	llm := &llmAdapter{client: oc}
	queryFn := knowledge.ResolveQuery(context.Background(), llm, cfg, manifest)

	question := "How does the CEO agent process client requests end-to-end?"
	if len(os.Args) > 2 {
		question = strings.Join(os.Args[2:], " ")
	}

	fmt.Printf("Q: %s\n\n", question)
	result, err := queryFn(context.Background(), question)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("Answer:\n%s\n", result.Answer)
	if len(result.Sources) > 0 {
		fmt.Printf("\nSources:\n")
		for _, s := range result.Sources {
			fmt.Printf("  - %s\n", s)
		}
	}
}
