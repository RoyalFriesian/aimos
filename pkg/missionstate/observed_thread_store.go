package missionstate

import (
	"fmt"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

type observedThreadStore struct {
	base    threads.Store
	runtime *Runtime
}

func NewObservedThreadStore(base threads.Store, runtime *Runtime) (threads.Store, error) {
	if base == nil {
		return nil, fmt.Errorf("base thread store is required")
	}
	if runtime == nil {
		return nil, fmt.Errorf("mission state runtime is required")
	}
	return &observedThreadStore{base: base, runtime: runtime}, nil
}

func (s *observedThreadStore) CreateThread(thread threads.Thread) error {
	return s.base.CreateThread(thread)
}

func (s *observedThreadStore) GetThread(threadID string) (threads.Thread, error) {
	return s.base.GetThread(threadID)
}

func (s *observedThreadStore) ListByMission(missionID string) ([]threads.Thread, error) {
	return s.base.ListByMission(missionID)
}

func (s *observedThreadStore) ListRootThreads() ([]threads.Thread, error) {
	return s.base.ListRootThreads()
}

func (s *observedThreadStore) SearchReusableThreads(query string, limit int) ([]threads.ReusableThreadMatch, error) {
	return s.base.SearchReusableThreads(query, limit)
}

func (s *observedThreadStore) AppendMessage(message threads.Message) error {
	if err := s.base.AppendMessage(message); err != nil {
		return err
	}
	thread, err := s.base.GetThread(message.ThreadID)
	if err != nil {
		return err
	}
	if _, _, err := s.runtime.RefreshMissionState(thread.MissionID, thread.ID); err != nil {
		return err
	}
	return nil
}

func (s *observedThreadStore) ListMessages(threadID string) ([]threads.Message, error) {
	return s.base.ListMessages(threadID)
}

func (s *observedThreadStore) UpdateThreadMode(threadID string, mode string) error {
	return s.base.UpdateThreadMode(threadID, mode)
}

func (s *observedThreadStore) UpdateThreadOwner(threadID string, ownerAgentID string) error {
	return s.base.UpdateThreadOwner(threadID, ownerAgentID)
}

var _ threads.Store = (*observedThreadStore)(nil)
