package agentregistry

import (
	"github.com/anthropics/claude-code-go/internal/permission"
)

// AgentMode 定义 Agent 的模式
type AgentMode string

const (
	ModePrimary  AgentMode = "primary"  // 主 Agent（用户直接交互）
	ModeSubagent AgentMode = "subagent" // 子 Agent（由其他 Agent 调用）
	ModeAll      AgentMode = "all"      // 两者都可以
)

// AgentInfo 定义 Agent 的完整信息
type AgentInfo struct {
	// 基本信息
	Name        string    `json:"name"`                   // Agent 名称（唯一标识）
	Description string    `json:"description,omitempty"`  // Agent 描述
	Mode        AgentMode `json:"mode"`                   // Agent 模式
	Native      bool      `json:"native"`                 // 是否内置
	Hidden      bool      `json:"hidden"`                 // 是否隐藏（不在列表中显示）

	// 模型配置
	Model       string  `json:"model,omitempty"`       // 默认模型（如果为空，使用全局配置）
	Temperature float64 `json:"temperature,omitempty"` // 温度参数
	TopP        float64 `json:"top_p,omitempty"`       // TopP 参数
	MaxSteps    int     `json:"max_steps,omitempty"`   // 最大步数（0 表示无限制）

	// 权限配置
	Permission permission.Ruleset `json:"permission"` // 权限规则集

	// 系统提示
	SystemPrompt string `json:"system_prompt,omitempty"` // 自定义系统提示

	// UI 配置
	Color string `json:"color,omitempty"` // UI 颜色（用于显示）

	// 其他选项
	Options map[string]interface{} `json:"options,omitempty"` // 其他模型选项
}

// DefaultAgentInfo 创建默认的 Agent 配置
func DefaultAgentInfo(name string) AgentInfo {
	return AgentInfo{
		Name:         name,
		Mode:         ModePrimary,
		Native:       false,
		Hidden:       false,
		Model:        "",
		Temperature:  0,
		TopP:         0,
		MaxSteps:     0,
		Permission:   permission.DefaultRuleset(),
		SystemPrompt: "",
		Color:        "",
		Options:      make(map[string]interface{}),
	}
}

// IsPrimary 检查是否是主 Agent
func (a *AgentInfo) IsPrimary() bool {
	return a.Mode == ModePrimary || a.Mode == ModeAll
}

// IsSubagent 检查是否是子 Agent
func (a *AgentInfo) IsSubagent() bool {
	return a.Mode == ModeSubagent || a.Mode == ModeAll
}

// CanBeCalledBy 检查是否可以被指定角色调用
func (a *AgentInfo) CanBeCalledBy(role AgentMode) bool {
	switch role {
	case ModePrimary:
		return a.IsPrimary()
	case ModeSubagent:
		return a.IsSubagent()
	case ModeAll:
		return true
	default:
		return false
	}
}

// Clone 克隆 Agent 配置
func (a *AgentInfo) Clone() AgentInfo {
	clone := *a

	// 深拷贝 Permission
	clone.Permission.Rules = make([]permission.Rule, len(a.Permission.Rules))
	copy(clone.Permission.Rules, a.Permission.Rules)

	// 深拷贝 Options
	if a.Options != nil {
		clone.Options = make(map[string]interface{})
		for k, v := range a.Options {
			clone.Options[k] = v
		}
	}

	return clone
}

// WithPermission 设置权限规则集
func (a *AgentInfo) WithPermission(ruleset permission.Ruleset) *AgentInfo {
	a.Permission = ruleset
	return a
}

// WithSystemPrompt 设置系统提示
func (a *AgentInfo) WithSystemPrompt(prompt string) *AgentInfo {
	a.SystemPrompt = prompt
	return a
}

// WithModel 设置模型
func (a *AgentInfo) WithModel(model string) *AgentInfo {
	a.Model = model
	return a
}

// WithTemperature 设置温度
func (a *AgentInfo) WithTemperature(temp float64) *AgentInfo {
	a.Temperature = temp
	return a
}

// WithMaxSteps 设置最大步数
func (a *AgentInfo) WithMaxSteps(steps int) *AgentInfo {
	a.MaxSteps = steps
	return a
}

// GetSystemPrompt 获取系统提示，如果有 workDir 则添加到提示中
func (a *AgentInfo) GetSystemPrompt(workDir string) string {
	if workDir == "" {
		return a.SystemPrompt
	}
	return "Working Directory: " + workDir + "\n\n" + a.SystemPrompt
}
