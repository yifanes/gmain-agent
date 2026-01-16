package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultBashTimeout = 2 * time.Minute
	MaxBashTimeout     = 10 * time.Minute
	MaxOutputSize      = 30000
)

// BashTool executes bash commands
type BashTool struct {
	workDir string
}

// NewBashTool creates a new Bash tool
func NewBashTool(workDir string) *BashTool {
	return &BashTool{workDir: workDir}
}

func (t *BashTool) Name() string {
	return "Bash"
}

func (t *BashTool) Description() string {
	return `Executes a given bash command in a persistent shell session with optional timeout.

This tool is for terminal operations like git, npm, docker, etc.
- The command argument is required
- You can specify an optional timeout in milliseconds (up to 600000ms / 10 minutes)
- If not specified, commands will timeout after 120000ms (2 minutes)
- If the output exceeds 30000 characters, output will be truncated`
}

func (t *BashTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The bash command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "number",
				"description": "Optional timeout in milliseconds (max 600000)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Clear, concise description of what this command does",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	command, ok := GetString(params, "command")
	if !ok || command == "" {
		return NewErrorResultString("command parameter is required"), nil
	}

	// Get timeout
	timeout := DefaultBashTimeout
	if timeoutMs, ok := GetInt(params, "timeout"); ok {
		timeout = time.Duration(timeoutMs) * time.Millisecond
		if timeout > MaxBashTimeout {
			timeout = MaxBashTimeout
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = t.workDir
	cmd.Env = os.Environ()

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	// Build output
	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(stderr.String())
	}

	result := output.String()

	// Truncate if necessary
	if len(result) > MaxOutputSize {
		result = result[:MaxOutputSize] + "\n... (output truncated)"
	}

	// Handle errors
	if ctx.Err() == context.DeadlineExceeded {
		return NewErrorResultString(fmt.Sprintf("Command timed out after %v\n%s", timeout, result)), nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if result == "" {
				result = fmt.Sprintf("Command exited with code %d", exitErr.ExitCode())
			}
			return &Result{Output: result, IsError: true}, nil
		}
		return NewErrorResult(err), nil
	}

	if result == "" {
		result = "(no output)"
	}

	return NewResult(result), nil
}
