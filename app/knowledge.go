package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/Sarnga/agent-platform/pkg/knowledge"
)

// KnowledgeService provides indexing and querying for repositories.
type KnowledgeService struct {
	client knowledge.CompletionClient
	cfg    knowledge.Config

	mu       sync.Mutex
	indexing map[string]*indexJob // keyed by repo path
}

type indexJob struct {
	Stage   string `json:"stage"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
	RepoID  string `json:"repoId,omitempty"`
	BaseDir string `json:"baseDir,omitempty"`
	Changed int    `json:"changed,omitempty"` // files changed (reindex only)
}

// indexingKey returns the map key for tracking an indexing job.
// Different baseDir values for the same repo path are independent jobs.
func indexingKey(path, baseDir string) string {
	if baseDir == "" {
		return path
	}
	return path + "\x00" + baseDir
}

// NewKnowledgeService creates a service wired to the given LLM client and config.
func NewKnowledgeService(client knowledge.CompletionClient, cfg knowledge.Config) *KnowledgeService {
	return &KnowledgeService{
		client:   client,
		cfg:      cfg,
		indexing: make(map[string]*indexJob),
	}
}

// RegisterRoutes adds knowledge API endpoints to the given mux.
func (ks *KnowledgeService) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/knowledge/repos", ks.handleListRepos)
	mux.HandleFunc("/api/knowledge/index", ks.handleIndex)
	mux.HandleFunc("/api/knowledge/index/status", ks.handleIndexStatus)
	mux.HandleFunc("/api/knowledge/reindex", ks.handleReindex)
	mux.HandleFunc("/api/knowledge/query", ks.handleQuery)
	mux.HandleFunc("/api/knowledge/check", ks.handleCheckIndex)
	mux.HandleFunc("/api/knowledge/master", ks.handleMasterContext)
}

func (ks *KnowledgeService) handleListRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	repos, err := knowledge.ListRepos(ks.cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if repos == nil {
		repos = []knowledge.Manifest{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": repos})
}

func (ks *KnowledgeService) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Path    string `json:"path"`
		Deep    bool   `json:"deep"`
		BaseDir string `json:"baseDir,omitempty"`
		Model   string `json:"model,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	key := indexingKey(payload.Path, payload.BaseDir)
	ks.mu.Lock()
	if job, exists := ks.indexing[key]; exists && !job.Done {
		ks.mu.Unlock()
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":  "already_indexing",
			"message": "indexing is already in progress for this repo",
			"job":     job,
		})
		return
	}

	job := &indexJob{Stage: "starting", Current: 0, Total: 0, BaseDir: payload.BaseDir}
	ks.indexing[key] = job
	ks.mu.Unlock()

	cfg := ks.cfg
	if payload.BaseDir != "" {
		cfg.BaseDir = payload.BaseDir
	}
	if payload.Model != "" {
		cfg.IndexModel = payload.Model
	}
	if payload.Deep {
		cfg.ScanMode = knowledge.ScanModeDeep
	}

	// Run indexing in background
	go func() {
		slog.Info("indexing goroutine started",
			"path", payload.Path,
			"baseDir", cfg.BaseDir,
			"model", cfg.GetIndexModel(),
		)
		progress := func(stage string, current, total int) {
			ks.mu.Lock()
			job.Stage = stage
			job.Current = current
			job.Total = total
			ks.mu.Unlock()
		}

		manifest, err := knowledge.IndexRepo(context.Background(), ks.client, payload.Path, cfg, progress)
		ks.mu.Lock()
		job.Done = true
		if err != nil {
			job.Error = err.Error()
			job.Stage = "failed"
		} else {
			job.Stage = "ready"
			job.RepoID = manifest.Repo.ID

			// If using project-local baseDir, remove stale global copy
			if payload.BaseDir != "" {
				globalDir := ks.cfg.RepoDir(manifest.Repo.ID)
				if _, statErr := os.Stat(globalDir); statErr == nil {
					_ = os.RemoveAll(globalDir)
					slog.Info("removed stale global index", "dir", globalDir)
				}
			}
		}
		ks.mu.Unlock()
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":  "started",
		"message": "indexing started in background",
		"baseDir": cfg.BaseDir,
	})
}

