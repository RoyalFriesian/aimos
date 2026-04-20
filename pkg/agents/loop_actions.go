package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/microai"
)

// ActionType enumerates the structured actions an agent loop LLM turn can produce.
type ActionType string

const (
	ActionPostMessage      ActionType = "post_message"
	ActionCheckChild       ActionType = "check_child"
	ActionCreateWorker     ActionType = "create_worker"
	ActionCompleteTodo     ActionType = "complete_todo"
	ActionBlockTodo        ActionType = "block_todo"
	ActionStartTodo        ActionType = "start_todo"
	ActionUpdateSummary    ActionType = "update_summary"
	ActionEscalate         ActionType = "escalate"
	ActionResolveConflict  ActionType = "resolve_conflict"
	ActionScheduleFollowup ActionType = "schedule_followup"
	ActionMarkDone         ActionType = "mark_done"
	ActionDeliverWork      ActionType = "deliver_work"
	ActionWriteFile        ActionType = "write_file"
	ActionReadFile         ActionType = "read_file"
	ActionMergeBranch      ActionType = "merge_branch"
	ActionRunQA            ActionType = "run_qa"
	ActionNoOp             ActionType = "no_op"
)

// LoopAction is a single structured action parsed from an LLM response.
type LoopAction struct {
	Type    ActionType      `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// PostMessagePayload is the payload for ActionPostMessage.
type PostMessagePayload struct {
	TargetThreadID string `json:"targetThreadId"`
	Content        string `json:"content"`
	Message        string `json:"message"`
	Text           string `json:"text"`
	ChildAgentID   string `json:"childAgentId"`
	AgentID        string `json:"agentId"`
}

// GetContent returns the actual message content, checking fallback fields.
func (p PostMessagePayload) GetContent() string {
	if p.Content != "" {
		return p.Content
	}
	if p.Message != "" {
		return p.Message
	}
	return p.Text
}

// GetTargetAgentID returns the child/agent ID from whichever field the LLM used.
func (p PostMessagePayload) GetTargetAgentID() string {
	if p.ChildAgentID != "" {
		return p.ChildAgentID
	}
	return p.AgentID
}

// CheckChildPayload is the payload for ActionCheckChild.
type CheckChildPayload struct {
	ChildAgentID string `json:"childAgentId"`
	AgentID      string `json:"agentId"`
	Question     string `json:"question"`
	Message      string `json:"message"`
	Content      string `json:"content"`
}

// GetChildAgentID returns child agent id from whichever field the LLM used.
func (p CheckChildPayload) GetChildAgentID() string {
	if p.ChildAgentID != "" {
		return p.ChildAgentID
	}
	return p.AgentID
}

// GetQuestion returns the question/message from whichever field the LLM used.
func (p CheckChildPayload) GetQuestion() string {
	if p.Question != "" {
		return p.Question
	}
	if p.Message != "" {
		return p.Message
	}
	return p.Content
}

// CreateWorkerPayload is the payload for ActionCreateWorker.
type CreateWorkerPayload struct {
	Name             string `json:"name"`
	Role             string `json:"role"`
	ProblemStatement string `json:"problemStatement"`
	// LLM snake_case and alternative field aliases
	ProblemStatementAlt string `json:"problem_statement"`
	Task                string `json:"task"`
	Description         string `json:"description"`
	Mission             string `json:"mission"`
	MissionTitle        string `json:"missionTitle"`
	MissionTitleAlt     string `json:"mission_title"`
	TodoTitle           string `json:"todoTitle"`
	TodoTitleAlt        string `json:"todo_title"`
	TodoDescription     string `json:"todoDescription"`
	TodoDescriptionAlt  string `json:"todo_description"`
}

// GetProblemStatement returns the problem statement, checking all fallback fields.
func (p CreateWorkerPayload) GetProblemStatement() string {
	if p.ProblemStatement != "" {
		return p.ProblemStatement
	}
	if p.ProblemStatementAlt != "" {
		return p.ProblemStatementAlt
	}
	if p.Task != "" {
		return p.Task
	}
	if p.Description != "" {
		return p.Description
	}
	if p.MissionTitle != "" {
		return p.MissionTitle
	}
	if p.MissionTitleAlt != "" {
		return p.MissionTitleAlt
	}
	if p.Mission != "" {
		return p.Mission
	}
	return p.Name // last resort: use the worker name
}

// GetTodoTitle returns the todo title, checking fallback fields.
func (p CreateWorkerPayload) GetTodoTitle() string {
	if p.TodoTitle != "" {
		return p.TodoTitle
	}
	return p.TodoTitleAlt
}

// GetTodoDescription returns the todo description, checking fallback fields.
func (p CreateWorkerPayload) GetTodoDescription() string {
	if p.TodoDescription != "" {
		return p.TodoDescription
	}
	return p.TodoDescriptionAlt
}

// TodoActionPayload is the payload for ActionCompleteTodo, ActionBlockTodo, ActionStartTodo.
type TodoActionPayload struct {
	TodoID   string `json:"todoId"`
	TodoIDv2 string `json:"todo_id"` // LLM snake_case alias
	ID       string `json:"id"`      // LLM alias
	Reason   string `json:"reason,omitempty"`
}

// GetTodoID returns the todo ID, checking fallback fields.
func (p TodoActionPayload) GetTodoID() string {
	if p.TodoID != "" {
		return p.TodoID
	}
	if p.TodoIDv2 != "" {
		return p.TodoIDv2
	}
	return p.ID
}

// EscalatePayload is the payload for ActionEscalate.
type EscalatePayload struct {
	Reason    string `json:"reason"`
	SiblingID string `json:"siblingId,omitempty"`
}

// ResolveConflictPayload is the payload for ActionResolveConflict.
type ResolveConflictPayload struct {
	TargetChildID string `json:"targetChildId"`
	Resolution    string `json:"resolution"`
}

// DeliverWorkPayload is the payload for ActionDeliverWork.
type DeliverWorkPayload struct {
	TodoID      string  `json:"todoId"`
	TodoIDv2    string  `json:"todo_id"` // LLM snake_case alias
	Deliverable string  `json:"deliverable"`
	Content     string  `json:"content"`
	Result      string  `json:"result"`
	Output      string  `json:"output"`
	Summary     string  `json:"summary"`  // LLM alias
	Text        string  `json:"text"`     // LLM alias
	Details     string  `json:"details"`  // LLM alias
	Message     string  `json:"message"`  // LLM alias
	Confidence  float64 `json:"confidence"`
}

// GetTodoID returns the todo ID, checking fallback fields.
func (p DeliverWorkPayload) GetTodoID() string {
	if p.TodoID != "" {
		return p.TodoID
	}
	return p.TodoIDv2
}

// GetDeliverable returns the actual deliverable content, checking fallback fields.
func (p DeliverWorkPayload) GetDeliverable() string {
	if p.Deliverable != "" {
		return p.Deliverable
	}
	if p.Content != "" {
		return p.Content
	}
	if p.Result != "" {
		return p.Result
	}
	if p.Output != "" {
		return p.Output
	}
	if p.Summary != "" {
		return p.Summary
	}
	if p.Text != "" {
		return p.Text
	}
	if p.Details != "" {
		return p.Details
	}
	return p.Message
}

// WriteFilePayload is the payload for ActionWriteFile.
type WriteFilePayload struct {
	FilePath   string          `json:"filePath"`
	Content    json.RawMessage `json:"content"`
	Path       string          `json:"path"`     // LLM alias
	Code       string          `json:"code"`     // LLM alias
	Body       string          `json:"body"`     // LLM alias
	FileName   string          `json:"fileName"` // LLM alias
	Filename   string          `json:"filename"` // LLM alias
}

// GetFilePath returns the file path, checking fallback fields.
func (p WriteFilePayload) GetFilePath() string {
	if p.FilePath != "" {
		return p.FilePath
	}
	if p.Path != "" {
		return p.Path
	}
	if p.FileName != "" {
		return p.FileName
	}
	return p.Filename
}

// GetContent returns the file content, checking fallback fields.
// Handles both string and object values — when the LLM returns a JSON object
// (e.g. for package.json), it is pretty-printed as a string.
func (p WriteFilePayload) GetContent() string {
	if len(p.Content) > 0 {
		// Try as a plain string.
		var s string
		if err := json.Unmarshal(p.Content, &s); err == nil && s != "" {
			return s
		}
		// Try as a JSON object — pretty-print it.
		raw := strings.TrimSpace(string(p.Content))
		if raw != "" && raw != "null" {
			var obj any
			if err := json.Unmarshal(p.Content, &obj); err == nil {
				if pretty, err := json.MarshalIndent(obj, "", "  "); err == nil {
					return string(pretty)
				}
			}
			return raw
		}
	}
	if p.Code != "" {
		return p.Code
	}
	return p.Body
}

// ReadFilePayload is the payload for ActionReadFile.
type ReadFilePayload struct {
	FilePath string `json:"filePath"`
	Path     string `json:"path"`     // LLM alias
	FileName string `json:"fileName"` // LLM alias
	Filename string `json:"filename"` // LLM alias
}

// GetFilePath returns the file path, checking fallback fields.
func (p ReadFilePayload) GetFilePath() string {
	if p.FilePath != "" {
		return p.FilePath
	}
	if p.Path != "" {
		return p.Path
	}
	if p.FileName != "" {
		return p.FileName
	}
	return p.Filename
}

// MergeBranchPayload is the payload for ActionMergeBranch.
type MergeBranchPayload struct {
	WorkerName string `json:"workerName"`
	AgentID    string `json:"agentId"`
	BranchName string `json:"branchName"`
}

// GetWorkerName returns the worker identifier, checking fallback fields.
func (p MergeBranchPayload) GetWorkerName() string {
	if p.WorkerName != "" {
		return p.WorkerName
	}
	if p.AgentID != "" {
		return p.AgentID
	}
	return p.BranchName
}

// RunQAPayload is the payload for ActionRunQA.
type RunQAPayload struct {
	Scope  string `json:"scope"`  // "full" or specific area
	Reason string `json:"reason"` // why QA is being triggered
}

// QAValidationPayload is the input for QA agent validation of the integrated codebase.
type QAValidationPayload struct {
	ProjectDir  string   `json:"projectDir"`
	MissionGoal string   `json:"missionGoal"`
	FileList    string   `json:"fileList"`
	Issues      []string `json:"priorIssues,omitempty"` // issues from merge failures
}

// LoopTurnResponse is the full structured response from an agent loop LLM turn.
type LoopTurnResponse struct {
	Thinking json.RawMessage `json:"thinking"`
	Summary  json.RawMessage `json:"summary"`
	Actions  []LoopAction    `json:"actions"`
}

// UnmarshalJSON handles LLM responses where the actions array may contain
// strings instead of objects (e.g. ["start_todo"] instead of [{"type":"start_todo","payload":{}}]).
func (r *LoopTurnResponse) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion.
	type Alias struct {
		Thinking json.RawMessage   `json:"thinking"`
		Summary  json.RawMessage   `json:"summary"`
		Actions  json.RawMessage   `json:"actions"`
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	r.Thinking = alias.Thinking
	r.Summary = alias.Summary

	// Try parsing actions as []LoopAction first.
	if len(alias.Actions) > 0 {
		var actions []LoopAction
		if err := json.Unmarshal(alias.Actions, &actions); err != nil {
			// Fallback: parse as array of raw items and handle mixed types.
			var rawItems []json.RawMessage
			if err2 := json.Unmarshal(alias.Actions, &rawItems); err2 != nil {
				// Not an array at all — might be a single object.
				var single LoopAction
				if err3 := json.Unmarshal(alias.Actions, &single); err3 == nil {
					r.Actions = []LoopAction{single}
					return nil
				}
				return err
			}
			for _, item := range rawItems {
				trimmed := strings.TrimSpace(string(item))
				// If the item is a string, treat it as an action type with empty payload.
				var str string
				if json.Unmarshal(item, &str) == nil {
					r.Actions = append(r.Actions, LoopAction{
						Type:    ActionType(str),
						Payload: json.RawMessage("{}"),
					})
					continue
				}
				// Otherwise it should be an action object.
				var action LoopAction
				if json.Unmarshal(item, &action) == nil {
					r.Actions = append(r.Actions, action)
					continue
				}
				// Skip unparseable items but log for awareness.
				_ = trimmed
			}
		} else {
			r.Actions = actions
		}
	}
	return nil
}

// GetThinking returns the thinking field as a string, handling both string
// and object values from the LLM.
func (r LoopTurnResponse) GetThinking() string {
	return rawMessageToString(r.Thinking)
}

// GetSummary returns the summary field as a string, handling both string
// and object values from the LLM.
func (r LoopTurnResponse) GetSummary() string {
	return rawMessageToString(r.Summary)
}

// rawMessageToString extracts a string from a json.RawMessage that may be
// either a JSON string or an arbitrary JSON object.
func rawMessageToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// knownActionPrefixes maps the canonical action type string to its ActionType
// constant. Used by NormalizeActionType to detect when the LLM embeds a
// description or JSON payload in the type field (e.g. "create_worker: build the API").
var knownActionPrefixes = map[string]ActionType{
	"post_message":      ActionPostMessage,
	"check_child":       ActionCheckChild,
	"create_worker":     ActionCreateWorker,
	"complete_todo":     ActionCompleteTodo,
	"block_todo":        ActionBlockTodo,
	"start_todo":        ActionStartTodo,
	"update_summary":    ActionUpdateSummary,
	"escalate":          ActionEscalate,
	"resolve_conflict":  ActionResolveConflict,
	"schedule_followup": ActionScheduleFollowup,
	"mark_done":         ActionMarkDone,
	"deliver_work":      ActionDeliverWork,
	"write_file":        ActionWriteFile,
	"read_file":         ActionReadFile,
	"merge_branch":      ActionMergeBranch,
	"run_qa":            ActionRunQA,
	"no_op":             ActionNoOp,
}

// NormalizeActionType maps LLM-invented action names to canonical action types.
// Uses a small exact-match alias map for known variants, then falls back to AI
// classification for anything unknown.
func NormalizeActionType(ctx context.Context, reasoner microai.Interface, raw ActionType) ActionType {
	// Fast path: exact match against canonical names.
	rawStr := string(raw)
	if _, ok := knownActionPrefixes[rawStr]; ok {
		return raw
	}

	// Fast path: known aliases.
	if mapped, ok := actionAliases[strings.ToLower(strings.TrimSpace(rawStr))]; ok {
		return mapped
	}

	// Prefix stripping: if the raw string starts with a known action prefix
	// followed by a separator (colon, dash, space), extract just the prefix.
	// Common LLM pattern: "create_worker: build the auth module".
	lowerStr := strings.ToLower(rawStr)
	for prefix, at := range knownActionPrefixes {
		if !strings.HasPrefix(lowerStr, prefix) || len(lowerStr) == len(prefix) {
			continue
		}
		sep := lowerStr[len(prefix)]
		if sep == ':' || sep == '-' || sep == ' ' || sep == '_' {
			return at
		}
	}

	// AI fallback for unknown or sentence-like action types.
	if reasoner != nil {
		canonicalList := "post_message, check_child, create_worker, complete_todo, block_todo, start_todo, update_summary, escalate, resolve_conflict, schedule_followup, mark_done, deliver_work, write_file, read_file, merge_branch, run_qa, no_op"
		task := "You are an action type classifier. Given a raw action string from an LLM, return the single best matching canonical action type from this list: " + canonicalList + ". If none match, return the input unchanged. Respond with ONLY the action type, nothing else."
		result, err := reasoner.Reason(ctx, task, rawStr)
		if err == nil {
			result = strings.TrimSpace(strings.Trim(result, "\"'` \n"))
			if result != "" {
				return ActionType(result)
			}
		}
	}

	return raw
}

