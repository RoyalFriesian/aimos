package ceo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/contextpacks"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

const reuseSearchLimit = 3

type roadmapPlanResponse struct {
	Message       string                   `json:"message"`
	ReuseDecision roadmapReuseDecision     `json:"reuseDecision"`
	Proposed      []roadmapMissionProposal `json:"proposedMissions"`
	NextActions   []string                 `json:"nextActions"`
}

type roadmapReuseDecision struct {
	Strategy  string `json:"strategy"`
	Rationale string `json:"rationale"`
}

type roadmapMissionProposal struct {
	Title          string   `json:"title"`
	Charter        string   `json:"charter"`
	Goal           string   `json:"goal"`
	Scope          string   `json:"scope"`
	MissionType    string   `json:"missionType"`
	AuthorityLevel string   `json:"authorityLevel"`
	ReuseRefs      []string `json:"reuseRefs"`
	Reasoning      string   `json:"reasoning"`
}

type missionDelegationSummary struct {
	MissionID            string
	ThreadID             string
	AgentID              string
	AgentRole            string
	SelectionSource      string
	StartupState         string
	DelegateStatus       string
	RequiredCapabilities []string
	MatchedCapabilities  []string
	MissingCapabilities  []string
	SelectionRationale   string
	ReportingToAgentID   string
	AuthorityScope       map[string]any
}

type reusableMissionSummary struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Goal         string   `json:"goal"`
	Scope        string   `json:"scope"`
	Score        float64  `json:"score"`
	MatchedTerms []string `json:"matchedTerms"`
}

type reusableThreadSummary struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Context      string   `json:"context"`
	Score        float64  `json:"score"`
	MatchedTerms []string `json:"matchedTerms"`
}

func (s *Service) planRoadmap(ctx context.Context, pack contextpacks.ContextPack, prompt string, model string) (map[string]any, string, error) {
	missionMatches, err := s.missionStore.SearchReusableMissions(reuseQuery(pack, prompt), reuseSearchLimit)
	if err != nil {
		return nil, "", logValidationError("failed to search reusable missions", err, "missionID", pack.Mission.ID)
	}
	threadMatches, err := s.threadStore.SearchReusableThreads(reuseQuery(pack, prompt), reuseSearchLimit)
	if err != nil {
		return nil, "", logValidationError("failed to search reusable threads", err, "missionID", pack.Mission.ID)
	}

	planned, rawPlan, err := s.generateRoadmapPlan(ctx, pack, prompt, model, missionMatches, threadMatches)
	if err != nil {
		return nil, "", err
	}
	if planned == nil {
		fallback := fallbackRoadmapPlan(pack, prompt, missionMatches, threadMatches)
		planned = &fallback
	}
	if strings.TrimSpace(planned.Message) == "" {
		fallback := fallbackRoadmapPlan(pack, prompt, missionMatches, threadMatches)
		planned.Message = fallback.Message
	}
	if len(planned.Proposed) == 0 {
		fallback := fallbackRoadmapPlan(pack, prompt, missionMatches, threadMatches)
		planned.Proposed = fallback.Proposed
		if len(planned.NextActions) == 0 {
			planned.NextActions = fallback.NextActions
		}
		if planned.ReuseDecision.Strategy == "" {
			planned.ReuseDecision = fallback.ReuseDecision
		}
	}

	persistedMissions, persistedThreads, err := s.persistRoadmapMissions(pack, planned.Proposed)
	if err != nil {
		return nil, "", logValidationError("failed to persist roadmap proposals", err, "missionID", pack.Mission.ID, "threadID", pack.Thread.ID)
	}
	handoffs, err := s.delegateRoadmapMissions(pack, planned.Proposed, persistedMissions)
	if err != nil {
		return nil, "", logValidationError("failed to delegate roadmap missions", err, "missionID", pack.Mission.ID, "threadID", pack.Thread.ID)
	}
	refreshedSummaries, refreshedRollups, err := s.refreshRoadmapMissionState(persistedMissions, persistedThreads)
	if err != nil {
		return nil, "", logValidationError("failed to refresh roadmap mission state", err, "missionID", pack.Mission.ID, "threadID", pack.Thread.ID)
	}

	payload := map[string]any{
		"message":                strings.TrimSpace(planned.Message),
		"mode":                   ModeRoadmap,
		"model":                  model,
		"reuseDecision":          normalizeReuseDecision(planned.ReuseDecision, len(missionMatches), len(threadMatches)),
		"reusableMissionMatches": summarizeMissionMatches(missionMatches),
		"reusableThreadMatches":  summarizeThreadMatches(threadMatches),
		"proposedMissions":       normalizeRoadmapProposals(planned.Proposed, persistedMissions, persistedThreads, handoffs),
		"persistedChildMissions": summarizePersistedChildMissions(persistedMissions, persistedThreads),
		"delegationHandoffs":     summarizeDelegationHandoffs(handoffs),
		"refreshedSummaries":     summarizeRefreshedSummaries(refreshedSummaries),
		"publishedRollups":       summarizePublishedRollups(refreshedRollups),
		"nextActions":            defaultStringSlice(planned.NextActions),
	}
	if strings.TrimSpace(rawPlan) != "" {
		payload["plannerRaw"] = unwrapJSONResponse(rawPlan)
	}

	return payload, strings.TrimSpace(planned.Message), nil
}

