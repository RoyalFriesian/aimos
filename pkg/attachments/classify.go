package attachments

import (
	"path/filepath"
	"strings"
)

var textCodeExtensions = map[string]struct{}{
	".go": {}, ".py": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {},
	".java": {}, ".kt": {}, ".rb": {}, ".rs": {}, ".sh": {}, ".sql": {},
	".html": {}, ".css": {}, ".scss": {},
}

var textDocExtensions = map[string]struct{}{
	".md": {}, ".markdown": {}, ".txt": {}, ".csv": {},
	".json": {}, ".yaml": {}, ".yml": {}, ".toml": {}, ".xml": {},
}

var imageExtensions = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".svg": {},
}

var richDocExtensions = map[string]struct{}{
	".pdf": {}, ".doc": {}, ".docx": {},
}

var archiveExtensions = map[string]struct{}{
	".zip": {}, ".gz": {}, ".tgz": {},
}

// ClassifyFile returns the FileCategory for a given filename based on its extension.
func ClassifyFile(filename string) FileCategory {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		// Check basenames like Dockerfile, Makefile, README, LICENSE
		lower := strings.ToLower(filepath.Base(filename))
		switch lower {
		case "dockerfile", "makefile", "readme", "license":
			return CategoryTextDoc
		}
		return CategoryArchive
	}
	if _, ok := textCodeExtensions[ext]; ok {
		return CategoryTextCode
	}
	if _, ok := textDocExtensions[ext]; ok {
		return CategoryTextDoc
	}
	if _, ok := imageExtensions[ext]; ok {
		return CategoryImage
	}
	if _, ok := richDocExtensions[ext]; ok {
		return CategoryRichDoc
	}
	if _, ok := archiveExtensions[ext]; ok {
		return CategoryArchive
	}
	return CategoryArchive
}

// EstimateTokens returns a rough token estimate for a file given its size in bytes
// and file category. Uses ~4 bytes per token for text files.
func EstimateTokens(sizeBytes int64, category FileCategory) int {
	switch category {
	case CategoryTextCode, CategoryTextDoc:
		return int(sizeBytes / 4)
	case CategoryImage:
		// Images are handled via multimodal; token cost depends on resolution.
		// Use a conservative estimate of 1000 tokens per image.
		return 1000
	default:
		return 0
	}
}
