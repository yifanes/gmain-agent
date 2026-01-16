package permission

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	// DoomLoopThreshold 触发 Doom Loop 的阈值
	DoomLoopThreshold = 3

	// CleanupInterval 清理间隔
	CleanupInterval = 1 * time.Hour
)

// DoomLoopDetector Doom Loop 检测器
type DoomLoopDetector struct {
	mu sync.Mutex

	// sessionID -> toolName -> hash -> count
	history map[string]map[string]map[string]int

	// 最后清理时间
	lastCleanup time.Time
}

// NewDoomLoopDetector 创建新的 Doom Loop 检测器
func NewDoomLoopDetector() *DoomLoopDetector {
	return &DoomLoopDetector{
		history:     make(map[string]map[string]map[string]int),
		lastCleanup: time.Now(),
	}
}

// Check 检查是否陷入 Doom Loop
// 如果同一工具使用相同参数被调用 N 次（默认3次），返回 true
func (d *DoomLoopDetector) Check(sessionID, toolName string, args interface{}) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 定期清理（每小时）
	if time.Since(d.lastCleanup) > CleanupInterval {
		d.cleanup()
	}

	// 计算参数哈希
	hash := d.hashArgs(args)

	// 初始化结构
	if d.history[sessionID] == nil {
		d.history[sessionID] = make(map[string]map[string]int)
	}
	if d.history[sessionID][toolName] == nil {
		d.history[sessionID][toolName] = make(map[string]int)
	}

	// 增加计数
	d.history[sessionID][toolName][hash]++

	// 检查是否达到阈值
	count := d.history[sessionID][toolName][hash]
	return count >= DoomLoopThreshold
}

// GetCount 获取特定工具调用的计数
func (d *DoomLoopDetector) GetCount(sessionID, toolName string, args interface{}) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.history[sessionID] == nil {
		return 0
	}
	if d.history[sessionID][toolName] == nil {
		return 0
	}

	hash := d.hashArgs(args)
	return d.history[sessionID][toolName][hash]
}

// Reset 重置会话的 Doom Loop 检测
func (d *DoomLoopDetector) Reset(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.history, sessionID)
}

// ResetTool 重置特定工具的 Doom Loop 检测
func (d *DoomLoopDetector) ResetTool(sessionID, toolName string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.history[sessionID] != nil {
		delete(d.history[sessionID], toolName)
	}
}

// hashArgs 计算参数哈希
func (d *DoomLoopDetector) hashArgs(args interface{}) string {
	// 将参数序列化为 JSON
	data, err := json.Marshal(args)
	if err != nil {
		// 如果序列化失败，使用字符串表示
		data = []byte(fmt.Sprintf("%v", args))
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(data)

	// 返回前 8 字节的十六进制表示
	return fmt.Sprintf("%x", hash[:8])
}

// cleanup 清理旧数据
func (d *DoomLoopDetector) cleanup() {
	// 简单实现：清空所有历史
	// 生产环境可以使用更智能的策略（如 TTL）
	d.history = make(map[string]map[string]map[string]int)
	d.lastCleanup = time.Now()
}

// GetStats 获取统计信息
func (d *DoomLoopDetector) GetStats(sessionID string) map[string]map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.history[sessionID] == nil {
		return map[string]map[string]int{}
	}

	// 返回副本
	result := make(map[string]map[string]int)
	for tool, hashes := range d.history[sessionID] {
		result[tool] = make(map[string]int)
		for hash, count := range hashes {
			result[tool][hash] = count
		}
	}
	return result
}
