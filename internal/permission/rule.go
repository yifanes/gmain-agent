package permission

import "encoding/json"

// Action 权限动作
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
)

// Rule 权限规则
type Rule struct {
	Permission string `json:"permission"` // 工具名称：bash, edit, read, etc.
	Pattern    string `json:"pattern"`    // 匹配模式：/path/to/*, *.go, etc.
	Action     Action `json:"action"`     // allow, deny, ask
}

// Ruleset 规则集
type Ruleset struct {
	Rules      []Rule `json:"rules"`
	AllowAll   bool   `json:"allow_all"`   // 全部允许
	DenyAll    bool   `json:"deny_all"`    // 全部拒绝
	DefaultAsk bool   `json:"default_ask"` // 默认询问
}

// RejectedError 权限拒绝错误
type RejectedError struct {
	Permission string
	Pattern    string
	Message    string
}

func (e *RejectedError) Error() string {
	return e.Message
}

// DefaultRuleset 返回默认规则集（默认询问）
func DefaultRuleset() Ruleset {
	return Ruleset{
		Rules:      []Rule{},
		AllowAll:   false,
		DenyAll:    false,
		DefaultAsk: true,
	}
}

// AllowAllRuleset 返回允许所有的规则集
func AllowAllRuleset() Ruleset {
	return Ruleset{
		Rules:      []Rule{},
		AllowAll:   true,
		DenyAll:    false,
		DefaultAsk: false,
	}
}

// DenyAllRuleset 返回拒绝所有的规则集
func DenyAllRuleset() Ruleset {
	return Ruleset{
		Rules:      []Rule{},
		AllowAll:   false,
		DenyAll:    true,
		DefaultAsk: false,
	}
}

// UnmarshalJSON 自定义 JSON 解析
func (r *Ruleset) UnmarshalJSON(data []byte) error {
	type Alias Ruleset
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 初始化默认值
	if r.Rules == nil {
		r.Rules = []Rule{}
	}

	return nil
}

// AddRule 添加规则
func (r *Ruleset) AddRule(permission, pattern string, action Action) {
	r.Rules = append(r.Rules, Rule{
		Permission: permission,
		Pattern:    pattern,
		Action:     action,
	})
}
