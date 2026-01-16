package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PlanExitTool 退出计划模式的工具
type PlanExitTool struct {
	workDir      string
	onModeSwitch func(toAgent string) error
}

// NewPlanExitTool 创建新的 PlanExit 工具
func NewPlanExitTool(workDir string, onModeSwitch func(string) error) *PlanExitTool {
	return &PlanExitTool{
		workDir:      workDir,
		onModeSwitch: onModeSwitch,
	}
}

func (t *PlanExitTool) Name() string {
	return "plan_exit"
}

func (t *PlanExitTool) Description() string {
	return `Exit planning mode and return to build mode to implement the plan.

Use this tool when:
- You have finished analyzing the codebase
- Your implementation plan is complete
- You are ready to start coding

This will switch you back to BUILD MODE with full permissions to:
- Edit source code files
- Create new files
- Execute commands
- Implement the plan you created

The plan documents will be available for reference during implementation.`
}

func (t *PlanExitTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ready_to_implement": map[string]interface{}{
				"type":        "boolean",
				"description": "Confirm you are ready to exit planning mode and start implementation",
				"default":     true,
			},
		},
		"required": []string{"ready_to_implement"},
	}
}

// PlanExitInput PlanExit 工具的输入
type PlanExitInput struct {
	ReadyToImplement bool `json:"ready_to_implement"`
}

func (t *PlanExitTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
	// 解析输入
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	var exitInput PlanExitInput
	if err := json.Unmarshal(inputJSON, &exitInput); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if !exitInput.ReadyToImplement {
		return nil, fmt.Errorf("you must confirm you are ready to implement")
	}

	// 查找最新的计划文件
	planDir := filepath.Join(t.workDir, ".gmain-agent", "plans")
	latestPlan, err := findLatestPlan(planDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find plan: %w", err)
	}

	// 切换到 build agent
	if t.onModeSwitch != nil {
		if err := t.onModeSwitch("build"); err != nil {
			return nil, fmt.Errorf("failed to switch to build mode: %w", err)
		}
	}

	output := fmt.Sprintf(`✓ Exited planning mode

You are now in BUILD MODE with full permissions.

Your plan is available at: %s

You can now:
- Edit source code files
- Create new files and directories
- Execute commands
- Make any changes needed to implement the plan

Start implementing the plan step by step. Refer to the plan document as needed.

Good luck with the implementation!`, latestPlan)

	return &Result{
		Output: output,
	}, nil
}

// findLatestPlan 查找最新的计划文件
func findLatestPlan(planDir string) (string, error) {
	// 检查目录是否存在
	if _, err := os.Stat(planDir); os.IsNotExist(err) {
		return "", fmt.Errorf("no plans found")
	}

	// 读取目录
	entries, err := os.ReadDir(planDir)
	if err != nil {
		return "", fmt.Errorf("failed to read plan directory: %w", err)
	}

	// 过滤出计划文件
	var planFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			planFiles = append(planFiles, entry.Name())
		}
	}

	if len(planFiles) == 0 {
		return "", fmt.Errorf("no plan files found")
	}

	// 按文件名排序（文件名包含时间戳）
	sort.Strings(planFiles)

	// 返回最新的文件
	latestFile := planFiles[len(planFiles)-1]
	return filepath.Join(planDir, latestFile), nil
}
