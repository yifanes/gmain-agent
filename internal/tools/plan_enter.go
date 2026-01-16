package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PlanEnterTool 进入计划模式的工具
type PlanEnterTool struct {
	workDir       string
	onModeSwitch  func(toAgent string) error
}

// NewPlanEnterTool 创建新的 PlanEnter 工具
func NewPlanEnterTool(workDir string, onModeSwitch func(string) error) *PlanEnterTool {
	return &PlanEnterTool{
		workDir:      workDir,
		onModeSwitch: onModeSwitch,
	}
}

func (t *PlanEnterTool) Name() string {
	return "plan_enter"
}

func (t *PlanEnterTool) Description() string {
	return `Enter planning mode for code analysis and implementation planning.

In planning mode:
- You have read-only access to the codebase
- You can create plan documents in .gmain-agent/plans/
- Focus on understanding requirements and designing solutions
- DO NOT make code changes

Use this before implementing complex features to:
1. Analyze the codebase
2. Understand requirements
3. Design the implementation approach
4. Create a detailed plan

Use plan_exit when ready to implement the plan.`
}

func (t *PlanEnterTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the task you want to plan for",
			},
		},
		"required": []string{"task_description"},
	}
}

// PlanEnterInput PlanEnter 工具的输入
type PlanEnterInput struct {
	TaskDescription string `json:"task_description"`
}

func (t *PlanEnterTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
	// 解析输入
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	var planInput PlanEnterInput
	if err := json.Unmarshal(inputJSON, &planInput); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if planInput.TaskDescription == "" {
		return nil, fmt.Errorf("task_description is required")
	}

	// 创建计划目录
	planDir := filepath.Join(t.workDir, ".gmain-agent", "plans")
	if err := os.MkdirAll(planDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plan directory: %w", err)
	}

	// 创建计划文件
	timestamp := time.Now().Format("20060102-150405")
	planFile := filepath.Join(planDir, fmt.Sprintf("plan-%s.md", timestamp))

	initialContent := fmt.Sprintf(`# Implementation Plan

**Task**: %s
**Created**: %s
**Status**: Planning

## Requirements Analysis

[Analyze the requirements here]

## Current State Analysis

[Analyze the current codebase here]

## Proposed Solution

[Describe your proposed solution here]

## Implementation Steps

1. [Step 1]
2. [Step 2]
3. [Step 3]

## Potential Issues

[List potential issues and how to handle them]

## Testing Strategy

[Describe how to test the implementation]

---
*This plan will be used to guide the implementation when you exit planning mode.*
`, planInput.TaskDescription, time.Now().Format("2006-01-02 15:04:05"))

	if err := os.WriteFile(planFile, []byte(initialContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create plan file: %w", err)
	}

	// 切换到 plan agent
	if t.onModeSwitch != nil {
		if err := t.onModeSwitch("plan"); err != nil {
			return nil, fmt.Errorf("failed to switch to plan mode: %w", err)
		}
	}

	output := fmt.Sprintf(`✓ Entered planning mode

You are now in PLAN MODE with read-only access.

Plan file created: %s

In this mode you can:
- Explore and analyze the codebase (read-only)
- Create and edit plan documents
- Search for code and dependencies
- Design the implementation approach

You CANNOT:
- Edit source code files
- Make any changes to the project

Focus on:
1. Understanding the current codebase
2. Analyzing requirements
3. Designing the solution
4. Creating a detailed implementation plan

When your plan is ready, use the plan_exit tool to switch to implementation mode.`, planFile)

	return &Result{
		Output: output,
	}, nil
}
