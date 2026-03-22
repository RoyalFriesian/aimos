package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/agents/ceo"
	"github.com/Sarnga/agent-platform/pkg/feedback"
)

type stubCEOService struct {
	respondFunc        func(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error)
	submitFeedbackFunc func(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error)
}

func (s *stubCEOService) Respond(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error) {
	return s.respondFunc(ctx, request)
}

func (s *stubCEOService) SubmitFeedback(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error) {
	return s.submitFeedbackFunc(ctx, submission)
}

func TestServerRespondEndpoint(t *testing.T) {
	server, err := NewServer(&stubCEOService{
		respondFunc: func(_ context.Context, request ceo.Request) (ceo.ResponseEnvelope, error) {
			if request.Prompt != "Help me plan." {
				t.Fatalf("unexpected prompt %#v", request)
			}
			return ceo.ResponseEnvelope{
				ResponseID: "response-1",
				ThreadID:   "thread-1",
				TraceID:    "trace-1",
				Mode:       ceo.ModeDiscovery,
				Payload:    json.RawMessage(`{"message":"hello"}`),
				RatingPrompt: ceo.RatingPrompt{
					Enabled:  true,
					Question: "How would you rate this response?",
					Scale:    []int{1, 2, 3, 4, 5},
				},
				CreatedAt: time.Now().UTC(),
			}, nil
		},
		submitFeedbackFunc: func(_ context.Context, _ ceo.FeedbackSubmission) (feedback.Record, error) {
			return feedback.Record{}, fmt.Errorf("unexpected feedback call")
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	body := bytes.NewBufferString(`{"prompt":"Help me plan.","threadId":"thread-1","traceId":"trace-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/ceo/respond", body)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with %s", response.Code, response.Body.String())
	}
	var payload ceo.ResponseEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.ResponseID != "response-1" {
		t.Fatalf("unexpected response payload %#v", payload)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected CORS header, got %q", got)
	}
}

func TestServerFeedbackEndpoint(t *testing.T) {
	server, err := NewServer(&stubCEOService{
		respondFunc: func(_ context.Context, _ ceo.Request) (ceo.ResponseEnvelope, error) {
			return ceo.ResponseEnvelope{}, fmt.Errorf("unexpected respond call")
		},
		submitFeedbackFunc: func(_ context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error) {
			if submission.ResponseID != "response-1" || submission.Rating != 5 {
				t.Fatalf("unexpected submission %#v", submission)
			}
			return feedback.Record{ID: "feedback-1", ThreadID: submission.ThreadID, ResponseID: submission.ResponseID, Rating: submission.Rating, CreatedAt: submission.CreatedAt}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	body := bytes.NewBufferString(`{"threadId":"thread-1","responseId":"response-1","rating":5,"createdAt":"2026-03-21T00:00:00Z"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/ceo/feedback", body)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with %s", response.Code, response.Body.String())
	}
	var payload feedback.Record
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.ID != "feedback-1" {
		t.Fatalf("unexpected feedback payload %#v", payload)
	}
}

func TestServerValidationError(t *testing.T) {
	server, err := NewServer(&stubCEOService{
		respondFunc: func(_ context.Context, _ ceo.Request) (ceo.ResponseEnvelope, error) {
			return ceo.ResponseEnvelope{}, fmt.Errorf("prompt is required")
		},
		submitFeedbackFunc: func(_ context.Context, _ ceo.FeedbackSubmission) (feedback.Record, error) {
			return feedback.Record{}, fmt.Errorf("unexpected feedback call")
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/ceo/respond", bytes.NewBufferString(`{}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with %s", response.Code, response.Body.String())
	}
}

func TestServerHealthz(t *testing.T) {
	server, err := NewServer(&stubCEOService{
		respondFunc: func(_ context.Context, _ ceo.Request) (ceo.ResponseEnvelope, error) {
			return ceo.ResponseEnvelope{}, nil
		},
		submitFeedbackFunc: func(_ context.Context, _ ceo.FeedbackSubmission) (feedback.Record, error) {
			return feedback.Record{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
}
