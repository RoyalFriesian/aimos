package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Sarnga/agent-platform/agents/ceo"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

type CEOService interface {
	Respond(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error)
	SubmitFeedback(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error)
	GenerateProjectName(ctx context.Context, prompt string) (string, error)
	ListOpenAIModels(ctx context.Context) ([]string, error)
	UploadProjectAttachments(ctx context.Context, threadID string, projectLocation string, files []ceo.ProjectAttachmentInput) ([]ceo.StoredProjectAttachment, error)
	RenameProject(ctx context.Context, threadID string, newName string) error
	ListRootThreads(ctx context.Context) ([]threads.Thread, error)
	LoadProject(ctx context.Context, threadID string) ([]threads.Thread, map[string][]threads.Message, error)
	RefinePrompt(ctx context.Context, rawPrompt string, model string) (string, error)
	ModelGuidance(ctx context.Context, projectDescription string, availableModels []string, model string) (string, error)
}

// ServerOption configures optional features on the HTTP server.
type ServerOption func(*Server)

// WithKnowledge attaches repository indexing and query capabilities.
func WithKnowledge(ks *KnowledgeService) ServerOption {
	return func(s *Server) { s.knowledge = ks }
}

type Server struct {
	service             CEOService
	knowledge           *KnowledgeService
	mux                 *http.ServeMux
	pickProjectLocation func() (string, error)
}

func NewServer(service CEOService, opts ...ServerOption) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("ceo service is required")
	}
	mux := http.NewServeMux()
	server := &Server{service: service, mux: mux, pickProjectLocation: pickProjectLocationNative}

	for _, opt := range opts {
		opt(server)
	}

	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/api/generate-project-name", server.handleGenerateProjectName)
	mux.HandleFunc("/api/openai/models", server.handleListOpenAIModels)
	mux.HandleFunc("/api/system/project-location/pick", server.handlePickProjectLocation)
	mux.HandleFunc("/api/projects", server.handleGetProjects)
	mux.HandleFunc("/api/projects/attachments/upload", server.handleUploadProjectAttachments)
	mux.HandleFunc("/api/projects/rename", server.handleRenameProject)
	mux.HandleFunc("/api/projects/load", server.handleLoadProject)
	mux.HandleFunc("/api/ceo/respond", server.handleRespond)
	mux.HandleFunc("/api/ceo/feedback", server.handleFeedback)
	mux.HandleFunc("/api/ceo/refine-prompt", server.handleRefinePrompt)
	mux.HandleFunc("/api/ceo/model-guidance", server.handleModelGuidance)

	if server.knowledge != nil {
		server.knowledge.RegisterRoutes(mux)
	}

	// Serve the web UI statically
	uiDistPath := "web-ui/dist"
	fs := http.FileServer(http.Dir(uiDistPath))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/assets/") {
			// For SPA routes, explicitly serve index.html to allow client side routing
			// but only if it's not looking for a specific static asset file.
			http.ServeFile(w, r, filepath.Join(uiDistPath, "index.html"))
			return
		}

		// Let FileServer handle the directory indexing for / and the /assets/ files
		fs.ServeHTTP(w, r)
	})

	return server, nil
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		applyCORS(writer)
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(writer, request)
	})
}

func (s *Server) handleHealthz(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleRespond(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload ceo.Request
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Handle reindex action at the server level (before CEO service)
	if payload.Action != nil && payload.Action.Type == ceo.ActionReindex {
		s.handleReindexAction(writer, &payload)
		return
	}

	s.enrichRequestWithKnowledge(request.Context(), &payload)

	response, err := s.service.Respond(request.Context(), payload)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

// handleReindexAction triggers incremental re-indexing from a CEO action request.
func (s *Server) handleReindexAction(writer http.ResponseWriter, payload *ceo.Request) {
	if s.knowledge == nil {
		writeError(writer, http.StatusBadRequest, "knowledge service is not available")
		return
	}
	projectPath := extractProjectPath(payload.Context, payload.Prompt)
	if projectPath == "" {
		writeError(writer, http.StatusBadRequest, "could not determine project path for reindex")
		return
	}

	baseDir := projectPath + "/.aimos-knowledge"
	s.knowledge.StartReindex(projectPath, baseDir)

	writeJSON(writer, http.StatusAccepted, map[string]any{
		"status":      "reindex_started",
		"message":     "Re-indexing started for recently changed files.",
		"projectPath": projectPath,
		"baseDir":     baseDir,
	})
}

func (s *Server) handleFeedback(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload ceo.FeedbackSubmission
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	record, err := s.service.SubmitFeedback(request.Context(), payload)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, record)
}

func applyCORS(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{"error": message})
}