// actionAliases maps common LLM-invented names to canonical types.
var actionAliases = map[string]ActionType{
	"message_child":    ActionPostMessage,
	"send_message":     ActionPostMessage,
	"message_agent":    ActionPostMessage,
	"send_to_child":    ActionPostMessage,
	"broadcast":        ActionPostMessage,
	"update_plan":      ActionUpdateSummary,
	"revise_plan":      ActionUpdateSummary,
	"create_sub_worker": ActionCreateWorker,
	"delegate":         ActionCreateWorker,
	"hire_worker":      ActionCreateWorker,
	"spawn_worker":     ActionCreateWorker,
	"done":             ActionMarkDone,
	"complete":         ActionMarkDone,
	"finish":           ActionMarkDone,
	"create_file":      ActionWriteFile,
	"save_file":        ActionWriteFile,
	"output_file":      ActionWriteFile,
	"get_file":         ActionReadFile,
	"open_file":        ActionReadFile,
	"load_file":        ActionReadFile,
	"cat_file":         ActionReadFile,
	"view_file":        ActionReadFile,
	"merge":            ActionMergeBranch,
	"merge_worker":     ActionMergeBranch,
	"git_merge":        ActionMergeBranch,
	"qa":               ActionRunQA,
	"quality_check":    ActionRunQA,
	"validate":         ActionRunQA,
	"run_tests":        ActionRunQA,
	"run_qa_check":     ActionRunQA,
	"noop":             ActionNoOp,
	"wait":             ActionNoOp,
	"idle":             ActionNoOp,
	"skip":             ActionNoOp,
}