func (s *Service) refreshRoadmapMissionState(persistedMissions []missions.Mission, persistedThreads []threads.Thread) ([]missionstate.Summary, []missionstate.Rollup, error) {
	threadByMissionID := make(map[string]threads.Thread, len(persistedThreads))
	for _, thread := range persistedThreads {
		threadByMissionID[thread.MissionID] = thread
	}
	summaries := make([]missionstate.Summary, 0, len(persistedMissions))
	rollups := make([]missionstate.Rollup, 0, len(persistedMissions))
	for _, mission := range persistedMissions {
		thread, exists := threadByMissionID[mission.ID]
		if !exists {
			continue
		}
		summary, rollup, err := s.missionStateRuntime.RefreshMissionState(mission.ID, thread.ID)
		if err != nil {
			return nil, nil, err
		}
		summaries = append(summaries, summary)
		if rollup != nil {
			rollups = append(rollups, *rollup)
		}
	}
	return summaries, rollups, nil
}

func (s *Service) persistRoadmapMissions(pack contextpacks.ContextPack, proposals []roadmapMissionProposal) ([]missions.Mission, []threads.Thread, error) {
	planned := make([]missions.PlannedChildMissionInput, 0, len(proposals))
	for _, proposal := range proposals {
		if strings.TrimSpace(proposal.Title) == "" {
			continue
		}
		planned = append(planned, missions.PlannedChildMissionInput{
			Title:          strings.TrimSpace(proposal.Title),
			Charter:        strings.TrimSpace(proposal.Charter),
			Goal:           strings.TrimSpace(proposal.Goal),
			Scope:          strings.TrimSpace(proposal.Scope),
			MissionType:    emptyFallback(strings.TrimSpace(proposal.MissionType), "domain"),
			AuthorityLevel: emptyFallback(strings.TrimSpace(proposal.AuthorityLevel), "domain"),
			ReuseTrace:     buildReuseTrace(proposal.ReuseRefs, proposal.Reasoning),
			ThreadKind:     "strategy",
			ThreadSummary:  strings.TrimSpace(proposal.Goal),
			ThreadContext:  roadmapThreadContext(pack.Mission, proposal),
		})
	}
	return s.missionRuntime.PersistPlannedChildMissions(pack.Mission.ID, pack.Thread.ID, planned)
}

