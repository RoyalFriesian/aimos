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
}

type Server struct {
	service             CEOService
	mux                 *http.ServeMux
	pickProjectLocation func() (string, error)
}

func NewServer(service CEOService) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("ceo service is required")
	}
	mux := http.NewServeMux()
	server := &Server{service: service, mux: mux, pickProjectLocation: pickProjectLocationNative}
	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/api/generate-project-name", server.handleGenerateProjectName)
	mux.HandleFunc("/api/openai/models", server.handleListOpenAIModels)
	mux.HandleFunc("/api/system/project-location/pick", server.handlePickProjectLocation)
	mux.HandleFunc("/api/projects", server.handleGetProjects)
	mux.HandleFunc("/api/projects/attachments/upload", server.handleUploadProjectAttachments)
	mux.HandleFunc("/api/projects/rename", server.handleRenameProject)
	mux.HandleFunc("/api/projects/load", server.handleLoadProject)
	mux.HandleFunc("/api/ceo/respond", server.handleRespond)

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
	response, err := s.service.Respond(request.Context(), payload)
	if err != nil {
		writeError(writer, statusForError(err), err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, response)
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
