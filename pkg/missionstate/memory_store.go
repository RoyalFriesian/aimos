package missionstate

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

type MemoryStore struct {
	mu             sync.RWMutex
	summaries      map[string][]Summary
	rollups        map[string]Rollup
	rollupByParent map[string][]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		summaries:      map[string][]Summary{},
		rollups:        map[string]Rollup{},
		rollupByParent: map[string][]string{},
	}
}

func (s *MemoryStore) CreateSummary(summary Summary) error {
	if summary.ID == "" {
		return fmt.Errorf("summary id is required")
	}
	if summary.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if summary.Level == "" {
		return fmt.Errorf("summary level is required")
	}
	if summary.Kind == "" {
		return fmt.Errorf("summary kind is required")
	}
	if summary.CoverageStartRef == "" {
		return fmt.Errorf("coverage start ref is required")
	}
	if summary.CoverageEndRef == "" {
		return fmt.Errorf("coverage end ref is required")
	}
	if summary.SummaryText == "" {
		return fmt.Errorf("summary text is required")
	}
	if len(summary.KeyDecisions) == 0 {
		summary.KeyDecisions = []byte(`[]`)
	}
	if len(summary.OpenQuestions) == 0 {
		summary.OpenQuestions = []byte(`[]`)
	}
	if len(summary.Blockers) == 0 {
		summary.Blockers = []byte(`[]`)
	}
	if len(summary.NextActions) == 0 {
		summary.NextActions = []byte(`[]`)
	}
	if summary.CreatedAt.IsZero() {
		summary.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.summaries[summary.MissionID]
	for _, existing := range entries {
		if existing.ID == summary.ID {
			return nil
		}
	}
	s.summaries[summary.MissionID] = append(entries, summary)
	return nil
}

func (s *MemoryStore) GetLatestSummary(missionID string) (Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.summaries[missionID]
	if len(entries) == 0 {
		return Summary{}, ErrSummaryNotFound
	}
	return entries[len(entries)-1], nil
}

func (s *MemoryStore) ListSummaries(missionID string) ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.summaries[missionID]
	result := make([]Summary, len(entries))
	copy(result, entries)
	return result, nil
}

func (s *MemoryStore) UpsertRollup(rollup Rollup) error {
	if rollup.ID == "" {
		return fmt.Errorf("rollup id is required")
	}
	if rollup.ParentMissionID == "" {
		return fmt.Errorf("parent mission id is required")
	}
	if rollup.ChildMissionID == "" {
		return fmt.Errorf("child mission id is required")
	}
	if rollup.Status == "" {
		return fmt.Errorf("rollup status is required")
	}
	if rollup.Health == "" {
		return fmt.Errorf("rollup health is required")
	}
	if rollup.LatestSummary == "" {
		return fmt.Errorf("latest summary is required")
	}
	if len(rollup.OverdueFlags) == 0 {
		rollup.OverdueFlags = []byte(`[]`)
	}
	if len(rollup.ExecutionSummary) == 0 {
		rollup.ExecutionSummary = []byte(`{}`)
	}
	if rollup.UpdatedAt.IsZero() {
		rollup.UpdatedAt = time.Now().UTC()
	}

	key := rollupKey(rollup.ParentMissionID, rollup.ChildMissionID)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.rollups[key]; !exists {
		s.rollupByParent[rollup.ParentMissionID] = append(s.rollupByParent[rollup.ParentMissionID], key)
	}
	s.rollups[key] = rollup
	return nil
}

func (s *MemoryStore) GetRollup(parentMissionID string, childMissionID string) (Rollup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rollup, exists := s.rollups[rollupKey(parentMissionID, childMissionID)]
	if !exists {
		return Rollup{}, ErrRollupNotFound
	}
	return rollup, nil
}

func (s *MemoryStore) ListRollups(parentMissionID string) ([]Rollup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := s.rollupByParent[parentMissionID]
	result := make([]Rollup, 0, len(keys))
	for _, key := range keys {
		result = append(result, s.rollups[key])
	}
	return result, nil
}

func rollupKey(parentMissionID string, childMissionID string) string {
	return parentMissionID + "::" + childMissionID
}

var _ Store = (*MemoryStore)(nil)
var _ = missions.MissionStatusActive
