package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool performs string replacements in files
type EditTool struct {
	workDir string
}

// NewEditTool creates a new Edit tool
func NewEditTool(workDir string) *EditTool {
	return &EditTool{workDir: workDir}
}

func (t *EditTool) Name() string {
	return "Edit"
}

func (t *EditTool) Description() string {
	return `Performs exact string replacements in files.

Usage:
- The edit will FAIL if old_string is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use replace_all to change every instance of old_string.
- Use replace_all for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.`
}

func (t *EditTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "The text to replace",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "The text to replace it with (must be different from old_string)",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all occurrences of old_string (default false)",
				"default":     false,
			},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	filePath, ok := GetString(params, "file_path")
	if !ok || filePath == "" {
		return NewErrorResultString("file_path parameter is required"), nil
	}

	oldString, ok := GetString(params, "old_string")
	if !ok {
		return NewErrorResultString("old_string parameter is required"), nil
	}

	newString, ok := GetString(params, "new_string")
	if !ok {
		return NewErrorResultString("new_string parameter is required"), nil
	}

	if oldString == newString {
		return NewErrorResultString("old_string and new_string must be different"), nil
	}

	replaceAll := GetBoolDefault(params, "replace_all", false)

	// Resolve path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewErrorResultString(fmt.Sprintf("File not found: %s", filePath)), nil
		}
		return NewErrorResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	fileContent := string(content)

	// Count occurrences
	count := strings.Count(fileContent, oldString)

	if count == 0 {
		return NewErrorResultString(fmt.Sprintf("old_string not found in file: %s", filePath)), nil
	}

	if count > 1 && !replaceAll {
		return NewErrorResultString(fmt.Sprintf("old_string found %d times in file. Either provide a larger string with more context to make it unique, or set replace_all to true.", count)), nil
	}

	// Perform replacement
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(fileContent, oldString, newString)
	} else {
		newContent = strings.Replace(fileContent, oldString, newString, 1)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	if replaceAll {
		return NewResult(fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", count, filePath)), nil
	}
	return NewResult(fmt.Sprintf("Successfully edited %s", filePath)), nil
}