func (s *Service) delegateRoadmapMissions(pack contextpacks.ContextPack, proposals []roadmapMissionProposal, persistedMissions []missions.Mission) ([]missionDelegationSummary, error) {
	if len(persistedMissions) == 0 {
		return []missionDelegationSummary{}, nil
	}
	proposalByKey := make(map[string]roadmapMissionProposal, len(proposals))
	for _, proposal := range proposals {
		proposalByKey[roadmapProposalKey(proposal.Title, proposal.MissionType)] = proposal
	}
	handoffs := make([]missionDelegationSummary, 0, len(persistedMissions))
	for _, mission := range persistedMissions {
		proposal := proposalByKey[roadmapProposalKey(mission.Title, mission.MissionType)]
		requirement, selection, err := s.delegateSelector.SelectDelegate(mission, proposal)
		if err != nil {
			return nil, err
		}
		authorityScope := buildDelegationAuthorityScope(pack.Mission, mission, proposal, requirement, selection)
		delegatedMission, delegatedThread, assignment, err := s.missionRuntime.DelegateMission(missions.DelegationHandoffInput{
			MissionID:          mission.ID,
			AgentID:            selection.AgentID,
			AgentRole:          selection.AgentRole,
			AuthorityScope:     authorityScope,
			ReportingToAgentID: pack.Mission.OwnerAgentID,
		})
		if err != nil {
			return nil, err
		}
		handoffs = append(handoffs, missionDelegationSummary{
			MissionID:            delegatedMission.ID,
			ThreadID:             delegatedThread.ID,
			AgentID:              assignment.AgentID,
			AgentRole:            assignment.AgentRole,
			SelectionSource:      selection.SelectionSource,
			StartupState:         selection.StartupState,
			DelegateStatus:       selection.DelegateStatus,
			RequiredCapabilities: append([]string(nil), selection.RequiredCapabilities...),
			MatchedCapabilities:  append([]string(nil), selection.MatchedCapabilities...),
			MissingCapabilities:  append([]string(nil), selection.MissingCapabilities...),
			SelectionRationale:   selection.SelectionRationale,
			ReportingToAgentID:   assignment.ReportingToAgentID,
			AuthorityScope:       decodeObjectMap(assignment.AuthorityScope),
		})
	}
	return handoffs, nil
}

func (s *Service) generateRoadmapPlan(ctx context.Context, pack contextpacks.ContextPack, prompt string, model string, missionMatches []missions.ReusableMissionMatch, threadMatches []threads.ReusableThreadMatch) (*roadmapPlanResponse, string, error) {
	roadmapPrompt, err := loadSystemPrompt(ModeRoadmap)
	if err != nil {
		return nil, "", err
	}
	plannerPrompt := roadmapPrompt + " Respond as JSON with keys: message, reuseDecision, proposedMissions, nextActions. reuseDecision must include strategy and rationale. proposedMissions must be an array of 2 to 5 mission proposal objects with keys: title, charter, goal, scope, missionType, authorityLevel, reuseRefs, reasoning. Prefer adapting reusable work when the matches are strong, and only choose build_net_new when reuse is weak."

	userPrompt := formatRoadmapPlannerInput(pack, prompt, missionMatches, threadMatches)
	rawPlan, err := s.llm.Generate(ctx, model, plannerPrompt, userPrompt)
	if err != nil {
		return nil, "", err
	}

	var plan roadmapPlanResponse
	if err := json.Unmarshal([]byte(unwrapJSONResponse(rawPlan)), &plan); err != nil {
		return nil, rawPlan, nil
	}
	return &plan, rawPlan, nil
}

