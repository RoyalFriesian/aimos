package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const l1SystemPrompt = `You are a code analysis agent. Your job is to create a precise, structured summary of the source files provided.

Your summary MUST include:
- Package/module name and purpose (1 line)
- All imports/dependencies (list)
- Every exported/public symbol: type, name, line number, signature
- Every function/method: name, receiver (if any), parameters, return types, line range, one-line purpose
- Interface definitions: name, method signatures, line numbers
- Struct/class definitions: name, key fields summary, line numbers
- Constants/enums if present
- Key logic flows and algorithmic patterns (brief)
- File-level purpose summary (2-3 sentences)

Rules:
- Keep total output at most %d tokens (approximately %d%% of input size)
- Use a structured format with clear headings per file
- Include line numbers for all symbols
- Do NOT include full source code — only structured summaries
- Do NOT add commentary or opinions — just facts
- If a file is a test file, summarize what is tested, not how`

// SummarizeAgent produces an L1 summary for a single agent assignment.
func SummarizeAgent(ctx context.Context, client CompletionClient, model string, repoRoot string,
	assignment AgentAssignment, compressionRatio float64) (AgentSummary, error) {

	var fileContents strings.Builder
	totalInputTokens := 0

	for _, fp := range assignment.FilePaths {
		absPath := filepath.Join(repoRoot, fp)
		data, err := os.ReadFile(absPath)
		if err != nil {
			// File may have been deleted since scan; note it and continue
			fileContents.WriteString(fmt.Sprintf("\n--- File: %s ---\n[unreadable: %v]\n", fp, err))
			continue
		}
		content := string(data)
		tokens := estimateTokens(int64(len(data)))
		totalInputTokens += tokens

		fileContents.WriteString(fmt.Sprintf("\n--- File: %s (lines: %d, tokens: ~%d) ---\n",
			fp, countLines(content), tokens))
		fileContents.WriteString(content)
		fileContents.WriteString("\n")
	}

	if totalInputTokens == 0 {
		return AgentSummary{
			Level:     1,
			Index:     assignment.Index,
			FilePaths: assignment.FilePaths,
			Summary:   "No readable content.",
			Tokens:    0,
		}, nil
	}

	targetTokens := int(float64(totalInputTokens) * compressionRatio)
	if targetTokens < 100 {
		targetTokens = 100
	}
	pct := int(compressionRatio * 100)

	systemPrompt := fmt.Sprintf(l1SystemPrompt, targetTokens, pct)
	userPrompt := fmt.Sprintf("Summarize the following %d source files:\n%s",
		len(assignment.FilePaths), fileContents.String())

	response, err := client.Generate(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return AgentSummary{}, fmt.Errorf("LLM summarize agent %d: %w", assignment.Index, err)
	}

	return AgentSummary{
		Level:     1,
		Index:     assignment.Index,
		FilePaths: assignment.FilePaths,
		Summary:   response,
		Tokens:    estimateTokens(int64(len(response))),
	}, nil
}

// SummarizeAllAgents runs L1 summarization across all assignments concurrently.
func SummarizeAllAgents(ctx context.Context, client CompletionClient, model string, repoRoot string,
	assignments []AgentAssignment, cfg Config, progress ProgressFunc) ([]AgentSummary, error) {

	results := make([]AgentSummary, len(assignments))
	errs := make([]error, len(assignments))

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for i, a := range assignments {
		wg.Add(1)
		go func(idx int, assign AgentAssignment) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				errs[idx] = ctx.Err()
				return
			}

			result, err := SummarizeAgent(ctx, client, model, repoRoot, assign, cfg.CompressionRatio)
			if err != nil {
				errs[idx] = err
				return
			}
			result.RepoID = "" // will be set by caller
			results[idx] = result

			if progress != nil {
				progress("l1-summarize", idx+1, len(assignments))
			}
		}(i, a)
	}

	wg.Wait()

	// Collect errors
	var firstErr error
	for _, err := range errs {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

func countLines(s string) int {
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}
