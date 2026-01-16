package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/anthropics/claude-code-go/internal/agentregistry"
)

// TaskTool 任务工具，用于调用子 Agent
type TaskTool struct {
	agentRegistry *agentregistry.Registry
	executor      TaskExecutor
}

// TaskExecutor 定义执行子 Agent 的接口
type TaskExecutor interface {
	ExecuteAgent(ctx context.Context, agentName string, prompt string) (string, error)
}

// NewTaskTool 创建新的任务工具
func NewTaskTool(registry *agentregistry.Registry, executor TaskExecutor) *TaskTool {
	return &TaskTool{
		agentRegistry: registry,
		executor:      executor,
	}
}

func (t *TaskTool) Name() string {
	return "task"
}

func (t *TaskTool) Description() string {
	return `Launch a specialized agent to handle complex, multi-step tasks autonomously.

Available agents:
- explore: Fast agent for codebase exploration (read-only)
- general: General-purpose agent for multi-step tasks

Usage:
- Use @explore for quick code searches and analysis
- Use @general for complex tasks requiring multiple steps
- Agents run independently and return their results`
}

func (t *TaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"subagent_type": map[string]interface{}{
				"type":        "string",
				"description": "The type of agent to launch (explore, general)",
				"enum":        []string{"explore", "general"},
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "A short description (3-5 words) of what the agent will do",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
			"run_in_background": map[string]interface{}{
				"type":        "boolean",
				"description": "Set to true to run this agent in the background",
				"default":     false,
			},
		},
		"required": []string{"subagent_type", "description", "prompt"},
	}
}

// TaskInput 任务工具的输入
type TaskInput struct {
	SubagentType     string `json:"subagent_type"`
	Description      string `json:"description"`
	Prompt           string `json:"prompt"`
	RunInBackground  bool   `json:"run_in_background"`
}

func (t *TaskTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
	// 解析输入
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	var taskInput TaskInput
	if err := json.Unmarshal(inputJSON, &taskInput); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	// 验证必需字段
	if taskInput.SubagentType == "" {
		return nil, fmt.Errorf("subagent_type is required")
	}
	if taskInput.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// 映射 subagent_type 到 agent name
	agentName := mapSubagentType(taskInput.SubagentType)

	// 检查 agent 是否存在
	agent, err := t.agentRegistry.Get(agentName)
	if err != nil {
		return nil, fmt.Errorf("agent %s not found: %w", agentName, err)
	}

	// 检查是否是 subagent
	if !agent.IsSubagent() {
		return nil, fmt.Errorf("agent %s is not a subagent", agentName)
	}

	// 执行 agent
	if taskInput.RunInBackground {
		// 后台执行
		go func() {
			_, _ = t.executor.ExecuteAgent(context.Background(), agentName, taskInput.Prompt)
		}()

		return &Result{
			Output: fmt.Sprintf("Agent '%s' launched in background: %s", agentName, taskInput.Description),
		}, nil
	} else {
		// 同步执行
		result, err := t.executor.ExecuteAgent(ctx, agentName, taskInput.Prompt)
		if err != nil {
			return nil, fmt.Errorf("agent execution failed: %w", err)
		}

		return &Result{
			Output: result,
		}, nil
	}
}

// mapSubagentType 将 subagent_type 映射到实际的 agent 名称
func mapSubagentType(subagentType string) string {
	switch subagentType {
	case "explore":
		return "explore"
	case "general":
		return "build" // general 使用 build agent
	default:
		return subagentType
	}
}

// ParallelTaskExecutor 并行任务执行器
type ParallelTaskExecutor struct {
	maxConcurrency int
	executor       TaskExecutor
}

// NewParallelTaskExecutor 创建新的并行任务执行器
func NewParallelTaskExecutor(executor TaskExecutor, maxConcurrency int) *ParallelTaskExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = 3 // 默认最多3个并发
	}
	return &ParallelTaskExecutor{
		maxConcurrency: maxConcurrency,
		executor:       executor,
	}
}

// ExecuteTask 执行单个任务
type ExecuteTask struct {
	AgentName string
	Prompt    string
}

// TaskResult 任务结果
type TaskResult struct {
	AgentName string
	Result    string
	Error     error
}

// ExecuteParallel 并行执行多个任务
func (p *ParallelTaskExecutor) ExecuteParallel(ctx context.Context, tasks []ExecuteTask) []TaskResult {
	results := make([]TaskResult, len(tasks))
	var wg sync.WaitGroup

	// 使用 channel 限制并发
	sem := make(chan struct{}, p.maxConcurrency)

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t ExecuteTask) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 执行任务
			result, err := p.executor.ExecuteAgent(ctx, t.AgentName, t.Prompt)
			results[index] = TaskResult{
				AgentName: t.AgentName,
				Result:    result,
				Error:     err,
			}
		}(i, task)
	}

	wg.Wait()
	return results
}
