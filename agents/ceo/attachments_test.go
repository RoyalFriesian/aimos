package ceo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadProjectAttachmentsRejectsDisallowedType(t *testing.T) {
	service, missionStore, threadStore, _, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-attach-1", "thread-attach-1")

	_, err := service.UploadProjectAttachments(context.Background(), "thread-attach-1", t.TempDir(), []ProjectAttachmentInput{
		{Filename: "payload.exe", ContentType: "application/octet-stream", Data: []byte("x")},
	})
	if err == nil {
		t.Fatalf("expected disallowed attachment error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not an allowed attachment type") {
		t.Fatalf("expected allowlist error, got %v", err)
	}
}

func TestUploadProjectAttachmentsRejectsOversizedRequest(t *testing.T) {
	service, missionStore, threadStore, _, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-attach-2", "thread-attach-2")

	big := make([]byte, 16*1024*1024)
	_, err := service.UploadProjectAttachments(context.Background(), "thread-attach-2", t.TempDir(), []ProjectAttachmentInput{
		{Filename: "part-1.txt", ContentType: "text/plain", Data: big},
		{Filename: "part-2.txt", ContentType: "text/plain", Data: big},
	})
	if err == nil {
		t.Fatalf("expected total-size guardrail error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "total attachment size") {
		t.Fatalf("expected total attachment size error, got %v", err)
	}
}

func TestUploadProjectAttachmentsPersistsFilesAndLogsEvent(t *testing.T) {
	service, missionStore, threadStore, _, _ := newTestService(t, &stubCompletionClient{responses: []string{"unused"}})
	seedMissionThread(t, missionStore, threadStore, "mission-attach-3", "thread-attach-3")

	projectDir := t.TempDir()
	stored, err := service.UploadProjectAttachments(context.Background(), "thread-attach-3", projectDir, []ProjectAttachmentInput{
		{Filename: "requirements.md", ContentType: "text/markdown", Data: []byte("requirements")},
		{Filename: "notes.txt", ContentType: "text/plain", Data: []byte("notes")},
	})
	if err != nil {
		t.Fatalf("UploadProjectAttachments returned error: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 stored attachments, got %d", len(stored))
	}

	for _, file := range stored {
		if !strings.HasPrefix(file.RelativePath, "attachments/") {
			t.Fatalf("expected attachments relative path, got %q", file.RelativePath)
		}
		if _, statErr := os.Stat(filepath.Join(projectDir, filepath.FromSlash(file.RelativePath))); statErr != nil {
			t.Fatalf("expected stored file %q to exist: %v", file.RelativePath, statErr)
		}
	}

	messages, err := threadStore.ListMessages("thread-attach-3")
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected attachment log message")
	}
	last := messages[len(messages)-1]
	if last.MessageType != "project_attachments_uploaded" {
		t.Fatalf("expected project_attachments_uploaded message, got %q", last.MessageType)
	}
}