// extractEmbeddedPayload checks whether the raw action type string has a known
// action prefix followed by a separator and an embedded JSON payload.
// For example: "create_worker: {\"name\":\"auth\",...}" returns (create_worker, <json>).
// If the remainder after the prefix is not valid JSON, only the action prefix is extracted.
func extractEmbeddedPayload(ctx context.Context, reasoner microai.Interface, raw ActionType) (ActionType, json.RawMessage) {
	rawStr := strings.TrimSpace(string(raw))
	lower := strings.ToLower(rawStr)

	for prefix := range knownActionPrefixes {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := rawStr[len(prefix):]
		if rest == "" {
			return raw, nil
		}
		// Must start with a separator.
		if rest[0] != ':' && rest[0] != ' ' && rest[0] != '-' {
			continue
		}
		// Strip the separator and whitespace.
		body := strings.TrimSpace(strings.TrimLeft(rest, ": -"))
		if body == "" {
			return ActionType(prefix), nil
		}
		// If the body starts with '{', try to parse it as JSON.
		if body[0] == '{' {
			var obj json.RawMessage
			if json.Unmarshal([]byte(body), &obj) == nil {
				return ActionType(prefix), obj
			}
			// Try AI-based repair.
			if repaired := repairJSONWithAI(ctx, reasoner, body); repaired != "" {
				if json.Unmarshal([]byte(repaired), &obj) == nil {
					return ActionType(prefix), obj
				}
			}
		}
		// Non-JSON remainder — if it looks like a file path and the action is
		// path-based (read_file, write_file), wrap it in a {"path":"..."} payload.
		if body != "" && !strings.ContainsAny(body, "{}[]") {
			switch ActionType(prefix) {
			case ActionReadFile, ActionWriteFile:
				payloadJSON, _ := json.Marshal(map[string]string{"path": body})
				return ActionType(prefix), payloadJSON
			}
		}
		return ActionType(prefix), nil
	}
	return raw, nil
}

