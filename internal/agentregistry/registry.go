package agentregistry

import (
	"fmt"
	"sync"
)

// Registry Agent 注册表
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo // name -> AgentInfo

	defaultAgent string // 默认 Agent 名称
}

// NewRegistry 创建新的 Agent 注册表
func NewRegistry() *Registry {
	return &Registry{
		agents:       make(map[string]*AgentInfo),
		defaultAgent: "build", // 默认使用 build agent
	}
}

// Register 注册一个 Agent
func (r *Registry) Register(info AgentInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info.Name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	if _, exists := r.agents[info.Name]; exists {
		return fmt.Errorf("agent %s already registered", info.Name)
	}

	// 克隆以防止外部修改
	clone := info.Clone()
	r.agents[info.Name] = &clone

	return nil
}

// Unregister 注销一个 Agent
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	delete(r.agents, name)
	return nil
}

// Get 获取指定的 Agent
func (r *Registry) Get(name string) (*AgentInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", name)
	}

	// 返回克隆以防止外部修改
	clone := info.Clone()
	return &clone, nil
}

// GetDefault 获取默认 Agent
func (r *Registry) GetDefault() (*AgentInfo, error) {
	return r.Get(r.defaultAgent)
}

// SetDefault 设置默认 Agent
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	r.defaultAgent = name
	return nil
}

// List 列出所有 Agent（可选择是否包含隐藏的）
func (r *Registry) List(includeHidden bool) []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []AgentInfo
	for _, info := range r.agents {
		if !includeHidden && info.Hidden {
			continue
		}
		result = append(result, info.Clone())
	}

	return result
}

// ListByMode 列出指定模式的 Agent
func (r *Registry) ListByMode(mode AgentMode, includeHidden bool) []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []AgentInfo
	for _, info := range r.agents {
		if !includeHidden && info.Hidden {
			continue
		}

		if info.CanBeCalledBy(mode) {
			result = append(result, info.Clone())
		}
	}

	return result
}

// Exists 检查 Agent 是否存在
func (r *Registry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.agents[name]
	return exists
}

// Count 返回注册的 Agent 数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.agents)
}

// Clear 清空所有 Agent（仅用于测试）
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents = make(map[string]*AgentInfo)
	r.defaultAgent = ""
}

// Update 更新 Agent 配置
func (r *Registry) Update(name string, updater func(*AgentInfo) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.agents[name]
	if !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	// 克隆以便在更新失败时不影响原始数据
	clone := info.Clone()

	if err := updater(&clone); err != nil {
		return err
	}

	r.agents[name] = &clone
	return nil
}

// GetNames 获取所有 Agent 名称
func (r *Registry) GetNames(includeHidden bool) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, info := range r.agents {
		if !includeHidden && info.Hidden {
			continue
		}
		names = append(names, name)
	}

	return names
}
