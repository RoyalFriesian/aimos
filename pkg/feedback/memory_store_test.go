package feedback

import (
	"testing"
	"time"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	createdAt := time.Now().UTC()
	record := Record{
		ID:             "feedback-1",
		ThreadID:       "thread-1",
		ResponseID:     "response-1",
		MissionID:      "mission-1",
		Rating:         4,
		AnalysisStatus: AnalysisStatusRaw,
		CreatedAt:      createdAt,
	}
	if err := store.CreateFeedback(record); err != nil {
		t.Fatalf("CreateFeedback returned error: %v", err)
	}
	loaded, err := store.GetFeedback("feedback-1")
	if err != nil {
		t.Fatalf("GetFeedback returned error: %v", err)
	}
	if loaded.ResponseID != "response-1" {
		t.Fatalf("expected response-1, got %#v", loaded)
	}
	byThread, err := store.ListByThread("thread-1")
	if err != nil {
		t.Fatalf("ListByThread returned error: %v", err)
	}
	if len(byThread) != 1 || !byThread[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected thread feedback listing: %#v", byThread)
	}
}