// repairJSONWithAI sends malformed JSON to the micro model for repair.
// Returns the fixed JSON string, or empty string on failure.
func repairJSONWithAI(ctx context.Context, reasoner microai.Interface, malformed string) string {
	if reasoner == nil {
		return ""
	}
	task := "Fix this malformed JSON so it parses correctly. Return ONLY the corrected JSON, no explanation, no markdown fences."
	result, err := reasoner.Reason(ctx, task, malformed)
	if err != nil {
		return ""
	}
	result = strings.TrimSpace(result)
	// Strip markdown fences if model wrapped it.
	if strings.HasPrefix(result, "```") {
		if idx := strings.Index(result, "\n"); idx >= 0 {
			result = result[idx+1:]
		}
		if idx := strings.LastIndex(result, "```"); idx >= 0 {
			result = result[:idx]
		}
		result = strings.TrimSpace(result)
	}
	return result
}

// extractJSON strips markdown code fences and finds the first JSON object in the LLM response.
func extractJSON(raw string) string {
	s := strings.TrimSpace(raw)
	// Strip markdown fences.
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	// Find the first '{' and paired '}' — simple brace-matching.
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	// Fallback: return from first brace to end.
	return s[start:]
}

// ParseLoopTurnResponse parses a raw LLM JSON string into LoopTurnResponse.
// Handles markdown fences, extra text around JSON, normalizes action types,
// and converts flattened action objects into canonical {type, payload} form.
func ParseLoopTurnResponse(ctx context.Context, reasoner microai.Interface, raw string) (LoopTurnResponse, error) {
	cleaned := extractJSON(raw)
	var resp LoopTurnResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		// Try AI-based repair before giving up.
		if repaired := repairJSONWithAI(ctx, reasoner, cleaned); repaired != "" {
			if err2 := json.Unmarshal([]byte(repaired), &resp); err2 == nil {
				cleaned = repaired
				goto parsed
			}
		}
		return LoopTurnResponse{}, fmt.Errorf("parse loop turn response: %w", err)
	}
