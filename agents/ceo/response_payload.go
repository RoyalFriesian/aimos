package ceo

import (
	"encoding/json"
	"strings"
)

// QuestionItem represents a structured question the CEO needs answered.
type QuestionItem struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	AllowCustom bool     `json:"allowCustom"`
}

// TeamProposalMember represents one proposed team member with their assignment.
type TeamProposalMember struct {
	Role               string   `json:"role"`
	Name               string   `json:"name"`
	Capabilities       []string `json:"capabilities"`
	MissionTitle       string   `json:"missionTitle"`
	MissionDescription string   `json:"missionDescription"`
}

// TeamProposal describes a proposed execution team for user review.
type TeamProposal struct {
	Members []TeamProposalMember `json:"members"`
	Summary string               `json:"summary"`
}

// ceoActionItem represents a single tool action the CEO wants to execute.
type ceoActionItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ceoUnifiedResponse is the single parsing target for all CEO LLM outputs.
// The LLM is instructed to always respond in this shape regardless of mode.
type ceoUnifiedResponse struct {
	Thinking     string           `json:"thinking"`
	UserMessage  string           `json:"userMessage"`
	Message      string           `json:"message"` // legacy fallback
	Questions    []QuestionItem   `json:"questions"`
	TeamProposal *TeamProposal    `json:"teamProposal"`
	Actions      []ceoActionItem  `json:"actions"`
}

func buildResponsePayload(mode Mode, rawResponse string, model string) (map[string]any, string, error) {
	clean := strings.TrimSpace(rawResponse)
	decoded := unwrapJSONResponse(clean)

	var resp ceoUnifiedResponse
	if err := json.Unmarshal([]byte(decoded), &resp); err == nil {
		userMsg := strings.TrimSpace(resp.UserMessage)
		if userMsg == "" {
			userMsg = strings.TrimSpace(resp.Message)
		}
		if userMsg != "" {
			payload := map[string]any{
				"mode":        mode,
				"model":       model,
				"userMessage": userMsg,
				"thinking":    strings.TrimSpace(resp.Thinking),
				"questions":   normalizeQuestions(resp.Questions),
			}
			if resp.TeamProposal != nil && len(resp.TeamProposal.Members) > 0 {
				payload["teamProposal"] = resp.TeamProposal
			}
			if len(resp.Actions) > 0 {
				payload["actions"] = resp.Actions
			}
			return payload, userMsg, nil
		}
	}

	// Fallback: treat the entire raw output as the user-facing message
	return map[string]any{
		"mode":        mode,
		"model":       model,
		"userMessage": clean,
		"thinking":    "",
		"questions":   []QuestionItem{},
	}, clean, nil
}

func normalizeQuestions(questions []QuestionItem) []QuestionItem {
	if len(questions) == 0 {
		return []QuestionItem{}
	}
	result := make([]QuestionItem, 0, len(questions))
	for _, q := range questions {
		q.Text = strings.TrimSpace(q.Text)
		if q.Text == "" {
			continue
		}
		if q.Options == nil {
			q.Options = []string{}
		}
		result = append(result, q)
	}
	if len(result) == 0 {
		return []QuestionItem{}
	}
	return result
}

func defaultStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return []string{}
	}
	return cleaned
}

func unwrapJSONResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}
