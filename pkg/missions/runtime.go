package missions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

type Runtime struct {
	missions Store
	threads  threads.Store
}

type RootMissionInput struct {
	ProgramID      string
	ClientID       string
	ProgramTitle   string
	MissionID      string
	ThreadID       string
	OwnerAgentID   string
	OwnerRole      string
	MissionType    string
	ThreadKind     string
	MissionTitle   string
	Charter        string
	Goal           string
	Scope          string
	AuthorityLevel string
	ThreadTitle    string
	ThreadSummary  string
	ThreadContext  string
}

type ChildMissionInput struct {
	MissionID       string
	ParentMissionID string
	ThreadID        string
	OwnerAgentID    string
	OwnerRole       string
	MissionType     string
	ThreadKind      string
	MissionTitle    string
	Charter         string
	Goal            string
	Scope           string
	AuthorityLevel  string
	ThreadTitle     string
	ThreadSummary   string
	ThreadContext   string
	ParentThreadID  string
	ReuseTrace      json.RawMessage
}

type PlannedChildMissionInput struct {
	Title          string
	Charter        string
	Goal           string
	Scope          string
	MissionType    string
	AuthorityLevel string
	ReuseTrace     json.RawMessage
	ThreadKind     string
	ThreadSummary  string
	ThreadContext  string
}

type DelegationHandoffInput struct {
	MissionID          string
	AgentID            string
	AgentRole          string
	AuthorityScope     json.RawMessage
	ReportingToAgentID string
}

func NewRuntime(missionStore Store, threadStore threads.Store) (*Runtime, error) {
	if missionStore == nil {
		return nil, fmt.Errorf("mission store is required")
	}
	if threadStore == nil {
		return nil, fmt.Errorf("thread store is required")
	}
	return &Runtime{missions: missionStore, threads: threadStore}, nil
}

func (r *Runtime) CreateProgramWithRootMission(input RootMissionInput) (Program, Mission, threads.Thread, error) {
	if input.ProgramID == "" || input.MissionID == "" || input.ThreadID == "" {
		return Program{}, Mission{}, threads.Thread{}, fmt.Errorf("program id, mission id, and thread id are required")
	}

	program := Program{
		ID:       input.ProgramID,
		ClientID: input.ClientID,
		Title:    input.ProgramTitle,
		Status:   ProgramStatusActive,
	}
	if err := r.missions.CreateProgram(program); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}

	mission := Mission{
		ID:             input.MissionID,
		ProgramID:      input.ProgramID,
		RootMissionID:  input.MissionID,
		OwnerAgentID:   input.OwnerAgentID,
		OwnerRole:      input.OwnerRole,
		MissionType:    input.MissionType,
		Title:          input.MissionTitle,
		Charter:        input.Charter,
		Goal:           input.Goal,
		Scope:          input.Scope,
		AuthorityLevel: input.AuthorityLevel,
		Status:         MissionStatusActive,
		Priority:       PriorityCritical,
		RiskLevel:      PriorityHigh,
	}
	if err := r.missions.CreateMission(mission); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}

	thread := threads.Thread{
		ID:            input.ThreadID,
		MissionID:     input.MissionID,
		RootMissionID: input.MissionID,
		Kind:          input.ThreadKind,
		Title:         input.ThreadTitle,
		Summary:       input.ThreadSummary,
		Context:       input.ThreadContext,
		OwnerAgentID:  input.OwnerAgentID,
		Status:        threads.ThreadStatusActive,
		CreatedAt:     time.Now().UTC(),
	}
	if err := r.threads.CreateThread(thread); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}

	mission.OwningThreadID = input.ThreadID
	if err := r.missions.UpdateMission(mission); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}
	if err := r.missions.AssignMission(Assignment{
		ID:         fmt.Sprintf("assign-%s", input.MissionID),
		MissionID:  input.MissionID,
		AgentID:    input.OwnerAgentID,
		AgentRole:  input.OwnerRole,
		AssignedAt: time.Now().UTC(),
	}); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}

	program.RootMissionID = input.MissionID
	if err := r.missions.UpdateProgram(program); err != nil {
		return Program{}, Mission{}, threads.Thread{}, err
	}

	program, _ = r.missions.GetProgram(input.ProgramID)
	mission, _ = r.missions.GetMission(input.MissionID)
	thread, _ = r.threads.GetThread(input.ThreadID)
	return program, mission, thread, nil
}