func formatRoadmapPlannerInput(pack contextpacks.ContextPack, prompt string, missionMatches []missions.ReusableMissionMatch, threadMatches []threads.ReusableThreadMatch) string {
	var builder strings.Builder
	builder.WriteString("Current mission for decomposition:\n")
	builder.WriteString(fmt.Sprintf("- ID: %s\n", pack.Mission.ID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", pack.Mission.Title))
	builder.WriteString(fmt.Sprintf("- Goal: %s\n", pack.Mission.Goal))
	builder.WriteString(fmt.Sprintf("- Scope: %s\n", pack.Mission.Scope))
	builder.WriteString(fmt.Sprintf("- Authority: %s\n", pack.Mission.AuthorityLevel))
	if pack.LatestSummary != nil {
		builder.WriteString(fmt.Sprintf("- Latest summary: %s\n", pack.LatestSummary.SummaryText))
	}
	builder.WriteString("Client roadmap request:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\nReusable mission matches:\n")
	if len(missionMatches) == 0 {
		builder.WriteString("- none\n")
	} else {
		for _, match := range missionMatches {
			builder.WriteString(fmt.Sprintf("- %s | title=%s | goal=%s | scope=%s | score=%.2f | matchedTerms=%s\n",
				match.Mission.ID,
				match.Mission.Title,
				match.Mission.Goal,
				match.Mission.Scope,
				match.Score,
				strings.Join(match.MatchedTerms, ", "),
			))
		}
	}
	builder.WriteString("Reusable thread matches:\n")
	if len(threadMatches) == 0 {
		builder.WriteString("- none\n")
	} else {
		for _, match := range threadMatches {
			builder.WriteString(fmt.Sprintf("- %s | title=%s | summary=%s | score=%.2f | matchedTerms=%s\n",
				match.Thread.ID,
				match.Thread.Title,
				match.Thread.Summary,
				match.Score,
				strings.Join(match.MatchedTerms, ", "),
			))
		}
	}
	return builder.String()
}

func fallbackRoadmapPlan(pack contextpacks.ContextPack, prompt string, missionMatches []missions.ReusableMissionMatch, threadMatches []threads.ReusableThreadMatch) roadmapPlanResponse {
	strategy := "build_net_new"
	rationale := "No sufficiently similar retained work was available, so the roadmap should start from net-new mission decomposition."
	if len(missionMatches) > 0 || len(threadMatches) > 0 {
		strategy = "adapt_existing"
		rationale = "Reusable retained work exists, so the roadmap should adapt those patterns before creating net-new mission structure."
	}

	components := deriveMissionComponents(prompt, pack.Mission)
	proposals := make([]roadmapMissionProposal, 0, len(components))
	for index, component := range components {
		proposal := roadmapMissionProposal{
			Title:          titleCase(component),
			Charter:        fmt.Sprintf("Own the %s workstream for %s.", component, pack.Mission.Title),
			Goal:           fmt.Sprintf("Define and deliver the %s capability for %s.", component, pack.Mission.Title),
			Scope:          fmt.Sprintf("Execution planning and delivery for %s within %s.", component, pack.Mission.Scope),
			MissionType:    "domain",
			AuthorityLevel: "domain",
			Reasoning:      fmt.Sprintf("Breaks the parent mission into an execution-owned slice focused on %s.", component),
		}
		if index == 0 {
			proposal.ReuseRefs = collectFallbackReuseRefs(missionMatches, threadMatches)
		}
		proposals = append(proposals, proposal)
	}

	message := fmt.Sprintf("I would decompose %s into %d mission-owned workstreams, using retained prior work first where it is relevant.", pack.Mission.Title, len(proposals))
	return roadmapPlanResponse{
		Message: message,
		ReuseDecision: roadmapReuseDecision{
			Strategy:  strategy,
			Rationale: rationale,
		},
		Proposed: proposals,
		NextActions: []string{
			"Review the proposed mission boundaries",
			"Confirm which reuse candidates should be adapted",
			"Create child missions from the approved roadmap structure",
		},
	}
}

func summarizeMissionMatches(matches []missions.ReusableMissionMatch) []map[string]any {
	results := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		results = append(results, map[string]any{
			"id":           match.Mission.ID,
			"title":        match.Mission.Title,
			"goal":         match.Mission.Goal,
			"scope":        match.Mission.Scope,
			"score":        match.Score,
			"matchedTerms": defaultStringSlice(match.MatchedTerms),
		})
	}
	return results
}

func summarizeThreadMatches(matches []threads.ReusableThreadMatch) []map[string]any {
	results := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		results = append(results, map[string]any{
			"id":           match.Thread.ID,
			"title":        match.Thread.Title,
			"summary":      match.Thread.Summary,
			"context":      match.Thread.Context,
			"score":        match.Score,
			"matchedTerms": defaultStringSlice(match.MatchedTerms),
		})
	}
	return results
}

