package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sarnga/agent-platform/agents/ceo"
	aiclients "github.com/Sarnga/agent-platform/ai-clients"
	"github.com/Sarnga/agent-platform/pkg/agents"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/threads"
)

type CEOService interface {
	Respond(ctx context.Context, request ceo.Request) (ceo.ResponseEnvelope, error)
	SubmitFeedback(ctx context.Context, submission ceo.FeedbackSubmission) (feedback.Record, error)
	GenerateProjectName(ctx context.Context, prompt string) (string, error)
	ListOpenAIModels(ctx context.Context) ([]string, error)
	ListOllamaModels(ctx context.Context) ([]string, error)
	GetLLMProviderStatus(ctx context.Context) (ceo.LLMProviderStatus, error)
	SwitchLLMProvider(ctx context.Context, provider string, ceoModel string, workerModel string) error
	UploadProjectAttachments(ctx context.Context, threadID string, projectLocation string, files []ceo.ProjectAttachmentInput) ([]ceo.StoredProjectAttachment, error)
	RenameProject(ctx context.Context, threadID string, newName string) error
	ListRootThreads(ctx context.Context) ([]threads.Thread, error)
	LoadProject(ctx context.Context, threadID string) ([]threads.Thread, map[string][]threads.Message, error)
	LoadProjectAgents(ctx context.Context, projectID string) ([]agents.AgentNode, error)
	RefinePrompt(ctx context.Context, rawPrompt string, model string) (string, error)
	ModelGuidance(ctx context.Context, projectDescription string, availableModels []string, model string) (string, error)
	PauseProject(ctx context.Context, projectID string) error
	ResumeProject(ctx context.Context, projectID string) error
	UpdateAgentModel(ctx context.Context, agentID string, model string) error
	GetAgentStatuses(ctx context.Context, projectID string) ([]agents.LoopStatus, error)
	GetTokenBudgetConfig() aiclients.BudgetSnapshot
	UpdateTokenBudgetConfig(enabled *bool, threshold *int64, target *int64) aiclients.BudgetSnapshot
	GetWakeIntervalConfig() agents.WakeIntervalSnapshot
	UpdateWakeIntervalConfig(ceoSec *int64, managerSec *int64, workerSec *int64) agents.WakeIntervalSnapshot
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
	mux.HandleFunc("/api/projects/pause", server.handlePauseProject)
	mux.HandleFunc("/api/projects/resume", server.handleResumeProject)
	mux.HandleFunc("/api/agents/model", server.handleUpdateAgentModel)
	mux.HandleFunc("/api/agents/status", server.handleGetAgentStatuses)
	mux.HandleFunc("/api/settings/llm-provider", server.handleLLMProvider)
	mux.HandleFunc("/api/ollama/models", server.handleListOllamaModels)
	mux.HandleFunc("/api/settings/token-budget", server.handleTokenBudget)
	mux.HandleFunc("/api/settings/wake-interval", server.handleWakeInterval)

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

	// Use a detached context so the LLM call survives client disconnects.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := s.service.Respond(ctx, payload)
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
	agentNodes, _ := s.service.LoadProjectAgents(request.Context(), payload.ThreadID)
	writeJSON(writer, http.StatusOK, map[string]any{
		"threads":  projectThreads,
		"messages": msgsMap,
		"agents":   agentNodes,
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
			// No index exists yet — provide project path awareness and trigger initial indexing
			payload.KnowledgeSummary = fmt.Sprintf(
				"[Project Path Awareness]\n"+
					"Project path: %s\n"+
					"The codebase has not been indexed yet. An initial index is being started now.\n"+
					"You have awareness of this project path. If the directory is empty or newly created, "+
					"this is a fresh project — proceed with planning for a new build.\n"+
					"If the directory contains existing code, the index will be available shortly and you will "+
					"have full codebase knowledge on subsequent turns.\n"+
					"NEVER tell the user you cannot inspect or access the project path — you have access through the platform.",
				projectPath,
			)
			// Auto-start indexing for the project
			s.knowledge.StartReindex(projectPath, baseDir)
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

func (s *Server) handlePauseProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ProjectID string `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" {
		http.Error(w, "projectId is required", http.StatusBadRequest)
		return
	}
	if err := s.service.PauseProject(r.Context(), req.ProjectID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleResumeProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ProjectID string `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" {
		http.Error(w, "projectId is required", http.StatusBadRequest)
		return
	}
	if err := s.service.ResumeProject(r.Context(), req.ProjectID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleUpdateAgentModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AgentID string `json:"agentId"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentID == "" || req.Model == "" {
		http.Error(w, "agentId and model are required", http.StatusBadRequest)
		return
	}
	if err := s.service.UpdateAgentModel(r.Context(), req.AgentID, req.Model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleGetAgentStatuses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		http.Error(w, "projectId query param is required", http.StatusBadRequest)
		return
	}
	statuses, err := s.service.GetAgentStatuses(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"statuses": statuses})
}

func (s *Server) handleLLMProvider(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status, err := s.service.GetLLMProviderStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, status)

	case http.MethodPost:
		var req struct {
			Provider    string `json:"provider"`
			CEOModel    string `json:"ceoModel"`
			WorkerModel string `json:"workerModel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Provider == "" {
			writeError(w, http.StatusBadRequest, "provider is required")
			return
		}
		if req.CEOModel == "" || req.WorkerModel == "" {
			writeError(w, http.StatusBadRequest, "ceoModel and workerModel are required")
			return
		}
		if err := s.service.SwitchLLMProvider(r.Context(), req.Provider, req.CEOModel, req.WorkerModel); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleListOllamaModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	models, err := s.service.ListOllamaModels(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// handleTokenBudget supports GET (read current config) and POST (update config)
// for the token budget middleware. Changes take effect immediately without restart.
func (s *Server) handleTokenBudget(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.service.GetTokenBudgetConfig())

	case http.MethodPost:
		var req struct {
			Enabled   *bool  `json:"enabled"`
			Threshold *int64 `json:"threshold"`
			Target    *int64 `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		snapshot := s.service.UpdateTokenBudgetConfig(req.Enabled, req.Threshold, req.Target)
		writeJSON(w, http.StatusOK, snapshot)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleWakeInterval supports GET (read current config) and POST (update config)
// for the agent loop wake intervals. Changes take effect on the next tick.
func (s *Server) handleWakeInterval(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.service.GetWakeIntervalConfig())

	case http.MethodPost:
		var req struct {
			CEOSeconds     *int64 `json:"ceoSeconds"`
			ManagerSeconds *int64 `json:"managerSeconds"`
			WorkerSeconds  *int64 `json:"workerSeconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		snapshot := s.service.UpdateWakeIntervalConfig(req.CEOSeconds, req.ManagerSeconds, req.WorkerSeconds)
		writeJSON(w, http.StatusOK, snapshot)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
