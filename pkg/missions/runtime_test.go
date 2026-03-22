package missions

import (
	"encoding/json"
	"testing"

	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestRuntimeCreateProgramWithRootMission(t *testing.T) {
	missionStore := NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	runtime, err := NewRuntime(missionStore, threadStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	program, mission, thread, err := runtime.CreateProgramWithRootMission(RootMissionInput{
		ProgramID:      "program-1",
		ClientID:       "client-1",
		ProgramTitle:   "Google Cloud Replica",
		MissionID:      "mission-root",
		ThreadID:       "thread-root",
		OwnerAgentID:   "ceo-root",
		OwnerRole:      "CEO",
		MissionType:    "root",
		ThreadKind:     "strategy",
		MissionTitle:   "Build full cloud platform",
		Charter:        "Own the full product vision and delegation",
		Goal:           "Deliver a Google Cloud class platform",
		Scope:          "Whole company mission",
		AuthorityLevel: "global",
		ThreadTitle:    "Root CEO strategy thread",
		ThreadSummary:  "Top-level strategy thread for the whole cloud platform program.",
		ThreadContext:  "Created so the top CEO can own product vision, delegation, and high-level program decisions without mixing them with child team execution threads.",
	})
	if err != nil {
		t.Fatalf("CreateProgramWithRootMission returned error: %v", err)
	}
	if program.RootMissionID != mission.ID {
		t.Fatalf("expected program root mission %q, got %q", mission.ID, program.RootMissionID)
	}
	if mission.OwningThreadID != thread.ID {
		t.Fatalf("expected mission owning thread %q, got %q", thread.ID, mission.OwningThreadID)
	}
	if thread.MissionID != mission.ID {
		t.Fatalf("expected thread mission %q, got %q", mission.ID, thread.MissionID)
	}
	if thread.Context == "" {
		t.Fatal("expected thread context to be populated")
	}
}

func TestRuntimeCreateChildMission(t *testing.T) {
	missionStore := NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	runtime, err := NewRuntime(missionStore, threadStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	_, parentMission, parentThread, err := runtime.CreateProgramWithRootMission(RootMissionInput{
		ProgramID:      "program-1",
		ClientID:       "client-1",
		ProgramTitle:   "Google Cloud Replica",
		MissionID:      "mission-root",
		ThreadID:       "thread-root",
		OwnerAgentID:   "ceo-root",
		OwnerRole:      "CEO",
		MissionType:    "root",
		ThreadKind:     "strategy",
		MissionTitle:   "Build full cloud platform",
		Charter:        "Own the full product vision and delegation",
		Goal:           "Deliver a Google Cloud class platform",
		Scope:          "Whole company mission",
		AuthorityLevel: "global",
		ThreadTitle:    "Root CEO strategy thread",
		ThreadSummary:  "Top-level strategy thread for the whole cloud platform program.",
		ThreadContext:  "Created so the top CEO can own product vision, delegation, and high-level program decisions without mixing them with child team execution threads.",
	})
	if err != nil {
		t.Fatalf("CreateProgramWithRootMission returned error: %v", err)
	}

	childMission, childThread, err := runtime.CreateChildMission(ChildMissionInput{
		MissionID:       "mission-networking",
		ParentMissionID: parentMission.ID,
		ThreadID:        "thread-networking",
		OwnerAgentID:    "ceo-network",
		OwnerRole:       "CEO",
		MissionType:     "domain",
		ThreadKind:      "strategy",
		MissionTitle:    "Build networking domain",
		Charter:         "Own VPC, LB, DNS, firewall, connectivity",
		Goal:            "Deliver networking platform",
		Scope:           "Networking mission only",
		AuthorityLevel:  "domain",
		ThreadTitle:     "Networking CEO strategy thread",
		ThreadSummary:   "Networking domain strategy and execution-supervision thread.",
		ThreadContext:   "Created for the networking sub-CEO to own VPC, DNS, firewall, load balancing, and connectivity decisions while reporting rollups to the root CEO.",
		ParentThreadID:  parentThread.ID,
	})
	if err != nil {
		t.Fatalf("CreateChildMission returned error: %v", err)
	}
	if childMission.ParentMissionID != parentMission.ID {
		t.Fatalf("expected parent mission %q, got %q", parentMission.ID, childMission.ParentMissionID)
	}
	if childMission.RootMissionID != parentMission.RootMissionID {
		t.Fatalf("expected root mission %q, got %q", parentMission.RootMissionID, childMission.RootMissionID)
	}
	if childThread.ParentThreadID != parentThread.ID {
		t.Fatalf("expected parent thread %q, got %q", parentThread.ID, childThread.ParentThreadID)
	}
	if childThread.MissionID != childMission.ID {
		t.Fatalf("expected child thread mission %q, got %q", childMission.ID, childThread.MissionID)
	}
	if childThread.Summary == "" || childThread.Context == "" {
		t.Fatalf("expected child thread summary and context to be populated, got summary=%q context=%q", childThread.Summary, childThread.Context)
	}

	children, err := missionStore.ListChildMissions(parentMission.ID)
	if err != nil {
		t.Fatalf("ListChildMissions returned error: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("expected 1 child mission, got %d", len(children))
	}

	threadsForMission, err := threadStore.ListByMission(childMission.ID)
	if err != nil {
		t.Fatalf("ListByMission returned error: %v", err)
	}
	if len(threadsForMission) != 1 {
		t.Fatalf("expected 1 thread for mission, got %d", len(threadsForMission))
	}
}

func TestRuntimePersistPlannedChildMissions(t *testing.T) {
	missionStore := NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	runtime, err := NewRuntime(missionStore, threadStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	_, parentMission, parentThread, err := runtime.CreateProgramWithRootMission(RootMissionInput{
		ProgramID:      "program-1",
		ClientID:       "client-1",
		ProgramTitle:   "Google Cloud Replica",
		MissionID:      "mission-root",
		ThreadID:       "thread-root",
		OwnerAgentID:   "ceo-root",
		OwnerRole:      "CEO",
		MissionType:    "root",
		ThreadKind:     "strategy",
		MissionTitle:   "Build full cloud platform",
		Charter:        "Own the full product vision and delegation",
		Goal:           "Deliver a Google Cloud class platform",
		Scope:          "Whole company mission",
		AuthorityLevel: "global",
		ThreadTitle:    "Root CEO strategy thread",
		ThreadSummary:  "Top-level strategy thread for the whole cloud platform program.",
		ThreadContext:  "Created so the top CEO can own product vision, delegation, and high-level program decisions without mixing them with child team execution threads.",
	})
	if err != nil {
		t.Fatalf("CreateProgramWithRootMission returned error: %v", err)
	}

	plans := []PlannedChildMissionInput{
		{
			Title:          "Networking foundation",
			Charter:        "Own the networking base layer.",
			Goal:           "Adapt retained networking work.",
			Scope:          "Routing and firewall.",
			MissionType:    "domain",
			AuthorityLevel: "domain",
			ReuseTrace:     json.RawMessage(`[{"sourceType":"mission","sourceId":"mission-network-legacy","reason":"Adapt retained networking work"}]`),
			ThreadKind:     "strategy",
			ThreadSummary:  "Networking foundation thread",
			ThreadContext:  "Owns networking mission planning.",
		},
		{
			Title:          "Compute substrate",
			Charter:        "Own the compute base layer.",
			Goal:           "Define VM and scheduling primitives.",
			Scope:          "Compute APIs and lifecycle.",
			MissionType:    "domain",
			AuthorityLevel: "domain",
			ThreadKind:     "strategy",
			ThreadSummary:  "Compute substrate thread",
			ThreadContext:  "Owns compute mission planning.",
		},
	}

	persistedMissions, persistedThreads, err := runtime.PersistPlannedChildMissions(parentMission.ID, parentThread.ID, plans)
	if err != nil {
		t.Fatalf("PersistPlannedChildMissions returned error: %v", err)
	}
	if len(persistedMissions) != 2 || len(persistedThreads) != 2 {
		t.Fatalf("expected 2 persisted missions and threads, got missions=%d threads=%d", len(persistedMissions), len(persistedThreads))
	}
	if persistedMissions[0].ParentMissionID != parentMission.ID {
		t.Fatalf("expected child parent mission %q, got %q", parentMission.ID, persistedMissions[0].ParentMissionID)
	}
	if string(persistedMissions[0].ReuseTrace) == "" || string(persistedMissions[0].ReuseTrace) == "[]" {
		t.Fatalf("expected reuse trace on persisted mission, got %s", string(persistedMissions[0].ReuseTrace))
	}

	updatedPlans := []PlannedChildMissionInput{
		{
			Title:          "Networking foundation",
			Charter:        "Own the updated networking base layer.",
			Goal:           "Refine networking mission.",
			Scope:          "Routing, firewall, and ingress.",
			MissionType:    "domain",
			AuthorityLevel: "domain",
			ReuseTrace:     json.RawMessage(`[{"sourceType":"thread","sourceId":"thread-network-legacy","reason":"Reuse execution lane"}]`),
		},
	}

	persistedAgain, _, err := runtime.PersistPlannedChildMissions(parentMission.ID, parentThread.ID, updatedPlans)
	if err != nil {
		t.Fatalf("PersistPlannedChildMissions second call returned error: %v", err)
	}
	if len(persistedAgain) != 1 {
		t.Fatalf("expected 1 persisted mission on update, got %d", len(persistedAgain))
	}
	children, err := missionStore.ListChildMissions(parentMission.ID)
	if err != nil {
		t.Fatalf("ListChildMissions returned error: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected child mission count to remain 2, got %d", len(children))
	}
	if children[0].Goal != "Refine networking mission." {
		t.Fatalf("expected existing mission to be updated, got goal=%q", children[0].Goal)
	}
}

func TestRuntimeDelegateMission(t *testing.T) {
	missionStore := NewMemoryStore()
	threadStore := threads.NewMemoryStore()
	runtime, err := NewRuntime(missionStore, threadStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	_, parentMission, parentThread, err := runtime.CreateProgramWithRootMission(RootMissionInput{
		ProgramID:      "program-1",
		ClientID:       "client-1",
		ProgramTitle:   "Google Cloud Replica",
		MissionID:      "mission-root",
		ThreadID:       "thread-root",
		OwnerAgentID:   "ceo-root",
		OwnerRole:      "CEO",
		MissionType:    "root",
		ThreadKind:     "strategy",
		MissionTitle:   "Build full cloud platform",
		Charter:        "Own the full product vision and delegation",
		Goal:           "Deliver a Google Cloud class platform",
		Scope:          "Whole company mission",
		AuthorityLevel: "global",
		ThreadTitle:    "Root CEO strategy thread",
		ThreadSummary:  "Top-level strategy thread for the whole cloud platform program.",
		ThreadContext:  "Created so the top CEO can own product vision, delegation, and high-level program decisions without mixing them with child team execution threads.",
	})
	if err != nil {
		t.Fatalf("CreateProgramWithRootMission returned error: %v", err)
	}

	childMission, childThread, err := runtime.CreateChildMission(ChildMissionInput{
		MissionID:       "mission-networking",
		ParentMissionID: parentMission.ID,
		ThreadID:        "thread-networking",
		OwnerAgentID:    "ceo-root",
		OwnerRole:       "CEO",
		MissionType:     "domain",
		ThreadKind:      "strategy",
		MissionTitle:    "Networking foundation",
		Charter:         "Own the network base layer.",
		Goal:            "Ship the networking platform.",
		Scope:           "Routing and firewall.",
		AuthorityLevel:  "domain",
		ThreadTitle:     "Networking strategy thread",
		ThreadSummary:   "Networking summary",
		ThreadContext:   "Networking thread context",
		ParentThreadID:  parentThread.ID,
	})
	if err != nil {
		t.Fatalf("CreateChildMission returned error: %v", err)
	}

	delegatedMission, delegatedThread, assignment, err := runtime.DelegateMission(DelegationHandoffInput{
		MissionID:          childMission.ID,
		AgentID:            "sub-ceo-networking-foundation",
		AgentRole:          "sub_ceo",
		AuthorityScope:     json.RawMessage(`{"handoffType":"sub_ceo","authorityLevel":"domain"}`),
		ReportingToAgentID: parentMission.OwnerAgentID,
	})
	if err != nil {
		t.Fatalf("DelegateMission returned error: %v", err)
	}
	if delegatedMission.OwnerAgentID != "sub-ceo-networking-foundation" {
		t.Fatalf("expected delegated mission owner to be updated, got %q", delegatedMission.OwnerAgentID)
	}
	if delegatedMission.OwnerRole != "sub_ceo" {
		t.Fatalf("expected delegated mission role to be updated, got %q", delegatedMission.OwnerRole)
	}
	if delegatedThread.OwnerAgentID != "sub-ceo-networking-foundation" {
		t.Fatalf("expected delegated thread owner to be updated, got %q", delegatedThread.OwnerAgentID)
	}
	if assignment.AgentID != "sub-ceo-networking-foundation" || assignment.AgentRole != "sub_ceo" {
		t.Fatalf("unexpected assignment: %#v", assignment)
	}
	assignments, err := missionStore.ListAssignments(childMission.ID)
	if err != nil {
		t.Fatalf("ListAssignments returned error: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected placeholder plus delegated assignment, got %d", len(assignments))
	}
	if assignments[1].ReportingToAgentID != "ceo-root" {
		t.Fatalf("expected delegated assignment to report to ceo-root, got %q", assignments[1].ReportingToAgentID)
	}
	if delegatedThread.OwnerAgentID == childThread.OwnerAgentID {
		t.Fatalf("expected thread owner to change from %q, got %q", childThread.OwnerAgentID, delegatedThread.OwnerAgentID)
	}
}
