package feedback

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	records  map[string]Record
	byThread map[string][]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records:  map[string]Record{},
		byThread: map[string][]string{},
	}
}

func (s *MemoryStore) CreateFeedback(record Record) error {
	if record.ID == "" {
		return fmt.Errorf("feedback id is required")
	}
	if record.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if record.ResponseID == "" {
		return fmt.Errorf("response id is required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.AnalysisStatus == "" {
		record.AnalysisStatus = AnalysisStatusRaw
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[record.ID]; exists {
		return nil
	}
	s.records[record.ID] = record
	s.byThread[record.ThreadID] = append(s.byThread[record.ThreadID], record.ID)
	return nil
}

func (s *MemoryStore) GetFeedback(feedbackID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, exists := s.records[feedbackID]
	if !exists {
		return Record{}, ErrFeedbackNotFound
	}
	return record, nil
}

func (s *MemoryStore) ListByThread(threadID string) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byThread[threadID]
	records := make([]Record, 0, len(ids))
	for _, id := range ids {
		records = append(records, s.records[id])
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	return records, nil
}
