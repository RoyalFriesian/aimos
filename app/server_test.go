package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sarnga/agent-platform/agents/ceo"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

type stubCEOService struct {
	respondFunc        func(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error)
	submitFeedbackFunc func(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error)
	uploadFunc         func(ctx context.Context, threadID string, projectLocation string, files []ceo.ProjectAttachmentInput) ([]ceo.StoredProjectAttachment, error)
}

func (s *stubCEOService) Respond(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error) {
	return s.respondFunc(ctx, request)
}

func (s *stubCEOService) SubmitFeedback(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error) {
	return s.submitFeedbackFunc(ctx, submission)
}

func (s *stubCEOService) GenerateProjectName(_ context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	return "Generated Project", nil
}

func (s *stubCEOService) ListOpenAIModels(_ context.Context) ([]string, error) {
	return []string{"gpt-5.4", "gpt-4.1-mini"}, nil
}

func (s *stubCEOService) UploadProjectAttachments(ctx context.Context, threadID string, projectLocation string, files []ceo.ProjectAttachmentInput) ([]ceo.StoredProjectAttachment, error) {
	if s.uploadFunc != nil {
		return s.uploadFunc(ctx, threadID, projectLocation, files)
	}
	return []ceo.StoredProjectAttachment{}, nil
}

func (s *stubCEOService) RenameProject(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *stubCEOService) ListRootThreads(_ context.Context) ([]threads.Thread, error) {
	return []threads.Thread{}, nil
}

func (s *stubCEOService) LoadProject(_ context.Context, _ string) ([]threads.Thread, map[string][]threads.Message, error) {
	return []threads.Thread{}, map[string][]threads.Message{}, nil
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

func TestServerUploadProjectAttachmentsSuccess(t *testing.T) {
	server, err := NewServer(&stubCEOService{
		respondFunc: func(_ context.Context, _ ceo.Request) (ceo.ResponseEnvelope, error) {
			return ceo.ResponseEnvelope{}, nil
		},
		submitFeedbackFunc: func(_ context.Context, _ ceo.FeedbackSubmission) (feedback.Record, error) {
			return feedback.Record{}, nil
		},
		uploadFunc: func(_ context.Context, threadID string, projectLocation string, files []ceo.ProjectAttachmentInput) ([]ceo.StoredProjectAttachment, error) {
			if threadID != "thread-1" {
				t.Fatalf("unexpected threadID: %s", threadID)
			}
			if projectLocation != "/tmp/demo" {
				t.Fatalf("unexpected projectLocation: %s", projectLocation)
			}
			if len(files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(files))
			}
			if files[0].Filename != "requirements.md" {
				t.Fatalf("unexpected filename: %s", files[0].Filename)
			}
			if string(files[0].Data) != "project requirements" {
				t.Fatalf("unexpected file data: %s", string(files[0].Data))
			}
			now := time.Now().UTC()
			return []ceo.StoredProjectAttachment{{
				Filename:     "requirements.md",
				ContentType:  "text/markdown",
				SizeBytes:    int64(len(files[0].Data)),
				RelativePath: "attachments/requirements.md",
				AbsolutePath: "/tmp/demo/attachments/requirements.md",
				UploadedAt:   now,
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("threadId", "thread-1"); err != nil {
		t.Fatalf("write threadId field: %v", err)
	}
	if err := writer.WriteField("projectLocation", "/tmp/demo"); err != nil {
		t.Fatalf("write projectLocation field: %v", err)
	}
	part, err := writer.CreateFormFile("files", "requirements.md")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("project requirements")); err != nil {
		t.Fatalf("write file payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/projects/attachments/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with %s", response.Code, response.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["threadId"] != "thread-1" {
		t.Fatalf("unexpected threadId in payload: %#v", payload)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("unexpected count in payload: %#v", payload)
	}
}

func TestServerUploadProjectAttachmentsValidationFailure(t *testing.T) {
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("threadId", "thread-1"); err != nil {
		t.Fatalf("write threadId field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/projects/attachments/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with %s", response.Code, response.Body.String())
	}
}

func TestServerPickProjectLocationSuccess(t *testing.T) {
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

	server.pickProjectLocation = func() (string, error) {
		return "/Users/demo/projects/sample", nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/system/project-location/pick", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with %s", response.Code, response.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["path"] != "/Users/demo/projects/sample" {
		t.Fatalf("unexpected picker path payload: %#v", payload)
	}
}
