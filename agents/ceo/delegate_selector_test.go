package ceo

import (
	"testing"

	"github.com/Sarnga/agent-platform/pkg/agents"
	"github.com/Sarnga/agent-platform/pkg/missions"
)

func TestDelegateSelectorChoosesDirectoryMatchByCapabilities(t *testing.T) {
	selector, err := newDelegateSelector()
	if err != nil {
		t.Fatalf("newDelegateSelector returned error: %v", err)
	}

	requirement, selection, err := selector.SelectDelegate(missions.Mission{
		ID:             "mission-network",
		MissionType:    "domain",
		Title:          "Networking foundation",
		Goal:           "Deliver routing and connectivity foundations",
		Scope:          "Networking and connectivity domain",
		AuthorityLevel: "domain",
	}, roadmapMissionProposal{
		Title:       "Networking foundation",
		Goal:        "Adapt networking foundations",
		Scope:       "Connectivity and routing",
		Reasoning:   "Leverages retained networking work",
		MissionType: "domain",
	})
	if err != nil {
		t.Fatalf("SelectDelegate returned error: %v", err)
	}
	if selection.AgentID != "sub-ceo-networking" {
		t.Fatalf("expected sub-ceo-networking, got %q", selection.AgentID)
	}
	if selection.AgentRole != "sub_ceo" {
		t.Fatalf("expected sub_ceo role, got %q", selection.AgentRole)
	}
	if selection.SelectionSource != "directory" {
		t.Fatalf("expected directory selection source, got %q", selection.SelectionSource)
	}
	if selection.StartupState != "claimed" {
		t.Fatalf("expected claimed startup state, got %q", selection.StartupState)
	}
	if selection.DelegateStatus != "busy" {
		t.Fatalf("expected busy delegate status after claim, got %q", selection.DelegateStatus)
	}
	if len(requirement.RequiredCapabilities) < 4 {
		t.Fatalf("expected multiple required capabilities, got %#v", requirement.RequiredCapabilities)
	}
	if len(selection.MissingCapabilities) != 0 {
		t.Fatalf("expected no missing capabilities, got %#v", selection.MissingCapabilities)
	}
	profile, err := selector.directory.GetProfile("sub-ceo-networking")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.Status != "busy" {
		t.Fatalf("expected selected profile status busy, got %q", profile.Status)
	}
}

func TestDelegateSelectorFallsBackWhenDirectoryCannotSatisfyRequirement(t *testing.T) {
	selector := &delegateSelector{directory: emptyDelegateDirectory{}}

	_, selection, err := selector.SelectDelegate(missions.Mission{
		ID:             "mission-identity",
		MissionType:    "domain",
		Title:          "Identity plane",
		Goal:           "Ship identity controls",
		Scope:          "Identity and access domain",
		AuthorityLevel: "domain",
	}, roadmapMissionProposal{Title: "Identity plane", MissionType: "domain"})
	if err != nil {
		t.Fatalf("SelectDelegate returned error: %v", err)
	}
	if selection.SelectionSource != "fallback" {
		t.Fatalf("expected fallback selection source, got %q", selection.SelectionSource)
	}
	if selection.StartupState != "placeholder" {
		t.Fatalf("expected placeholder startup state, got %q", selection.StartupState)
	}
	if selection.DelegateStatus != "unregistered" {
		t.Fatalf("expected unregistered delegate status, got %q", selection.DelegateStatus)
	}
	if selection.AgentID != "sub-ceo-identity-plane" {
		t.Fatalf("expected fallback delegate id, got %q", selection.AgentID)
	}
	if len(selection.MissingCapabilities) == 0 {
		t.Fatalf("expected fallback to report missing capabilities, got %#v", selection.MissingCapabilities)
	}
}

type emptyDelegateDirectory struct{}

func (emptyDelegateDirectory) Register(profile agents.Profile) error { return nil }

func (emptyDelegateDirectory) FindByCapabilities(role string, capabilities []string, limit int) ([]agents.CapabilityMatch, error) {
	return []agents.CapabilityMatch{}, nil
}

func (emptyDelegateDirectory) ClaimByCapabilities(role string, capabilities []string) (agents.CapabilityMatch, error) {
	return agents.CapabilityMatch{}, agents.ErrAgentNotFound
}

func (emptyDelegateDirectory) GetProfile(agentID string) (agents.Profile, error) {
	return agents.Profile{}, agents.ErrAgentNotFound
}

func (emptyDelegateDirectory) UpdateStatus(agentID string, status string) error {
	return agents.ErrAgentNotFound
}
