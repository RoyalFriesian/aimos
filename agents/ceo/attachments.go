package ceo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

const maxProjectAttachmentSizeBytes = 25 * 1024 * 1024
const maxProjectAttachmentTotalSizeBytes = 30 * 1024 * 1024
const maxProjectAttachmentCount = 32

var allowedAttachmentExtensions = map[string]struct{}{
	".md":       {},
	".markdown": {},
	".txt":      {},
	".pdf":      {},
	".doc":      {},
	".docx":     {},
	".json":     {},
	".yaml":     {},
	".yml":      {},
	".toml":     {},
	".csv":      {},
	".ts":       {},
	".tsx":      {},
	".js":       {},
	".jsx":      {},
	".go":       {},
	".py":       {},
	".java":     {},
	".kt":       {},
	".rb":       {},
	".rs":       {},
	".sh":       {},
	".sql":      {},
	".xml":      {},
	".html":     {},
	".css":      {},
	".scss":     {},
	".png":      {},
	".jpg":      {},
	".jpeg":     {},
	".gif":      {},
	".webp":     {},
	".svg":      {},
	".zip":      {},
	".gz":       {},
	".tgz":      {},
}

var allowedAttachmentBasenames = map[string]struct{}{
	"dockerfile": {},
	"makefile":   {},
	"readme":     {},
	"license":    {},
}

type ProjectAttachmentInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

type StoredProjectAttachment struct {
	Filename     string    `json:"filename"`
	ContentType  string    `json:"contentType,omitempty"`
	SizeBytes    int64     `json:"sizeBytes"`
	RelativePath string    `json:"relativePath"`
	AbsolutePath string    `json:"absolutePath"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

func (s *Service) UploadProjectAttachments(ctx context.Context, threadID string, projectLocation string, files []ProjectAttachmentInput) ([]StoredProjectAttachment, error) {
	if strings.TrimSpace(threadID) == "" {
		return nil, fmt.Errorf("threadId is required")
	}
	if strings.TrimSpace(projectLocation) == "" {
		return nil, fmt.Errorf("projectLocation is required")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("at least one file is required")
	}
	if len(files) > maxProjectAttachmentCount {
		return nil, fmt.Errorf("too many files: max %d attachments per request", maxProjectAttachmentCount)
	}

	thread, err := s.threadStore.GetThread(threadID)
	if err != nil {
		return nil, fmt.Errorf("resolve thread: %w", err)
	}
	missionID := thread.MissionID

	absProjectLocation, err := filepath.Abs(strings.TrimSpace(projectLocation))
	if err != nil {
		return nil, fmt.Errorf("resolve project location: %w", err)
	}
	attachmentsDir := filepath.Join(absProjectLocation, "attachments")
	if err := os.MkdirAll(attachmentsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create attachments directory: %w", err)
	}

	stored := make([]StoredProjectAttachment, 0, len(files))
	now := time.Now().UTC()
	totalSize := 0
	for idx, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := sanitizeAttachmentFilename(file.Filename, idx)
		if !isAllowedAttachmentFilename(name) {
			return nil, fmt.Errorf("file %q is not an allowed attachment type", name)
		}
		if len(file.Data) == 0 {
			return nil, fmt.Errorf("file %q is empty", name)
		}
		if len(file.Data) > maxProjectAttachmentSizeBytes {
			return nil, fmt.Errorf("file %q exceeds max size of %d bytes", name, maxProjectAttachmentSizeBytes)
		}
		totalSize += len(file.Data)
		if totalSize > maxProjectAttachmentTotalSizeBytes {
			return nil, fmt.Errorf("total attachment size exceeds max of %d bytes", maxProjectAttachmentTotalSizeBytes)
		}

		resolvedName, err := uniqueAttachmentName(attachmentsDir, name)
		if err != nil {
			return nil, fmt.Errorf("prepare attachment name %q: %w", name, err)
		}
		absolutePath := filepath.Join(attachmentsDir, resolvedName)
		if err := os.WriteFile(absolutePath, file.Data, 0o644); err != nil {
			return nil, fmt.Errorf("write attachment %q: %w", resolvedName, err)
		}

		fileSize := int64(len(file.Data))
		relPath := filepath.ToSlash(filepath.Join("attachments", resolvedName))
		contentType := strings.TrimSpace(file.ContentType)
		category := attachments.ClassifyFile(resolvedName)

		stored = append(stored, StoredProjectAttachment{
			Filename:     resolvedName,
			ContentType:  contentType,
			SizeBytes:    fileSize,
			RelativePath: relPath,
			AbsolutePath: absolutePath,
			UploadedAt:   now,
		})

		if s.attachmentStore != nil {
			if regErr := s.attachmentStore.Create(attachments.Attachment{
				ID:           fmt.Sprintf("att-%s-%d-%d", threadID, now.UnixNano(), idx),
				MissionID:    missionID,
				ThreadID:     threadID,
				Filename:     resolvedName,
				ContentType:  contentType,
				SizeBytes:    fileSize,
				RelativePath: relPath,
				AbsolutePath: absolutePath,
				FileCategory: category,
				TokenEstimate: attachments.EstimateTokens(fileSize, category),
				Status:       attachments.StatusActive,
				CreatedAt:    now,
			}); regErr != nil {
				return nil, fmt.Errorf("register attachment %q: %w", resolvedName, regErr)
			}
		}
	}

	payload, err := json.Marshal(map[string]any{
		"projectLocation": absProjectLocation,
		"attachmentsDir":  attachmentsDir,
		"attachments":     stored,
		"count":           len(stored),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal attachment log payload: %w", err)
	}

	if err := s.threadStore.AppendMessage(threads.Message{
		ID:            fmt.Sprintf("attachment-%d", time.Now().UTC().UnixNano()),
		ThreadID:      threadID,
		Role:          threads.RoleAssistant,
		AuthorAgentID: "system",
		AuthorRole:    "system",
		MessageType:   "project_attachments_uploaded",
		Content:       fmt.Sprintf("Uploaded %d attachment(s) to %s", len(stored), filepath.ToSlash(attachmentsDir)),
		ContentJSON:   payload,
		Mode:          string(ModeExecutionPrep),
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("append attachment log message: %w", err)
	}

	return stored, nil
}

func isAllowedAttachmentFilename(name string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	ext := filepath.Ext(lowerName)
	if ext != "" {
		_, ok := allowedAttachmentExtensions[ext]
		return ok
	}
	_, ok := allowedAttachmentBasenames[lowerName]
	return ok
}

func sanitizeAttachmentFilename(name string, fallbackIndex int) string {
	trimmed := strings.TrimSpace(name)
	trimmed = filepath.Base(trimmed)
	trimmed = strings.ReplaceAll(trimmed, "\\", "_")
	trimmed = strings.ReplaceAll(trimmed, "/", "_")
	trimmed = strings.ReplaceAll(trimmed, "\x00", "")
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return fmt.Sprintf("attachment-%d.bin", fallbackIndex+1)
	}
	return trimmed
}

func uniqueAttachmentName(dir string, preferred string) (string, error) {
	candidate := preferred
	ext := filepath.Ext(preferred)
	base := strings.TrimSuffix(preferred, ext)

	for i := 0; i < 1000; i++ {
		path := filepath.Join(dir, candidate)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i+1, ext)
	}

	return "", fmt.Errorf("could not allocate unique filename for %q", preferred)
}
