package microai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

const (
	DefaultModel      = "gpt-4.1-nano"
	DefaultTimeout    = 5 * time.Second
	DefaultMaxEntries = 10000
)

// CompletionClient is the LLM interface used by the reasoner.
type CompletionClient interface {
	Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error)
	GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error)
}

// Reasoner makes tiny, fast AI calls for classification, normalization,
// and other micro-reasoning tasks that would otherwise require large
// hardcoded switch/case or if/else logic.
type Reasoner struct {
	llm        CompletionClient
	model      string
	timeout    time.Duration
	cache      sync.Map
	entryCount atomic.Int64
	maxEntries int64
	logger     *slog.Logger
}

// Option configures a Reasoner.
type Option func(*Reasoner)

// WithModel sets the model name.
func WithModel(model string) Option { return func(r *Reasoner) { r.model = model } }

// WithTimeout sets the per-call timeout.
func WithTimeout(d time.Duration) Option { return func(r *Reasoner) { r.timeout = d } }

// WithMaxEntries sets the cache size limit.
func WithMaxEntries(n int64) Option { return func(r *Reasoner) { r.maxEntries = n } }

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option { return func(r *Reasoner) { r.logger = l } }

// New creates a Reasoner backed by the given LLM client.
func New(llm CompletionClient, opts ...Option) *Reasoner {
	r := &Reasoner{
		llm:        llm,
		model:      DefaultModel,
		timeout:    DefaultTimeout,
		maxEntries: DefaultMaxEntries,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Reason sends a focused micro-prompt to the tiny model and returns the
// raw response string. Results are cached by SHA-256 of (task + input).
// If the call fails or times out, returns empty string + error.
func (r *Reasoner) Reason(ctx context.Context, task string, input string) (string, error) {
	key := cacheKey(task, input)

	if cached, ok := r.cache.Load(key); ok {
		return cached.(string), nil
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	result, err := r.llm.Generate(callCtx, r.model, task, input)
	if err != nil {
		r.logger.Debug("microai reason failed", "model", r.model, "error", err)
		return "", fmt.Errorf("microai reason: %w", err)
	}

	if r.entryCount.Load() < r.maxEntries {
		r.cache.Store(key, result)
		r.entryCount.Add(1)
	}

	return result, nil
}

func cacheKey(task, input string) string {
	h := sha256.New()
	h.Write([]byte(task))
	h.Write([]byte{0})
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

// Interface is the common interface satisfied by both Reasoner and MockReasoner.
type Interface interface {
	Reason(ctx context.Context, task string, input string) (string, error)
}

// MockReasoner is a test double that delegates to a user-supplied function.
type MockReasoner struct {
	Fn func(task, input string) string
}

// Reason calls Fn if set, otherwise returns the input unchanged.
func (m *MockReasoner) Reason(_ context.Context, task, input string) (string, error) {
	if m.Fn != nil {
		return m.Fn(task, input), nil
	}
	return input, nil
}
