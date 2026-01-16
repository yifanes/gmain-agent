package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultReadLimit     = 2000
	MaxLineLength        = 2000
)

// ReadTool reads files from the filesystem
type ReadTool struct {
	workDir string
}

// NewReadTool creates a new Read tool
func NewReadTool(workDir string) *ReadTool {
	return &ReadTool{workDir: workDir}
}

func (t *ReadTool) Name() string {
	return "Read"
}

func (t *ReadTool) Description() string {
	return `Reads a file from the local filesystem.

Usage:
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- You can optionally specify a line offset and limit (especially handy for long files)
- Any lines longer than 2000 characters will be truncated
- Results are returned using cat -n format, with line numbers starting at 1`
}

func (t *ReadTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to read",
			},
			"offset": map[string]interface{}{
				"type":        "number",
				"description": "The line number to start reading from (1-indexed). Only provide if the file is too large to read at once",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "The number of lines to read. Only provide if the file is too large to read at once",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	filePath, ok := GetString(params, "file_path")
	if !ok || filePath == "" {
		return NewErrorResultString("file_path parameter is required"), nil
	}

	// Resolve path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewErrorResultString(fmt.Sprintf("File not found: %s", filePath)), nil
		}
		return NewErrorResult(err), nil
	}

	if info.IsDir() {
		return NewErrorResultString(fmt.Sprintf("%s is a directory, not a file. Use Bash with 'ls' to list directory contents.", filePath)), nil
	}

	// Get offset and limit
	offset := GetIntDefault(params, "offset", 1)
	if offset < 1 {
		offset = 1
	}
	limit := GetIntDefault(params, "limit", DefaultReadLimit)
	if limit <= 0 {
		limit = DefaultReadLimit
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return NewErrorResult(err), nil
	}
	defer file.Close()

	// Read lines
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Allow larger lines

	var output strings.Builder
	lineNum := 0
	linesRead := 0

	for scanner.Scan() {
		lineNum++

		// Skip lines before offset
		if lineNum < offset {
			continue
		}

		// Check limit
		if linesRead >= limit {
			break
		}

		line := scanner.Text()

		// Truncate long lines
		if len(line) > MaxLineLength {
			line = line[:MaxLineLength] + "..."
		}

		// Format like cat -n
		output.WriteString(fmt.Sprintf("%6d\t%s\n", lineNum, line))
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		return NewErrorResult(fmt.Errorf("error reading file: %w", err)), nil
	}

	result := output.String()
	if result == "" {
		if offset > 1 {
			return NewErrorResultString(fmt.Sprintf("No content found starting at line %d", offset)), nil
		}
		return NewResult("(empty file)"), nil
	}

	return NewResult(result), nil
}
