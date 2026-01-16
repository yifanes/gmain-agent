package permission

import (
	"context"
	"fmt"
)

// Manager 权限管理器（组合 Evaluator 和 DoomLoopDetector）
type Manager struct {
	evaluator        *Evaluator
	doomLoopDetector *DoomLoopDetector
}

// NewManager 创建新的权限管理器
func NewManager() *Manager {
	return &Manager{
		evaluator:        NewEvaluator(),
		doomLoopDetector: NewDoomLoopDetector(),
	}
}

// CheckInput 权限检查输入
type CheckInput struct {
	SessionID  string
	Permission string
	Pattern    string
	Args       interface{}
	Ruleset    Ruleset
	Message    string
	AskFunc    func(AskRequest) (AskResponse, error)
}

// Check 检查权限（包含 Doom Loop 检测）
func (m *Manager) Check(ctx context.Context, input CheckInput) error {
	// 1. Doom Loop 检测
	if m.doomLoopDetector.Check(input.SessionID, input.Permission, input.Args) {
		count := m.doomLoopDetector.GetCount(input.SessionID, input.Permission, input.Args)

		// 如果陷入 Doom Loop，强制询问用户
		if input.AskFunc != nil {
			message := fmt.Sprintf(
				"⚠️  Potential infinite loop detected: tool '%s' has been called %d times with the same arguments.\n"+
					"This might indicate an issue. Do you want to continue?",
				input.Permission,
				count,
			)

			response, err := input.AskFunc(AskRequest{
				Permission: input.Permission,
				Pattern:    input.Pattern,
				Message:    message,
			})
			if err != nil {
				return err
			}

			if response.Rejected {
				return &RejectedError{
					Permission: input.Permission,
					Pattern:    input.Pattern,
					Message:    "User rejected due to potential infinite loop",
				}
			}

			// 重置计数器（用户已确认）
			m.doomLoopDetector.ResetTool(input.SessionID, input.Permission)
		}
	}

	// 2. 权限评估
	return m.evaluator.Ask(ctx, AskInput{
		SessionID:  input.SessionID,
		Permission: input.Permission,
		Pattern:    input.Pattern,
		Ruleset:    input.Ruleset,
		Message:    input.Message,
		AskFunc:    input.AskFunc,
	})
}

// CheckSimple 简单权限检查（不包含 Doom Loop 检测）
func (m *Manager) CheckSimple(ctx context.Context, input AskInput) error {
	return m.evaluator.Ask(ctx, input)
}

// Evaluate 仅评估规则（不触发询问）
func (m *Manager) Evaluate(permission, pattern string, ruleset Ruleset) Action {
	return m.evaluator.Evaluate(permission, pattern, ruleset)
}

// ClearSession 清除会话的所有数据
func (m *Manager) ClearSession(sessionID string) {
	m.evaluator.ClearSession(sessionID)
	m.doomLoopDetector.Reset(sessionID)
}

// GetSessionApprovals 获取会话的所有批准
func (m *Manager) GetSessionApprovals(sessionID string) map[string]bool {
	return m.evaluator.GetSessionApprovals(sessionID)
}

// GetDoomLoopStats 获取 Doom Loop 统计信息
func (m *Manager) GetDoomLoopStats(sessionID string) map[string]map[string]int {
	return m.doomLoopDetector.GetStats(sessionID)
}

// IsRejectedError 检查是否是权限拒绝错误
func IsRejectedError(err error) bool {
	_, ok := err.(*RejectedError)
	return ok
}
