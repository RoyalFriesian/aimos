package threads

import "testing"

func TestMemoryStoreAppendAndListMessages(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateThread(Thread{ID: "thread-1", MissionID: "mission-1", Title: "Root thread"}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	if err := store.AppendMessage(Message{ID: "m-1", ThreadID: "thread-1", Role: RoleUser, Content: "hello"}); err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}
	if err := store.AppendMessage(Message{ID: "m-2", ThreadID: "thread-1", Role: RoleAssistant, Content: "hi"}); err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}

	messages, err := store.ListMessages("thread-1")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "hello" || messages[1].Content != "hi" {
		t.Fatalf("messages out of order: %#v", messages)
	}
}

func TestMemoryStoreUpdateThreadMode(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateThread(Thread{ID: "thread-1", MissionID: "mission-1", Title: "Root thread"}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	if err := store.UpdateThreadMode("thread-1", "discovery"); err != nil {
		t.Fatalf("UpdateThreadMode returned error: %v", err)
	}

	thread, err := store.GetThread("thread-1")
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if thread.CurrentMode != "discovery" {
		t.Fatalf("expected mode discovery, got %q", thread.CurrentMode)
	}
}

func TestMemoryStoreUpdateThreadOwner(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateThread(Thread{ID: "thread-1", MissionID: "mission-1", Title: "Root thread", OwnerAgentID: "ceo-root"}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	if err := store.UpdateThreadOwner("thread-1", "sub-ceo-networking"); err != nil {
		t.Fatalf("UpdateThreadOwner returned error: %v", err)
	}

	thread, err := store.GetThread("thread-1")
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if thread.OwnerAgentID != "sub-ceo-networking" {
		t.Fatalf("expected owner sub-ceo-networking, got %q", thread.OwnerAgentID)
	}
}

func TestMemoryStoreListByMission(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateThread(Thread{ID: "thread-1", MissionID: "mission-1", Title: "Thread 1"}); err != nil {
		t.Fatalf("CreateThread thread-1 returned error: %v", err)
	}
	if err := store.CreateThread(Thread{ID: "thread-2", MissionID: "mission-1", Title: "Thread 2"}); err != nil {
		t.Fatalf("CreateThread thread-2 returned error: %v", err)
	}

	threadsForMission, err := store.ListByMission("mission-1")
	if err != nil {
		t.Fatalf("ListByMission returned error: %v", err)
	}
	if len(threadsForMission) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threadsForMission))
	}
}

func TestMemoryStoreCreateThreadDefaultsSummaryAndContext(t *testing.T) {
	store := NewMemoryStore()
	if err := store.CreateThread(Thread{ID: "thread-1", MissionID: "mission-1", Title: "Networking execution"}); err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}

	thread, err := store.GetThread("thread-1")
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if thread.Summary != "Networking execution" {
		t.Fatalf("expected summary to default to title, got %q", thread.Summary)
	}
	if thread.Context != "Networking execution" {
		t.Fatalf("expected context to default to summary, got %q", thread.Context)
	}
}

func TestMemoryStoreSearchReusableThreads(t *testing.T) {
	store := NewMemoryStore()
	for _, thread := range []Thread{
		{
			ID:            "thread-network-finished",
			MissionID:     "mission-1",
			RootMissionID: "mission-1",
			Kind:          "execution",
			Title:         "Networking migration lane",
			Summary:       "Firewall and routing execution summary",
			Context:       "Tracks firewall rollout, routing changes, and network cutover.",
			Status:        ThreadStatusFinished,
		},
		{
			ID:            "thread-storage-completed",
			MissionID:     "mission-2",
			RootMissionID: "mission-2",
			Kind:          "execution",
			Title:         "Storage lane",
			Summary:       "Object storage implementation summary",
			Context:       "Tracks bucket lifecycle and replication.",
			Status:        ThreadStatusCompleted,
		},
		{
			ID:            "thread-network-active",
			MissionID:     "mission-3",
			RootMissionID: "mission-3",
			Kind:          "execution",
			Title:         "Networking draft lane",
			Summary:       "Still in progress",
			Context:       "Not reusable yet.",
			Status:        ThreadStatusActive,
		},
	} {
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread(%s) returned error: %v", thread.ID, err)
		}
	}

	matches, err := store.SearchReusableThreads("network firewall routing", 5)
	if err != nil {
		t.Fatalf("SearchReusableThreads returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 reusable thread match, got %d", len(matches))
	}
	if matches[0].Thread.ID != "thread-network-finished" {
		t.Fatalf("expected thread-network-finished, got %q", matches[0].Thread.ID)
	}
	if matches[0].Score <= 0 {
		t.Fatalf("expected positive score, got %v", matches[0].Score)
	}
	if len(matches[0].MatchedTerms) != 3 {
		t.Fatalf("expected matched terms to be captured, got %#v", matches[0].MatchedTerms)
	}
}