func normalizeReuseDecision(decision roadmapReuseDecision, missionMatchCount int, threadMatchCount int) map[string]any {
	strategy := strings.TrimSpace(decision.Strategy)
	if strategy == "" {
		if missionMatchCount > 0 || threadMatchCount > 0 {
			strategy = "adapt_existing"
		} else {
			strategy = "build_net_new"
		}
	}
	rationale := strings.TrimSpace(decision.Rationale)
	if rationale == "" {
		if strategy == "adapt_existing" {
			rationale = "Retained reusable work exists and should be adapted before creating net-new structure."
		} else {
			rationale = "No strong retained match exists, so the roadmap should start net new."
		}
	}
	return map[string]any{
		"strategy":  strategy,
		"rationale": rationale,
	}
}

func normalizeRoadmapProposals(proposals []roadmapMissionProposal, persistedMissions []missions.Mission, persistedThreads []threads.Thread, handoffs []missionDelegationSummary) []map[string]any {
	persistedByKey := make(map[string]missions.Mission, len(persistedMissions))
	for _, mission := range persistedMissions {
		persistedByKey[roadmapProposalKey(mission.Title, mission.MissionType)] = mission
	}
	threadByMissionID := make(map[string]threads.Thread, len(persistedThreads))
	for _, thread := range persistedThreads {
		threadByMissionID[thread.MissionID] = thread
	}
	handoffByMissionID := make(map[string]missionDelegationSummary, len(handoffs))
	for _, handoff := range handoffs {
		handoffByMissionID[handoff.MissionID] = handoff
	}

	results := make([]map[string]any, 0, len(proposals))
	for _, proposal := range proposals {
		title := strings.TrimSpace(proposal.Title)
		if title == "" {
			continue
		}
		missionType := emptyFallback(strings.TrimSpace(proposal.MissionType), "domain")
		entry := map[string]any{
			"title":          title,
			"charter":        strings.TrimSpace(proposal.Charter),
			"goal":           strings.TrimSpace(proposal.Goal),
			"scope":          strings.TrimSpace(proposal.Scope),
			"missionType":    missionType,
			"authorityLevel": emptyFallback(strings.TrimSpace(proposal.AuthorityLevel), "domain"),
			"reuseRefs":      defaultStringSlice(proposal.ReuseRefs),
			"reasoning":      strings.TrimSpace(proposal.Reasoning),
		}
		if mission, ok := persistedByKey[roadmapProposalKey(title, missionType)]; ok {
			entry["missionId"] = mission.ID
			entry["status"] = mission.Status
			entry["reuseTrace"] = decodeReuseTrace(mission.ReuseTrace)
			if thread, exists := threadByMissionID[mission.ID]; exists {
				entry["threadId"] = thread.ID
			}
			if handoff, exists := handoffByMissionID[mission.ID]; exists {
				entry["delegatedToAgentId"] = handoff.AgentID
				entry["delegatedToRole"] = handoff.AgentRole
				entry["selectionSource"] = handoff.SelectionSource
				entry["startupState"] = handoff.StartupState
				entry["delegateStatus"] = handoff.DelegateStatus
				entry["requiredCapabilities"] = defaultStringSlice(handoff.RequiredCapabilities)
				entry["matchedCapabilities"] = defaultStringSlice(handoff.MatchedCapabilities)
				entry["missingCapabilities"] = defaultStringSlice(handoff.MissingCapabilities)
				entry["selectionRationale"] = handoff.SelectionRationale
				entry["reportingToAgentId"] = handoff.ReportingToAgentID
				entry["authorityScope"] = handoff.AuthorityScope
			}
		}
		results = append(results, entry)
	}
	return results
}

