package skills

import (
	"context"
	"encoding/json"
)

// Handler is the function signature for a skill implementation.
// It receives the execution environment and the raw JSON arguments
// from the LLM function call. It returns a result string that is fed
// back to the model, or an error.
type Handler func(ctx context.Context, env *Env, args json.RawMessage) (string, error)

// Skill describes a single tool the LLM can invoke via function calling.
type Skill struct {
	// Name is the function name exposed to the LLM (e.g. "write_file").
	Name string

	// Description is the human-readable description shown to the LLM.
	Description string

	// Parameters is the JSON Schema object describing the function's
	// arguments. It is passed directly to FunctionToolParam.Parameters.
	Parameters map[string]any

	// Strict enables OpenAI strict mode for this tool, which enforces
	// that the LLM output exactly matches the schema.
	Strict bool

	// Handler executes the skill when the LLM calls it.
	Handler Handler

	// AllowedRoles restricts which agent roles may use this skill.
	// An empty slice means all roles are allowed.
	AllowedRoles []string
}
