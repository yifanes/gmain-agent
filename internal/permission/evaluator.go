package permission

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
)

// Evaluator 权限评估器
type Evaluator struct {
	mu sync.RWMutex

	// 会话级别的临时授权（"always" 选项）
	sessionApprovals map[string]map[string]bool // sessionID -> key -> approved
}

// NewEvaluator 创建新的权限评估器
func NewEvaluator() *Evaluator {
	return &Evaluator{
		sessionApprovals: make(map[string]map[string]bool),
	}
}

// AskInput 权限请求输入
type AskInput struct {
	SessionID  string
	Permission string
	Pattern    string
	Ruleset    Ruleset
	Message    string
	AskFunc    func(AskRequest) (AskResponse, error)
}

// AskRequest 权限请求
type AskRequest struct {
	Permission string
	Pattern    string
	Message    string
}

// AskResponse 权限响应
type AskResponse struct {
	Approved bool // 是否批准
	Rejected bool // 是否拒绝
	Always   bool // 是否总是允许（会话级别）
}

// Ask 请求权限
func (e *Evaluator) Ask(ctx context.Context, input AskInput) error {
	// 1. 检查是否有会话级别的批准
	if e.hasSessionApproval(input.SessionID, input.Permission, input.Pattern) {
		return nil
	}

	// 2. 评估规则
	action := e.Evaluate(input.Permission, input.Pattern, input.Ruleset)

	switch action {
	case ActionAllow:
		return nil

	case ActionDeny:
		return &RejectedError{
			Permission: input.Permission,
			Pattern:    input.Pattern,
			Message:    fmt.Sprintf("Permission denied: %s %s", input.Permission, input.Pattern),
		}

	case ActionAsk:
		// 3. 如果没有提供 AskFunc，默认拒绝
		if input.AskFunc == nil {
			return &RejectedError{
				Permission: input.Permission,
				Pattern:    input.Pattern,
				Message:    "Permission required but no ask function provided",
			}
		}

		// 4. 询问用户
		response, err := input.AskFunc(AskRequest{
			Permission: input.Permission,
			Pattern:    input.Pattern,
			Message:    input.Message,
		})
		if err != nil {
			return err
		}

		if response.Rejected {
			return &RejectedError{
				Permission: input.Permission,
				Pattern:    input.Pattern,
				Message:    "User rejected permission request",
			}
		}

		// 5. 如果选择 "always"，记录会话批准
		if response.Always {
			e.addSessionApproval(input.SessionID, input.Permission, input.Pattern)
		}

		return nil
	}

	return nil
}

// Evaluate 评估权限规则
func (e *Evaluator) Evaluate(permission, pattern string, ruleset Ruleset) Action {
	// 1. 检查全局规则
	if ruleset.AllowAll {
		return ActionAllow
	}
	if ruleset.DenyAll {
		return ActionDeny
	}

	// 2. 遍历规则，寻找匹配
	for _, rule := range ruleset.Rules {
		// 检查权限是否匹配
		if rule.Permission != permission && rule.Permission != "*" {
			continue
		}

		// 检查模式是否匹配（使用 filepath.Match 进行 glob 匹配）
		matched, err := filepath.Match(rule.Pattern, pattern)
		if err != nil {
			// 如果模式无效，跳过
			continue
		}

		if matched {
			// 第一个匹配的规则生效
			return rule.Action
		}
	}

	// 3. 默认动作
	if ruleset.DefaultAsk {
		return ActionAsk
	}
	return ActionAsk // 默认询问
}

// hasSessionApproval 检查是否有会话级别的批准
func (e *Evaluator) hasSessionApproval(sessionID, permission, pattern string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	approvals, exists := e.sessionApprovals[sessionID]
	if !exists {
		return false
	}

	key := fmt.Sprintf("%s:%s", permission, pattern)
	return approvals[key]
}

// addSessionApproval 添加会话级别的批准
func (e *Evaluator) addSessionApproval(sessionID, permission, pattern string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.sessionApprovals[sessionID] == nil {
		e.sessionApprovals[sessionID] = make(map[string]bool)
	}

	key := fmt.Sprintf("%s:%s", permission, pattern)
	e.sessionApprovals[sessionID][key] = true
}

// ClearSession 清除会话的所有批准
func (e *Evaluator) ClearSession(sessionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.sessionApprovals, sessionID)
}

// GetSessionApprovals 获取会话的所有批准
func (e *Evaluator) GetSessionApprovals(sessionID string) map[string]bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	approvals, exists := e.sessionApprovals[sessionID]
	if !exists {
		return map[string]bool{}
	}

	// 返回副本
	result := make(map[string]bool)
	for k, v := range approvals {
		result[k] = v
	}
	return result
}
