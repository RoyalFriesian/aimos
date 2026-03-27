package attachments

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

// MemoryStore is an in-memory implementation of Store for tests.
type MemoryStore struct {
	mu         sync.RWMutex
	records    map[string]Attachment
	byMission  map[string][]string
	byThread   map[string][]string
	missionGet func(string) (missions.Mission, error)
}

func NewMemoryStore(missionGetter func(string) (missions.Mission, error)) *MemoryStore {
	return &MemoryStore{
		records:    map[string]Attachment{},
		byMission:  map[string][]string{},
		byThread:   map[string][]string{},
		missionGet: missionGetter,
	}
}

func (s *MemoryStore) Create(attachment Attachment) error {
	if attachment.ID == "" {
		return fmt.Errorf("attachment id is required")
	}
	if attachment.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if attachment.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if attachment.Filename == "" {
		return fmt.Errorf("filename is required")
	}
	if attachment.AbsolutePath == "" {
		return fmt.Errorf("absolute path is required")
	}
	if attachment.FileCategory == "" {
		return fmt.Errorf("file category is required")
	}
	if attachment.Status == "" {
		attachment.Status = StatusActive
	}
	if attachment.CreatedAt.IsZero() {
		attachment.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[attachment.ID]; exists {
		return nil // idempotent
	}
	s.records[attachment.ID] = attachment
	s.byMission[attachment.MissionID] = append(s.byMission[attachment.MissionID], attachment.ID)
	s.byThread[attachment.ThreadID] = append(s.byThread[attachment.ThreadID], attachment.ID)
	return nil
}

func (s *MemoryStore) Get(attachmentID string) (Attachment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, exists := s.records[attachmentID]
	if !exists {
		return Attachment{}, ErrAttachmentNotFound
	}
	return record, nil
}

func (s *MemoryStore) ListByMission(missionID string) ([]Attachment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byMission[missionID]
	result := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		record, exists := s.records[id]
		if !exists {
			continue
		}
		if record.Status != StatusActive {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *MemoryStore) ListByThread(threadID string) ([]Attachment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byThread[threadID]
	result := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		record, exists := s.records[id]
		if !exists {
			continue
		}
		if record.Status != StatusActive {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *MemoryStore) ListInheritedAttachments(missionID string) ([]Attachment, error) {
	if s.missionGet == nil {
		return s.ListByMission(missionID)
	}

	// Walk the parent chain to collect mission IDs from root to target.
	chain := []string{missionID}
	current := missionID
	for {
		mission, err := s.missionGet(current)
		if err != nil {
			break
		}
		if mission.ParentMissionID == "" {
			break
		}
		chain = append(chain, mission.ParentMissionID)
		current = mission.ParentMissionID
	}
	// Reverse so root comes first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Attachment
	for _, mid := range chain {
		ids := s.byMission[mid]
		for _, id := range ids {
			record, exists := s.records[id]
			if !exists || record.Status != StatusActive {
				continue
			}
			result = append(result, record)
		}
	}
	return result, nil
}