func (r *Runtime) CreateChildMission(input ChildMissionInput) (Mission, threads.Thread, error) {
	if input.MissionID == "" || input.ParentMissionID == "" || input.ThreadID == "" {
		return Mission{}, threads.Thread{}, fmt.Errorf("mission id, parent mission id, and thread id are required")
	}
	parentMission, err := r.missions.GetMission(input.ParentMissionID)
	if err != nil {
		return Mission{}, threads.Thread{}, err
	}

	mission := Mission{
		ID:              input.MissionID,
		ProgramID:       parentMission.ProgramID,
		ParentMissionID: parentMission.ID,
		RootMissionID:   parentMission.RootMissionID,
		OwnerAgentID:    input.OwnerAgentID,
		OwnerRole:       input.OwnerRole,
		MissionType:     input.MissionType,
		Title:           input.MissionTitle,
		Charter:         input.Charter,
		Goal:            input.Goal,
		Scope:           input.Scope,
		ReuseTrace:      input.ReuseTrace,
		AuthorityLevel:  input.AuthorityLevel,
		Status:          MissionStatusActive,
		Priority:        PriorityHigh,
		RiskLevel:       PriorityMedium,
	}
	if err := r.missions.CreateMission(mission); err != nil {
		return Mission{}, threads.Thread{}, err
	}

	thread := threads.Thread{
		ID:             input.ThreadID,
		MissionID:      input.MissionID,
		RootMissionID:  parentMission.RootMissionID,
		ParentThreadID: input.ParentThreadID,
		Kind:           input.ThreadKind,
		Title:          input.ThreadTitle,
		Summary:        input.ThreadSummary,
		Context:        input.ThreadContext,
		OwnerAgentID:   input.OwnerAgentID,
		Status:         threads.ThreadStatusActive,
		CreatedAt:      time.Now().UTC(),
	}
	if err := r.threads.CreateThread(thread); err != nil {
		return Mission{}, threads.Thread{}, err
	}

	mission.OwningThreadID = input.ThreadID
	if err := r.missions.UpdateMission(mission); err != nil {
		return Mission{}, threads.Thread{}, err
	}
	if err := r.missions.AssignMission(Assignment{
		ID:                 fmt.Sprintf("assign-%s", input.MissionID),
		MissionID:          input.MissionID,
		AgentID:            input.OwnerAgentID,
		AgentRole:          input.OwnerRole,
		ReportingToAgentID: parentMission.OwnerAgentID,
		AssignedAt:         time.Now().UTC(),
	}); err != nil {
		return Mission{}, threads.Thread{}, err
	}

	mission, _ = r.missions.GetMission(input.MissionID)
	thread, _ = r.threads.GetThread(input.ThreadID)
	return mission, thread, nil
}

