package feedback

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrFeedbackNotFound = errors.New("feedback not found")

const (
	AnalysisStatusRaw      = "raw"
	AnalysisStatusEnriched = "enriched"
	AnalysisStatusReviewed = "reviewed"
	AnalysisStatusActioned = "actioned"
)

type Record struct {
	ID                      string
	MissionID               string
	ThreadID                string
	ResponseID              string
	ClientMessageID         string
	TaskID                  string
	TraceID                 string
	Rating                  int
	Reason                  string
	Categories              json.RawMessage
	ClientMessage           string
	CEOResponse             string
	Mode                    string
	ArtifactPaths           json.RawMessage
	TodoRefs                json.RawMessage
	ContextSummary          string
	EvidenceRefs            json.RawMessage
	EnrichedByFeedbackAgent bool
	AnalysisStatus          string
	CreatedAt               time.Time
}

type Store interface {
	CreateFeedback(record Record) error
	GetFeedback(feedbackID string) (Record, error)
	ListByThread(threadID string) ([]Record, error)
}
