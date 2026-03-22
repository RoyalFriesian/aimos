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

func (s *Service) SubmitFeedback(_ context.Context, submission FeedbackSubmission) (feedback.Record, error) {
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
		Categories:              encodeStringSlice(classifyFeedbackReason(submission.Reason)),
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

func classifyFeedbackReason(reason string) []string {
	trimmed := strings.ToLower(strings.TrimSpace(reason))
	if trimmed == "" {
		return []string{}
	}
	categories := []string{}
	if strings.Contains(trimmed, "unclear") || strings.Contains(trimmed, "confus") || strings.Contains(trimmed, "follow") {
		categories = append(categories, "unclear")
	}
	if strings.Contains(trimmed, "shallow") || strings.Contains(trimmed, "surface") || strings.Contains(trimmed, "deeper") {
		categories = append(categories, "too_shallow")
	}
	if strings.Contains(trimmed, "wrong direction") || strings.Contains(trimmed, "wrong") || strings.Contains(trimmed, "off target") {
		categories = append(categories, "wrong_direction")
	}
	if strings.Contains(trimmed, "detail") || strings.Contains(trimmed, "specific") || strings.Contains(trimmed, "missing") {
		categories = append(categories, "missing_detail")
	}
	if strings.Contains(trimmed, "present") || strings.Contains(trimmed, "format") || strings.Contains(trimmed, "structure") {
		categories = append(categories, "poor_presentation")
	}
	if strings.Contains(trimmed, "verbose") || strings.Contains(trimmed, "too long") {
		categories = append(categories, "too_verbose")
	}
	if strings.Contains(trimmed, "actionable") || strings.Contains(trimmed, "next step") {
		categories = append(categories, "not_actionable")
	}
	if strings.Contains(trimmed, "business") || strings.Contains(trimmed, "intent") || strings.Contains(trimmed, "goal") {
		categories = append(categories, "did_not_understand_business_intent")
	}
	return uniqueStrings(categories)
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
