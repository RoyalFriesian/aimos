package knowledge

import (
	"context"
	"fmt"
	"strings"
)

const compressSystemPrompt = `You are a knowledge compression agent. You receive multiple code summaries from different parts of a codebase and must compress them into a single cohesive summary.

Your compressed output MUST:
- Preserve all package/module names and their purposes
- Preserve all key exported symbols and their roles
- Preserve architectural patterns and data flow
- Preserve cross-package dependencies and interfaces
- Group related functionality together
- Remove redundancy across summaries
- Keep the most important technical details

Rules:
- Keep total output at most %d tokens (approximately %d%% of input)
- Use clear section headings
- Maintain a hierarchical structure: packages → types → functions
- Do NOT invent information — only compress what is provided
- Highlight key interfaces and data contracts`

const masterContextSystemPrompt = `You are a master knowledge compiler. You receive compressed summaries of an entire codebase and must produce the definitive architectural overview.

Your output MUST include:
1. **Repository Overview**: What this codebase does, its primary purpose, and target users
2. **Architecture**: Major packages/modules, their responsibilities, and how they connect
3. **Key Interfaces & Contracts**: The most important interfaces, data types, and API surfaces
4. **Data Flow**: How data moves through the system (e.g., request lifecycle, event pipeline)
5. **Entry Points**: Main executables, server startup, CLI commands
6. **Configuration**: How the system is configured (env vars, config files, flags)
7. **Dependencies**: External services, databases, APIs, third-party libraries of note
8. **Package Directory**: Brief 1-line description of every package/module

Rules:
- Keep total output under %d tokens
- This will be used as context for answering questions about the codebase
- Optimize for an AI agent that needs to understand the codebase to answer developer questions
- Structure with markdown headings for easy navigation
- Be precise about types, function signatures, and package paths`

// CompressLevel takes agent summaries from level K and produces compressed summaries for level K+1.
// If the total tokens of input summaries fit within targetTokens, it produces the master context instead.
func CompressLevel(ctx context.Context, client CompletionClient, model string,
	summaries []AgentSummary, level int, cfg Config) ([]AgentSummary, bool, error) {

	totalTokens := 0
	for _, s := range summaries {
		totalTokens += s.Tokens
	}

	// If already fits in target window, produce master context
	if totalTokens <= cfg.TargetTokens {
		master, err := produceMasterContext(ctx, client, model, summaries, cfg)
		if err != nil {
			return nil, false, err
		}
		return []AgentSummary{master}, true, nil
	}

	// Group summaries into clusters of ~groupSize
	groupSize := 6
	groups := groupSummaries(summaries, groupSize)

	compressed := make([]AgentSummary, 0, len(groups))
	for i, group := range groups {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}

		result, err := compressGroup(ctx, client, model, group, level+1, i, cfg.CompressionRatio)
		if err != nil {
			return nil, false, fmt.Errorf("compress group %d at level %d: %w", i, level+1, err)
		}
		compressed = append(compressed, result)
	}

	return compressed, false, nil
}

func compressGroup(ctx context.Context, client CompletionClient, model string,
	group []AgentSummary, newLevel int, groupIndex int, ratio float64) (AgentSummary, error) {

	var input strings.Builder
	totalInputTokens := 0
	var childIDs []int

	for _, s := range group {
		input.WriteString(fmt.Sprintf("\n--- Agent %d (Level %d, %d tokens) ---\n", s.Index, s.Level, s.Tokens))
		if len(s.FilePaths) > 0 {
			input.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(s.FilePaths, ", ")))
		}
		input.WriteString(s.Summary)
		input.WriteString("\n")
		totalInputTokens += s.Tokens
		childIDs = append(childIDs, s.Index)
	}

	targetTokens := int(float64(totalInputTokens) * ratio)
	if targetTokens < 200 {
		targetTokens = 200
	}
	pct := int(ratio * 100)

	systemPrompt := fmt.Sprintf(compressSystemPrompt, targetTokens, pct)
	userPrompt := fmt.Sprintf("Compress these %d summaries into a single cohesive summary:\n%s",
		len(group), input.String())

	response, err := client.Generate(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return AgentSummary{}, err
	}

	return AgentSummary{
		Level:    newLevel,
		Index:    groupIndex,
		GroupIDs: childIDs,
		Summary:  response,
		Tokens:   estimateTokens(int64(len(response))),
	}, nil
}

func produceMasterContext(ctx context.Context, client CompletionClient, model string,
	summaries []AgentSummary, cfg Config) (AgentSummary, error) {

	var input strings.Builder
	for _, s := range summaries {
		input.WriteString(fmt.Sprintf("\n--- Summary %d ---\n", s.Index))
		if len(s.FilePaths) > 0 {
			input.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(s.FilePaths, ", ")))
		}
		input.WriteString(s.Summary)
		input.WriteString("\n")
	}

	systemPrompt := fmt.Sprintf(masterContextSystemPrompt, cfg.TargetTokens)
	userPrompt := fmt.Sprintf("Produce the master architectural context from these %d summaries:\n%s",
		len(summaries), input.String())

	response, err := client.Generate(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return AgentSummary{}, fmt.Errorf("produce master context: %w", err)
	}

	var allIDs []int
	for _, s := range summaries {
		allIDs = append(allIDs, s.Index)
	}

	return AgentSummary{
		Level:    0, // will be set by caller to actual master level
		Index:    0,
		GroupIDs: allIDs,
		Summary:  response,
		Tokens:   estimateTokens(int64(len(response))),
	}, nil
}

func groupSummaries(summaries []AgentSummary, groupSize int) [][]AgentSummary {
	if groupSize <= 0 {
		groupSize = 6
	}
	var groups [][]AgentSummary
	for i := 0; i < len(summaries); i += groupSize {
		end := i + groupSize
		if end > len(summaries) {
			end = len(summaries)
		}
		groups = append(groups, summaries[i:end])
	}
	return groups
}
