package builtins

import (
	"context"
	"encoding/json"

	"github.com/Sarnga/agent-platform/pkg/skills"
)

// HandleNoOp does nothing. The LLM can call this to explicitly skip an action.
func HandleNoOp(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
	return "no action taken", nil
}

var noOpSkill = &skills.Skill{
	Name:        "no_op",
	Description: "Explicitly do nothing this turn. Use when no action is needed.",
	Parameters: map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []any{},
		"additionalProperties": false,
	},
	Strict:  true,
	Handler: HandleNoOp,
}
