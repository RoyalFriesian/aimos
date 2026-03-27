package knowledge

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Scanner walks a repository directory and produces a TreeNode hierarchy.
type Scanner struct {
	ignoreRules []ignoreRule
}

type ignoreRule struct {
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool // pattern contains '/' (anchored to gitignore location)
}

// defaultIgnorePatterns are always skipped regardless of .gitignore.
var defaultIgnorePatterns = []string{
	".git",
	"node_modules",
	"__pycache__",
	".DS_Store",
	"vendor",
}

// binaryExtensions are file extensions treated as binary and skipped.
var binaryExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".o": true,
	".bin": true, ".dat": true, ".db": true, ".sqlite": true, ".sqlite3": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true,
	".webp": true, ".svg": true, ".tiff": true, ".tif": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".wav": true,
	".flac": true, ".ogg": true, ".webm": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true,
	".rar": true, ".jar": true, ".war": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".pptx": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".pyc": true, ".pyo": true, ".class": true,
	".wasm": true, ".min.js": true, ".min.css": true,
}

// extensionToLanguage maps common file extensions to language names.
var extensionToLanguage = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".jsx":   "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".java":  "java",
	".kt":    "kotlin",
	".rs":    "rust",
	".c":     "c",
	".cpp":   "cpp",
	".cc":    "cpp",
	".h":     "c",
	".hpp":   "cpp",
	".cs":    "csharp",
	".rb":    "ruby",
	".php":   "php",
	".swift": "swift",
	".m":     "objectivec",
	".scala": "scala",
	".r":     "r",
	".R":     "r",
	".lua":   "lua",
	".pl":    "perl",
	".sh":    "shell",
	".bash":  "shell",
	".zsh":   "shell",
	".fish":  "shell",
	".ps1":   "powershell",
	".sql":   "sql",
	".html":  "html",
	".htm":   "html",
	".css":   "css",
	".scss":  "scss",
	".less":  "less",
	".json":  "json",
	".yaml":  "yaml",
	".yml":   "yaml",
	".toml":  "toml",
	".xml":   "xml",
	".md":    "markdown",
	".rst":   "restructuredtext",
	".proto": "protobuf",
	".tf":    "terraform",
	".vue":   "vue",
	".svelte": "svelte",
	".dart":  "dart",
	".ex":    "elixir",
	".exs":   "elixir",
	".erl":   "erlang",
	".hs":    "haskell",
	".clj":   "clojure",
	".ml":    "ocaml",
	".zig":   "zig",
	".nim":   "nim",
	".v":     "v",
	".sol":   "solidity",
}

// ScanResult holds the output of a repository scan.
type ScanResult struct {
	Tree       []TreeNode
	FileCount  int
	TotalBytes int64
}

// ScanRepo walks the repository at rootPath and returns a flat list of file TreeNodes.
func ScanRepo(rootPath string) (*ScanResult, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absRoot)
	}

	s := &Scanner{}
	s.loadGitignore(absRoot)

	var files []TreeNode
	var totalBytes int64

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		name := d.Name()

		// Always skip default ignore dirs
		if d.IsDir() {
			for _, p := range defaultIgnorePatterns {
				if name == p {
					return filepath.SkipDir
				}
			}
		}

		// Check gitignore rules
		if s.isIgnored(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil // don't add dirs to the flat list
		}

		// Skip binary files
		ext := strings.ToLower(filepath.Ext(name))
		if binaryExtensions[ext] {
			return nil
		}

		// Check for compound extensions like .min.js
		if strings.HasSuffix(strings.ToLower(name), ".min.js") || strings.HasSuffix(strings.ToLower(name), ".min.css") {
			return nil
		}

		// Skip very large files (>1MB)
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if fi.Size() > 1<<20 {
			return nil
		}

		// Quick binary check: read first 512 bytes looking for null bytes
		if isBinaryFile(path) {
			return nil
		}

		lang := extensionToLanguage[ext]
		tokens := estimateTokens(fi.Size())

		files = append(files, TreeNode{
			Path:     rel,
			Name:     name,
			Type:     "file",
			Size:     fi.Size(),
			Hash:     fileHash(path),
			Language: lang,
			Tokens:   tokens,
		})
		totalBytes += fi.Size()

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repo: %w", err)
	}

	return &ScanResult{
		Tree:       files,
		FileCount:  len(files),
		TotalBytes: totalBytes,
	}, nil
}

func (s *Scanner) loadGitignore(rootPath string) {
	gitignorePath := filepath.Join(rootPath, ".gitignore")
	f, err := os.Open(gitignorePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := ignoreRule{}
		if strings.HasPrefix(line, "!") {
			rule.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		if strings.Contains(line, "/") {
			rule.anchored = true
		}
		rule.pattern = line
		s.ignoreRules = append(s.ignoreRules, rule)
	}
}

func (s *Scanner) isIgnored(relPath string, isDir bool) bool {
	ignored := false
	for _, rule := range s.ignoreRules {
		if rule.dirOnly && !isDir {
			continue
		}

		var matched bool
		if rule.anchored {
			matched, _ = filepath.Match(rule.pattern, relPath)
		} else {
			// Match against the basename
			base := filepath.Base(relPath)
			matched, _ = filepath.Match(rule.pattern, base)
			if !matched {
				// Also try matching the full relative path
				matched, _ = filepath.Match(rule.pattern, relPath)
			}
		}

		if matched {
			ignored = !rule.negate
		}
	}
	return ignored
}

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16])
}

// estimateTokens gives a rough token count for a file based on byte size.
// Approximation: ~4 bytes per token for code.
func estimateTokens(sizeBytes int64) int {
	return int(sizeBytes / 4)
}
