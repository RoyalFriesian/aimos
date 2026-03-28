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
	"time"

	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/knowledge"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

// loadEnvFile reads a .env file and sets any variables not already in the environment.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // silently skip if no .env
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
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
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
	// Try loading .env from the repo root (walk up from the binary or use known path).
	// 1. Try relative to working directory
	loadEnvFile(".env")
	// 2. Try relative to the source file location (for `go run`)
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	loadEnvFile(filepath.Join(repoRoot, ".env"))

	repo := repoRoot
	if len(os.Args) > 1 {
		repo = os.Args[1]
	}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		log.Fatal("OPENAI_API_KEY required (set in environment or .env file)")
	}
	cfg := knowledge.DefaultConfig()
	cfg.APIKey = key
	cfg.Model = "gpt-4o-mini"
	cfg.Concurrency = 3
	oc := aiclients.NewOpenAIClient(aiclients.OpenAIConfig{APIKey: key}, slog.Default())
	llm := &llmAdapter{client: oc}
	t0 := time.Now()
	prog := func(s string, c, n int) {
		d := time.Since(t0).Round(time.Second)
		if n > 0 {
			fmt.Printf("[%s] %s: %d/%d\n", d, s, c, n)
		} else {
			fmt.Printf("[%s] %s\n", d, s)
		}
	}
	fmt.Printf("Indexing %s ...\n", repo)
	m, err := knowledge.IndexRepo(context.Background(), llm, repo, cfg, prog)
	if err != nil {
		log.Fatalf("IndexRepo: %v", err)
	}
	fmt.Println("\n=== DONE ===")
	fmt.Printf("ID=%s files=%d levels=%d tokens=%d status=%s took=%s\n",
		m.Repo.ID, m.Repo.FileCount, m.Repo.LevelsCount, m.Repo.TotalTokens,
		m.Repo.Status, time.Since(t0).Round(time.Second))
	for _, l := range m.Levels {
		fmt.Printf("  L%d: %d agents, %d tokens\n", l.Number, l.AgentCount, l.TotalTokens)
	}
	mc, err := knowledge.ReadMasterContext(cfg, m.Repo.ID, *m)
	if err == nil {
		fmt.Printf("\nMaster context: %d bytes (~%d tokens)\n", len(mc), len(mc)/4)
		p := mc
		if len(p) > 2000 {
			p = p[:2000] + "\n...(truncated)"
		}
		fmt.Println(p)
	}
}
