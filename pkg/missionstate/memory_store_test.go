package missionstate

import (
	"encoding/json"
	"testing"

	"github.com/Sarnga/agent-platform/pkg/missions"
)

func TestMemoryStoreCreateAndGetLatestSummary(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateSummary(Summary{
		ID:               "summary-1",
		MissionID:        "mission-1",
		ThreadID:         "thread-1",
		Level:            "mission",
		Kind:             "rolling",
		CoverageStartRef: "msg-1",
		CoverageEndRef:   "msg-5",
		SummaryText:      "Networking planning has started.",
	}); err != nil {
		t.Fatalf("CreateSummary returned error: %v", err)
	}
	if err := store.CreateSummary(Summary{
		ID:               "summary-2",
		MissionID:        "mission-1",
		Level:            "mission",
		Kind:             "checkpoint",
		CoverageStartRef: "msg-6",
		CoverageEndRef:   "msg-10",
		SummaryText:      "Networking scope narrowed to VPC and firewall first.",
	}); err != nil {
		t.Fatalf("CreateSummary second returned error: %v", err)
	}

	latest, err := store.GetLatestSummary("mission-1")
	if err != nil {
		t.Fatalf("GetLatestSummary returned error: %v", err)
	}
	if latest.ID != "summary-2" {
		t.Fatalf("expected latest summary summary-2, got %q", latest.ID)
	}

	entries, err := store.ListSummaries("mission-1")
	if err != nil {
		t.Fatalf("ListSummaries returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(entries))
	}
}

func TestMemoryStoreUpsertAndListRollups(t *testing.T) {
	store := NewMemoryStore()
	if err := store.UpsertRollup(Rollup{
		ID:              "rollup-1",
		ParentMissionID: "mission-root",
		ChildMissionID:  "mission-network",
		Status:          missions.MissionStatusActive,
		ProgressPercent: 25,
		Health:          "green",
		LatestSummary:   "Networking mission is progressing with no blockers.",
	}); err != nil {
		t.Fatalf("UpsertRollup returned error: %v", err)
	}
	if err := store.UpsertRollup(Rollup{
		ID:              "rollup-1b",
		ParentMissionID: "mission-root",
		ChildMissionID:  "mission-network",
		Status:          missions.MissionStatusBlocked,
		ProgressPercent: 40,
		Health:          "yellow",
		CurrentBlocker:  "Waiting for infra access",
		LatestSummary:   "Networking mission is blocked on infra access.",
	}); err != nil {
		t.Fatalf("UpsertRollup second returned error: %v", err)
	}

	rollup, err := store.GetRollup("mission-root", "mission-network")
	if err != nil {
		t.Fatalf("GetRollup returned error: %v", err)
	}
	if rollup.Status != missions.MissionStatusBlocked {
		t.Fatalf("expected blocked status, got %q", rollup.Status)
	}
	if rollup.CurrentBlocker != "Waiting for infra access" {
		t.Fatalf("unexpected blocker %q", rollup.CurrentBlocker)
	}
	var executionSummary map[string]any
	if err := json.Unmarshal(rollup.ExecutionSummary, &executionSummary); err != nil {
		t.Fatalf("unmarshal execution summary: %v", err)
	}

	rollups, err := store.ListRollups("mission-root")
	if err != nil {
		t.Fatalf("ListRollups returned error: %v", err)
	}
	if len(rollups) != 1 {
		t.Fatalf("expected 1 rollup, got %d", len(rollups))
	}
}
