package aiclients

import "sync/atomic"

// BudgetConfig holds runtime-tunable token budget settings.
// All fields are safe for concurrent read/write without a mutex.
type BudgetConfig struct {
	// enabled controls whether the token budget middleware is active.
	enabled atomic.Bool
	// threshold is the max estimated tokens before summarization kicks in.
	threshold atomic.Int64
	// target is the desired token count after summarization.
	target atomic.Int64
}

const (
	DefaultBudgetThreshold int64 = 250
	DefaultBudgetTarget    int64 = 300
)

// NewBudgetConfig returns a config with sensible defaults.
func NewBudgetConfig() *BudgetConfig {
	c := &BudgetConfig{}
	c.enabled.Store(true)
	c.threshold.Store(DefaultBudgetThreshold)
	c.target.Store(DefaultBudgetTarget)
	return c
}

func (c *BudgetConfig) Enabled() bool        { return c.enabled.Load() }
func (c *BudgetConfig) Threshold() int64     { return c.threshold.Load() }
func (c *BudgetConfig) Target() int64        { return c.target.Load() }
func (c *BudgetConfig) SetEnabled(v bool)    { c.enabled.Store(v) }
func (c *BudgetConfig) SetThreshold(v int64) { c.threshold.Store(v) }
func (c *BudgetConfig) SetTarget(v int64)    { c.target.Store(v) }

// Snapshot returns a plain struct copy for JSON serialization.
func (c *BudgetConfig) Snapshot() BudgetSnapshot {
	return BudgetSnapshot{
		Enabled:   c.Enabled(),
		Threshold: c.Threshold(),
		Target:    c.Target(),
	}
}

// BudgetSnapshot is a plain JSON-friendly representation.
type BudgetSnapshot struct {
	Enabled   bool  `json:"enabled"`
	Threshold int64 `json:"threshold"`
	Target    int64 `json:"target"`
}