func summarizePersistedChildMissions(persistedMissions []missions.Mission, persistedThreads []threads.Thread) []map[string]any {
	threadByMissionID := make(map[string]threads.Thread, len(persistedThreads))
	for _, thread := range persistedThreads {
		threadByMissionID[thread.MissionID] = thread
	}
	results := make([]map[string]any, 0, len(persistedMissions))
	for _, mission := range persistedMissions {
		entry := map[string]any{
			"missionId":      mission.ID,
			"title":          mission.Title,
			"missionType":    mission.MissionType,
			"authorityLevel": mission.AuthorityLevel,
			"status":         mission.Status,
			"reuseTrace":     decodeReuseTrace(mission.ReuseTrace),
		}
		if thread, exists := threadByMissionID[mission.ID]; exists {
			entry["threadId"] = thread.ID
		}
		results = append(results, entry)
	}
	return results
}

func summarizeDelegationHandoffs(handoffs []missionDelegationSummary) []map[string]any {
	results := make([]map[string]any, 0, len(handoffs))
	for _, handoff := range handoffs {
		results = append(results, map[string]any{
			"missionId":            handoff.MissionID,
			"threadId":             handoff.ThreadID,
			"agentId":              handoff.AgentID,
			"agentRole":            handoff.AgentRole,
			"selectionSource":      handoff.SelectionSource,
			"startupState":         handoff.StartupState,
			"delegateStatus":       handoff.DelegateStatus,
			"requiredCapabilities": defaultStringSlice(handoff.RequiredCapabilities),
			"matchedCapabilities":  defaultStringSlice(handoff.MatchedCapabilities),
			"missingCapabilities":  defaultStringSlice(handoff.MissingCapabilities),
			"selectionRationale":   handoff.SelectionRationale,
			"reportingToAgentId":   handoff.ReportingToAgentID,
			"authorityScope":       handoff.AuthorityScope,
		})
	}
	return results
}

func summarizeRefreshedSummaries(summaries []missionstate.Summary) []map[string]any {
	results := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		results = append(results, map[string]any{
			"summaryId":     summary.ID,
			"missionId":     summary.MissionID,
			"threadId":      summary.ThreadID,
			"summaryText":   summary.SummaryText,
			"coverageStart": summary.CoverageStartRef,
			"coverageEnd":   summary.CoverageEndRef,
		})
	}
	return results
}

func summarizePublishedRollups(rollups []missionstate.Rollup) []map[string]any {
	results := make([]map[string]any, 0, len(rollups))
	for _, rollup := range rollups {
		results = append(results, map[string]any{
			"rollupId":             rollup.ID,
			"parentMissionId":      rollup.ParentMissionID,
			"childMissionId":       rollup.ChildMissionID,
			"status":               rollup.Status,
			"health":               rollup.Health,
			"latestSummary":        rollup.LatestSummary,
			"currentBlocker":       rollup.CurrentBlocker,
			"progressPercent":      rollup.ProgressPercent,
			"executionSummary":     decodeObjectMap(rollup.ExecutionSummary),
			"nextExpectedUpdateAt": rollup.NextExpectedUpdateAt,
		})
	}
	return results
}

func buildReuseTrace(reuseRefs []string, reasoning string) json.RawMessage {
	refs := make([]missions.ReuseTraceRef, 0, len(reuseRefs))
	for _, ref := range reuseRefs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		refs = append(refs, missions.ReuseTraceRef{
			SourceType: inferReuseSourceType(trimmed),
			SourceID:   trimmed,
			Reason:     strings.TrimSpace(reasoning),
		})
	}
	encoded, err := json.Marshal(refs)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return encoded
}

func inferReuseSourceType(ref string) string {
	if strings.HasPrefix(ref, "thread-") {
		return "thread"
	}
	if strings.HasPrefix(ref, "mission-") {
		return "mission"
	}
	return "artifact"
}

