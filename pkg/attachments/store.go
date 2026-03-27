package attachments

import (
	"errors"
	"time"
)

var ErrAttachmentNotFound = errors.New("attachment not found")

// FileCategory classifies an attachment for context-pack loading decisions.
type FileCategory string

const (
	CategoryTextCode FileCategory = "text_code"
	CategoryTextDoc  FileCategory = "text_doc"
	CategoryImage    FileCategory = "image"
	CategoryRichDoc  FileCategory = "rich_doc"
	CategoryArchive  FileCategory = "archive"
)

type AttachmentStatus string

const (
	StatusActive   AttachmentStatus = "active"
	StatusArchived AttachmentStatus = "archived"
	StatusFailed   AttachmentStatus = "failed"
)

// Attachment is the durable registry record for a file uploaded to a mission.
type Attachment struct {
	ID                  string
	MissionID           string
	ThreadID            string
	UploadedByMessageID string
	Filename            string
	ContentType         string
	SizeBytes           int64
	RelativePath        string
	AbsolutePath        string
	FileCategory        FileCategory
	TokenEstimate       int
	ExtractedText       *string
	ParentAttachmentID  string
	Status              AttachmentStatus
	CreatedAt           time.Time
}

// IsTextInjectable returns true when the file content can be read and injected
// directly into the LLM context as plain text.
func (a Attachment) IsTextInjectable() bool {
	return a.FileCategory == CategoryTextCode || a.FileCategory == CategoryTextDoc
}

// Store is the persistence interface for the mission attachment registry.
type Store interface {
	Create(attachment Attachment) error
	Get(attachmentID string) (Attachment, error)
	ListByMission(missionID string) ([]Attachment, error)
	ListByThread(threadID string) ([]Attachment, error)
	// ListInheritedAttachments returns all active attachments for the given
	// mission plus every ancestor mission up to the root. The missions store
	// is used to walk the parent chain. Callers receive a flat slice ordered
	// from the root mission down to the target mission.
	ListInheritedAttachments(missionID string) ([]Attachment, error)
}