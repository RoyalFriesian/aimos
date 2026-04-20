package ceo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

func (s *Service) SubmitFeedback(ctx context.Context, submission FeedbackSubmission) (feedback.Record, error) {
	if err := submission.Validate(); err != nil {
		return feedback.Record{}, err
	}
	if s == nil || s.feedbackStore == nil {
		return feedback.Record{}, logValidationError("failed to persist CEO feedback", errors.New("feedback store is required"))
	}

	thread, err := s.threadStore.GetThread(submission.ThreadID)
	if err != nil {
		return feedback.Record{}, logValidationError("failed to load feedback thread", err, "threadID", submission.ThreadID)
	}
	messages, err := s.threadStore.ListMessages(submission.ThreadID)
	if err != nil {
		return feedback.Record{}, logValidationError("failed to list feedback thread messages", err, "threadID", submission.ThreadID)
	}
	responseMessage, err := feedbackResponseMessage(messages, submission.ResponseID)
	if err != nil {
		return feedback.Record{}, logValidationError("failed to resolve feedback response", err, "threadID", submission.ThreadID, "responseID", submission.ResponseID)
	}
	clientMessage, hasClientMessage := feedbackClientMessage(messages, responseMessage)
	latestSummary, err := s.missionStateStore.GetLatestSummary(thread.MissionID)
	if err != nil && !errors.Is(err, missionstate.ErrSummaryNotFound) {
		return feedback.Record{}, logValidationError("failed to load feedback context summary", err, "missionID", thread.MissionID)
	}
	artifactPaths, todoRefs := extractFeedbackRefs(responseMessage.ContentJSON)
	evidenceRefs := []string{
		fmt.Sprintf("mission:%s", thread.MissionID),
		fmt.Sprintf("thread:%s", submission.ThreadID),
		fmt.Sprintf("response:%s", responseMessage.ID),
	}
	clientMessageID := ""
	clientMessageText := ""
	if hasClientMessage {
		clientMessageID = clientMessage.ID
		clientMessageText = clientMessage.Content
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("client_message:%s", clientMessage.ID))
	}
	contextSummary := ""
	if latestSummary.ID != "" {
		contextSummary = latestSummary.SummaryText
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("summary:%s", latestSummary.ID))
	}
	record := feedback.Record{
		ID:                      fmt.Sprintf("feedback-%d", submission.CreatedAt.UTC().UnixNano()),
		MissionID:               thread.MissionID,
		ThreadID:                submission.ThreadID,
		ResponseID:              submission.ResponseID,
		ClientMessageID:         clientMessageID,
		TraceID:                 strings.TrimSpace(submission.TraceID),
		Rating:                  submission.Rating,
		Reason:                  strings.TrimSpace(submission.Reason),
		Categories:              encodeStringSlice(classifyFeedbackReason(ctx, s, submission.Reason)),
		ClientMessage:           clientMessageText,
		CEOResponse:             responseMessage.Content,
		Mode:                    responseMessage.Mode,
		ArtifactPaths:           encodeStringSlice(artifactPaths),
		TodoRefs:                encodeStringSlice(todoRefs),
		ContextSummary:          contextSummary,
		EvidenceRefs:            encodeStringSlice(uniqueStrings(evidenceRefs)),
		EnrichedByFeedbackAgent: false,
		AnalysisStatus:          feedback.AnalysisStatusRaw,
		CreatedAt:               submission.CreatedAt.UTC(),
	}
	if err := s.feedbackStore.CreateFeedback(record); err != nil {
		return feedback.Record{}, logValidationError("failed to persist CEO feedback", err, "threadID", submission.ThreadID, "responseID", submission.ResponseID)
	}
	return record, nil
}

func feedbackResponseMessage(messages []threads.Message, responseID string) (threads.Message, error) {
	for _, message := range messages {
		if message.ID == responseID {
			if message.Role != threads.RoleAssistant {
				return threads.Message{}, fmt.Errorf("response %q is not an assistant message", responseID)
			}
			return message, nil
		}
	}
	return threads.Message{}, fmt.Errorf("response %q not found", responseID)
}

