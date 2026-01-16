package compaction

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// MaxOutputLength 最大输出长度（字符）
	MaxOutputLength = 30000

	// TruncateMessage 截断提示消息
	TruncateMessage = "\n\n... (output truncated, %d more characters) ...\n\nFull output saved to: %s"
)

// TruncateResult 截断结果
type TruncateResult struct {
	Content   string // 截断后的内容
	Truncated bool   // 是否被截断
	FilePath  string // 完整输出文件路径
	Original  int    // 原始长度
	Final     int    // 最终长度
}

// TruncateOutput 截断工具输出
func TruncateOutput(output string, sessionID, toolName, callID string) TruncateResult {
	originalLen := len(output)

	// 如果输出未超出限制，直接返回
	if originalLen <= MaxOutputLength {
		return TruncateResult{
			Content:   output,
			Truncated: false,
			FilePath:  "",
			Original:  originalLen,
			Final:     originalLen,
		}
	}

	// 截断输出
	truncated := output[:MaxOutputLength]
	remaining := originalLen - MaxOutputLength

	// 生成文件路径
	outputDir := filepath.Join(os.TempDir(), "gmain-agent", sessionID, "outputs")
	filename := fmt.Sprintf("%s-%s.txt", toolName, callID)
	filePath := filepath.Join(outputDir, filename)

	// 创建目录
	if err := os.MkdirAll(outputDir, 0755); err == nil {
		// 保存完整输出到文件
		if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
			// 如果写入失败，不影响主流程，只是无法保存完整输出
			filePath = "(failed to save)"
		}
	} else {
		filePath = "(failed to save)"
	}

	// 添加截断提示
	message := fmt.Sprintf(TruncateMessage, remaining, filePath)
	finalContent := truncated + message

	return TruncateResult{
		Content:   finalContent,
		Truncated: true,
		FilePath:  filePath,
		Original:  originalLen,
		Final:     len(finalContent),
	}
}

// ShouldTruncate 检查是否应该截断
func ShouldTruncate(output string) bool {
	return len(output) > MaxOutputLength
}

// TruncateWithLimit 使用自定义限制截断
func TruncateWithLimit(output string, limit int, sessionID, toolName, callID string) TruncateResult {
	originalLen := len(output)

	if originalLen <= limit {
		return TruncateResult{
			Content:   output,
			Truncated: false,
			Original:  originalLen,
			Final:     originalLen,
		}
	}

	truncated := output[:limit]
	remaining := originalLen - limit

	outputDir := filepath.Join(os.TempDir(), "gmain-agent", sessionID, "outputs")
	filename := fmt.Sprintf("%s-%s.txt", toolName, callID)
	filePath := filepath.Join(outputDir, filename)

	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filePath, []byte(output), 0644)

	message := fmt.Sprintf(TruncateMessage, remaining, filePath)
	finalContent := truncated + message

	return TruncateResult{
		Content:   finalContent,
		Truncated: true,
		FilePath:  filePath,
		Original:  originalLen,
		Final:     len(finalContent),
	}
}
