package contextpacks

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/Sarnga/agent-platform/pkg/attachments"
	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

const defaultRecentMessagesLimit = 8
const defaultAttachmentTokenBudget = 32768
const defaultMaxSingleFileTokens = 8192
const defaultMaxImageAttachments = 5
const maxImageFileSize = 20 * 1024 * 1024 // 20 MB

type BuildOptions struct {
	RecentMessagesLimit   int
	IncludeChildRollups   bool
	AttachmentTokenBudget int
	MaxSingleFileTokens   int
	MaxImageAttachments   int
}

// AttachmentContent holds the loaded text content of a single text-injectable attachment.
type AttachmentContent struct {
	AttachmentID string
	Filename     string
	Category     attachments.FileCategory
	Content      string
	Truncated    bool
	Tokens       int
}

type ContextPack struct {
	Mission            missions.Mission
	Thread             threads.Thread
	LatestSummary      *missionstate.Summary
	ChildRollups       []missionstate.Rollup
	DueTodos           []execution.Todo
	DueTimers          []execution.Timer
	RecentMessages     []threads.Message
	Attachments        []attachments.Attachment
	AttachmentContents []AttachmentContent
	ImageDataURLs      []string
}

type Builder struct {
	missions     missions.Store
	threads      threads.Store
	missionState missionstate.Store
	execution    execution.Store
	attachments  attachments.Store
}

func NewBuilder(missionStore missions.Store, threadStore threads.Store, missionStateStore missionstate.Store, executionStore execution.Store, attachmentStore attachments.Store) (*Builder, error) {
	if missionStore == nil {
		return nil, fmt.Errorf("mission store is required")
	}
	if threadStore == nil {
		return nil, fmt.Errorf("thread store is required")
	}
	if missionStateStore == nil {
		return nil, fmt.Errorf("mission state store is required")
	}
	if executionStore == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return &Builder{
		missions:     missionStore,
		threads:      threadStore,
		missionState: missionStateStore,
		execution:    executionStore,
		attachments:  attachmentStore,
	}, nil
}

func (b *Builder) BuildRootCEOPack(rootMissionID string, options BuildOptions) (ContextPack, error) {
	return b.BuildMissionPack(rootMissionID, "", options)
}

func (b *Builder) BuildMissionPack(missionID string, threadID string, options BuildOptions) (ContextPack, error) {
	mission, err := b.missions.GetMission(missionID)
	if err != nil {
		return ContextPack{}, err
	}

	if threadID == "" {
		threadID = mission.OwningThreadID
	}
	if threadID == "" {
		return ContextPack{}, fmt.Errorf("mission %q does not have an owning thread", mission.ID)
	}

	thread, err := b.threads.GetThread(threadID)
	if err != nil {
		return ContextPack{}, err
	}

	messages, err := b.threads.ListMessages(threadID)
	if err != nil {
		return ContextPack{}, err
	}

	pack := ContextPack{
		Mission:        mission,
		Thread:         thread,
		RecentMessages: lastMessages(messages, normalizeRecentMessageLimit(options.RecentMessagesLimit)),
	}

	latestSummary, err := b.missionState.GetLatestSummary(missionID)
	if err == nil {
		pack.LatestSummary = &latestSummary
	} else if err != missionstate.ErrSummaryNotFound {
		return ContextPack{}, err
	}

	if options.IncludeChildRollups {
		rollups, err := b.missionState.ListRollups(missionID)
		if err != nil {
			return ContextPack{}, err
		}
		pack.ChildRollups = rollups
	}

	dueTodos, err := b.execution.ListDueTodos(time.Now().UTC(), 64)
	if err != nil {
		return ContextPack{}, err
	}
	pack.DueTodos = filterDueTodosForMission(dueTodos, missionID)

	dueTimers, err := b.execution.ListDueTimers(time.Now().UTC(), 64)
	if err != nil {
		return ContextPack{}, err
	}
	pack.DueTimers = filterDueTimersForMission(dueTimers, missionID)

	if b.attachments != nil {
		allAttachments, err := b.attachments.ListInheritedAttachments(missionID)
		if err != nil {
			return ContextPack{}, err
		}
		pack.Attachments = allAttachments
		pack.AttachmentContents = loadAttachmentContents(allAttachments, options)
		pack.ImageDataURLs = loadImageDataURLs(allAttachments, options)
	}

	return pack, nil
}

