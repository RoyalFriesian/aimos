package ceo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/agents"
	"github.com/Sarnga/agent-platform/pkg/microai"
	"github.com/Sarnga/agent-platform/pkg/missions"
)

const delegateSearchLimit = 3

type delegateRequirement struct {
	Role                 string
	RequiredCapabilities []string
	MissionType          string
	AuthorityLevel       string
	Topics               []string
}

type delegateSelection struct {
	AgentID              string
	AgentRole            string
	SelectionSource      string
	StartupState         string
	DelegateStatus       string
	RequiredCapabilities []string
	MatchedCapabilities  []string
	MissingCapabilities  []string
	SelectionRationale   string
}

type delegateSelector struct {
	directory agents.Directory
}

func newDelegateSelector() (*delegateSelector, error) {
	directory := agents.NewMemoryDirectory()
	for _, profile := range defaultDelegateProfiles() {
		if err := directory.Register(profile); err != nil {
			return nil, err
		}
	}
	return &delegateSelector{directory: directory}, nil
}

func (s *delegateSelector) SelectDelegate(ctx context.Context, reasoner microai.Interface, mission missions.Mission, proposal roadmapMissionProposal) (delegateRequirement, delegateSelection, error) {
	requirement := buildDelegateRequirement(ctx, reasoner, mission, proposal)
	claimed, err := s.directory.ClaimByCapabilities(requirement.Role, requirement.RequiredCapabilities)
	if err == nil {
		return requirement, delegateSelection{
			AgentID:              claimed.Profile.ID,
			AgentRole:            claimed.Profile.Role,
			SelectionSource:      "directory",
			StartupState:         "claimed",
			DelegateStatus:       claimed.Profile.Status,
			RequiredCapabilities: append([]string(nil), requirement.RequiredCapabilities...),
			MatchedCapabilities:  append([]string(nil), claimed.MatchedCapabilities...),
			MissingCapabilities:  append([]string(nil), claimed.MissingCapabilities...),
			SelectionRationale:   selectionRationale(requirement, claimed),
		}, nil
	}
	if !errors.Is(err, agents.ErrAgentNotFound) {
		return delegateRequirement{}, delegateSelection{}, err
	}
	fallbackAgentID, fallbackRole := fallbackDelegateForRequirement(mission, requirement)
	return requirement, delegateSelection{
		AgentID:              fallbackAgentID,
		AgentRole:            fallbackRole,
		SelectionSource:      "fallback",
		StartupState:         "placeholder",
		DelegateStatus:       "unregistered",
		RequiredCapabilities: append([]string(nil), requirement.RequiredCapabilities...),
		MissingCapabilities:  append([]string(nil), requirement.RequiredCapabilities...),
		SelectionRationale:   "No directory-registered delegate satisfied the mission requirements, so a placeholder delegate was generated.",
	}, nil
}

func buildDelegateRequirement(ctx context.Context, reasoner microai.Interface, mission missions.Mission, proposal roadmapMissionProposal) delegateRequirement {
	role := delegationKindForMission(mission)
	topics := inferMissionTopics(ctx, reasoner, mission, proposal)
	capabilities := []string{
		"handoff:" + role,
		"mission:" + strings.TrimSpace(strings.ToLower(mission.MissionType)),
		"authority:" + strings.TrimSpace(strings.ToLower(mission.AuthorityLevel)),
	}
	for _, topic := range topics {
		capabilities = append(capabilities, "topic:"+topic)
	}
	return delegateRequirement{
		Role:                 role,
		RequiredCapabilities: normalizeRequiredCapabilities(capabilities),
		MissionType:          mission.MissionType,
		AuthorityLevel:       mission.AuthorityLevel,
		Topics:               topics,
	}
}

func normalizeRequiredCapabilities(capabilities []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		trimmed := strings.TrimSpace(strings.ToLower(capability))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func defaultDelegateProfiles() []agents.Profile {
	return []agents.Profile{
		{ID: "sub-ceo-platform", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "authority:global", "topic:platform", "topic:orchestration"}},
		{ID: "sub-ceo-networking", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:networking", "topic:connectivity", "topic:security"}},
		{ID: "sub-ceo-compute", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:compute", "topic:scheduling", "topic:runtime"}},
		{ID: "sub-ceo-storage", Role: "sub_ceo", Status: "active", Capabilities: []string{"handoff:sub_ceo", "mission:domain", "authority:domain", "topic:storage", "topic:data"}},
		{ID: "execution-owner-platform", Role: "execution_owner", Status: "active", Capabilities: []string{"handoff:execution_owner", "mission:domain", "authority:execution", "topic:platform", "topic:execution"}},
		{ID: "execution-owner-networking", Role: "execution_owner", Status: "active", Capabilities: []string{"handoff:execution_owner", "mission:domain", "authority:execution", "topic:networking", "topic:connectivity"}},
		{ID: "execution-owner-compute", Role: "execution_owner", Status: "active", Capabilities: []string{"handoff:execution_owner", "mission:domain", "authority:execution", "topic:compute", "topic:runtime"}},
	}
}

func fallbackDelegateForRequirement(mission missions.Mission, requirement delegateRequirement) (string, string) {
	slug := missionDelegateSlug(mission.Title)
	if requirement.Role == "sub_ceo" {
		return "sub-ceo-" + slug, "sub_ceo"
	}
	return "execution-owner-" + slug, "execution_owner"
}

func selectionRationale(requirement delegateRequirement, match agents.CapabilityMatch) string {
	if len(match.MissingCapabilities) == 0 {
		return fmt.Sprintf("Selected directory delegate %s because it satisfies all required capabilities for %s ownership.", match.Profile.ID, requirement.Role)
	}
	return fmt.Sprintf("Selected directory delegate %s because it is the strongest available capability match for %s ownership.", match.Profile.ID, requirement.Role)
}

func inferMissionTopics(ctx context.Context, reasoner microai.Interface, mission missions.Mission, proposal roadmapMissionProposal) []string {
	text := strings.Join([]string{mission.Title, mission.Goal, mission.Scope, proposal.Title, proposal.Goal, proposal.Scope, proposal.Reasoning}, " ")
	if reasoner != nil {
		task := "Extract the most relevant topic tags for this mission from this list: networking, connectivity, compute, runtime, scheduling, storage, data, identity, security, orchestration, platform, execution. Return ONLY a comma-separated list of matching topics. If none match, return platform."
		result, err := reasoner.Reason(ctx, task, text)
		if err == nil {
			result = strings.TrimSpace(result)
			if result != "" {
				parts := strings.Split(result, ",")
				topics := make([]string, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(strings.ToLower(p))
					if p != "" {
						topics = append(topics, p)
					}
				}
				if len(topics) > 0 {
					return topics
				}
			}
		}
	}
	// Keyword fallback when no reasoner is available.
	lower := strings.ToLower(text)
	knownTopics := []string{"networking", "connectivity", "compute", "runtime", "scheduling", "storage", "data", "identity", "security", "orchestration", "platform", "execution"}
	var found []string
	for _, t := range knownTopics {
		if strings.Contains(lower, t) {
			found = append(found, t)
		}
	}
	if len(found) > 0 {
		return found
	}
	return []string{"platform"}
}
