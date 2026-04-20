package skills_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sarnga/agent-platform/pkg/skills"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := skills.NewRegistry()
	s := &skills.Skill{
		Name:        "greet",
		Description: "Says hello",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required":             []any{"name"},
			"additionalProperties": false,
		},
		Strict: true,
		Handler: func(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
			return "hello", nil
		},
	}
	r.Register(s)

	got := r.Get("greet")
	if got == nil {
		t.Fatal("expected skill, got nil")
	}
	if got.Name != "greet" {
		t.Fatalf("expected name greet, got %s", got.Name)
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := skills.NewRegistry()
	if r.Get("nope") != nil {
		t.Fatal("expected nil for unregistered skill")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := skills.NewRegistry()
	s := &skills.Skill{Name: "dup"}
	r.Register(s)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	r.Register(s)
}

func TestRegistryAll(t *testing.T) {
	r := skills.NewRegistry()
	r.Register(&skills.Skill{Name: "a"})
	r.Register(&skills.Skill{Name: "b"})
	r.Register(&skills.Skill{Name: "c"})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(all))
	}
}

func TestRegistryToolParams(t *testing.T) {
	r := skills.NewRegistry()
	r.Register(&skills.Skill{
		Name:        "write_file",
		Description: "Write a file to disk",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required":             []any{"path", "content"},
			"additionalProperties": false,
		},
		Strict: true,
	})

	tools := r.ToolParams()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool param, got %d", len(tools))
	}

	tp := tools[0]
	if tp.OfFunction == nil {
		t.Fatal("expected OfFunction to be set")
	}
	if tp.OfFunction.Name != "write_file" {
		t.Fatalf("expected name write_file, got %s", tp.OfFunction.Name)
	}
	if tp.OfFunction.Parameters == nil {
		t.Fatal("expected parameters to be set")
	}
}

func TestRegistryToolParamsForRole(t *testing.T) {
	r := skills.NewRegistry()
	r.Register(&skills.Skill{
		Name:         "worker_only",
		AllowedRoles: []string{"Worker"},
	})
	r.Register(&skills.Skill{
		Name: "all_roles",
	})

	workerTools := r.ToolParamsForRole("Worker")
	if len(workerTools) != 2 {
		t.Fatalf("worker should see 2 tools, got %d", len(workerTools))
	}

	ceoTools := r.ToolParamsForRole("CEO")
	if len(ceoTools) != 1 {
		t.Fatalf("CEO should see 1 tool, got %d", len(ceoTools))
	}
	if ceoTools[0].OfFunction.Name != "all_roles" {
		t.Fatalf("CEO tool should be all_roles, got %s", ceoTools[0].OfFunction.Name)
	}
}

func TestRegistryExecute(t *testing.T) {
	r := skills.NewRegistry()
	r.Register(&skills.Skill{
		Name: "echo",
		Handler: func(ctx context.Context, env *skills.Env, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	})

	result, err := r.Execute(context.Background(), &skills.Env{}, "echo", json.RawMessage(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"msg":"hi"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestRegistryExecuteUnknown(t *testing.T) {
	r := skills.NewRegistry()
	_, err := r.Execute(context.Background(), &skills.Env{}, "nope", nil)
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestRegistryExecuteNilHandler(t *testing.T) {
	r := skills.NewRegistry()
	r.Register(&skills.Skill{Name: "nohandler"})
	_, err := r.Execute(context.Background(), &skills.Env{}, "nohandler", nil)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}
