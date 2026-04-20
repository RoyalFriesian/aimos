package agents

import "sync/atomic"

// WakeIntervalConfig holds runtime-tunable wake interval settings for agent loops.
// All fields are safe for concurrent read/write without a mutex.
type WakeIntervalConfig struct {
	ceoSeconds     atomic.Int64
	managerSeconds atomic.Int64
	workerSeconds  atomic.Int64
}

// NewWakeIntervalConfig returns a config with the current default intervals.
func NewWakeIntervalConfig() *WakeIntervalConfig {
	c := &WakeIntervalConfig{}
	c.ceoSeconds.Store(int64(DefaultCEOWakeInterval.Seconds()))
	c.managerSeconds.Store(int64(DefaultManagerWakeInterval.Seconds()))
	c.workerSeconds.Store(int64(DefaultWorkerWakeInterval.Seconds()))
	return c
}

func (c *WakeIntervalConfig) CEOSeconds() int64     { return c.ceoSeconds.Load() }
func (c *WakeIntervalConfig) ManagerSeconds() int64  { return c.managerSeconds.Load() }
func (c *WakeIntervalConfig) WorkerSeconds() int64   { return c.workerSeconds.Load() }
func (c *WakeIntervalConfig) SetCEOSeconds(v int64)  { c.ceoSeconds.Store(v) }
func (c *WakeIntervalConfig) SetManagerSeconds(v int64) { c.managerSeconds.Store(v) }
func (c *WakeIntervalConfig) SetWorkerSeconds(v int64)  { c.workerSeconds.Store(v) }

// Snapshot returns a plain struct copy for JSON serialization.
func (c *WakeIntervalConfig) Snapshot() WakeIntervalSnapshot {
	return WakeIntervalSnapshot{
		CEOSeconds:     c.CEOSeconds(),
		ManagerSeconds: c.ManagerSeconds(),
		WorkerSeconds:  c.WorkerSeconds(),
	}
}

// WakeIntervalSnapshot is a plain JSON-friendly representation.
type WakeIntervalSnapshot struct {
	CEOSeconds     int64 `json:"ceoSeconds"`
	ManagerSeconds int64 `json:"managerSeconds"`
	WorkerSeconds  int64 `json:"workerSeconds"`
}
