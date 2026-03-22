package missions

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu          sync.RWMutex
	programs    map[string]Program
	missions    map[string]Mission
	children    map[string][]string
	assignments map[string][]Assignment
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		programs:    map[string]Program{},
		missions:    map[string]Mission{},
		children:    map[string][]string{},
		assignments: map[string][]Assignment{},
	}
}

func (s *MemoryStore) CreateProgram(program Program) error {
	if program.ID == "" {
		return fmt.Errorf("program id is required")
	}
	if program.ClientID == "" {
		return fmt.Errorf("client id is required")
	}
	if program.Title == "" {
		return fmt.Errorf("program title is required")
	}
	if program.Status == "" {
		program.Status = ProgramStatusDrafted
	}
	now := time.Now().UTC()
	if program.CreatedAt.IsZero() {
		program.CreatedAt = now
	}
	program.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.programs[program.ID]; exists {
		return nil
	}
	s.programs[program.ID] = program
	return nil
}

func (s *MemoryStore) GetProgram(programID string) (Program, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	program, exists := s.programs[programID]
	if !exists {
		return Program{}, ErrProgramNotFound
	}
	return program, nil
}

func (s *MemoryStore) UpdateProgram(program Program) error {
	if program.ID == "" {
		return fmt.Errorf("program id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.programs[program.ID]
	if !exists {
		return ErrProgramNotFound
	}
	if program.CreatedAt.IsZero() {
		program.CreatedAt = existing.CreatedAt
	}
	program.UpdatedAt = time.Now().UTC()
	s.programs[program.ID] = program
	return nil
}

func (s *MemoryStore) CreateMission(mission Mission) error {
	if mission.ID == "" {
		return fmt.Errorf("mission id is required")
	}
	if mission.ProgramID == "" {
		return fmt.Errorf("program id is required")
	}
	if mission.Title == "" {
		return fmt.Errorf("mission title is required")
	}
	if mission.OwnerAgentID == "" {
		return fmt.Errorf("owner agent id is required")
	}
	if mission.OwnerRole == "" {
		return fmt.Errorf("owner role is required")
	}
	if mission.MissionType == "" {
		return fmt.Errorf("mission type is required")
	}
	if mission.Status == "" {
		mission.Status = MissionStatusDrafted
	}
	if len(mission.ReuseTrace) == 0 {
		mission.ReuseTrace = []byte(`[]`)
	}
	if mission.Priority == "" {
		mission.Priority = PriorityMedium
	}
	if mission.RiskLevel == "" {
		mission.RiskLevel = PriorityMedium
	}
	now := time.Now().UTC()
	if mission.CreatedAt.IsZero() {
		mission.CreatedAt = now
	}
	mission.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.programs[mission.ProgramID]; !exists {
		return ErrProgramNotFound
	}
	if _, exists := s.missions[mission.ID]; exists {
		return nil
	}
	if mission.ParentMissionID != "" {
		if _, exists := s.missions[mission.ParentMissionID]; !exists {
			return ErrMissionNotFound
		}
	}
	if mission.RootMissionID == "" {
		mission.RootMissionID = mission.ID
	}
	s.missions[mission.ID] = mission
	if mission.ParentMissionID != "" {
		s.children[mission.ParentMissionID] = append(s.children[mission.ParentMissionID], mission.ID)
	}
	return nil
}

func (s *MemoryStore) GetMission(missionID string) (Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mission, exists := s.missions[missionID]
	if !exists {
		return Mission{}, ErrMissionNotFound
	}
	return mission, nil
}

func (s *MemoryStore) UpdateMission(mission Mission) error {
	if mission.ID == "" {
		return fmt.Errorf("mission id is required")
	}
	if len(mission.ReuseTrace) == 0 {
		mission.ReuseTrace = []byte(`[]`)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.missions[mission.ID]
	if !exists {
		return ErrMissionNotFound
	}
	if mission.CreatedAt.IsZero() {
		mission.CreatedAt = existing.CreatedAt
	}
	if mission.RootMissionID == "" {
		mission.RootMissionID = existing.RootMissionID
	}
	mission.UpdatedAt = time.Now().UTC()
	s.missions[mission.ID] = mission
	return nil
}

func (s *MemoryStore) ListChildMissions(parentMissionID string) ([]Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	childIDs := s.children[parentMissionID]
	missions := make([]Mission, 0, len(childIDs))
	for _, childID := range childIDs {
		missions = append(missions, s.missions[childID])
	}
	return missions, nil
}

func (s *MemoryStore) SearchReusableMissions(query string, limit int) ([]ReusableMissionMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := make([]Mission, 0, len(s.missions))
	for _, mission := range s.missions {
		if IsReusableMissionStatus(mission.Status) {
			candidates = append(candidates, mission)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})

	return findReusableMissionMatches(candidates, query, limit), nil
}

func (s *MemoryStore) AssignMission(assignment Assignment) error {
	if assignment.ID == "" {
		return fmt.Errorf("assignment id is required")
	}
	if assignment.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if assignment.AgentID == "" {
		return fmt.Errorf("agent id is required")
	}
	if assignment.AgentRole == "" {
		return fmt.Errorf("agent role is required")
	}
	if assignment.AssignedAt.IsZero() {
		assignment.AssignedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.missions[assignment.MissionID]; !exists {
		return ErrMissionNotFound
	}
	s.assignments[assignment.MissionID] = append(s.assignments[assignment.MissionID], assignment)
	return nil
}

func (s *MemoryStore) ListAssignments(missionID string) ([]Assignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.missions[missionID]; !exists {
		return nil, ErrMissionNotFound
	}
	assignments := make([]Assignment, len(s.assignments[missionID]))
	copy(assignments, s.assignments[missionID])
	return assignments, nil
}