func statusForError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "invalid") || strings.Contains(message, "required") || strings.Contains(message, "does not match") || strings.Contains(message, "belongs to mission") || strings.Contains(message, "cancel") || strings.Contains(message, "unsupported") {
		return http.StatusBadRequest
	}
	if strings.Contains(message, "not found") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func (s *Server) handleGenerateProjectName(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name, err := s.service.GenerateProjectName(request.Context(), payload.Prompt)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"name": name})
}

func (s *Server) handleListOpenAIModels(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	models, err := s.service.ListOpenAIModels(request.Context())
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) handlePickProjectLocation(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.pickProjectLocation == nil {
		writeError(writer, http.StatusInternalServerError, "project location picker is not configured")
		return
	}

	path, err := s.pickProjectLocation()
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}

	writeJSON(writer, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleGetProjects(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	threadsList, err := s.service.ListRootThreads(request.Context())
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"projects": threadsList})
}

func (s *Server) handleLoadProject(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		ThreadID string `json:"threadId"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	projectThreads, msgsMap, err := s.service.LoadProject(request.Context(), payload.ThreadID)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"threads":  projectThreads,
		"messages": msgsMap,
	})
}

func (s *Server) handleRenameProject(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodPatch {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		ThreadID string `json:"threadId"`
		NewName  string `json:"newName"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.ThreadID == "" || payload.NewName == "" {
		writeError(writer, http.StatusBadRequest, "threadId and newName are required")
		return
	}
	if err := s.service.RenameProject(request.Context(), payload.ThreadID, payload.NewName); err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleUploadProjectAttachments(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	const maxFormSizeBytes = 100 * 1024 * 1024
	if err := request.ParseMultipartForm(maxFormSizeBytes); err != nil {
		writeError(writer, http.StatusBadRequest, fmt.Sprintf("invalid multipart form: %v", err))
		return
	}

	threadID := strings.TrimSpace(request.FormValue("threadId"))
	projectLocation := strings.TrimSpace(request.FormValue("projectLocation"))
	if threadID == "" || projectLocation == "" {
		writeError(writer, http.StatusBadRequest, "threadId and projectLocation are required")
		return
	}

	fileHeaders := request.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		writeError(writer, http.StatusBadRequest, "at least one file is required")
		return
	}

	files := make([]ceo.ProjectAttachmentInput, 0, len(fileHeaders))
	for _, header := range fileHeaders {
		file, err := header.Open()
		if err != nil {
			writeError(writer, http.StatusBadRequest, fmt.Sprintf("failed to open file %q: %v", header.Filename, err))
			return
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			writeError(writer, http.StatusBadRequest, fmt.Sprintf("failed to read file %q: %v", header.Filename, readErr))
			return
		}
		if closeErr != nil {
			writeError(writer, http.StatusBadRequest, fmt.Sprintf("failed to close file %q: %v", header.Filename, closeErr))
			return
		}
		files = append(files, ceo.ProjectAttachmentInput{
			Filename:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			Data:        data,
		})
	}

	stored, err := s.service.UploadProjectAttachments(request.Context(), threadID, projectLocation, files)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"threadId":        threadID,
		"projectLocation": projectLocation,
		"stored":          stored,
		"count":           len(stored),
	})
}

