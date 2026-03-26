package threads

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.RWMutex
	threads   map[string]Thread
	byMission map[string][]string
	messages  map[string][]Message
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		threads:   map[string]Thread{},
		byMission: map[string][]string{},
		messages:  map[string][]Message{},
	}
}

func (s *MemoryStore) CreateThread(thread Thread) error {
	if thread.ID == "" {
		return fmt.Errorf("thread id is required")
	}
	if thread.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if thread.Title == "" {
		return fmt.Errorf("thread title is required")
	}

	now := time.Now().UTC()
	if thread.CreatedAt.IsZero() {
		thread.CreatedAt = now
	}
	thread.UpdatedAt = now
	if thread.Summary == "" {
		thread.Summary = thread.Title
	}
	if thread.Context == "" {
		thread.Context = thread.Summary
	}
	if thread.Status == "" {
		thread.Status = ThreadStatusActive
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.threads[thread.ID]; exists {
		return nil
	}
	s.threads[thread.ID] = thread
	s.byMission[thread.MissionID] = append(s.byMission[thread.MissionID], thread.ID)
	return nil
}

func (s *MemoryStore) GetThread(threadID string) (Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threads[threadID]
	if !exists {
		return Thread{}, ErrThreadNotFound
	}
	return thread, nil
}

func (s *MemoryStore) ListByMission(missionID string) ([]Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	threadIDs := s.byMission[missionID]
	threadsForMission := make([]Thread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		threadsForMission = append(threadsForMission, s.threads[threadID])
	}
	return threadsForMission, nil
}

func (s *MemoryStore) SearchReusableThreads(query string, limit int) ([]ReusableThreadMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := make([]Thread, 0, len(s.threads))
	for _, thread := range s.threads {
		if IsReusableThreadStatus(thread.Status) {
			candidates = append(candidates, thread)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})

	return findReusableThreadMatches(candidates, query, limit), nil
}

func (s *MemoryStore) AppendMessage(message Message) error {
	if message.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if message.Role == "" {
		return fmt.Errorf("message role is required")
	}
	if message.Content == "" {
		return fmt.Errorf("message content is required")
	}

	now := time.Now().UTC()
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.threads[message.ThreadID]; !exists {
		return ErrThreadNotFound
	}
	s.messages[message.ThreadID] = append(s.messages[message.ThreadID], message)

	thread := s.threads[message.ThreadID]
	thread.UpdatedAt = now
	s.threads[message.ThreadID] = thread
	return nil
}

func (s *MemoryStore) ListMessages(threadID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.threads[threadID]; !exists {
		return nil, ErrThreadNotFound
	}
	snapshot := s.messages[threadID]
	messages := make([]Message, len(snapshot))
	copy(messages, snapshot)
	return messages, nil
}

func (s *MemoryStore) UpdateThreadMode(threadID string, mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	thread, exists := s.threads[threadID]
	if !exists {
		return ErrThreadNotFound
	}
	thread.CurrentMode = mode
	thread.UpdatedAt = time.Now().UTC()
	s.threads[threadID] = thread
	return nil
}

func (s *MemoryStore) UpdateThreadOwner(threadID string, ownerAgentID string) error {
	if ownerAgentID == "" {
		return fmt.Errorf("owner agent id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	thread, exists := s.threads[threadID]
	if !exists {
		return ErrThreadNotFound
	}
	thread.OwnerAgentID = ownerAgentID
	thread.UpdatedAt = time.Now().UTC()
	s.threads[threadID] = thread
	return nil
}

func (s *MemoryStore) ListRootThreads() ([]Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Thread
	for _, t := range s.threads {
		if t.ParentThreadID == "" {
			results = append(results, t)
		}
	}

	// sort here if needed, omitted for memory store simplicity
	return results, nil
}
func (s *MemoryStore) UpdateThreadTitle(threadID string, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	thread, exists := s.threads[threadID]
	if !exists {
		return ErrThreadNotFound
	}

	thread.Title = title
	thread.UpdatedAt = time.Now().UTC()
	s.threads[threadID] = thread

	return nil
}
