package agentregistry

import (
	"github.com/anthropics/claude-code-go/internal/permission"
)

// RegisterBuiltinAgents 注册所有内置 Agent
func RegisterBuiltinAgents(registry *Registry) error {
	agents := []AgentInfo{
		BuildAgent(),
		PlanAgent(),
		ExploreAgent(),
	}

	for _, agent := range agents {
		if err := registry.Register(agent); err != nil {
			return err
		}
	}

	return nil
}

// BuildAgent 返回 build agent 的配置
func BuildAgent() AgentInfo {
	return AgentInfo{
		Name:        "build",
		Description: "Complete development workflow agent with full permissions",
		Mode:        ModePrimary,
		Native:      true,
		Hidden:      false,
		Temperature: 0,
		Permission:  buildPermissions(),
		SystemPrompt: `You are a helpful AI assistant for software development. You have access to various tools to help with coding, file management, and system operations.

Key guidelines:
- Always read files before editing them
- Be careful with destructive operations
- Ask for confirmation before making significant changes
- Provide clear explanations of what you're doing`,
		Color: "#2563eb", // blue
	}
}

// PlanAgent 返回 plan agent 的配置
func PlanAgent() AgentInfo {
	return AgentInfo{
		Name:        "plan",
		Description: "Planning mode agent with read-only access for code analysis and planning",
		Mode:        ModePrimary,
		Native:      true,
		Hidden:      false,
		Temperature: 0,
		Permission:  planPermissions(),
		SystemPrompt: `You are a planning and analysis assistant. Your role is to:
1. Analyze the codebase and understand requirements
2. Create detailed implementation plans
3. Identify potential issues and dependencies
4. Suggest architectural approaches

You have read-only access to the codebase and can create plan documents in .gmain-agent/plans/.

DO NOT make any code changes - your focus is on planning and analysis only.`,
		Color: "#7c3aed", // purple
	}
}

// ExploreAgent 返回 explore agent 的配置
func ExploreAgent() AgentInfo {
	return AgentInfo{
		Name:        "explore",
		Description: "Fast exploration agent with read-only tools for codebase discovery",
		Mode:        ModeSubagent,
		Native:      true,
		Hidden:      false,
		Temperature: 0,
		Permission:  explorePermissions(),
		SystemPrompt: `You are a code exploration specialist. Your task is to quickly navigate and understand codebases.

Focus on:
- Finding relevant files and code patterns
- Understanding project structure
- Identifying key components and dependencies
- Summarizing findings clearly

Use glob, grep, read, and list tools efficiently. Be concise in your responses.`,
		Color:   "#059669", // green
		Options: map[string]interface{}{"maxSteps": 10},
	}
}

// buildPermissions 返回 build agent 的权限配置
func buildPermissions() permission.Ruleset {
	return permission.Ruleset{
		Rules: []permission.Rule{
			// 允许常见的只读操作
			{Permission: "read", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "glob", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "grep", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "webfetch", Pattern: "*", Action: permission.ActionAllow},

			// 编辑操作需要确认
			{Permission: "edit", Pattern: "*.go", Action: permission.ActionAllow},
			{Permission: "edit", Pattern: "*.js", Action: permission.ActionAllow},
			{Permission: "edit", Pattern: "*.ts", Action: permission.ActionAllow},
			{Permission: "edit", Pattern: "*.py", Action: permission.ActionAllow},
			{Permission: "write", Pattern: "*.md", Action: permission.ActionAllow},

			// 危险操作需要询问
			{Permission: "bash", Pattern: "rm *", Action: permission.ActionAsk},
			{Permission: "bash", Pattern: "sudo *", Action: permission.ActionDeny},
			{Permission: "edit", Pattern: "/etc/*", Action: permission.ActionDeny},
		},
		AllowAll:   false,
		DenyAll:    false,
		DefaultAsk: true, // 默认询问
	}
}

// planPermissions 返回 plan agent 的权限配置
func planPermissions() permission.Ruleset {
	return permission.Ruleset{
		Rules: []permission.Rule{
			// 只读工具全部允许
			{Permission: "read", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "glob", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "grep", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "webfetch", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "websearch", Pattern: "*", Action: permission.ActionAllow},

			// 允许写入计划文件
			{Permission: "write", Pattern: ".gmain-agent/plans/*", Action: permission.ActionAllow},
			{Permission: "edit", Pattern: ".gmain-agent/plans/*", Action: permission.ActionAllow},

			// bash 命令需要询问（只允许安全的只读命令）
			{Permission: "bash", Pattern: "ls *", Action: permission.ActionAllow},
			{Permission: "bash", Pattern: "cat *", Action: permission.ActionAllow},
			{Permission: "bash", Pattern: "*", Action: permission.ActionAsk},

			// 禁止所有写入操作（除了计划文件）
			{Permission: "edit", Pattern: "*", Action: permission.ActionDeny},
			{Permission: "write", Pattern: "*", Action: permission.ActionDeny},
		},
		AllowAll:   false,
		DenyAll:    false,
		DefaultAsk: false,
	}
}

// explorePermissions 返回 explore agent 的权限配置
func explorePermissions() permission.Ruleset {
	return permission.Ruleset{
		Rules: []permission.Rule{
			// 只允许只读工具
			{Permission: "read", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "glob", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "grep", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "webfetch", Pattern: "*", Action: permission.ActionAllow},
			{Permission: "websearch", Pattern: "*", Action: permission.ActionAllow},

			// 允许安全的 bash 命令
			{Permission: "bash", Pattern: "ls *", Action: permission.ActionAllow},
			{Permission: "bash", Pattern: "find *", Action: permission.ActionAllow},
			{Permission: "bash", Pattern: "tree *", Action: permission.ActionAllow},

			// 禁止所有写入操作
			{Permission: "edit", Pattern: "*", Action: permission.ActionDeny},
			{Permission: "write", Pattern: "*", Action: permission.ActionDeny},
			{Permission: "bash", Pattern: "*", Action: permission.ActionDeny},
		},
		AllowAll:   false,
		DenyAll:    false,
		DefaultAsk: false,
	}
}

// GetBuiltinAgentNames 返回所有内置 Agent 的名称
func GetBuiltinAgentNames() []string {
	return []string{"build", "plan", "explore"}
}