func decodeReuseTrace(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return []map[string]any{}
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []map[string]any{}
	}
	return decoded
}

func decodeObjectMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func buildDelegationAuthorityScope(parent missions.Mission, child missions.Mission, proposal roadmapMissionProposal, requirement delegateRequirement, selection delegateSelection) json.RawMessage {
	scope := map[string]any{
		"handoffType":          delegationKindForMission(child),
		"parentMissionId":      parent.ID,
		"parentMissionTitle":   parent.Title,
		"missionType":          child.MissionType,
		"authorityLevel":       child.AuthorityLevel,
		"goal":                 child.Goal,
		"scope":                child.Scope,
		"selectionSource":      selection.SelectionSource,
		"startupState":         selection.StartupState,
		"delegateStatus":       selection.DelegateStatus,
		"requiredCapabilities": defaultStringSlice(requirement.RequiredCapabilities),
		"matchedCapabilities":  defaultStringSlice(selection.MatchedCapabilities),
		"missingCapabilities":  defaultStringSlice(selection.MissingCapabilities),
		"selectionRationale":   selection.SelectionRationale,
	}
	if reasoning := strings.TrimSpace(proposal.Reasoning); reasoning != "" {
		scope["handoffReason"] = reasoning
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func delegationKindForMission(mission missions.Mission) string {
	authority := strings.TrimSpace(strings.ToLower(mission.AuthorityLevel))
	if authority == "global" || authority == "program" || authority == "domain" {
		return "sub_ceo"
	}
	return "execution_owner"
}

func missionDelegateSlug(title string) string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
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

func roadmapThreadContext(parent missions.Mission, proposal roadmapMissionProposal) string {
	if strings.TrimSpace(proposal.Reasoning) != "" {
		return fmt.Sprintf("Created from roadmap decomposition under parent mission %s. %s", parent.Title, strings.TrimSpace(proposal.Reasoning))
	}
	return fmt.Sprintf("Created from roadmap decomposition under parent mission %s.", parent.Title)
}

func roadmapProposalKey(title string, missionType string) string {
	return strings.ToLower(strings.TrimSpace(missionType)) + "::" + strings.ToLower(strings.TrimSpace(title))
}

func reuseQuery(pack contextpacks.ContextPack, prompt string) string {
	parts := []string{prompt, pack.Mission.Title, pack.Mission.Goal, pack.Mission.Scope}
	if pack.LatestSummary != nil {
		parts = append(parts, pack.LatestSummary.SummaryText)
	}
	return strings.Join(parts, " ")
}

func deriveMissionComponents(prompt string, mission missions.Mission) []string {
	source := strings.TrimSpace(prompt)
	if source == "" {
		source = strings.TrimSpace(mission.Goal)
	}
	if source == "" {
		source = strings.TrimSpace(mission.Scope)
	}

	replacer := strings.NewReplacer("/", ",", " and ", ",", "\n", ",", ";", ",")
	segments := strings.Split(replacer.Replace(strings.ToLower(source)), ",")
	components := make([]string, 0, len(segments))
	seen := map[string]struct{}{}
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" || len(trimmed) < 4 {
			continue
		}
		if strings.HasPrefix(trimmed, "build ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "build "))
		}
		if strings.HasPrefix(trimmed, "create ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "create "))
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		components = append(components, trimmed)
		if len(components) == 3 {
			break
		}
	}
	if len(components) == 0 {
		components = []string{"platform foundation", "execution coordination"}
	}
	return components
}

func collectFallbackReuseRefs(missionMatches []missions.ReusableMissionMatch, threadMatches []threads.ReusableThreadMatch) []string {
	refs := make([]string, 0, 2)
	if len(missionMatches) > 0 {
		refs = append(refs, missionMatches[0].Mission.ID)
	}
	if len(threadMatches) > 0 {
		refs = append(refs, threadMatches[0].Thread.ID)
	}
	return refs
}

func titleCase(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
