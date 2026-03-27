package attachments_test

import (
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/missions"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	store := attachments.NewMemoryStore(nil)

	att := attachments.Attachment{
		ID:           "att-1",
		MissionID:    "m-1",
		ThreadID:     "t-1",
		Filename:     "main.go",
		AbsolutePath: "/tmp/attachments/main.go",
		RelativePath: "attachments/main.go",
		FileCategory: attachments.CategoryTextCode,
		SizeBytes:    1024,
		Status:       attachments.StatusActive,
		CreatedAt:    time.Now().UTC(),
	}
	if err := store.Create(att); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := store.Get("att-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Filename != "main.go" {
		t.Errorf("filename = %q, want main.go", got.Filename)
	}
}

func TestMemoryStore_CreateIdempotent(t *testing.T) {
	store := attachments.NewMemoryStore(nil)
	att := attachments.Attachment{
		ID: "att-1", MissionID: "m-1", ThreadID: "t-1",
		Filename: "a.go", AbsolutePath: "/a.go", FileCategory: attachments.CategoryTextCode,
	}
	if err := store.Create(att); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(att); err != nil {
		t.Fatalf("second create should be idempotent: %v", err)
	}
}

func TestMemoryStore_CreateValidation(t *testing.T) {
	store := attachments.NewMemoryStore(nil)
	tests := []struct {
		name string
		att  attachments.Attachment
	}{
		{"empty id", attachments.Attachment{MissionID: "m", ThreadID: "t", Filename: "f", AbsolutePath: "/f", FileCategory: attachments.CategoryTextCode}},
		{"empty mission", attachments.Attachment{ID: "a", ThreadID: "t", Filename: "f", AbsolutePath: "/f", FileCategory: attachments.CategoryTextCode}},
		{"empty thread", attachments.Attachment{ID: "a", MissionID: "m", Filename: "f", AbsolutePath: "/f", FileCategory: attachments.CategoryTextCode}},
		{"empty filename", attachments.Attachment{ID: "a", MissionID: "m", ThreadID: "t", AbsolutePath: "/f", FileCategory: attachments.CategoryTextCode}},
		{"empty path", attachments.Attachment{ID: "a", MissionID: "m", ThreadID: "t", Filename: "f", FileCategory: attachments.CategoryTextCode}},
		{"empty category", attachments.Attachment{ID: "a", MissionID: "m", ThreadID: "t", Filename: "f", AbsolutePath: "/f"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Create(tc.att); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestMemoryStore_ListByMission(t *testing.T) {
	store := attachments.NewMemoryStore(nil)
	now := time.Now().UTC()
	for i, name := range []string{"a.go", "b.go", "c.go"} {
		_ = store.Create(attachments.Attachment{
			ID: name, MissionID: "m-1", ThreadID: "t-1",
			Filename: name, AbsolutePath: "/" + name, FileCategory: attachments.CategoryTextCode,
			Status: attachments.StatusActive, CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	// Add archived — should not appear.
	_ = store.Create(attachments.Attachment{
		ID: "d.go", MissionID: "m-1", ThreadID: "t-1",
		Filename: "d.go", AbsolutePath: "/d.go", FileCategory: attachments.CategoryTextCode,
		Status: attachments.StatusArchived, CreatedAt: now,
	})

	list, err := store.ListByMission("m-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	if list[0].Filename != "a.go" || list[2].Filename != "c.go" {
		t.Errorf("unexpected order: %v, %v", list[0].Filename, list[2].Filename)
	}
}

func TestMemoryStore_ListByThread(t *testing.T) {
	store := attachments.NewMemoryStore(nil)
	_ = store.Create(attachments.Attachment{
		ID: "att-1", MissionID: "m-1", ThreadID: "t-1",
		Filename: "x.go", AbsolutePath: "/x.go", FileCategory: attachments.CategoryTextCode,
		Status: attachments.StatusActive, CreatedAt: time.Now().UTC(),
	})
	_ = store.Create(attachments.Attachment{
		ID: "att-2", MissionID: "m-1", ThreadID: "t-2",
		Filename: "y.go", AbsolutePath: "/y.go", FileCategory: attachments.CategoryTextCode,
		Status: attachments.StatusActive, CreatedAt: time.Now().UTC(),
	})

	list, err := store.ListByThread("t-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Filename != "x.go" {
		t.Errorf("unexpected: %+v", list)
	}
}

func TestMemoryStore_ListInheritedAttachments(t *testing.T) {
	missionData := map[string]missions.Mission{
		"root":  {ID: "root", ParentMissionID: ""},
		"child": {ID: "child", ParentMissionID: "root"},
		"leaf":  {ID: "leaf", ParentMissionID: "child"},
	}
	getter := func(id string) (missions.Mission, error) {
		m, ok := missionData[id]
		if !ok {
			return missions.Mission{}, missions.ErrMissionNotFound
		}
		return m, nil
	}
	store := attachments.NewMemoryStore(getter)
	now := time.Now().UTC()

	_ = store.Create(attachments.Attachment{
		ID: "root-att", MissionID: "root", ThreadID: "t-root",
		Filename: "root.md", AbsolutePath: "/root.md", FileCategory: attachments.CategoryTextDoc,
		Status: attachments.StatusActive, CreatedAt: now,
	})
	_ = store.Create(attachments.Attachment{
		ID: "child-att", MissionID: "child", ThreadID: "t-child",
		Filename: "child.go", AbsolutePath: "/child.go", FileCategory: attachments.CategoryTextCode,
		Status: attachments.StatusActive, CreatedAt: now.Add(time.Second),
	})
	_ = store.Create(attachments.Attachment{
		ID: "leaf-att", MissionID: "leaf", ThreadID: "t-leaf",
		Filename: "leaf.py", AbsolutePath: "/leaf.py", FileCategory: attachments.CategoryTextCode,
		Status: attachments.StatusActive, CreatedAt: now.Add(2 * time.Second),
	})

	list, err := store.ListInheritedAttachments("leaf")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	// Should be ordered root -> child -> leaf.
	if list[0].Filename != "root.md" {
		t.Errorf("first = %q, want root.md", list[0].Filename)
	}
	if list[1].Filename != "child.go" {
		t.Errorf("second = %q, want child.go", list[1].Filename)
	}
	if list[2].Filename != "leaf.py" {
		t.Errorf("third = %q, want leaf.py", list[2].Filename)
	}
}

func TestMemoryStore_GetNotFound(t *testing.T) {
	store := attachments.NewMemoryStore(nil)
	_, err := store.Get("nonexistent")
	if err != attachments.ErrAttachmentNotFound {
		t.Errorf("err = %v, want ErrAttachmentNotFound", err)
	}
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		filename string
		want     attachments.FileCategory
	}{
		{"main.go", attachments.CategoryTextCode},
		{"app.tsx", attachments.CategoryTextCode},
		{"README.md", attachments.CategoryTextDoc},
		{"data.csv", attachments.CategoryTextDoc},
		{"config.yaml", attachments.CategoryTextDoc},
		{"photo.png", attachments.CategoryImage},
		{"logo.svg", attachments.CategoryImage},
		{"report.pdf", attachments.CategoryRichDoc},
		{"doc.docx", attachments.CategoryRichDoc},
		{"archive.zip", attachments.CategoryArchive},
		{"Dockerfile", attachments.CategoryTextDoc},
		{"Makefile", attachments.CategoryTextDoc},
	}
	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := attachments.ClassifyFile(tc.filename)
			if got != tc.want {
				t.Errorf("ClassifyFile(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := attachments.EstimateTokens(4000, attachments.CategoryTextCode); got != 1000 {
		t.Errorf("text code tokens = %d, want 1000", got)
	}
	if got := attachments.EstimateTokens(100000, attachments.CategoryImage); got != 1000 {
		t.Errorf("image tokens = %d, want 1000", got)
	}
	if got := attachments.EstimateTokens(100000, attachments.CategoryArchive); got != 0 {
		t.Errorf("archive tokens = %d, want 0", got)
	}
}