parsed:
	// Check if any actions have empty payloads or empty types — this often means
	// the LLM used flattened format or "action" key instead of "type".
	needsReparse := false
	for _, a := range resp.Actions {
		if a.Type == "" {
			needsReparse = true
			break
		}
		s := strings.TrimSpace(string(a.Payload))
		if s == "" || s == "null" || s == "{}" {
			needsReparse = true
			break
		}
	}
	if needsReparse && len(resp.Actions) > 0 {
		if reparsed, err := parseRawActionsWithFlatPayloads(ctx, reasoner, cleaned); err == nil && len(reparsed) > 0 {
			resp.Actions = reparsed
		}
	}
	// ALWAYS normalize action types and extract embedded payloads.
	for i := range resp.Actions {
		normalized, embedded := extractEmbeddedPayload(ctx, reasoner, resp.Actions[i].Type)
		resp.Actions[i].Type = NormalizeActionType(ctx, reasoner, normalized)
		if len(embedded) > 0 {
			ps := strings.TrimSpace(string(resp.Actions[i].Payload))
			if ps == "" || ps == "null" || ps == "{}" {
				resp.Actions[i].Payload = embedded
			}
		}
	}
	return resp, nil
}

// normalizeFlattenedAction handles cases where the LLM places payload fields
// at the same level as "type" instead of nesting them under "payload".
// Example: {"type":"write_file","path":"x","content":"y"}
// becomes: {"type":"write_file","payload":{"path":"x","content":"y"}}
func normalizeFlattenedAction(action LoopAction) LoopAction {
	// If the payload is already populated with a non-empty/non-null object, keep it.
	if len(action.Payload) > 0 {
		s := strings.TrimSpace(string(action.Payload))
		if s != "" && s != "null" && s != "{}" {
			return action
		}
	}
	// If payload is empty but we have extra fields from the LLM that didn't unmarshal,
	// we need to re-parse the raw action object to extract them.
	// We can't do that from the already-parsed LoopAction because extra fields were lost.
	// So instead, we handle this at a higher level: see parseActionsFromRaw below.
	return action
}

