package microai

import (
	"context"
	"errors"
	"testing"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

// stubLLM implements CompletionClient for tests.
type stubLLM struct {
	result string
	err    error
	calls  int
}

func (s *stubLLM) Generate(_ context.Context, _, _, _ string) (string, error) {
	s.calls++
	return s.result, s.err
}

func (s *stubLLM) GenerateFromMessages(_ context.Context, _ string, _ []threads.Message) (string, error) {
	s.calls++
	return s.result, s.err
}

func TestReasonCachesResult(t *testing.T) {
	llm := &stubLLM{result: "write_file"}
	r := New(llm)

	got1, err := r.Reason(context.Background(), "classify", "some input")
	if err != nil {
		t.Fatal(err)
	}

	got2, err := r.Reason(context.Background(), "classify", "some input")
	if err != nil {
		t.Fatal(err)
	}

	if got1 != got2 {
		t.Errorf("expected same result, got %q and %q", got1, got2)
	}
	if llm.calls != 1 {
		t.Errorf("expected 1 LLM call (cache hit), got %d", llm.calls)
	}
}

func TestReasonDifferentInputsNotCached(t *testing.T) {
	llm := &stubLLM{result: "ok"}
	r := New(llm)

	r.Reason(context.Background(), "task", "input-a")
	r.Reason(context.Background(), "task", "input-b")

	if llm.calls != 2 {
		t.Errorf("expected 2 LLM calls for different inputs, got %d", llm.calls)
	}
}

func TestReasonReturnsErrorOnFailure(t *testing.T) {
	llm := &stubLLM{err: errors.New("timeout")}
	r := New(llm)

	_, err := r.Reason(context.Background(), "task", "input")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReasonRespectsMaxEntries(t *testing.T) {
	llm := &stubLLM{result: "cached"}
	r := New(llm, WithMaxEntries(2))

	r.Reason(context.Background(), "t", "a")
	r.Reason(context.Background(), "t", "b")
	r.Reason(context.Background(), "t", "c") // should NOT be cached

	// Reset result to detect if cache is used
	llm.result = "fresh"
	got, _ := r.Reason(context.Background(), "t", "c")
	if got != "fresh" {
		t.Errorf("entry c should not have been cached, got %q", got)
	}
}

func TestMockReasonerWithFunction(t *testing.T) {
	m := &MockReasoner{
		Fn: func(task, input string) string { return "mock:" + task + ":" + input },
	}
	got, err := m.Reason(context.Background(), "classify", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock:classify:hello" {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestMockReasonerPassthrough(t *testing.T) {
	m := &MockReasoner{}
	got, err := m.Reason(context.Background(), "task", "echo")
	if err != nil {
		t.Fatal(err)
	}
	if got != "echo" {
		t.Errorf("expected passthrough, got %q", got)
	}
}
