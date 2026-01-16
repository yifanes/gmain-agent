package compaction

import (
	"time"

	"github.com/anthropics/claude-code-go/internal/api"
)

const (
	// ProtectRecent 保护最近的 N 个对话回合
	ProtectRecent = 2

	// ProtectTokens 保护最近 N tokens 的工具输出
	ProtectTokens = 40000

	// PruneMinimum 最少修剪量（字符数）
	PruneMinimum = 20000
)

// ProtectedTools 特殊工具不被修剪
var ProtectedTools = map[string]bool{
	"skill":      true,
	"plan_exit":  true,
	"plan_enter": true,
}

// PruneResult 修剪结果
type PruneResult struct {
	PrunedCount int       // 修剪的工具结果数量
	PrunedChars int       // 修剪的字符数
	Messages    []api.Message // 修剪后的消息列表
}

// Prune 修剪工具输出
func Prune(messages []api.Message) PruneResult {
	// 如果消息太少，不修剪
	if len(messages) < ProtectRecent*2 {
		return PruneResult{
			PrunedCount: 0,
			PrunedChars: 0,
			Messages:    messages,
		}
	}

	prunedCount := 0
	prunedChars := 0
	protectFromIndex := len(messages) - ProtectRecent*2

	// 创建消息副本
	result := make([]api.Message, len(messages))
	copy(result, messages)

	// 向后遍历消息（从旧到新）
	for i := protectFromIndex - 1; i >= 0; i-- {
		msg := &result[i]

		if msg.Role != api.RoleAssistant {
			continue
		}

		// 遍历内容块
		for j := range msg.Content {
			content := &msg.Content[j]

			// 只修剪 tool_result
			if content.Type != api.ContentTypeToolResult {
				continue
			}

			// 检查是否是保护的工具
			if ProtectedTools[content.Name] {
				continue
			}

			// 检查是否已经被标记为已修剪
			if content.Pruned {
				continue
			}

			// 修剪输出
			originalLen := len(content.Content)
			if originalLen > 0 {
				content.Content = "[Output pruned to save context]"
				content.Pruned = true
				content.PrunedAt = time.Now()

				prunedChars += originalLen
				prunedCount++

				// 如果修剪量足够，停止
				if prunedChars >= PruneMinimum {
					return PruneResult{
						PrunedCount: prunedCount,
						PrunedChars: prunedChars,
						Messages:    result,
					}
				}
			}
		}
	}

	return PruneResult{
		PrunedCount: prunedCount,
		PrunedChars: prunedChars,
		Messages:    result,
	}
}

// CanPrune 检查是否可以修剪
func CanPrune(messages []api.Message) bool {
	return len(messages) >= ProtectRecent*2
}

// CountPrunableContent 统计可修剪的内容量
func CountPrunableContent(messages []api.Message) int {
	if !CanPrune(messages) {
		return 0
	}

	count := 0
	protectFromIndex := len(messages) - ProtectRecent*2

	for i := protectFromIndex - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Role != api.RoleAssistant {
			continue
		}

		for _, content := range msg.Content {
			if content.Type != api.ContentTypeToolResult && !content.Pruned && !ProtectedTools[content.Name] {
				count += len(content.Content)
			}
		}
	}

	return count
}
