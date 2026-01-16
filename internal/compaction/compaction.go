package compaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/claude-code-go/internal/api"
)

// Compactor 压缩器
type Compactor struct {
	client *api.Client
}

// NewCompactor 创建新的压缩器
func NewCompactor(client *api.Client) *Compactor {
	return &Compactor{
		client: client,
	}
}

// CompactInput 压缩输入
type CompactInput struct {
	Messages     []api.Message
	Model        string
	MaxTokens    int
	KeepRecent   int // 保留最近的 N 条消息
}

// CompactResult 压缩结果
type CompactResult struct {
	Summary       string        // 生成的摘要
	OriginalCount int           // 原始消息数量
	CompactedCount int          // 被压缩的消息数量
	Messages      []api.Message // 压缩后的消息列表
}

// Compact 压缩会话（简化版本）
// 完整版本需要调用 compaction agent，这里先实现简单版本
func (c *Compactor) Compact(ctx context.Context, input CompactInput) (*CompactResult, error) {
	if input.KeepRecent == 0 {
		input.KeepRecent = 2 // 默认保留最近 2 轮对话
	}

	if input.Model == "" {
		input.Model = "claude-sonnet-4-20250514"
	}

	if input.MaxTokens == 0 {
		input.MaxTokens = 4000
	}

	originalCount := len(input.Messages)

	// 如果消息太少，不压缩
	if originalCount <= input.KeepRecent*2 {
		return &CompactResult{
			Summary:        "",
			OriginalCount:  originalCount,
			CompactedCount: 0,
			Messages:       input.Messages,
		}, nil
	}

	// 计算需要压缩的消息范围
	compactUntilIndex := originalCount - input.KeepRecent*2

	// 提取需要压缩的消息
	messagesToCompact := input.Messages[:compactUntilIndex]
	messagesToKeep := input.Messages[compactUntilIndex:]

	// 生成摘要
	summary, err := c.generateSummary(ctx, messagesToCompact, input.Model, input.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// 创建新的消息列表
	// 1. 添加摘要消息
	compactedMessages := []api.Message{
		{
			Role: api.RoleUser,
			Content: []api.Content{
				{
					Type: api.ContentTypeText,
					Text: fmt.Sprintf("[Previous conversation summary]\n\n%s", summary),
				},
			},
		},
	}

	// 2. 添加保留的消息
	compactedMessages = append(compactedMessages, messagesToKeep...)

	return &CompactResult{
		Summary:        summary,
		OriginalCount:  originalCount,
		CompactedCount: compactUntilIndex,
		Messages:       compactedMessages,
	}, nil
}

// generateSummary 生成摘要
func (c *Compactor) generateSummary(ctx context.Context, messages []api.Message, model string, maxTokens int) (string, error) {
	// 1. 构建历史文本
	historyText := c.buildHistoryText(messages)

	// 2. 生成摘要请求
	systemPrompt := `You are a summarization assistant. Your task is to create a concise but comprehensive summary of the conversation history.

Focus on:
- Key decisions and actions taken
- Important technical details and context
- Unresolved issues or ongoing tasks
- File changes and code modifications

Keep the summary clear and organized.`

	req := &api.MessagesRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []api.Message{
			{
				Role: api.RoleUser,
				Content: []api.Content{
					{
						Type: api.ContentTypeText,
						Text: fmt.Sprintf("Please summarize the following conversation:\n\n%s", historyText),
					},
				},
			},
		},
		System: systemPrompt,
	}

	// 3. 调用 API
	resp, err := c.client.CreateMessage(ctx, req)
	if err != nil {
		return "", err
	}

	// 4. 提取摘要文本
	if len(resp.Content) > 0 && resp.Content[0].Type == api.ContentTypeText {
		return resp.Content[0].Text, nil
	}

	return "", fmt.Errorf("failed to extract summary from response")
}

// buildHistoryText 构建历史文本
func (c *Compactor) buildHistoryText(messages []api.Message) string {
	var builder strings.Builder

	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("\n[%s]\n", msg.Role))

		for _, content := range msg.Content {
			switch content.Type {
			case api.ContentTypeText:
				builder.WriteString(content.Text)
				builder.WriteString("\n")

			case api.ContentTypeToolUse:
				builder.WriteString(fmt.Sprintf("[Tool Called: %s]\n", content.Name))
				if len(content.Input) > 0 {
					builder.WriteString(fmt.Sprintf("Input: %s\n", string(content.Input)))
				}

			case api.ContentTypeToolResult:
				builder.WriteString(fmt.Sprintf("[Tool Result: %s]\n", content.ToolUseID))
				// 限制工具结果长度
				if len(content.Content) > 500 {
					builder.WriteString(content.Content[:500])
					builder.WriteString("...\n")
				} else {
					builder.WriteString(content.Content)
					builder.WriteString("\n")
				}
			}
		}
	}

	return builder.String()
}

// ShouldCompact 检查是否应该压缩
func (c *Compactor) ShouldCompact(usage TokenUsage, limits ModelLimits) bool {
	return NeedsCompaction(usage, limits)
}
