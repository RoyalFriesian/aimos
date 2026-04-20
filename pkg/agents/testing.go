package agents

import (
	"context"
	"encoding/json"
	"fmt"
)

// TestInput holds the inputs for testing agent validation.
type TestInput struct {
	Deliverable        string
	TodoTitle          string
	TodoDescription    string
	AcceptanceCriteria string
	MissionGoal        string
}

// TestResult holds the output of a testing agent validation.
type TestResult struct {
	Status  string   `json:"status"` // "PASS" or "FAIL"
	Summary string   `json:"summary"`
	Issues  []string `json:"issues,omitempty"`
}

// TestingAgent validates worker deliverables via an independent LLM call.
type TestingAgent struct {
	llm   LoopCompletionClient
	model string
}

// NewTestingAgent creates a testing agent.
func NewTestingAgent(llm LoopCompletionClient, model string) *TestingAgent {
	if model == "" {
		model = "gpt-4.1"
	}
	return &TestingAgent{llm: llm, model: model}
}

// Validate runs the testing agent on a deliverable.
func (t *TestingAgent) Validate(ctx context.Context, input TestInput) (TestResult, error) {
	if t.llm == nil {
		return TestResult{Status: "PASS", Summary: "No LLM configured, auto-passing."}, nil
	}

	systemPrompt := testingSystemPrompt
	userPrompt := fmt.Sprintf(`## Deliverable to Validate

**Todo**: %s
**Description**: %s
**Mission Goal**: %s
**Acceptance Criteria**: %s

---

### Deliverable Content

%s

---

Evaluate this deliverable. Respond with a JSON object:
{"status": "PASS" or "FAIL", "summary": "...", "issues": ["..."]}`,
		input.TodoTitle,
		input.TodoDescription,
		input.MissionGoal,
		input.AcceptanceCriteria,
		input.Deliverable,
	)

	raw, err := t.llm.Generate(ctx, t.model, systemPrompt, userPrompt)
	if err != nil {
		return TestResult{}, fmt.Errorf("testing agent LLM call: %w", err)
	}

	var result TestResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return TestResult{
			Status:  "FAIL",
			Summary: fmt.Sprintf("Could not parse test result: %s", raw),
		}, nil
	}
	return result, nil
}

const testingSystemPrompt = `You are an independent testing agent. Your job is to validate deliverables produced by worker agents.

You must be strict and thorough:
1. Check if the deliverable meets the stated acceptance criteria.
2. Check if the deliverable aligns with the mission goal and todo description.
3. Look for incompleteness, vagueness, errors, or missing edge cases.
4. If ANY issue exists, return FAIL.
5. Only return PASS if the deliverable fully meets all criteria.

You MUST respond with a JSON object:
{"status": "PASS" or "FAIL", "summary": "brief explanation", "issues": ["list of issues if FAIL"]}

Be fair but strict. Do not pass incomplete or low-quality work.`