func (s *Server) handleRefinePrompt(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model,omitempty"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.Prompt == "" {
		writeError(writer, http.StatusBadRequest, "prompt is required")
		return
	}
	refined, err := s.service.RefinePrompt(request.Context(), payload.Prompt, payload.Model)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"refined": refined, "original": payload.Prompt})
}

func (s *Server) handleModelGuidance(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		ProjectDescription string   `json:"projectDescription"`
		AvailableModels    []string `json:"availableModels"`
		Model              string   `json:"model,omitempty"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.ProjectDescription == "" {
		writeError(writer, http.StatusBadRequest, "projectDescription is required")
		return
	}
	guidance, err := s.service.ModelGuidance(request.Context(), payload.ProjectDescription, payload.AvailableModels, payload.Model)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"guidance": guidance})
}

// enrichRequestWithKnowledge injects codebase knowledge context into the CEO
// request when the knowledge service is available and the project has been indexed.
func (s *Server) enrichRequestWithKnowledge(_ context.Context, payload *ceo.Request) {
	if s.knowledge == nil || payload.Action != nil {
		return
	}

	// Extract projectPath from request context JSON
	projectPath := extractProjectPath(payload.Context, payload.Prompt)
	if projectPath == "" {
		return
	}

	// Determine the baseDir (project-local .aimos-knowledge)
	baseDir := projectPath + "/.aimos-knowledge"

	// Check for in-progress indexing job
	if job := s.knowledge.GetJobStatus(projectPath); job != nil && !job.Done {
		payload.KnowledgeSummary = fmt.Sprintf(
			"[Codebase Knowledge Status]\nThe codebase at %s is currently being indexed (stage: %s, progress: %d/%d). "+
				"You can answer general questions but should let the user know that codebase-specific answers will be more accurate after indexing completes.",
			projectPath, job.Stage, job.Current, job.Total,
		)
		return
	}

	// Check if index exists (project-local first, then default)
	found, _ := s.knowledge.CheckIndex(projectPath, baseDir)
	if !found {
		found, _ = s.knowledge.CheckIndex(projectPath, "")
		if !found {
			return
		}
		baseDir = "" // use default location
	}

	// Read master context summary (no LLM call, just disk read)
	masterCtx, err := s.knowledge.GetMasterContext(projectPath, baseDir)
	if err != nil || masterCtx == "" {
		payload.KnowledgeSummary = fmt.Sprintf(
			"[Codebase Knowledge Status]\nThe codebase at %s has been indexed. You have knowledge of the project structure and can answer codebase-specific questions.",
			projectPath,
		)
		return
	}

	// Truncate to avoid overwhelming the context window
	const maxKnowledgeChars = 12000
	if len(masterCtx) > maxKnowledgeChars {
		masterCtx = masterCtx[:maxKnowledgeChars] + "\n... [truncated for context budget]"
	}

	payload.KnowledgeSummary = fmt.Sprintf(
		"[Codebase Knowledge Base — Indexed Summary]\n"+
			"Project path: %s\n"+
			"Index location: %s\n"+
			"The following is a compressed summary of the entire codebase. Use it to answer questions about the project's architecture, structure, and code.\n"+
			"If the user asks to reindex or you detect the codebase may have changed, you can trigger a reindex by sending an action with type \"reindex\" "+
			"(e.g. {\"action\":{\"type\":\"reindex\",\"payload\":{}}}). The reindex will detect and process only changed files.\n\n%s",
		projectPath, baseDir, masterCtx,
	)
}

// extractProjectPath attempts to find a project path from the request context or prompt.
func extractProjectPath(ctxJSON json.RawMessage, prompt string) string {
	if len(ctxJSON) > 0 {
		var ctxMap map[string]interface{}
		if json.Unmarshal(ctxJSON, &ctxMap) == nil {
			if p, ok := ctxMap["projectPath"].(string); ok && p != "" {
				return p
			}
		}
	}
	// Fallback: extract from prompt "Location: /path/to/project"
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Location:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Location:"))
		}
	}
	return ""
}