func (r *Runtime) PersistPlannedChildMissions(parentMissionID string, parentThreadID string, plans []PlannedChildMissionInput) ([]Mission, []threads.Thread, error) {
	if parentMissionID == "" {
		return nil, nil, fmt.Errorf("parent mission id is required")
	}
	if len(plans) == 0 {
		return []Mission{}, []threads.Thread{}, nil
	}

	parentMission, err := r.missions.GetMission(parentMissionID)
	if err != nil {
		return nil, nil, err
	}
	existingChildren, err := r.missions.ListChildMissions(parentMissionID)
	if err != nil {
		return nil, nil, err
	}
	existingByKey := make(map[string]Mission, len(existingChildren))
	for _, child := range existingChildren {
		existingByKey[plannedMissionKey(child.Title, child.MissionType)] = child
	}

	persistedMissions := make([]Mission, 0, len(plans))
	persistedThreads := make([]threads.Thread, 0, len(plans))
	for _, plan := range plans {
		key := plannedMissionKey(plan.Title, plan.MissionType)
		if key == "" {
			continue
		}

		existing, exists := existingByKey[key]
		if exists {
			existing.Charter = emptyMissionField(plan.Charter, existing.Charter)
			existing.Goal = emptyMissionField(plan.Goal, existing.Goal)
			existing.Scope = emptyMissionField(plan.Scope, existing.Scope)
			existing.AuthorityLevel = emptyMissionField(plan.AuthorityLevel, existing.AuthorityLevel)
			existing.MissionType = emptyMissionField(plan.MissionType, existing.MissionType)
			if len(plan.ReuseTrace) > 0 {
				existing.ReuseTrace = plan.ReuseTrace
			}
			if err := r.missions.UpdateMission(existing); err != nil {
				return nil, nil, err
			}
			thread, err := r.upsertChildThread(existing, parentMission, parentThreadID, plan)
			if err != nil {
				return nil, nil, err
			}
			persistedMissions = append(persistedMissions, existing)
			persistedThreads = append(persistedThreads, thread)
			continue
		}

		missionID := plannedMissionID(parentMissionID, plan.Title)
		threadID := plannedThreadID(missionID)
		childMission, childThread, err := r.CreateChildMission(ChildMissionInput{
			MissionID:       missionID,
			ParentMissionID: parentMissionID,
			ThreadID:        threadID,
			OwnerAgentID:    parentMission.OwnerAgentID,
			OwnerRole:       parentMission.OwnerRole,
			MissionType:     emptyMissionField(plan.MissionType, "domain"),
			ThreadKind:      emptyMissionField(plan.ThreadKind, "strategy"),
			MissionTitle:    plan.Title,
			Charter:         emptyMissionField(plan.Charter, fmt.Sprintf("Own the %s mission.", plan.Title)),
			Goal:            emptyMissionField(plan.Goal, fmt.Sprintf("Deliver %s.", plan.Title)),
			Scope:           emptyMissionField(plan.Scope, parentMission.Scope),
			AuthorityLevel:  emptyMissionField(plan.AuthorityLevel, "domain"),
			ThreadTitle:     fmt.Sprintf("%s thread", plan.Title),
			ThreadSummary:   emptyMissionField(plan.ThreadSummary, plan.Title),
			ThreadContext:   emptyMissionField(plan.ThreadContext, fmt.Sprintf("Owns mission %s under parent mission %s.", plan.Title, parentMission.Title)),
			ParentThreadID:  parentThreadID,
			ReuseTrace:      plan.ReuseTrace,
		})
		if err != nil {
			return nil, nil, err
		}
		persistedMissions = append(persistedMissions, childMission)
		persistedThreads = append(persistedThreads, childThread)
	}

	return persistedMissions, persistedThreads, nil
}