// parseRawActionsWithFlatPayloads re-parses the raw JSON to handle flattened action objects.
// This is called when actions have empty payloads to recover flattened fields.
func parseRawActionsWithFlatPayloads(ctx context.Context, reasoner microai.Interface, cleaned string) ([]LoopAction, error) {
	// Parse the response as a generic map to access raw action arrays.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, err
	}
	actionsRaw, ok := raw["actions"]
	if !ok {
		return nil, nil
	}
	var rawActions []json.RawMessage
	if err := json.Unmarshal(actionsRaw, &rawActions); err != nil {
		return nil, err
	}
	actions := make([]LoopAction, 0, len(rawActions))
	for _, ra := range rawActions {
		var actionMap map[string]json.RawMessage
		if err := json.Unmarshal(ra, &actionMap); err != nil {
			continue
		}
		var actionType ActionType
		if t, ok := actionMap["type"]; ok {
			json.Unmarshal(t, &actionType)
		} else if t, ok := actionMap["action"]; ok {
			// LLMs sometimes use "action" instead of "type".
			json.Unmarshal(t, &actionType)
		} else if t, ok := actionMap["action_type"]; ok {
			// LLMs sometimes use "action_type" instead of "type".
			json.Unmarshal(t, &actionType)
		}

		// Before normalizing, check if the raw type string has embedded JSON
		// (e.g. "create_worker: {\"name\":\"auth\",...}") and extract it as payload.
		var embeddedPayload json.RawMessage
		actionType, embeddedPayload = extractEmbeddedPayload(ctx, reasoner, actionType)
		actionType = NormalizeActionType(ctx, reasoner, actionType)

		// Check if there's an explicit "payload" key.
		if payloadRaw, ok := actionMap["payload"]; ok {
			s := strings.TrimSpace(string(payloadRaw))
			if s != "" && s != "null" && s != "{}" {
				actions = append(actions, LoopAction{Type: actionType, Payload: payloadRaw})
				continue
			}
		}
		// Use embedded payload extracted from the type field if available.
		if len(embeddedPayload) > 0 {
			actions = append(actions, LoopAction{Type: actionType, Payload: embeddedPayload})
			continue
		}
		// Build payload from all non-"type"/"action"/"action_type" fields (flattened format).
		payloadMap := make(map[string]json.RawMessage)
		for key, val := range actionMap {
			if key == "type" || key == "action" || key == "action_type" || key == "payload" {
				continue
			}
			// Normalize common snake_case field names to camelCase.
			normalizedKey := normalizeFieldName(ctx, reasoner, key)
			payloadMap[normalizedKey] = val
		}
		if len(payloadMap) > 0 {
			payloadBytes, _ := json.Marshal(payloadMap)
			actions = append(actions, LoopAction{Type: actionType, Payload: payloadBytes})
		} else {
			actions = append(actions, LoopAction{Type: actionType})
		}
	}
	return actions, nil
}

