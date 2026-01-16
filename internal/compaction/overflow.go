package compaction

// TokenUsage Token 使用量
type TokenUsage struct {
	Input     int
	Output    int
	CacheRead int
}

// ModelLimits 模型限制
type ModelLimits struct {
	ContextLimit int // 上下文窗口大小
	OutputLimit  int // 输出 token 限制
}

// DefaultModelLimits 返回默认模型限制（Claude Sonnet 4）
func DefaultModelLimits() ModelLimits {
	return ModelLimits{
		ContextLimit: 200000, // 200K context
		OutputLimit:  8192,   // 8K output
	}
}

// IsOverflow 检查是否上下文溢出
func IsOverflow(usage TokenUsage, limits ModelLimits) bool {
	// 计算已用 token
	used := usage.Input + usage.CacheRead + usage.Output

	// 计算可用 token（上下文限制 - 输出限制）
	available := limits.ContextLimit - limits.OutputLimit

	// 如果已用 > 可用，触发压缩
	return used > available
}

// NeedsCompaction 检查是否需要压缩
// 当使用量超过 80% 时建议压缩
func NeedsCompaction(usage TokenUsage, limits ModelLimits) bool {
	used := usage.Input + usage.CacheRead + usage.Output
	available := limits.ContextLimit - limits.OutputLimit

	threshold := float64(available) * 0.8
	return float64(used) > threshold
}

// CalculateUsage 计算总使用量
func CalculateUsage(usage TokenUsage) int {
	return usage.Input + usage.CacheRead + usage.Output
}

// CalculateAvailable 计算可用空间
func CalculateAvailable(limits ModelLimits) int {
	return limits.ContextLimit - limits.OutputLimit
}

// UsagePercentage 计算使用百分比
func UsagePercentage(usage TokenUsage, limits ModelLimits) float64 {
	used := float64(CalculateUsage(usage))
	available := float64(CalculateAvailable(limits))

	if available == 0 {
		return 0
	}

	return (used / available) * 100
}
