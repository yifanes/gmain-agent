package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool writes files to the filesystem
type WriteTool struct {
	workDir string
}

// NewWriteTool creates a new Write tool
func NewWriteTool(workDir string) *WriteTool {
	return &WriteTool{workDir: workDir}
}

func (t *WriteTool) Name() string {
	return "Write"
}

func (t *WriteTool) Description() string {
	return `Writes a file to the local filesystem.

Usage:
- This tool will overwrite the existing file if there is one at the provided path
- The file_path parameter must be an absolute path, not a relative path`
}

func (t *WriteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to write (must be absolute, not relative)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	filePath, ok := GetString(params, "file_path")
	if !ok || filePath == "" {
		return NewErrorResultString("file_path parameter is required"), nil
	}

	content, ok := GetString(params, "content")
	if !ok {
		return NewErrorResultString("content parameter is required"), nil
	}

	// Resolve path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewErrorResult(fmt.Errorf("failed to create directory %s: %w", dir, err)), nil
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	return NewResult(fmt.Sprintf("File written successfully to: %s", filePath)), nil
}
