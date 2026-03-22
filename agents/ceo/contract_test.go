package ceo

import (
	"encoding/json"
	"testing"
	"time"
)

func resetModes(t *testing.T) {
	t.Helper()
	if err := ConfigureModes(DefaultModes()); err != nil {
		t.Fatalf("reset modes: %v", err)
	}
}

func TestNewResponseEnvelope(t *testing.T) {
	envelope, err := NewResponseEnvelope(
		"response-1",
		"thread-1",
		"trace-1",
		ModeDiscovery,
		map[string]any{"message": "hello"},
		RatingPrompt{
			Enabled:  true,
			Question: "How would you rate this response?",
			Scale:    []int{1, 2, 3, 4, 5},
		},
	)
	if err != nil {
		t.Fatalf("NewResponseEnvelope returned error: %v", err)
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["message"] != "hello" {
		t.Fatalf("unexpected payload message %q", payload["message"])
	}
}

func TestFeedbackSubmissionValidate(t *testing.T) {
	valid := FeedbackSubmission{
		ThreadID:   "thread-1",
		ResponseID: "response-1",
		Rating:     5,
		CreatedAt:  time.Now().UTC(),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid feedback, got error: %v", err)
	}

	missingReason := FeedbackSubmission{
		ThreadID:   "thread-1",
		ResponseID: "response-1",
		Rating:     3,
		CreatedAt:  time.Now().UTC(),
	}
	if err := missingReason.Validate(); err == nil {
		t.Fatal("expected error when low rating has no reason")
	}
}

func TestRequestValidateAllowsStructuredActionWithoutPrompt(t *testing.T) {
	request := Request{
		MissionID: "mission-1",
		Action:    &ActionRequest{Type: ActionCreateTodo},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("expected action-only request to validate, got error: %v", err)
	}
}

func TestModeValidate(t *testing.T) {
	resetModes(t)

	if err := ModeAlignment.Validate(); err != nil {
		t.Fatalf("expected valid mode, got error: %v", err)
	}
	if err := Mode("unknown").Validate(); err == nil {
		t.Fatal("expected invalid mode to fail validation")
	}
}

func TestConfigureModesAllowsCustomMode(t *testing.T) {
	defer resetModes(t)

	customMode := Mode("client_due_diligence")
	if err := ConfigureModes([]Mode{customMode}); err != nil {
		t.Fatalf("ConfigureModes returned error: %v", err)
	}
	if err := customMode.Validate(); err != nil {
		t.Fatalf("expected custom mode to validate, got error: %v", err)
	}
	if err := ModeDiscovery.Validate(); err == nil {
		t.Fatal("expected default mode to fail after registry replacement")
	}
}

func TestRegisterModesExtendsRegistry(t *testing.T) {
	defer resetModes(t)

	customMode := Mode("stakeholder_review")
	if err := RegisterModes(customMode); err != nil {
		t.Fatalf("RegisterModes returned error: %v", err)
	}
	if err := customMode.Validate(); err != nil {
		t.Fatalf("expected registered custom mode to validate, got error: %v", err)
	}
	if err := ModeDiscovery.Validate(); err != nil {
		t.Fatalf("expected default mode to remain valid, got error: %v", err)
	}
}

func TestConfigureModesRejectsEmptyRegistry(t *testing.T) {
	defer resetModes(t)

	if err := ConfigureModes(nil); err == nil {
		t.Fatal("expected ConfigureModes to reject an empty mode list")
	}
}
