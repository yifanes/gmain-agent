package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	MaxGrepResults = 500
	DefaultContext = 0
)

// GrepTool searches file contents using regex
type GrepTool struct {
	workDir string
}

// NewGrepTool creates a new Grep tool
func NewGrepTool(workDir string) *GrepTool {
	return &GrepTool{workDir: workDir}
}

func (t *GrepTool) Name() string {
	return "Grep"
}

func (t *GrepTool) Description() string {
	return `A powerful search tool built on regex pattern matching.

Usage:
- Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+")
- Filter files with glob parameter (e.g., "*.js", "**/*.tsx")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts
- Use -A, -B, or -C for context lines around matches`
}

func (t *GrepTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The regular expression pattern to search for in file contents",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory to search in. Defaults to current working directory.",
			},
			"glob": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\")",
			},
			"output_mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"content", "files_with_matches", "count"},
				"description": "Output mode: 'content' shows matching lines, 'files_with_matches' shows file paths (default), 'count' shows match counts",
			},
			"-i": map[string]interface{}{
				"type":        "boolean",
				"description": "Case insensitive search",
			},
			"-n": map[string]interface{}{
				"type":        "boolean",
				"description": "Show line numbers in output. Defaults to true.",
			},
			"-A": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to show after each match",
			},
			"-B": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to show before each match",
			},
			"-C": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to show before and after each match",
			},
			"head_limit": map[string]interface{}{
				"type":        "number",
				"description": "Limit output to first N lines/entries",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	pattern, ok := GetString(params, "pattern")
	if !ok || pattern == "" {
		return NewErrorResultString("pattern parameter is required"), nil
	}

	// Get options
	caseInsensitive := GetBoolDefault(params, "-i", false)
	showLineNumbers := GetBoolDefault(params, "-n", true)
	outputMode := GetStringDefault(params, "output_mode", "files_with_matches")
	headLimit := GetIntDefault(params, "head_limit", 0)

	// Get context lines
	contextLines := GetIntDefault(params, "-C", 0)
	beforeLines := GetIntDefault(params, "-B", contextLines)
	afterLines := GetIntDefault(params, "-A", contextLines)

	// Compile regex
	var re *regexp.Regexp
	var err error
	if caseInsensitive {
		re, err = regexp.Compile("(?i)" + pattern)
	} else {
		re, err = regexp.Compile(pattern)
	}
	if err != nil {
		return NewErrorResultString(fmt.Sprintf("Invalid regex pattern: %s", err.Error())), nil
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

	// Get file filter
	globPattern, _ := GetString(params, "glob")

	// Find files to search
	var files []string
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewErrorResultString(fmt.Sprintf("Path not found: %s", searchPath)), nil
		}
		return NewErrorResult(err), nil
	}

	if info.IsDir() {
		// Walk directory
		err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				// Skip hidden and common non-code directories
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files and binary files
			if strings.HasPrefix(filepath.Base(path), ".") {
				return nil
			}
			// Apply glob filter if specified
			if globPattern != "" {
				matched, _ := doublestar.PathMatch(globPattern, filepath.Base(path))
				if !matched {
					// Also try full path match
					relPath, _ := filepath.Rel(searchPath, path)
					matched, _ = doublestar.PathMatch(globPattern, relPath)
					if !matched {
						return nil
					}
				}
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return NewErrorResult(err), nil
		}
	} else {
		files = []string{searchPath}
	}

	// Search files
	var output strings.Builder
	matchCount := 0
	resultCount := 0

	for _, file := range files {
		if headLimit > 0 && resultCount >= headLimit {
			break
		}

		matches, err := searchFile(file, re, beforeLines, afterLines)
		if err != nil {
			continue // Skip files that can't be read
		}

		if len(matches) == 0 {
			continue
		}

		matchCount += len(matches)

		// Make path relative
		relPath, err := filepath.Rel(t.workDir, file)
		if err != nil || strings.HasPrefix(relPath, "..") {
			relPath = file
		}

		switch outputMode {
		case "files_with_matches":
			output.WriteString(relPath)
			output.WriteString("\n")
			resultCount++

		case "count":
			output.WriteString(fmt.Sprintf("%s:%d\n", relPath, len(matches)))
			resultCount++

		case "content":
			for _, m := range matches {
				if headLimit > 0 && resultCount >= headLimit {
					break
				}
				if showLineNumbers {
					output.WriteString(fmt.Sprintf("%s:%d:%s\n", relPath, m.lineNum, m.line))
				} else {
					output.WriteString(fmt.Sprintf("%s:%s\n", relPath, m.line))
				}
				resultCount++
			}
		}
	}

	if output.Len() == 0 {
		return NewResult("No matches found"), nil
	}

	return NewResult(strings.TrimSuffix(output.String(), "\n")), nil
}

type match struct {
	lineNum int
	line    string
}

func searchFile(filePath string, re *regexp.Regexp, beforeLines, afterLines int) ([]match, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []match
	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)

		if re.MatchString(line) {
			matches = append(matches, match{lineNum: lineNum, line: line})
		}
	}

	return matches, scanner.Err()
}
