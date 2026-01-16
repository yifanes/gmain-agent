package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const MaxGlobResults = 1000

// GlobTool performs glob pattern matching
type GlobTool struct {
	workDir string
}

// NewGlobTool creates a new Glob tool
func NewGlobTool(workDir string) *GlobTool {
	return &GlobTool{workDir: workDir}
}

func (t *GlobTool) Name() string {
	return "Glob"
}

func (t *GlobTool) Description() string {
	return `Fast file pattern matching tool that works with any codebase size.

- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns`
}

func (t *GlobTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The glob pattern to match files against",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory to search in. If not specified, the current working directory will be used.",
			},
		},
		"required": []string{"pattern"},
	}
}

type fileInfo struct {
	path    string
	modTime int64
}

func (t *GlobTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	pattern, ok := GetString(params, "pattern")
	if !ok || pattern == "" {
		return NewErrorResultString("pattern parameter is required"), nil
	}

	// Get search path
	searchPath := t.workDir
	if path, ok := GetString(params, "path"); ok && path != "" {
		if filepath.IsAbs(path) {
			searchPath = path
		} else {
			searchPath = filepath.Join(t.workDir, path)
		}
	}

	// Verify path exists
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewErrorResultString(fmt.Sprintf("Path not found: %s", searchPath)), nil
		}
		return NewErrorResult(err), nil
	}
	if !info.IsDir() {
		return NewErrorResultString(fmt.Sprintf("%s is not a directory", searchPath)), nil
	}

	// Combine path and pattern
	fullPattern := filepath.Join(searchPath, pattern)

	// Find matches using doublestar
	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return NewErrorResult(fmt.Errorf("invalid glob pattern: %w", err)), nil
	}

	if len(matches) == 0 {
		return NewResult("No files found"), nil
	}

	// Get file info for sorting by modification time
	var files []fileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		files = append(files, fileInfo{
			path:    match,
			modTime: info.ModTime().Unix(),
		})
	}

	// Sort by modification time (most recent first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	// Limit results
	truncated := false
	if len(files) > MaxGlobResults {
		files = files[:MaxGlobResults]
		truncated = true
	}

	// Build output
	var output strings.Builder
	for _, f := range files {
		// Make path relative to work directory if possible
		relPath, err := filepath.Rel(t.workDir, f.path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			output.WriteString(f.path)
		} else {
			output.WriteString(relPath)
		}
		output.WriteString("\n")
	}

	if truncated {
		output.WriteString(fmt.Sprintf("\n... (showing first %d of many results)", MaxGlobResults))
	}

	return NewResult(strings.TrimSuffix(output.String(), "\n")), nil
}