func (r *Runtime) DelegateMission(input DelegationHandoffInput) (Mission, threads.Thread, Assignment, error) {
	if input.MissionID == "" {
		return Mission{}, threads.Thread{}, Assignment{}, fmt.Errorf("mission id is required")
	}
	if input.AgentID == "" {
		return Mission{}, threads.Thread{}, Assignment{}, fmt.Errorf("agent id is required")
	}
	if input.AgentRole == "" {
		return Mission{}, threads.Thread{}, Assignment{}, fmt.Errorf("agent role is required")
	}

	mission, err := r.missions.GetMission(input.MissionID)
	if err != nil {
		return Mission{}, threads.Thread{}, Assignment{}, err
	}

	reportingToAgentID := input.ReportingToAgentID
	if reportingToAgentID == "" && mission.ParentMissionID != "" {
		parentMission, parentErr := r.missions.GetMission(mission.ParentMissionID)
		if parentErr != nil {
			return Mission{}, threads.Thread{}, Assignment{}, parentErr
		}
		reportingToAgentID = parentMission.OwnerAgentID
	}

	mission.OwnerAgentID = input.AgentID
	mission.OwnerRole = input.AgentRole
	if err := r.missions.UpdateMission(mission); err != nil {
		return Mission{}, threads.Thread{}, Assignment{}, err
	}

	thread, err := r.threads.GetThread(mission.OwningThreadID)
	if err != nil {
		return Mission{}, threads.Thread{}, Assignment{}, err
	}
	if err := r.threads.UpdateThreadOwner(thread.ID, input.AgentID); err != nil {
		return Mission{}, threads.Thread{}, Assignment{}, err
	}

	assignment := Assignment{
		ID:                 fmt.Sprintf("assign-%s-%s", mission.ID, slugifyMissionPart(input.AgentID)),
		MissionID:          mission.ID,
		AgentID:            input.AgentID,
		AgentRole:          input.AgentRole,
		AuthorityScope:     defaultObjectOrRawJSON(input.AuthorityScope),
		ReportingToAgentID: reportingToAgentID,
		AssignedAt:         time.Now().UTC(),
	}
	if err := r.missions.AssignMission(assignment); err != nil {
		return Mission{}, threads.Thread{}, Assignment{}, err
	}

	mission, _ = r.missions.GetMission(mission.ID)
	thread, _ = r.threads.GetThread(thread.ID)
	return mission, thread, assignment, nil
}

func (r *Runtime) upsertChildThread(mission Mission, parentMission Mission, parentThreadID string, plan PlannedChildMissionInput) (threads.Thread, error) {
	threadID := mission.OwningThreadID
	if threadID == "" {
		threadID = plannedThreadID(mission.ID)
	}
	thread, err := r.threads.GetThread(threadID)
	if err == nil {
		return thread, nil
	}
	if err != nil && err != threads.ErrThreadNotFound {
		return threads.Thread{}, err
	}

	createdThread := threads.Thread{
		ID:             threadID,
		MissionID:      mission.ID,
		RootMissionID:  parentMission.RootMissionID,
		ParentThreadID: parentThreadID,
		Kind:           emptyMissionField(plan.ThreadKind, "strategy"),
		Title:          fmt.Sprintf("%s thread", mission.Title),
		Summary:        emptyMissionField(plan.ThreadSummary, mission.Title),
		Context:        emptyMissionField(plan.ThreadContext, fmt.Sprintf("Owns mission %s under parent mission %s.", mission.Title, parentMission.Title)),
		OwnerAgentID:   mission.OwnerAgentID,
		Status:         threads.ThreadStatusActive,
		CreatedAt:      time.Now().UTC(),
	}
	if err := r.threads.CreateThread(createdThread); err != nil {
		return threads.Thread{}, err
	}
	mission.OwningThreadID = threadID
	if err := r.missions.UpdateMission(mission); err != nil {
		return threads.Thread{}, err
	}
	return r.threads.GetThread(threadID)
}

func plannedMissionKey(title string, missionType string) string {
	trimmedTitle := strings.TrimSpace(strings.ToLower(title))
	trimmedType := strings.TrimSpace(strings.ToLower(missionType))
	if trimmedTitle == "" {
		return ""
	}
	return trimmedType + "::" + trimmedTitle
}

func plannedMissionID(parentMissionID string, title string) string {
	return parentMissionID + "-" + slugifyMissionPart(title)
}

func plannedThreadID(missionID string) string {
	return "thread-" + missionID
}

func slugifyMissionPart(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "mission"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "mission"
	}
	return result
}

func emptyMissionField(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func defaultObjectOrRawJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}
