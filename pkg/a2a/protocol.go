package a2a

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MessageType identifies the kind of A2A message.
type MessageType string

const (
	MessageTypeCommand MessageType = "command"
	MessageTypeQuery   MessageType = "query"
	MessageTypeEvent   MessageType = "event"
)

// Message is the core A2A communication envelope.
type Message struct {
	MessageID  string            `json:"messageId"`
	From       string            `json:"from"`
	To         string            `json:"to,omitempty"`
	Capability string            `json:"capability,omitempty"`
	TaskID     string            `json:"taskId"`
	ThreadID   string            `json:"threadId"`
	TraceID    string            `json:"traceId"`
	Type       MessageType       `json:"type"`
	Name       string            `json:"name"`
	Payload    any               `json:"payload,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}

// Response is the standard A2A reply envelope.
type Response struct {
	MessageID string    `json:"messageId"`
	Status    string    `json:"status"` // "success" or "error"
	Payload   any       `json:"payload,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentEndpoint describes how to reach an agent.
type AgentEndpoint struct {
	AgentID  string `json:"agentId"`
	InProc   bool   `json:"inProc,omitempty"`
	HTTPAddr string `json:"httpAddr,omitempty"`
	GRPCAddr string `json:"grpcAddr,omitempty"`
}

// NewMessage creates and validates a new A2A message with a generated ID.
func NewMessage(from, to, taskID, threadID, traceID string, msgType MessageType, name string, payload any) (Message, error) {
	msg := Message{
		MessageID: uuid.New().String(),
		From:      from,
		To:        to,
		TaskID:    taskID,
		ThreadID:  threadID,
		TraceID:   traceID,
		Type:      msgType,
		Name:      name,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	if err := msg.Validate(); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// Validate checks that all required fields are present.
func (m Message) Validate() error {
	var missing []string
	if strings.TrimSpace(m.From) == "" {
		missing = append(missing, "from")
	}
	if strings.TrimSpace(m.TaskID) == "" {
		missing = append(missing, "taskId")
	}
	if strings.TrimSpace(m.ThreadID) == "" {
		missing = append(missing, "threadId")
	}
	if strings.TrimSpace(m.TraceID) == "" {
		missing = append(missing, "traceId")
	}
	if strings.TrimSpace(string(m.Type)) == "" {
		missing = append(missing, "type")
	}
	if strings.TrimSpace(m.Name) == "" {
		missing = append(missing, "name")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required A2A fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// SuccessResponse creates a successful A2A response.
func SuccessResponse(msg Message, payload any) Response {
	return Response{
		MessageID: msg.MessageID,
		Status:    "success",
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
}

// ErrorResponse creates an error A2A response.
func ErrorResponse(msg Message, err error) Response {
	errMsg := "unknown error"
	if err != nil && !errors.Is(err, nil) {
		errMsg = err.Error()
	}
	return Response{
		MessageID: msg.MessageID,
		Status:    "error",
		Error:     errMsg,
		Timestamp: time.Now().UTC(),
	}
}