// normalizeFieldName converts snake_case LLM field names to the camelCase
// expected by consuming payload structs, using AI for unknown names.
func normalizeFieldName(ctx context.Context, reasoner microai.Interface, name string) string {
	// Fast path: already camelCase or known mapping.
	if mapped, ok := fieldNameAliases[name]; ok {
		return mapped
	}
	// If it doesn't contain underscores, it's likely already camelCase.
	if !strings.Contains(name, "_") {
		return name
	}
	// AI fallback for unknown snake_case fields.
	if reasoner != nil {
		task := "Convert this snake_case JSON field name to camelCase. Return ONLY the camelCase name, nothing else."
		result, err := reasoner.Reason(ctx, task, name)
		if err == nil {
			result = strings.TrimSpace(strings.Trim(result, "\"'` \n"))
			if result != "" {
				return result
			}
		}
	}
	return name
}

// fieldNameAliases maps known snake_case field names to their camelCase equivalents.
var fieldNameAliases = map[string]string{
	"todo_id":           "todoId",
	"file_path":         "filePath",
	"filepath":          "filePath",
	"child_agent_id":    "childAgentId",
	"target_thread_id":  "targetThreadId",
	"target_child_id":   "targetChildId",
	"problem_statement": "problemStatement",
	"todo_title":        "todoTitle",
	"todo_description":  "todoDescription",
	"owner_agent_id":    "ownerAgentId",
}

// DecodePayload unmarshals a LoopAction's payload into the given target.
func DecodePayload(action LoopAction, target any) error {
	if len(action.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(action.Payload, target)
}
