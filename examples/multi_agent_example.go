// +build ignore

// 多 Agent 系统使用示例
package main

import (
	"fmt"

	"github.com/anthropics/claude-code-go/internal/agentregistry"
	"github.com/anthropics/claude-code-go/internal/permission"
)

func main() {
	fmt.Println("=== 多 Agent 系统示例 ===\n")

	// 1. 创建 Agent 注册表
	registry := agentregistry.NewRegistry()

	// 2. 注册内置 Agent
	if err := agentregistry.RegisterBuiltinAgents(registry); err != nil {
		panic(err)
	}

	fmt.Println("✓ 已注册内置 Agent")
	fmt.Printf("  总数: %d\n\n", registry.Count())

	// 3. 列出所有 Agent
	fmt.Println("--- 所有 Agent ---")
	agents := registry.List(false)
	for _, agent := range agents {
		fmt.Printf("  • %s (%s)\n", agent.Name, agent.Mode)
		fmt.Printf("    描述: %s\n", agent.Description)
		fmt.Printf("    颜色: %s\n", agent.Color)
		fmt.Println()
	}

	// 4. 获取并检查 build agent
	fmt.Println("--- Build Agent 详情 ---")
	buildAgent, _ := registry.Get("build")
	fmt.Printf("名称: %s\n", buildAgent.Name)
	fmt.Printf("模式: %s\n", buildAgent.Mode)
	fmt.Printf("温度: %.1f\n", buildAgent.Temperature)
	fmt.Printf("权限规则数: %d\n", len(buildAgent.Permission.Rules))
	fmt.Printf("默认动作: Ask=%v\n\n", buildAgent.Permission.DefaultAsk)

	// 5. 获取并检查 plan agent
	fmt.Println("--- Plan Agent 详情 ---")
	planAgent, _ := registry.Get("plan")
	fmt.Printf("名称: %s\n", planAgent.Name)
	fmt.Printf("模式: %s\n", planAgent.Mode)
	fmt.Printf("系统提示: %.80s...\n", planAgent.SystemPrompt)
	fmt.Println()

	// 6. 获取并检查 explore agent
	fmt.Println("--- Explore Agent 详情 ---")
	exploreAgent, _ := registry.Get("explore")
	fmt.Printf("名称: %s\n", exploreAgent.Name)
	fmt.Printf("模式: %s\n", exploreAgent.Mode)
	fmt.Printf("是否是子 Agent: %v\n", exploreAgent.IsSubagent())
	fmt.Printf("最大步数: %v\n\n", exploreAgent.Options["maxSteps"])

	// 7. 测试权限检查
	fmt.Println("--- 权限测试 ---")
	testPermissions(buildAgent)
	testPermissions(planAgent)
	testPermissions(exploreAgent)

	// 8. 创建自定义 Agent
	fmt.Println("\n--- 创建自定义 Agent ---")
	customAgent := agentregistry.AgentInfo{
		Name:        "custom-reviewer",
		Description: "代码审查专家",
		Mode:        agentregistry.ModeSubagent,
		Native:      false,
		Temperature: 0.3,
		Permission: permission.Ruleset{
			Rules: []permission.Rule{
				{Permission: "read", Pattern: "*", Action: permission.ActionAllow},
				{Permission: "grep", Pattern: "*", Action: permission.ActionAllow},
			},
			DefaultAsk: false,
		},
		SystemPrompt: "You are a code review specialist.",
		Color:        "#f59e0b",
	}

	if err := registry.Register(customAgent); err != nil {
		fmt.Printf("❌ 注册失败: %v\n", err)
	} else {
		fmt.Printf("✓ 成功注册自定义 Agent: %s\n", customAgent.Name)
		fmt.Printf("  总 Agent 数: %d\n", registry.Count())
	}

	// 9. 列出不同模式的 Agent
	fmt.Println("\n--- 按模式列出 Agent ---")
	primaryAgents := registry.ListByMode(agentregistry.ModePrimary, false)
	fmt.Printf("主 Agent (Primary): %d 个\n", len(primaryAgents))
	for _, a := range primaryAgents {
		fmt.Printf("  • %s\n", a.Name)
	}

	subagents := registry.ListByMode(agentregistry.ModeSubagent, false)
	fmt.Printf("\n子 Agent (Subagent): %d 个\n", len(subagents))
	for _, a := range subagents {
		fmt.Printf("  • %s\n", a.Name)
	}
}

func testPermissions(agent *agentregistry.AgentInfo) {
	fmt.Printf("Agent: %s\n", agent.Name)

	// 测试一些权限
	testCases := []struct {
		tool    string
		pattern string
	}{
		{"read", "main.go"},
		{"edit", "main.go"},
		{"bash", "rm -rf /"},
		{"write", ".gmain-agent/plans/test.md"},
	}

	evalMgr := permission.NewEvaluator()
	for _, tc := range testCases {
		action := evalMgr.Evaluate(tc.tool, tc.pattern, agent.Permission)
		symbol := getActionSymbol(action)
		fmt.Printf("  %s %s %s -> %s\n", symbol, tc.tool, tc.pattern, action)
	}
	fmt.Println()
}

func getActionSymbol(action permission.Action) string {
	switch action {
	case permission.ActionAllow:
		return "✓"
	case permission.ActionDeny:
		return "✗"
	case permission.ActionAsk:
		return "?"
	default:
		return " "
	}
}
