package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const resolverSystemPrompt = `You are a code knowledge query resolver. You have access to a hierarchical compressed knowledge base of a repository.

Current context level: %s

Your task: Given the user's question and the knowledge context provided, either:
1. ANSWER the question directly if you have enough detail, OR
2. REQUEST deeper detail by specifying which groups/agents to drill into

When answering:
- Be precise and reference specific files, types, functions, and line numbers when available
- Cite evidence from the summaries
- If the answer spans multiple packages, explain the full flow

When requesting drill-down, respond with EXACTLY this JSON format:
{"drillDown": [0, 3, 7]}
where the numbers are the agent/group indices you need more detail from.
Do NOT mix an answer with a drill-down request.`

const finalAnswerSystemPrompt = `You are a code knowledge expert. Answer the user's question using the repository knowledge provided.

Rules:
- Be precise: reference specific files, types, functions, and line numbers
- Structure your answer clearly
- If the question asks about implementation: describe the actual code flow
- If the question asks about architecture: describe packages, interfaces, and data flow
- If you're uncertain about something, say so
- Include a "Sources" section at the end listing the files and symbols you referenced`

// ResolveQuery performs multi-level knowledge resolution to answer a question.
func ResolveQuery(ctx context.Context, client CompletionClient, model string,
	cfg Config, manifest Manifest) func(ctx context.Context, question string) (*QueryResult, error) {

	return func(ctx context.Context, question string) (*QueryResult, error) {
		repoID := manifest.Repo.ID

		// Load master context
		masterContent, err := ReadMasterContext(cfg, repoID, manifest)
		if err != nil {
			return nil, fmt.Errorf("read master context: %w", err)
		}

		// Try answering from master context first
		systemPrompt := fmt.Sprintf(resolverSystemPrompt, "master")
		userPrompt := fmt.Sprintf("Repository: %s\n\nMaster Context:\n%s\n\nQuestion: %s",
			manifest.Repo.Path, masterContent, question)

		response, err := client.Generate(ctx, model, systemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("master query: %w", err)
		}

		// Check if the response is a drill-down request
		drillDown := parseDrillDown(response)
		if drillDown == nil {
			// Master context was sufficient
			return parseAnswer(response, manifest), nil
		}

		// Multi-level drill-down
		return drillDownResolve(ctx, client, model, cfg, manifest, question, masterContent, drillDown)
	}
}

func drillDownResolve(ctx context.Context, client CompletionClient, model string,
	cfg Config, manifest Manifest, question string, masterContent string, targetIndices []int) (*QueryResult, error) {

	repoID := manifest.Repo.ID
	maxLevel := manifest.Repo.LevelsCount

	// Collect context from each level, drilling down
	var accumulated strings.Builder
	accumulated.WriteString("Master Context:\n")
	accumulated.WriteString(masterContent)
	accumulated.WriteString("\n\n")

	// Walk down from the highest numbered non-master level to L1
	for level := maxLevel - 1; level >= 1; level-- {
		summaries, err := ReadAgentSummaries(cfg, repoID, level)
		if err != nil {
			break // level doesn't exist
		}

		// Pick the targeted summaries
		var picked []AgentSummary
		for _, idx := range targetIndices {
			if idx >= 0 && idx < len(summaries) {
				picked = append(picked, summaries[idx])
			}
		}
		if len(picked) == 0 {
			break
		}

		// Add picked summaries to accumulated context
		accumulated.WriteString(fmt.Sprintf("\n--- Level %d Detail ---\n", level))
		for _, s := range picked {
			accumulated.WriteString(fmt.Sprintf("\nAgent %d", s.Index))
			if len(s.FilePaths) > 0 {
				accumulated.WriteString(fmt.Sprintf(" (files: %s)", strings.Join(s.FilePaths, ", ")))
			}
			accumulated.WriteString(":\n")
			accumulated.WriteString(s.Summary)
			accumulated.WriteString("\n")
		}

		// Ask if we need to drill deeper
		if level > 1 {
			systemPrompt := fmt.Sprintf(resolverSystemPrompt, fmt.Sprintf("level %d", level))
			drillPrompt := fmt.Sprintf("Repository: %s\n\nAccumulated Context:\n%s\n\nQuestion: %s",
				manifest.Repo.Path, accumulated.String(), question)

			response, err := client.Generate(ctx, model, systemPrompt, drillPrompt)
			if err != nil {
				break
			}

			nextDrill := parseDrillDown(response)
			if nextDrill == nil {
				// Got an answer at this level
				return parseAnswer(response, manifest), nil
			}
			targetIndices = nextDrill
		}
	}

	// Final answer with all accumulated context
	userPrompt := fmt.Sprintf("Repository: %s\n\nFull Knowledge Context:\n%s\n\nQuestion: %s",
		manifest.Repo.Path, accumulated.String(), question)

	response, err := client.Generate(ctx, model, finalAnswerSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("final answer: %w", err)
	}

	return parseAnswer(response, manifest), nil
}

type drillDownResponse struct {
	DrillDown []int `json:"drillDown"`
}

func parseDrillDown(response string) []int {
	trimmed := strings.TrimSpace(response)

	// Try to parse as drill-down JSON
	var dd drillDownResponse
	if err := json.Unmarshal([]byte(trimmed), &dd); err == nil && len(dd.DrillDown) > 0 {
		return dd.DrillDown
	}

	// Try to find JSON embedded in the response
	start := strings.Index(trimmed, `{"drillDown"`)
	if start >= 0 {
		end := strings.Index(trimmed[start:], "}")
		if end >= 0 {
			candidate := trimmed[start : start+end+1]
			if err := json.Unmarshal([]byte(candidate), &dd); err == nil && len(dd.DrillDown) > 0 {
				return dd.DrillDown
			}
		}
	}

	return nil
}

func parseAnswer(response string, manifest Manifest) *QueryResult {
	result := &QueryResult{
		Answer: response,
	}

	// Extract sources section if present
	lower := strings.ToLower(response)
	idx := strings.LastIndex(lower, "sources")
	if idx > 0 {
		sourcesSection := response[idx:]
		lines := strings.Split(sourcesSection, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line == "" || strings.HasPrefix(strings.ToLower(line), "source") {
				continue
			}
			if strings.Contains(line, "/") || strings.Contains(line, ".") {
				result.Sources = append(result.Sources, Source{
					File: line,
				})
			}
		}
	}

	return result
}