func normalizeRecentMessageLimit(limit int) int {
	if limit <= 0 {
		return defaultRecentMessagesLimit
	}
	return limit
}

func lastMessages(messages []threads.Message, limit int) []threads.Message {
	if len(messages) <= limit {
		copied := make([]threads.Message, len(messages))
		copy(copied, messages)
		return copied
	}
	start := len(messages) - limit
	trimmed := make([]threads.Message, limit)
	copy(trimmed, messages[start:])
	return trimmed
}

func filterDueTodosForMission(todos []execution.Todo, missionID string) []execution.Todo {
	filtered := make([]execution.Todo, 0, len(todos))
	for _, todo := range todos {
		if todo.MissionID != missionID {
			continue
		}
		filtered = append(filtered, todo)
	}
	return filtered
}

func filterDueTimersForMission(timers []execution.Timer, missionID string) []execution.Timer {
	filtered := make([]execution.Timer, 0, len(timers))
	for _, timer := range timers {
		if timer.MissionID != missionID {
			continue
		}
		filtered = append(filtered, timer)
	}
	return filtered
}

// loadAttachmentContents reads text files from disk and applies the token budget.
func loadAttachmentContents(atts []attachments.Attachment, options BuildOptions) []AttachmentContent {
	totalBudget := options.AttachmentTokenBudget
	if totalBudget <= 0 {
		totalBudget = defaultAttachmentTokenBudget
	}
	maxPerFile := options.MaxSingleFileTokens
	if maxPerFile <= 0 {
		maxPerFile = defaultMaxSingleFileTokens
	}

	var contents []AttachmentContent
	tokensUsed := 0

	for _, att := range atts {
		if !att.IsTextInjectable() {
			continue
		}
		if tokensUsed >= totalBudget {
			break
		}

		data, err := os.ReadFile(att.AbsolutePath)
		if err != nil {
			continue // file may have been moved/deleted; skip gracefully
		}

		text := string(data)
		tokenCount := len(data) / 4 // rough char-to-token estimate
		truncated := false

		// Enforce per-file cap.
		if tokenCount > maxPerFile {
			byteLimit := maxPerFile * 4
			if byteLimit < len(data) {
				text = string(data[:byteLimit])
			}
			tokenCount = maxPerFile
			truncated = true
		}

		// Enforce total budget cap.
		remaining := totalBudget - tokensUsed
		if tokenCount > remaining {
			byteLimit := remaining * 4
			if byteLimit < len(text) {
				text = text[:byteLimit]
			}
			tokenCount = remaining
			truncated = true
		}

		contents = append(contents, AttachmentContent{
			AttachmentID: att.ID,
			Filename:     att.Filename,
			Category:     att.FileCategory,
			Content:      text,
			Truncated:    truncated,
			Tokens:       tokenCount,
		})
		tokensUsed += tokenCount
	}

	return contents
}

// loadImageDataURLs reads image files from disk and returns base64-encoded data URLs
// suitable for multimodal LLM input.
func loadImageDataURLs(atts []attachments.Attachment, options BuildOptions) []string {
	maxImages := options.MaxImageAttachments
	if maxImages <= 0 {
		maxImages = defaultMaxImageAttachments
	}

	var urls []string
	for _, att := range atts {
		if att.FileCategory != attachments.CategoryImage {
			continue
		}
		if len(urls) >= maxImages {
			break
		}
		if att.SizeBytes > maxImageFileSize {
			continue // skip files too large for the API
		}

		data, err := os.ReadFile(att.AbsolutePath)
		if err != nil {
			continue
		}

		mimeType := att.ContentType
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(att.Filename))
		}
		if mimeType == "" {
			mimeType = "image/png"
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		urls = append(urls, fmt.Sprintf("data:%s;base64,%s", mimeType, encoded))
	}
	return urls
}