func (ks *KnowledgeService) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	baseDir := r.URL.Query().Get("baseDir")
	key := indexingKey(path, baseDir)

	ks.mu.Lock()
	job, exists := ks.indexing[key]
	ks.mu.Unlock()

	if !exists {
		cfg := ks.cfg
		if baseDir != "" {
			cfg.BaseDir = baseDir
		}
		_, found, err := knowledge.FindRepoByPath(cfg, path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if found {
			writeJSON(w, http.StatusOK, map[string]any{
				"stage": "ready",
				"done":  true,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"stage": "not_started",
			"done":  false,
		})
		return
	}

	ks.mu.Lock()
	snapshot := *job
	ks.mu.Unlock()
	writeJSON(w, http.StatusOK, snapshot)
}

// handleReindex performs incremental re-indexing (detects changed files).
func (ks *KnowledgeService) handleReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Path    string `json:"path"`
		BaseDir string `json:"baseDir,omitempty"`
		Model   string `json:"model,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	key := indexingKey(payload.Path, payload.BaseDir)
	ks.mu.Lock()
	if job, exists := ks.indexing[key]; exists && !job.Done {
		ks.mu.Unlock()
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":  "already_indexing",
			"message": "indexing is already in progress for this repo",
			"job":     job,
		})
		return
	}

	job := &indexJob{Stage: "checking", Current: 0, Total: 0, BaseDir: payload.BaseDir}
	ks.indexing[key] = job
	ks.mu.Unlock()

	cfg := ks.cfg
	if payload.BaseDir != "" {
		cfg.BaseDir = payload.BaseDir
	}
	if payload.Model != "" {
		cfg.IndexModel = payload.Model
	}

	go func() {
		slog.Info("reindex goroutine started",
			"path", payload.Path,
			"baseDir", cfg.BaseDir,
			"model", cfg.GetIndexModel(),
		)
		progress := func(stage string, current, total int) {
			ks.mu.Lock()
			job.Stage = stage
			job.Current = current
			job.Total = total
			ks.mu.Unlock()
		}

		manifest, changed, err := knowledge.ReindexRepo(context.Background(), ks.client, payload.Path, cfg, progress)
		ks.mu.Lock()
		job.Done = true
		job.Changed = changed
		if err != nil {
			job.Error = err.Error()
			job.Stage = "failed"
		} else {
			job.Stage = "ready"
			job.RepoID = manifest.Repo.ID

			// If using project-local baseDir, remove stale global copy
			if payload.BaseDir != "" {
				globalDir := ks.cfg.RepoDir(manifest.Repo.ID)
				if _, statErr := os.Stat(globalDir); statErr == nil {
					_ = os.RemoveAll(globalDir)
					slog.Info("removed stale global index", "dir", globalDir)
				}
			}
		}
		ks.mu.Unlock()
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":  "started",
		"message": "reindex started in background",
		"baseDir": cfg.BaseDir,
	})
}

func (ks *KnowledgeService) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Path     string `json:"path"`
		Question string `json:"question"`
		BaseDir  string `json:"baseDir,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if payload.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if payload.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	cfg := ks.cfg
	if payload.BaseDir != "" {
		cfg.BaseDir = payload.BaseDir
	}

	manifest, found, err := knowledge.FindRepoByPath(cfg, payload.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "repo is not indexed — index it first")
		return
	}

	queryFn := knowledge.ResolveQuery(r.Context(), ks.client, cfg, manifest)
	result, err := queryFn(r.Context(), payload.Question)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleCheckIndex checks whether a repo index exists at the given path.
