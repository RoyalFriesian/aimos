package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMessage_ValidFields(t *testing.T) {
	msg, err := NewMessage("ceo", "worker-1", "task-1", "thread-1", "trace-1", MessageTypeCommand, "execute_mission", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MessageID == "" {
		t.Error("expected generated message ID")
	}
	if msg.Type != MessageTypeCommand {
		t.Errorf("expected type=command, got %q", msg.Type)
	}
	if msg.Name != "execute_mission" {
		t.Errorf("expected name=execute_mission, got %q", msg.Name)
	}
	if err := msg.Validate(); err != nil {
		t.Errorf("valid message failed validation: %v", err)
	}
}

func TestNewMessage_MissingFields(t *testing.T) {
	tests := []struct {
		name                                     string
		from, taskID, threadID, traceID, msgName string
		msgType                                  MessageType
	}{
		{"missing from", "", "t", "th", "tr", "n", MessageTypeCommand},
		{"missing taskID", "f", "", "th", "tr", "n", MessageTypeCommand},
		{"missing threadID", "f", "t", "", "tr", "n", MessageTypeCommand},
		{"missing traceID", "f", "t", "th", "", "n", MessageTypeCommand},
		{"missing type", "f", "t", "th", "tr", "n", ""},
		{"missing name", "f", "t", "th", "tr", "", MessageTypeCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMessage(tt.from, "to", tt.taskID, tt.threadID, tt.traceID, tt.msgType, tt.msgName, nil)
			if err == nil {
				t.Fatal("expected error for missing field")
			}
		})
	}
}

func TestSuccessResponse(t *testing.T) {
	msg := Message{MessageID: "msg-1"}
	resp := SuccessResponse(msg, map[string]string{"result": "ok"})
	if resp.MessageID != "msg-1" {
		t.Errorf("expected messageId=msg-1, got %q", resp.MessageID)
	}
	if resp.Status != "success" {
		t.Errorf("expected status=success, got %q", resp.Status)
	}
}

func TestErrorResponse(t *testing.T) {
	msg := Message{MessageID: "msg-2"}
	resp := ErrorResponse(msg, context.DeadlineExceeded)
	if resp.Status != "error" {
		t.Errorf("expected status=error, got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestInProcessTransport_SendSuccess(t *testing.T) {
	transport := NewInProcessTransport()
	transport.Register("worker-1", HandlerFunc(func(_ context.Context, msg Message) (Response, error) {
		return SuccessResponse(msg, map[string]string{"handled": "true"}), nil
	}))

	msg, err := NewMessage("ceo", "worker-1", "task-1", "thread-1", "trace-1", MessageTypeCommand, "execute", nil)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	resp, err := transport.Send(context.Background(), AgentEndpoint{AgentID: "worker-1", InProc: true}, msg)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
}

func TestInProcessTransport_SendUnregistered(t *testing.T) {
	transport := NewInProcessTransport()
	msg, _ := NewMessage("ceo", "unknown", "task-1", "thread-1", "trace-1", MessageTypeCommand, "ping", nil)
	_, err := transport.Send(context.Background(), AgentEndpoint{AgentID: "unknown"}, msg)
	if err == nil {
		t.Fatal("expected error for unregistered agent")
	}
}

func TestHTTPTransport_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/message" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}
		if r.Header.Get("X-A2A-From") == "" {
			t.Error("expected X-A2A-From header")
		}
		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		resp := SuccessResponse(msg, map[string]bool{"received": true})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transport := NewHTTPTransport()
	msg, _ := NewMessage("ceo", "worker-1", "task-1", "thread-1", "trace-1", MessageTypeCommand, "execute", nil)
	resp, err := transport.Send(context.Background(), AgentEndpoint{AgentID: "worker-1", HTTPAddr: server.URL}, msg)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
}

func TestHTTPTransport_NoAddr(t *testing.T) {
	transport := NewHTTPTransport()
	msg, _ := NewMessage("ceo", "worker-1", "task-1", "thread-1", "trace-1", MessageTypeCommand, "ping", nil)
	_, err := transport.Send(context.Background(), AgentEndpoint{AgentID: "worker-1"}, msg)
	if err == nil {
		t.Fatal("expected error for empty HTTP address")
	}
}
