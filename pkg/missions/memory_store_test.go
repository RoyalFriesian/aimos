package missions

import "testing"

func TestMemoryStoreCreateMissionTree(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateProgram(Program{ID: "program-1", ClientID: "client-1", Title: "Google Cloud Replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if err := store.CreateMission(Mission{ID: "mission-root", ProgramID: "program-1", OwnerAgentID: "ceo-root", OwnerRole: "CEO", MissionType: "root", Title: "Build platform"}); err != nil {
		t.Fatalf("CreateMission returned error: %v", err)
	}
	if err := store.CreateMission(Mission{ID: "mission-child", ProgramID: "program-1", ParentMissionID: "mission-root", RootMissionID: "mission-root", OwnerAgentID: "ceo-network", OwnerRole: "CEO", MissionType: "domain", Title: "Build networking"}); err != nil {
		t.Fatalf("CreateMission child returned error: %v", err)
	}

	children, err := store.ListChildMissions("mission-root")
	if err != nil {
		t.Fatalf("ListChildMissions returned error: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("expected 1 child mission, got %d", len(children))
	}
	if children[0].ID != "mission-child" {
		t.Fatalf("expected mission-child, got %q", children[0].ID)
	}
}

func TestMemoryStoreAssignments(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateProgram(Program{ID: "program-1", ClientID: "client-1", Title: "Google Cloud Replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if err := store.CreateMission(Mission{ID: "mission-root", ProgramID: "program-1", OwnerAgentID: "ceo-root", OwnerRole: "CEO", MissionType: "root", Title: "Build platform"}); err != nil {
		t.Fatalf("CreateMission returned error: %v", err)
	}
	if err := store.AssignMission(Assignment{ID: "assign-1", MissionID: "mission-root", AgentID: "ceo-root", AgentRole: "CEO"}); err != nil {
		t.Fatalf("AssignMission returned error: %v", err)
	}

	assignments, err := store.ListAssignments("mission-root")
	if err != nil {
		t.Fatalf("ListAssignments returned error: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	if assignments[0].AgentID != "ceo-root" {
		t.Fatalf("expected agent ceo-root, got %q", assignments[0].AgentID)
	}
}

func TestMemoryStoreSearchReusableMissions(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateProgram(Program{ID: "program-1", ClientID: "client-1", Title: "Cloud replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}

	for _, mission := range []Mission{
		{
			ID:           "mission-network-finished",
			ProgramID:    "program-1",
			OwnerAgentID: "ceo-root",
			OwnerRole:    "CEO",
			MissionType:  "domain",
			Title:        "Networking control plane",
			Charter:      "Own the VPC and firewall roadmap",
			Goal:         "Ship multi-region networking",
			Scope:        "VPCs, routing, and firewall rules",
			Status:       MissionStatusFinished,
		},
		{
			ID:           "mission-storage-completed",
			ProgramID:    "program-1",
			OwnerAgentID: "ceo-root",
			OwnerRole:    "CEO",
			MissionType:  "domain",
			Title:        "Object storage",
			Charter:      "Own durable blob storage",
			Goal:         "Ship S3-compatible APIs",
			Scope:        "Buckets and object lifecycle",
			Status:       MissionStatusCompleted,
		},
		{
			ID:           "mission-network-active",
			ProgramID:    "program-1",
			OwnerAgentID: "ceo-root",
			OwnerRole:    "CEO",
			MissionType:  "domain",
			Title:        "Networking experiments",
			Charter:      "Explore early ideas",
			Goal:         "Draft options",
			Scope:        "Unfinished discovery",
			Status:       MissionStatusActive,
		},
	} {
		if err := store.CreateMission(mission); err != nil {
			t.Fatalf("CreateMission(%s) returned error: %v", mission.ID, err)
		}
	}

	matches, err := store.SearchReusableMissions("network firewall routing", 5)
	if err != nil {
		t.Fatalf("SearchReusableMissions returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 reusable match, got %d", len(matches))
	}
	if matches[0].Mission.ID != "mission-network-finished" {
		t.Fatalf("expected mission-network-finished, got %q", matches[0].Mission.ID)
	}
	if matches[0].Score <= 0 {
		t.Fatalf("expected positive score, got %v", matches[0].Score)
	}
	if len(matches[0].MatchedTerms) != 3 {
		t.Fatalf("expected matched terms to be captured, got %#v", matches[0].MatchedTerms)
	}
}