func (ks *KnowledgeService) handleCheckIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	baseDir := r.URL.Query().Get("baseDir")

	// Check for in-progress indexing job first
	ks.mu.Lock()
	job, hasJob := ks.indexing[path]
	ks.mu.Unlock()
	if hasJob && !job.Done {
		writeJSON(w, http.StatusOK, map[string]any{
			"exists":   false,
			"indexing": true,
			"stage":    job.Stage,
			"current":  job.Current,
			"total":    job.Total,
		})
		return
	}

	cfg := ks.cfg
	if baseDir != "" {
		cfg.BaseDir = baseDir
	}

	manifest, found, err := knowledge.FindRepoByPath(cfg, path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		// Also check project-local location if we used default
		if baseDir == "" {
			localCfg := ks.cfg
			localCfg.BaseDir = path + "/.aimos-knowledge"
			manifest, found, err = knowledge.FindRepoByPath(localCfg, path)
			if err != nil {
				found = false
			}
		}
	}

	if found {
		writeJSON(w, http.StatusOK, map[string]any{
			"exists":      true,
			"indexing":    false,
			"repoId":      manifest.Repo.ID,
			"fileCount":   manifest.Repo.FileCount,
			"levels":      manifest.Repo.LevelsCount,
			"model":       manifest.Repo.Model,
			"totalTokens": manifest.Repo.TotalTokens,
			"updatedAt":   manifest.Repo.UpdatedAt,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":   false,
		"indexing": false,
	})
}

// handleMasterContext returns the master context summary for an indexed repo.
func (ks *KnowledgeService) handleMasterContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	baseDir := r.URL.Query().Get("baseDir")

	cfg := ks.cfg
	if baseDir != "" {
		cfg.BaseDir = baseDir
	}

	manifest, found, err := knowledge.FindRepoByPath(cfg, path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "repo is not indexed")
		return
	}

	content, err := knowledge.ReadMasterContext(cfg, manifest.Repo.ID, manifest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read master context: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repoId":  manifest.Repo.ID,
		"content": content,
	})
}

// --- Public methods for server-side integration ---

// CheckIndex checks if an index exists for the given repo path.
func (ks *KnowledgeService) CheckIndex(repoPath string, baseDir string) (found bool, repoID string) {
	cfg := ks.cfg
	if baseDir != "" {
		cfg.BaseDir = baseDir
	}
	manifest, found, err := knowledge.FindRepoByPath(cfg, repoPath)
	if err != nil || !found {
		return false, ""
	}
	return true, manifest.Repo.ID
}

// GetJobStatus returns the current in-memory indexing job status for a path, if any.
func (ks *KnowledgeService) GetJobStatus(repoPath string) *indexJob {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if job, ok := ks.indexing[repoPath]; ok {
		snapshot := *job
		return &snapshot
	}
	return nil
}

// GetMasterContext reads the master context summary for an indexed repo.
func (ks *KnowledgeService) GetMasterContext(repoPath string, baseDir string) (string, error) {
	cfg := ks.cfg
	if baseDir != "" {
		cfg.BaseDir = baseDir
	}
	manifest, found, err := knowledge.FindRepoByPath(cfg, repoPath)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return knowledge.ReadMasterContext(cfg, manifest.Repo.ID, manifest)
}

// StartReindex triggers incremental re-indexing for a repo path.
// It is safe to call programmatically (e.g. from the CEO handler).
// Returns immediately; indexing runs in the background.
func (ks *KnowledgeService) StartReindex(repoPath string, baseDir string) {
	key := indexingKey(repoPath, baseDir)
	ks.mu.Lock()
	if job, exists := ks.indexing[key]; exists && !job.Done {
		ks.mu.Unlock()
		return // already running
	}
	job := &indexJob{Stage: "checking", BaseDir: baseDir}
	ks.indexing[key] = job
	ks.mu.Unlock()

	cfg := ks.cfg
	if baseDir != "" {
		cfg.BaseDir = baseDir
	}

	go func() {
		slog.Info("programmatic reindex started", "path", repoPath, "baseDir", cfg.BaseDir)
		progress := func(stage string, current, total int) {
			ks.mu.Lock()
			job.Stage = stage
			job.Current = current
			job.Total = total
			ks.mu.Unlock()
		}

		manifest, changed, err := knowledge.ReindexRepo(context.Background(), ks.client, repoPath, cfg, progress)
		ks.mu.Lock()
		job.Done = true
		job.Changed = changed
		if err != nil {
			job.Error = err.Error()
			job.Stage = "failed"
		} else {
			job.Stage = "ready"
			job.RepoID = manifest.Repo.ID
			if baseDir != "" {
				globalDir := ks.cfg.RepoDir(manifest.Repo.ID)
				if _, statErr := os.Stat(globalDir); statErr == nil {
					_ = os.RemoveAll(globalDir)
					slog.Info("removed stale global index", "dir", globalDir)
				}
			}
		}
		ks.mu.Unlock()
	}()
}
