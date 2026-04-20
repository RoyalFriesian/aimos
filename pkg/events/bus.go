package events

import (
	"encoding/json"
	"sync"
	"time"
)

// EventType identifies what happened.
type EventType string

const (
	EventMissionStarted   EventType = "mission.started"
	EventMissionCompleted EventType = "mission.completed"
	EventMissionBlocked   EventType = "mission.blocked"
	EventMissionFailed    EventType = "mission.failed"
	EventTodoCreated      EventType = "todo.created"
	EventTodoCompleted    EventType = "todo.completed"
	EventTodoBlocked      EventType = "todo.blocked"
	EventTimerTriggered   EventType = "timer.triggered"
	EventAgentStarted     EventType = "agent.started"
	EventAgentStopped     EventType = "agent.stopped"
	EventAgentHeartbeat   EventType = "agent.heartbeat"
	EventTeamApproved     EventType = "team.approved"
	EventMessageSent      EventType = "message.sent"
)

// Event is a single occurrence in the system.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	MissionID string          `json:"missionId,omitempty"`
	ThreadID  string          `json:"threadId,omitempty"`
	AgentID   string          `json:"agentId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// Subscriber receives events that match its filter.
type Subscriber struct {
	ID     string
	Filter func(Event) bool
	ch     chan Event
}

// Bus is an in-process pub/sub event bus.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	history     []Event
	historyMax  int
}

// NewBus creates a new event bus. historyMax controls how many events are retained (0 = unlimited).
func NewBus(historyMax int) *Bus {
	return &Bus{
		subscribers: make(map[string]*Subscriber),
		historyMax:  historyMax,
	}
}

// Subscribe adds a subscriber. Events matching the filter are sent to the returned channel.
// If filter is nil, all events are received.
func (b *Bus) Subscribe(id string, filter func(Event) bool, bufferSize int) <-chan Event {
	if bufferSize < 1 {
		bufferSize = 64
	}
	ch := make(chan Event, bufferSize)
	sub := &Subscriber{
		ID:     id,
		Filter: filter,
		ch:     ch,
	}
	b.mu.Lock()
	b.subscribers[id] = sub
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	sub, exists := b.subscribers[id]
	if exists {
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
	if sub != nil {
		close(sub.ch)
	}
}

// Publish sends an event to all matching subscribers.
func (b *Bus) Publish(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	b.mu.Lock()
	b.history = append(b.history, event)
	if b.historyMax > 0 && len(b.history) > b.historyMax {
		b.history = b.history[len(b.history)-b.historyMax:]
	}
	subs := make([]*Subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	for _, sub := range subs {
		if sub.Filter != nil && !sub.Filter(event) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			// Drop event if subscriber is not keeping up.
		}
	}
}

// RecentEvents returns up to `limit` recent events, newest first.
func (b *Bus) RecentEvents(limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := len(b.history)
	if limit <= 0 || limit > n {
		limit = n
	}
	result := make([]Event, limit)
	for i := 0; i < limit; i++ {
		result[i] = b.history[n-1-i]
	}
	return result
}

// RecentEventsByMission returns recent events filtered by mission ID.
func (b *Bus) RecentEventsByMission(missionID string, limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]Event, 0, limit)
	for i := len(b.history) - 1; i >= 0 && len(result) < limit; i-- {
		if b.history[i].MissionID == missionID {
			result = append(result, b.history[i])
		}
	}
	return result
}