func feedbackClientMessage(messages []threads.Message, responseMessage threads.Message) (threads.Message, bool) {
	if responseMessage.ReplyToMessageID != "" {
		for _, message := range messages {
			if message.ID == responseMessage.ReplyToMessageID {
				return message, true
			}
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.ID == responseMessage.ID {
			for previous := index - 1; previous >= 0; previous-- {
				if messages[previous].Role == threads.RoleUser {
					return messages[previous], true
				}
			}
			break
		}
	}
	return threads.Message{}, false
}

func extractFeedbackRefs(raw json.RawMessage) ([]string, []string) {
	payload := decodeObjectMap(raw)
	artifactPaths := []string{}
	todoRefs := []string{}
	collectFeedbackRefs(payload, &artifactPaths, &todoRefs)
	return uniqueStrings(artifactPaths), uniqueStrings(todoRefs)
}

func collectFeedbackRefs(value any, artifactPaths *[]string, todoRefs *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if rawArtifactPaths, ok := typed["artifactPaths"]; ok {
			*artifactPaths = append(*artifactPaths, anyToStrings(rawArtifactPaths)...)
		}
		if rawTodoRefs, ok := typed["todoRefs"]; ok {
			*todoRefs = append(*todoRefs, anyToStrings(rawTodoRefs)...)
		}
		if todo, ok := typed["todo"].(map[string]any); ok {
			if id, ok := todo["id"].(string); ok && strings.TrimSpace(id) != "" {
				*todoRefs = append(*todoRefs, strings.TrimSpace(id))
			}
		}
		for _, nested := range typed {
			collectFeedbackRefs(nested, artifactPaths, todoRefs)
		}
	case []any:
		for _, nested := range typed {
			collectFeedbackRefs(nested, artifactPaths, todoRefs)
		}
	}
}

func anyToStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return []string{strings.TrimSpace(text)}
		}
		return []string{}
	}
	results := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			results = append(results, strings.TrimSpace(text))
		}
	}
	return results
}

func classifyFeedbackReason(ctx context.Context, s *Service, reason string) []string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return []string{}
	}
	if s != nil && s.reasoner != nil {
		task := "Classify this user feedback reason into one or more categories from this list: unclear, too_shallow, wrong_direction, missing_detail, poor_presentation, too_verbose, not_actionable, did_not_understand_business_intent. Return ONLY a comma-separated list of matching categories, nothing else. If none match, return empty."
		result, err := s.reasoner.Reason(ctx, task, trimmed)
		if err == nil {
			result = strings.TrimSpace(result)
			if result != "" && result != "empty" {
				parts := strings.Split(result, ",")
				categories := make([]string, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						categories = append(categories, p)
					}
				}
				if len(categories) > 0 {
					return uniqueStrings(categories)
				}
			}
		}
	}
	// Keyword fallback when no reasoner is available.
	lower := strings.ToLower(trimmed)
	categoryKeywords := map[string][]string{
		"unclear":                           {"unclear", "confusing", "vague", "ambiguous", "hard to understand"},
		"too_shallow":                       {"shallow", "surface", "not enough depth", "too brief", "superficial"},
		"wrong_direction":                   {"wrong direction", "off track", "different direction", "not what i asked", "misunderstood"},
		"missing_detail":                    {"missing detail", "not enough detail", "lacks detail", "more detail", "incomplete"},
		"poor_presentation":                 {"presentation", "formatting", "layout", "ugly", "hard to read"},
		"too_verbose":                       {"verbose", "too long", "too much", "wordy", "rambling"},
		"not_actionable":                    {"not actionable", "can't act on", "cannot act", "no next step", "what do i do"},
		"did_not_understand_business_intent": {"business intent", "business goal", "purpose", "why", "objective"},
	}
	var categories []string
	for category, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				categories = append(categories, category)
				break
			}
		}
	}
	if len(categories) > 0 {
		return uniqueStrings(categories)
	}
	return []string{}
}

func encodeStringSlice(values []string) json.RawMessage {
	encoded, err := json.Marshal(uniqueStrings(values))
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return encoded
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	results := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		results = append(results, trimmed)
	}
	return results
}

var _ = time.RFC3339
