package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultBashTimeout    = 15 * time.Second // 缩短默认超时，避免卡住太久
	MaxBashTimeout        = 2 * time.Minute
	BackgroundCmdTimeout  = 5 * time.Second  // 后台命令应该快速返回
	MaxOutputSize         = 30000
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
	return `Executes bash commands with timeout protection.

This tool is for terminal operations like git, npm, docker, etc.

IMPORTANT - Long-Running Processes:
For processes that run indefinitely (dev servers, watch tasks, daemons):
- Set "run_in_background": true, OR
- Append & to the command (e.g., "npm run dev &")
The process will run in background and return immediately with PID and log file path.

Examples:
  {"command": "npm run dev", "run_in_background": true}  // Background server
  {"command": "npm run dev &"}                            // Alternative syntax
  {"command": "npm install"}                              // Normal command

Timeouts:
- Default timeout: 15 seconds (not 2 minutes anymore!)
- Max timeout: 2 minutes
- Background commands: 5 seconds to start

Output:
- Output exceeding 30000 characters will be truncated`
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
				"description": "Optional timeout in milliseconds (max 120000)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Clear, concise description of what this command does",
			},
			"run_in_background": map[string]interface{}{
				"type":        "boolean",
				"description": "Set to true to run this command in background (for dev servers, watch tasks, etc.). Process will detach and return immediately with PID and log file path.",
				"default":     false,
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

	// 检查是否明确指定后台运行
	runInBackground := GetBoolDefault(params, "run_in_background", false)

	// 检测是否是后台命令（以 & 结尾）
	trimmedCmd := strings.TrimSpace(command)
	hasAmpersand := strings.HasSuffix(trimmedCmd, "&")

	// 决定是否后台运行：明确参数 或 & 符号
	isBackground := runInBackground || hasAmpersand

	if isBackground {
		// 处理后台命令：移除 &，使用 nohup 正确后台化
		command = strings.TrimSuffix(trimmedCmd, "&")
		command = strings.TrimSpace(command)

		// 创建日志文件路径
		logFile := filepath.Join(os.TempDir(), fmt.Sprintf("bg-cmd-%d.log", time.Now().Unix()))

		// 转义命令中的单引号
		escapedCmd := strings.ReplaceAll(command, "'", "'\\''")

		// 使用 nohup 将命令完全后台化，并记录输出到日志文件
		bgCommand := fmt.Sprintf(
			"nohup bash -c '%s' > %s 2>&1 & echo \"Background process started. PID: $! | Log file: %s\"",
			escapedCmd,
			logFile,
			logFile,
		)

		// 后台命令应该快速返回，使用短超时
		ctx, cancel := context.WithTimeout(ctx, BackgroundCmdTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", bgCommand)
		cmd.Dir = t.workDir
		cmd.Env = os.Environ()

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		result := stdout.String()
		if stderr.Len() > 0 {
			if result != "" {
				result += "\n"
			}
			result += stderr.String()
		}

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return NewErrorResultString(fmt.Sprintf("Background command failed to start within %v\n%s", BackgroundCmdTimeout, result)), nil
			}
			return &Result{Output: result, IsError: true}, nil
		}

		if result == "" {
			result = "Background process started (no immediate output)"
		}

		return NewResult(result), nil
	}

	// 普通命令的处理逻辑（原代码）
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
