package aiclients

import "context"

type logContextKey struct{ name string }

var (
	projectKey = logContextKey{"project"}
	missionKey = logContextKey{"mission"}
	threadKey  = logContextKey{"thread"}
	traceKey   = logContextKey{"trace"}
)

// LogContext holds project-scoped metadata that the logging middleware
// writes into per-project JSONL files.
type LogContext struct {
	ProjectSlug string // directory-safe project identifier (used as log subdirectory)
	MissionID   string
	ThreadID    string
	TraceID     string
}

// WithLogContext injects project metadata into the context so the logging
// middleware can categorise AI calls by project.
func WithLogContext(ctx context.Context, lc LogContext) context.Context {
	if lc.ProjectSlug != "" {
		ctx = context.WithValue(ctx, projectKey, lc.ProjectSlug)
	}
	if lc.MissionID != "" {
		ctx = context.WithValue(ctx, missionKey, lc.MissionID)
	}
	if lc.ThreadID != "" {
		ctx = context.WithValue(ctx, threadKey, lc.ThreadID)
	}
	if lc.TraceID != "" {
		ctx = context.WithValue(ctx, traceKey, lc.TraceID)
	}
	return ctx
}

// LogContextFromCtx extracts the project log metadata from the context.
func LogContextFromCtx(ctx context.Context) LogContext {
	lc := LogContext{}
	if v, ok := ctx.Value(projectKey).(string); ok {
		lc.ProjectSlug = v
	}
	if v, ok := ctx.Value(missionKey).(string); ok {
		lc.MissionID = v
	}
	if v, ok := ctx.Value(threadKey).(string); ok {
		lc.ThreadID = v
	}
	if v, ok := ctx.Value(traceKey).(string); ok {
		lc.TraceID = v
	}
	return lc
}
