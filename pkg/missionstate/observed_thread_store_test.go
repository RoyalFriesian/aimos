package missionstate

import (
	"testing"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func TestObservedThreadStoreRefreshesMissionStateOnAppend(t *testing.T) {
	missionStore := missions.NewMemoryStore()
	baseThreadStore := threads.NewMemoryStore()
	stateStore := NewMemoryStore()
	executionStore := execution.NewMemoryStore()
	runtime, err := NewRuntime(stateStore, missionStore, baseThreadStore, executionStore)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	observedStore, err := NewObservedThreadStore(baseThreadStore, runtime)
	if err != nil {
		t.Fatalf("NewObservedThreadStore returned error: %v", err)
	}

	if err := missionStore.CreateProgram(missions.Program{ID: "program-1", ClientID: "client-1", Title: "Cloud replica"}); err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	for _, mission := range []missions.Mission{
		{
			ID:             "mission-root",
			ProgramID:      "program-1",
			RootMissionID:  "mission-root",
			OwningThreadID: "thread-root",
			OwnerAgentID:   "ceo-root",
			OwnerRole:      "CEO",
			MissionType:    "root",
			Title:          "Build cloud",
			Charter:        "Own full cloud strategy",
			Goal:           "Deliver cloud platform",
			Scope:          "Top-level program",
			AuthorityLevel: "global",
			Status:         missions.MissionStatusActive,
		},
		{
			ID:              "mission-network",
			ProgramID:       "program-1",
			ParentMissionID: "mission-root",
			RootMissionID:   "mission-root",
			OwningThreadID:  "thread-network",
			OwnerAgentID:    "sub-ceo-networking",
			OwnerRole:       "sub_ceo",
			MissionType:     "domain",
			Title:           "Networking foundation",
			Charter:         "Own the networking base layer",
			Goal:            "Deliver networking foundations",
			Scope:           "Routing and firewall",
			AuthorityLevel:  "domain",
			Status:          missions.MissionStatusActive,
			ProgressPercent: 20,
		},
	} {
		if err := missionStore.CreateMission(mission); err != nil {
			t.Fatalf("CreateMission(%s) returned error: %v", mission.ID, err)
		}
	}
	for _, thread := range []threads.Thread{
		{ID: "thread-root", MissionID: "mission-root", RootMissionID: "mission-root", Kind: "strategy", Title: "Root thread", Summary: "Root summary", Context: "Root context", OwnerAgentID: "ceo-root", Status: threads.ThreadStatusActive},
		{ID: "thread-network", MissionID: "mission-network", RootMissionID: "mission-root", ParentThreadID: "thread-root", Kind: "execution", Title: "Networking thread", Summary: "Networking summary", Context: "Networking context", OwnerAgentID: "sub-ceo-networking", Status: threads.ThreadStatusActive},
	} {
		if err := observedStore.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread(%s) returned error: %v", thread.ID, err)
		}
	}

	if err := observedStore.AppendMessage(threads.Message{
		ID:            "msg-1",
		ThreadID:      "thread-network",
		Role:          threads.RoleAssistant,
		AuthorAgentID: "worker-networking",
		AuthorRole:    "worker",
		MessageType:   "worker_update",
		Content:       "Applied the routing and firewall execution update.",
	}); err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}

	latestSummary, err := stateStore.GetLatestSummary("mission-network")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if latestSummary.CoverageStartRef != "msg-1" || latestSummary.CoverageEndRef != "msg-1" {
		t.Fatalf("expected summary coverage to reflect the appended delegated message, got start=%q end=%q", latestSummary.CoverageStartRef, latestSummary.CoverageEndRef)
	}
	if latestSummary.SummaryText == "" {
		t.Fatalf("expected delegated activity to generate summary text, got %#v", latestSummary)
	}
	rollup, err := stateStore.GetRollup("mission-root", "mission-network")
	if err != nil {
		t.Fatalf("GetRollup returned error: %v", err)
	}
	if rollup.ChildMissionID != "mission-network" {
		t.Fatalf("unexpected rollup child mission: %#v", rollup)
	}
	if rollup.LatestSummary != latestSummary.SummaryText {
		t.Fatalf("expected rollup to reflect latest delegated summary, got %q want %q", rollup.LatestSummary, latestSummary.SummaryText)
	}
}
