package ceo

import (
	"encoding/json"
	"fmt"
	"strings"
)

type discoveryResponse struct {
	Message         string   `json:"message"`
	Assumptions     []string `json:"assumptions"`
	Gaps            []string `json:"gaps"`
	AccessNeeds     []string `json:"accessNeeds"`
	AmbitionLevel   string   `json:"ambitionLevel"`
	SuccessCriteria []string `json:"successCriteria"`
	NextQuestions   []string `json:"nextQuestions"`
}

type alignmentResponse struct {
	Message                 string   `json:"message"`
	RecommendedScopePosture string   `json:"recommendedScopePosture"`
	Tradeoffs               []string `json:"tradeoffs"`
	DecisionPoints          []string `json:"decisionPoints"`
	AccessNeeds             []string `json:"accessNeeds"`
	Risks                   []string `json:"risks"`
	NextActions             []string `json:"nextActions"`
}

type highLevelPlanResponse struct {
	Message      string   `json:"message"`
	Vision       string   `json:"vision"`
	Value        string   `json:"value"`
	AccessNeeds  []string `json:"accessNeeds"`
	Workstreams  []string `json:"workstreams"`
	Risks        []string `json:"risks"`
	StagePlan    []string `json:"stagePlan"`
	Assumptions  []string `json:"assumptions"`
	DecisionNeed []string `json:"decisionNeeds"`
}

func buildResponsePayload(mode Mode, rawResponse string, model string) (map[string]any, string, error) {
	clean := strings.TrimSpace(rawResponse)
	decoded := unwrapJSONResponse(clean)

	switch mode {
	case ModeDiscovery:
		var payload discoveryResponse
		if err := json.Unmarshal([]byte(decoded), &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
			return map[string]any{
				"message":         strings.TrimSpace(payload.Message),
				"mode":            mode,
				"model":           model,
				"assumptions":     defaultStringSlice(payload.Assumptions),
				"gaps":            defaultStringSlice(payload.Gaps),
				"accessNeeds":     defaultStringSlice(payload.AccessNeeds),
				"ambitionLevel":   strings.TrimSpace(payload.AmbitionLevel),
				"successCriteria": defaultStringSlice(payload.SuccessCriteria),
				"nextQuestions":   defaultStringSlice(payload.NextQuestions),
			}, strings.TrimSpace(payload.Message), nil
		}
		return map[string]any{
			"message":         clean,
			"mode":            mode,
			"model":           model,
			"assumptions":     []string{},
			"gaps":            []string{},
			"accessNeeds":     []string{},
			"ambitionLevel":   "",
			"successCriteria": []string{},
			"nextQuestions":   []string{},
		}, clean, nil
	case ModeAlignment:
		var payload alignmentResponse
		if err := json.Unmarshal([]byte(decoded), &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
			return map[string]any{
				"message":                 strings.TrimSpace(payload.Message),
				"mode":                    mode,
				"model":                   model,
				"recommendedScopePosture": strings.TrimSpace(payload.RecommendedScopePosture),
				"tradeoffs":               defaultStringSlice(payload.Tradeoffs),
				"decisionPoints":          defaultStringSlice(payload.DecisionPoints),
				"accessNeeds":             defaultStringSlice(payload.AccessNeeds),
				"risks":                   defaultStringSlice(payload.Risks),
				"nextActions":             defaultStringSlice(payload.NextActions),
			}, strings.TrimSpace(payload.Message), nil
		}
		return map[string]any{
			"message":                 clean,
			"mode":                    mode,
			"model":                   model,
			"recommendedScopePosture": "",
			"tradeoffs":               []string{},
			"decisionPoints":          []string{},
			"accessNeeds":             []string{},
			"risks":                   []string{},
			"nextActions":             []string{},
		}, clean, nil
	case ModeHighLevelPlan:
		var payload highLevelPlanResponse
		if err := json.Unmarshal([]byte(decoded), &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
			return map[string]any{
				"message":       strings.TrimSpace(payload.Message),
				"mode":          mode,
				"model":         model,
				"vision":        strings.TrimSpace(payload.Vision),
				"value":         strings.TrimSpace(payload.Value),
				"accessNeeds":   defaultStringSlice(payload.AccessNeeds),
				"workstreams":   defaultStringSlice(payload.Workstreams),
				"risks":         defaultStringSlice(payload.Risks),
				"stagePlan":     defaultStringSlice(payload.StagePlan),
				"assumptions":   defaultStringSlice(payload.Assumptions),
				"decisionNeeds": defaultStringSlice(payload.DecisionNeed),
			}, strings.TrimSpace(payload.Message), nil
		}
	}

	return map[string]any{
		"message": clean,
		"mode":    mode,
		"model":   model,
	}, clean, nil
}

func unwrapJSONResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
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

var _ = fmt.Sprintf
