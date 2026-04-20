package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

// Registry holds a set of named skills and can convert them to OpenAI
// tool parameters for function calling.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

// Register adds a skill to the registry. Panics on duplicate names.
func (r *Registry) Register(s *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.skills[s.Name]; exists {
		panic(fmt.Sprintf("skill %q already registered", s.Name))
	}
	r.skills[s.Name] = s
}

// Get returns the skill with the given name, or nil if not found.
func (r *Registry) Get(name string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[name]
}

// All returns a snapshot of every registered skill.
func (r *Registry) All() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// ToolParams converts all registered skills into the OpenAI ToolUnionParam
// slice expected by ResponseNewParams.Tools.
func (r *Registry) ToolParams() []responses.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]responses.ToolUnionParam, 0, len(r.skills))
	for _, s := range r.skills {
		ft := responses.FunctionToolParam{
			Name:       s.Name,
			Parameters: s.Parameters,
		}
		if s.Description != "" {
			ft.Description = param.NewOpt(s.Description)
		}
		if s.Strict {
			ft.Strict = param.NewOpt(true)
		}
		tools = append(tools, responses.ToolUnionParam{OfFunction: &ft})
	}
	return tools
}

// ToolParamsForRole returns only the tools allowed for the given agent role.
func (r *Registry) ToolParamsForRole(role string) []responses.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]responses.ToolUnionParam, 0, len(r.skills))
	for _, s := range r.skills {
		if !roleAllowed(s.AllowedRoles, role) {
			continue
		}
		ft := responses.FunctionToolParam{
			Name:       s.Name,
			Parameters: s.Parameters,
		}
		if s.Description != "" {
			ft.Description = param.NewOpt(s.Description)
		}
		if s.Strict {
			ft.Strict = param.NewOpt(true)
		}
		tools = append(tools, responses.ToolUnionParam{OfFunction: &ft})
	}
	return tools
}

// Execute dispatches a tool call to the named skill's handler.
func (r *Registry) Execute(ctx context.Context, env *Env, name string, args json.RawMessage) (string, error) {
	s := r.Get(name)
	if s == nil {
		return "", fmt.Errorf("unknown skill %q", name)
	}
	if s.Handler == nil {
		return "", fmt.Errorf("skill %q has no handler", name)
	}
	return s.Handler(ctx, env, args)
}

// roleAllowed returns true if the role is permitted by the allowedRoles list.
// An empty list means all roles are allowed.
func roleAllowed(allowedRoles []string, role string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	for _, r := range allowedRoles {
		if r == role {
			return true
		}
	}
	return false
}
