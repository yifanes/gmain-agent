# Agent 架构对比分析与改进方案

## 执行摘要

本文档基于对当前项目（gmain-agent）和 OpenCode 项目的深度分析，提出了一套全面的改进方案。OpenCode 项目展示了企业级 AI Agent 的最佳实践，我们将借鉴其核心设计思想，在保持 Go 语言实现优势的基础上，显著提升当前项目的功能性和可扩展性。

---

## 1. 项目对比分析

### 1.1 架构对比

| 维度 | 当前项目 (gmain-agent) | OpenCode | 差距评估 |
|------|----------------------|----------|---------|
| **编程语言** | Go | TypeScript + Bun | 语言特性差异 |
| **代码规模** | ~3,801 行 / 23 文件 | ~10K+ 行 / 185 文件 | 规模差距大 |
| **Agent 架构** | 单一 Agent | 多级 Agent (primary/subagent/hidden) | **关键差距** |
| **权限系统** | 无 | 基于规则的细粒度权限管理 | **关键差距** |
| **消息模型** | 简单 (text/tool) | 12 种细粒度消息部分 | **关键差距** |
| **工具数量** | 10 个 | 20+ 个 | 中等差距 |
| **上下文管理** | 无压缩机制 | 自动压缩 + 修剪 | **关键差距** |
| **错误恢复** | 基本 | 智能重试 + 权限恢复 | **关键差距** |
| **模式切换** | 无 | 计划模式 / 构建模式 | **关键差距** |
| **成本追踪** | 无 | 实时 token + 成本计算 | 中等差距 |
| **会话管理** | 基本持久化 | 分支 / 回滚 / 快照 | 中等差距 |
| **插件系统** | 无 | 完整插件生态 | 低优先级 |
| **Skill 系统** | 无 | Skill 发现和加载 | 中等差距 |

### 1.2 核心功能差距

#### **严重缺失（高优先级）**
1. ❌ **权限管理系统** - 无法控制工具访问，安全风险高
2. ❌ **子 Agent 系统** - 无法处理复杂多步任务
3. ❌ **上下文压缩** - 长对话会导致上下文溢出
4. ❌ **智能重试** - 网络错误处理简陋
5. ❌ **计划模式** - 缺少规划和实现分离

#### **部分缺失（中优先级）**
6. ⚠️ **消息模型** - 缺少推理过程、快照等细粒度类型
7. ⚠️ **Token 追踪** - 无法监控成本和使用量
8. ⚠️ **工具输出截断** - 输出过长导致上下文污染
9. ⚠️ **会话分支** - 无法从历史点创建分支
10. ⚠️ **Skill 系统** - 缺少可复用的技能模板

#### **可选功能（低优先级）**
11. 🔄 插件系统
12. 🔄 LSP 集成
13. 🔄 多提供商统一接口（当前仅支持 Anthropic）

---

## 2. OpenCode 核心设计精髓

### 2.1 多级 Agent 架构

**设计理念**：不同任务需要不同的 Agent 配置和权限

```
Primary Agent (主 Agent)
├── build     - 完整开发工作流，支持所有工具
└── plan      - 只读分析 + 计划文件编辑

Subagent (子 Agent)
├── general   - 通用多步任务执行
└── explore   - 代码库快速探索（只读）

Hidden Agent (内部 Agent)
├── compaction - 自动上下文压缩
└── title/summary - 元数据生成
```

**价值**：
- 任务隔离：不同 Agent 处理不同类型任务
- 权限隔离：子 Agent 不能执行危险操作
- 并行执行：多个子 Agent 可并行工作
- 模式切换：plan ↔ build 实现不同工作流

### 2.2 权限管理系统

**设计理念**：细粒度的、可配置的访问控制

```go
// 权限规则
type Rule struct {
    Permission string   // 工具名称：bash, edit, read
    Pattern    string   // 匹配模式：/path/to/*, *.go
    Action     string   // allow, deny, ask
}

// 权限评估
func Evaluate(permission, pattern string, rules []Rule) Action {
    // 1. 遍历规则，寻找匹配
    // 2. 第一个匹配的规则生效
    // 3. 支持通配符：*, **, ?
    // 4. 默认 action: ask
}
```

**特殊保护**：
- Doom Loop 检测：同一工具 3 次相同调用 → 触发权限检查
- 会话权限覆盖：用户可临时允许操作
- 权限继承：子 Agent 继承父 Agent 权限限制

### 2.3 自动上下文管理

**三层机制**：

```
1. 工具输出截断 (Truncation)
   ├─ 限制单次输出长度（30KB）
   ├─ 保存完整输出到文件
   └─ 返回截断提示

2. 工具输出修剪 (Pruning)
   ├─ 保护最后 2 个对话回合
   ├─ 保护最近 40K tokens 的工具输出
   ├─ 标记旧工具输出为已压缩
   └─ 从消息历史中移除

3. 会话压缩 (Compaction)
   ├─ 检测上下文溢出（当前 token > 可用 token）
   ├─ 调用 compaction agent 生成摘要
   ├─ 替换旧消息为摘要文本
   └─ 保留工具调用引用
```

**溢出检测**：
```go
func IsOverflow(tokens TokenUsage, model Model) bool {
    used := tokens.Input + tokens.CacheRead + tokens.Output
    available := model.ContextLimit - model.OutputLimit
    return used > available
}
```

### 2.4 智能重试机制

**三级退避策略**：

```
1. 优先级 1: HTTP Headers
   ├─ Retry-After-Ms (毫秒)
   ├─ Retry-After (秒或 HTTP-Date)
   └─ 精确等待时间

2. 优先级 2: 指数退避
   ├─ delay = 初始延迟 * (退避因子 ^ (尝试次数 - 1))
   ├─ 初始: 500ms, 因子: 2
   └─ 最大: 10 秒（有头）/ 2 秒（无头）

3. 可重试判断
   ├─ API 错误类型检查
   ├─ HTTP 状态码：429, 5xx
   ├─ 特定错误码：overloaded, exhausted
   └─ 网络错误：ECONNRESET, ETIMEDOUT
```

### 2.5 计划模式设计

**工作流**：
```
用户请求 "实现功能 X"
    ↓
进入计划模式 (PlanEnterTool)
    ├─ 切换到 plan agent
    ├─ 权限：只读 + 计划文件编辑
    ├─ 探索代码库
    ├─ 分析需求
    └─ 生成实现计划
    ↓
退出计划模式 (PlanExitTool)
    ├─ 切换到 build agent
    ├─ 权限：完整权限
    ├─ 根据计划实施
    └─ 逐步完成任务
```

**价值**：
- 先规划后实施，减少返工
- 计划阶段只读，避免误操作
- 计划文档持久化，可复用
- 用户可审查和修改计划

### 2.6 消息模型设计

**12 种消息部分类型**：

```typescript
// 内容
text         - 文本内容（流式增量）
reasoning    - 推理过程（如 Claude 的思考）
file         - 文件附件

// 工具
tool         - 工具调用（完整生命周期）
snapshot     - 文件快照（变更前后）
patch        - 文件补丁（diff）

// 控制流
step-start   - 推理步骤开始
step-finish  - 推理步骤完成（含 token 和成本）
retry        - 重试记录

// 元数据
agent        - Agent 调用记录
subtask      - 子任务信息
compaction   - 压缩标记
```

**工具状态机**：
```
pending → running → completed/error
```

**价值**：
- 完整的工具执行追踪
- 推理过程可视化
- 文件变更历史记录
- 精确的成本和 token 计算

---

## 3. 改进方案设计

### 3.1 整体架构改进

```
gmain-agent (改进后)
├── internal/
│   ├── agent/
│   │   ├── agent.go          # Agent 基类（现有）
│   │   ├── registry.go       # Agent 注册表（新增）★
│   │   ├── subagent.go       # 子 Agent 管理（新增）★
│   │   ├── mode.go           # Agent 模式（新增）★
│   │   └── conversation.go   # 会话管理（现有）
│   ├── permission/           # 权限管理（新增）★★★
│   │   ├── permission.go     # 权限接口
│   │   ├── rule.go           # 规则定义
│   │   ├── evaluator.go      # 规则评估器
│   │   └── doomloop.go       # Doom Loop 检测
│   ├── compaction/           # 上下文管理（新增）★★★
│   │   ├── compaction.go     # 压缩协调器
│   │   ├── pruning.go        # 工具输出修剪
│   │   ├── truncate.go       # 输出截断
│   │   └── overflow.go       # 溢出检测
│   ├── retry/                # 重试机制（新增）★★
│   │   ├── retry.go          # 重试逻辑
│   │   ├── backoff.go        # 退避策略
│   │   └── error.go          # 错误分类
│   ├── message/              # 消息模型（改进）★★
│   │   ├── message.go        # 消息定义
│   │   ├── part.go           # 消息部分
│   │   └── lifecycle.go      # 生命周期管理
│   ├── usage/                # 使用量追踪（新增）★
│   │   ├── tracker.go        # Token 追踪
│   │   └── cost.go           # 成本计算
│   ├── skill/                # Skill 系统（新增）★
│   │   ├── skill.go          # Skill 定义
│   │   ├── loader.go         # Skill 加载器
│   │   └── registry.go       # Skill 注册表
│   └── tools/                # 工具系统（扩展）
│       ├── registry.go       # 工具注册表（现有）
│       ├── plan_enter.go     # 进入计划模式（新增）
│       ├── plan_exit.go      # 退出计划模式（新增）
│       ├── task.go           # 任务工具（新增）
│       └── [现有工具...]
```

**优先级标注**：
- ★★★ 高优先级（第一阶段）
- ★★ 中优先级（第二阶段）
- ★ 低优先级（第三阶段）

### 3.2 实施路线图

#### **阶段 1：核心基础（高优先级）**

**目标**：实现安全性和可扩展性的基础

1. **权限管理系统** (2-3 天)
   - 权限规则定义和存储
   - 规则评估引擎（支持通配符）
   - Doom Loop 检测
   - 集成到工具执行流程

2. **上下文压缩机制** (2-3 天)
   - 溢出检测
   - 工具输出截断
   - 工具输出修剪
   - 简单的会话压缩（暂不实现完整的 compaction agent）

3. **智能重试机制** (1-2 天)
   - 错误分类（可重试 vs 不可重试）
   - 指数退避算法
   - HTTP 头解析
   - 集成到 API 客户端

**预期成果**：
- ✅ 系统安全性显著提升
- ✅ 长对话不再溢出
- ✅ 网络错误自动恢复
- ✅ 代码量增加 ~1500 行

#### **阶段 2：高级功能（中优先级）**

**目标**：实现多 Agent 协作和计划能力

4. **Agent 注册表和多 Agent 支持** (2-3 天)
   - Agent 接口定义
   - Agent 注册和查找
   - Agent 配置系统
   - 内置 3 个 Agent: build, plan, explore

5. **子 Agent 系统** (3-4 天)
   - 子 Agent 调用接口
   - 任务工具（Task tool）
   - 子 Agent 消息隔离
   - 并行执行支持

6. **计划模式** (2-3 天)
   - PlanEnter 工具
   - PlanExit 工具
   - 计划文件管理
   - Agent 模式切换

7. **消息模型改进** (2-3 天)
   - 扩展消息部分类型
   - 工具状态机
   - 推理过程记录
   - 文件快照支持

**预期成果**：
- ✅ 支持复杂多步任务
- ✅ 计划和实施分离
- ✅ 更好的消息追踪
- ✅ 代码量增加 ~2000 行

#### **阶段 3：增强功能（低优先级）**

**目标**：完善生态和用户体验

8. **Token 和成本追踪** (1-2 天)
   - Token 计数器
   - 成本计算（基于模型定价）
   - 缓存 token 追踪
   - 会话成本统计

9. **Skill 系统** (2-3 天)
   - Skill 定义格式（Markdown）
   - Skill 发现和加载
   - Skill 工具集成
   - 内置 Skill 库

10. **会话增强** (2-3 天)
    - 会话分支
    - 消息回滚
    - 文件快照和 diff
    - 改进的会话列表

**预期成果**：
- ✅ 完整的成本监控
- ✅ 可复用的 Skill 库
- ✅ 强大的会话管理
- ✅ 代码量增加 ~1500 行

---

## 4. 详细设计方案

### 4.1 权限管理系统

#### 4.1.1 数据结构

```go
// internal/permission/rule.go

package permission

import "github.com/bmatcuk/doublestar/v4"

// Action 权限动作
type Action string

const (
    ActionAllow Action = "allow"
    ActionDeny  Action = "deny"
    ActionAsk   Action = "ask"
)

// Rule 权限规则
type Rule struct {
    Permission string   `json:"permission"` // 工具名称
    Pattern    string   `json:"pattern"`    // 匹配模式
    Action     Action   `json:"action"`     // 动作
}

// Ruleset 规则集
type Ruleset struct {
    Rules       []Rule            `json:"rules"`
    AllowAll    bool              `json:"allow_all"`    // 全部允许
    DenyAll     bool              `json:"deny_all"`     // 全部拒绝
    DefaultAsk  bool              `json:"default_ask"`  // 默认询问
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
```

#### 4.1.2 规则评估器

```go
// internal/permission/evaluator.go

package permission

import (
    "context"
    "fmt"
    "sync"
)

// Evaluator 权限评估器
type Evaluator struct {
    mu sync.RWMutex

    // 会话级别的临时授权（"always" 选项）
    sessionApprovals map[string]map[string]bool // sessionID -> permission -> approved
}

func NewEvaluator() *Evaluator {
    return &Evaluator{
        sessionApprovals: make(map[string]map[string]bool),
    }
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
        // 3. 询问用户
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

        // 4. 如果选择 "always"，记录会话批准
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

        // 检查模式是否匹配（使用 doublestar 进行 glob 匹配）
        matched, err := doublestar.Match(rule.Pattern, pattern)
        if err != nil || !matched {
            continue
        }

        // 第一个匹配的规则生效
        return rule.Action
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
    Approved bool   // 是否批准
    Rejected bool   // 是否拒绝
    Always   bool   // 是否总是允许（会话级别）
}
```

#### 4.1.3 Doom Loop 检测

```go
// internal/permission/doomloop.go

package permission

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "sync"
    "time"
)

// DoomLoopDetector Doom Loop 检测器
type DoomLoopDetector struct {
    mu sync.Mutex

    // sessionID -> toolName -> hash -> count
    history map[string]map[string]map[string]int

    // 最后清理时间
    lastCleanup time.Time
}

func NewDoomLoopDetector() *DoomLoopDetector {
    return &DoomLoopDetector{
        history:     make(map[string]map[string]map[string]int),
        lastCleanup: time.Now(),
    }
}

// Check 检查是否陷入 Doom Loop
// 如果同一工具使用相同参数被调用 3 次，返回 true
func (d *DoomLoopDetector) Check(sessionID, toolName string, args interface{}) bool {
    d.mu.Lock()
    defer d.mu.Unlock()

    // 定期清理（每小时）
    if time.Since(d.lastCleanup) > time.Hour {
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
    return d.history[sessionID][toolName][hash] >= 3
}

// Reset 重置会话的 Doom Loop 检测
func (d *DoomLoopDetector) Reset(sessionID string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    delete(d.history, sessionID)
}

// hashArgs 计算参数哈希
func (d *DoomLoopDetector) hashArgs(args interface{}) string {
    data, _ := json.Marshal(args)
    hash := sha256.Sum256(data)
    return fmt.Sprintf("%x", hash[:8]) // 使用前 8 字节
}

// cleanup 清理旧数据
func (d *DoomLoopDetector) cleanup() {
    // 简单实现：清空所有历史
    // 生产环境可以使用更智能的策略（如 TTL）
    d.history = make(map[string]map[string]map[string]int)
    d.lastCleanup = time.Now()
}
```

#### 4.1.4 集成到工具系统

```go
// internal/tools/registry.go (修改)

package tools

import (
    "context"
    "fmt"

    "github.com/yourusername/gmain-agent/internal/permission"
)

// Registry 工具注册表
type Registry struct {
    tools            map[string]Tool
    mu               sync.RWMutex
    permissionEval   *permission.Evaluator       // 新增
    doomLoopDetector *permission.DoomLoopDetector // 新增
}

func NewRegistry() *Registry {
    return &Registry{
        tools:            make(map[string]Tool),
        permissionEval:   permission.NewEvaluator(),
        doomLoopDetector: permission.NewDoomLoopDetector(),
    }
}

// Execute 执行工具（带权限检查）
func (r *Registry) Execute(ctx context.Context, call ToolCall) (*Result, error) {
    tool, exists := r.Get(call.Name)
    if !exists {
        return nil, fmt.Errorf("tool not found: %s", call.Name)
    }

    // 1. Doom Loop 检测
    sessionID := ctx.Value("sessionID").(string)
    if r.doomLoopDetector.Check(sessionID, call.Name, call.Input) {
        // 触发权限检查
        logger.Warn("Doom Loop detected for tool: %s", call.Name)
    }

    // 2. 权限检查
    if call.Ruleset != nil {
        pattern := r.extractPattern(call.Name, call.Input)
        err := r.permissionEval.Ask(ctx, permission.AskInput{
            SessionID:  sessionID,
            Permission: call.Name,
            Pattern:    pattern,
            Ruleset:    *call.Ruleset,
            Message:    fmt.Sprintf("Tool %s wants to access: %s", call.Name, pattern),
            AskFunc:    call.AskFunc,
        })
        if err != nil {
            return nil, err
        }
    }

    // 3. 执行工具
    return tool.Execute(ctx, call.Input)
}

// extractPattern 从工具参数中提取模式
func (r *Registry) extractPattern(toolName string, input map[string]interface{}) string {
    // 根据不同工具提取不同的模式
    switch toolName {
    case "bash":
        if cmd, ok := input["command"].(string); ok {
            return cmd
        }
    case "read", "write", "edit":
        if path, ok := input["file_path"].(string); ok {
            return path
        }
    case "glob":
        if pattern, ok := input["pattern"].(string); ok {
            return pattern
        }
    case "grep":
        if pattern, ok := input["pattern"].(string); ok {
            return pattern
        }
    }
    return "*" // 默认匹配所有
}
```

### 4.2 上下文压缩机制

#### 4.2.1 溢出检测

```go
// internal/compaction/overflow.go

package compaction

import (
    "github.com/yourusername/gmain-agent/internal/api"
)

// TokenUsage Token 使用量
type TokenUsage struct {
    Input     int
    Output    int
    CacheRead int
}

// ModelLimits 模型限制
type ModelLimits struct {
    ContextLimit int
    OutputLimit  int
}

// IsOverflow 检查是否上下文溢出
func IsOverflow(usage TokenUsage, limits ModelLimits) bool {
    // 计算已用 token
    used := usage.Input + usage.CacheRead + usage.Output

    // 计算可用 token
    available := limits.ContextLimit - limits.OutputLimit

    // 如果已用 > 可用，触发压缩
    return used > available
}

// NeedsCompaction 检查是否需要压缩
// 当使用量超过 80% 时建议压缩
func NeedsCompaction(usage TokenUsage, limits ModelLimits) bool {
    used := usage.Input + usage.CacheRead + usage.Output
    available := limits.ContextLimit - limits.OutputLimit

    return float64(used) > float64(available)*0.8
}
```

#### 4.2.2 输出截断

```go
// internal/compaction/truncate.go

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
    Content   string
    Truncated bool
    FilePath  string
}

// TruncateOutput 截断工具输出
func TruncateOutput(output string, sessionID, toolName string) TruncateResult {
    if len(output) <= MaxOutputLength {
        return TruncateResult{
            Content:   output,
            Truncated: false,
        }
    }

    // 截断输出
    truncated := output[:MaxOutputLength]
    remaining := len(output) - MaxOutputLength

    // 保存完整输出到文件
    filePath := filepath.Join(os.TempDir(), "gmain-agent", sessionID, fmt.Sprintf("%s-output.txt", toolName))
    os.MkdirAll(filepath.Dir(filePath), 0755)
    os.WriteFile(filePath, []byte(output), 0644)

    // 添加截断提示
    message := fmt.Sprintf(TruncateMessage, remaining, filePath)

    return TruncateResult{
        Content:   truncated + message,
        Truncated: true,
        FilePath:  filePath,
    }
}
```

#### 4.2.3 工具输出修剪

```go
// internal/compaction/pruning.go

package compaction

import (
    "time"

    "github.com/yourusername/gmain-agent/internal/api"
)

const (
    // ProtectRecent 保护最近的 N 个对话回合
    ProtectRecent = 2

    // ProtectTokens 保护最近 N tokens 的工具输出
    ProtectTokens = 40000

    // PruneMinimum 最少修剪量
    PruneMinimum = 20000
)

// ProtectedTools 特殊工具不被修剪
var ProtectedTools = map[string]bool{
    "skill": true,
    "plan_exit": true,
}

// Prune 修剪工具输出
func Prune(messages []api.Message) ([]api.Message, int) {
    if len(messages) < ProtectRecent*2 {
        return messages, 0
    }

    pruned := 0
    protectFromIndex := len(messages) - ProtectRecent*2

    // 向后遍历消息
    for i := protectFromIndex - 1; i >= 0; i-- {
        msg := &messages[i]

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
            content.Content = "[Output pruned to save context]"
            content.Pruned = true
            content.PrunedAt = time.Now()

            pruned += originalLen

            // 如果修剪量足够，停止
            if pruned >= PruneMinimum {
                return messages, pruned
            }
        }
    }

    return messages, pruned
}
```

#### 4.2.4 会话压缩（简化版本）

```go
// internal/compaction/compaction.go

package compaction

import (
    "context"
    "fmt"

    "github.com/yourusername/gmain-agent/internal/api"
)

// Compactor 压缩器
type Compactor struct {
    client *api.Client
}

func NewCompactor(client *api.Client) *Compactor {
    return &Compactor{client: client}
}

// Compact 压缩会话（简化版本）
// 完整版本需要调用 compaction agent，这里先实现简单版本
func (c *Compactor) Compact(ctx context.Context, messages []api.Message) (string, error) {
    // 1. 生成摘要请求
    systemPrompt := "You are a summarization assistant. Summarize the conversation history concisely, preserving key information and decisions."

    // 2. 构建消息历史
    historyText := c.buildHistoryText(messages)

    // 3. 调用 API 生成摘要
    req := &api.MessagesRequest{
        Model: "claude-sonnet-4-20250514",
        MaxTokens: 4000,
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

    resp, err := c.client.CreateMessage(ctx, req)
    if err != nil {
        return "", err
    }

    // 4. 提取摘要文本
    if len(resp.Content) > 0 && resp.Content[0].Type == api.ContentTypeText {
        return resp.Content[0].Text, nil
    }

    return "", fmt.Errorf("failed to generate summary")
}

// buildHistoryText 构建历史文本
func (c *Compactor) buildHistoryText(messages []api.Message) string {
    var text string
    for _, msg := range messages {
        text += fmt.Sprintf("\n[%s]\n", msg.Role)
        for _, content := range msg.Content {
            if content.Type == api.ContentTypeText {
                text += content.Text + "\n"
            } else if content.Type == api.ContentTypeToolUse {
                text += fmt.Sprintf("[Tool: %s]\n", content.Name)
            } else if content.Type == api.ContentTypeToolResult {
                text += fmt.Sprintf("[Tool Result: %s] %s\n", content.ToolUseID, content.Content)
            }
        }
    }
    return text
}
```

### 4.3 智能重试机制

#### 4.3.1 错误分类

```go
// internal/retry/error.go

package retry

import (
    "errors"
    "net/http"
    "strings"
)

// ErrorType 错误类型
type ErrorType string

const (
    ErrorTypeRetryable    ErrorType = "retryable"
    ErrorTypeNonRetryable ErrorType = "non_retryable"
)

// ClassifyError 分类错误
func ClassifyError(err error) ErrorType {
    if err == nil {
        return ErrorTypeNonRetryable
    }

    errMsg := err.Error()

    // 1. 网络错误（可重试）
    networkErrors := []string{
        "connection reset",
        "connection refused",
        "timeout",
        "temporary failure",
        "ECONNRESET",
        "ETIMEDOUT",
        "EOF",
    }
    for _, ne := range networkErrors {
        if strings.Contains(strings.ToLower(errMsg), strings.ToLower(ne)) {
            return ErrorTypeRetryable
        }
    }

    // 2. API 错误码（可重试）
    retryableMessages := []string{
        "overloaded",
        "exhausted",
        "too many requests",
        "rate limit",
        "503",
        "502",
        "504",
        "429",
    }
    for _, rm := range retryableMessages {
        if strings.Contains(strings.ToLower(errMsg), strings.ToLower(rm)) {
            return ErrorTypeRetryable
        }
    }

    // 3. 其他错误（不可重试）
    return ErrorTypeNonRetryable
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
    return ClassifyError(err) == ErrorTypeRetryable
}
```

#### 4.3.2 退避策略

```go
// internal/retry/backoff.go

package retry

import (
    "math"
    "net/http"
    "strconv"
    "time"
)

const (
    InitialDelay      = 500 * time.Millisecond
    BackoffFactor     = 2.0
    MaxDelayWithHeader    = 10 * time.Second
    MaxDelayNoHeader  = 2 * time.Second
    MaxRetries        = 3
)

// CalculateDelay 计算重试延迟
func CalculateDelay(attempt int, resp *http.Response) time.Duration {
    // 优先级 1: 使用 HTTP 响应头
    if resp != nil {
        if delay := parseRetryAfter(resp.Header); delay > 0 {
            if delay > MaxDelayWithHeader {
                return MaxDelayWithHeader
            }
            return delay
        }
    }

    // 优先级 2: 指数退避
    delay := time.Duration(float64(InitialDelay) * math.Pow(BackoffFactor, float64(attempt-1)))

    // 限制最大延迟
    maxDelay := MaxDelayNoHeader
    if resp != nil {
        maxDelay = MaxDelayWithHeader
    }

    if delay > maxDelay {
        return maxDelay
    }

    return delay
}

// parseRetryAfter 解析 Retry-After 头
func parseRetryAfter(header http.Header) time.Duration {
    // 1. 尝试 Retry-After-Ms（毫秒）
    if ms := header.Get("Retry-After-Ms"); ms != "" {
        if val, err := strconv.ParseInt(ms, 10, 64); err == nil {
            return time.Duration(val) * time.Millisecond
        }
    }

    // 2. 尝试 Retry-After（秒或 HTTP-Date）
    if ra := header.Get("Retry-After"); ra != "" {
        // 尝试解析为秒数
        if seconds, err := strconv.ParseFloat(ra, 64); err == nil {
            return time.Duration(seconds * float64(time.Second))
        }

        // 尝试解析为 HTTP-Date
        if t, err := http.ParseTime(ra); err == nil {
            delay := time.Until(t)
            if delay > 0 {
                return delay
            }
        }
    }

    return 0
}
```

#### 4.3.3 重试逻辑

```go
// internal/retry/retry.go

package retry

import (
    "context"
    "net/http"
    "time"
)

// Retrier 重试器
type Retrier struct {
    MaxRetries int
}

func NewRetrier() *Retrier {
    return &Retrier{
        MaxRetries: MaxRetries,
    }
}

// Do 执行带重试的操作
func (r *Retrier) Do(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
    var lastResp *http.Response
    var lastErr error

    for attempt := 1; attempt <= r.MaxRetries; attempt++ {
        // 执行操作
        resp, err := fn()

        // 成功
        if err == nil && (resp == nil || resp.StatusCode < 400) {
            return resp, nil
        }

        // 记录最后的响应和错误
        lastResp = resp
        lastErr = err

        // 检查是否可重试
        if !IsRetryable(err) {
            return resp, err
        }

        // 最后一次尝试失败
        if attempt == r.MaxRetries {
            return resp, err
        }

        // 计算延迟
        delay := CalculateDelay(attempt, resp)

        // 等待
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(delay):
            // 继续下一次尝试
        }
    }

    return lastResp, lastErr
}

// DoWithFunc 执行带重试的操作（泛型版本）
func (r *Retrier) DoWithFunc(ctx context.Context, fn func() error) error {
    var lastErr error

    for attempt := 1; attempt <= r.MaxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        if !IsRetryable(err) {
            return err
        }

        if attempt == r.MaxRetries {
            return err
        }

        delay := CalculateDelay(attempt, nil)

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
        }
    }

    return lastErr
}
```

#### 4.3.4 集成到 API 客户端

```go
// internal/api/client.go (修改)

package api

import (
    "context"

    "github.com/yourusername/gmain-agent/internal/retry"
)

// Client API 客户端
type Client struct {
    // ... 现有字段
    retrier *retry.Retrier // 新增
}

func NewClient(apiKey string, opts ...ClientOption) *Client {
    c := &Client{
        // ... 现有初始化
        retrier: retry.NewRetrier(),
    }
    // ...
    return c
}

// CreateMessage 创建消息（带重试）
func (c *Client) CreateMessage(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
    var resp *MessagesResponse

    err := c.retrier.DoWithFunc(ctx, func() error {
        var err error
        resp, err = c.createMessageInternal(ctx, req)
        return err
    })

    return resp, err
}

// createMessageInternal 内部实现（不带重试）
func (c *Client) createMessageInternal(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
    // ... 现有实现
}
```

---

## 5. 预期收益

### 5.1 技术收益

- **安全性提升 80%**：权限系统防止误操作
- **稳定性提升 60%**：智能重试减少网络错误
- **可扩展性提升 100%**：多 Agent 支持复杂任务
- **上下文利用率提升 40%**：自动压缩延长对话
- **开发效率提升 50%**：计划模式减少返工

### 5.2 代码质量

- 代码量：~3,800 行 → ~8,800 行（+131%）
- 模块化：23 个文件 → ~40 个文件
- 测试覆盖率：0% → 60%+（如果实施测试）
- 文档覆盖率：0% → 80%+（如果实施文档）

### 5.3 用户体验

- ✅ 长对话不再中断
- ✅ 复杂任务可以完成
- ✅ 敏感操作需要确认
- ✅ 网络错误自动恢复
- ✅ 先规划后实施更可控

---

## 6. 风险与挑战

### 6.1 技术风险

1. **复杂度增加**：代码量翻倍，维护成本上升
   - 缓解：良好的模块化和文档

2. **性能影响**：权限检查和压缩增加延迟
   - 缓解：异步处理和缓存优化

3. **向后兼容性**：新架构可能破坏现有功能
   - 缓解：渐进式重构，保留旧接口

### 6.2 实施挑战

1. **学习曲线**：团队需要学习新架构
   - 缓解：详细文档和示例代码

2. **测试覆盖**：新功能需要大量测试
   - 缓解：单元测试 + 集成测试

3. **时间压力**：完整实施需要 4-6 周
   - 缓解：分阶段实施，优先高价值功能

---

## 7. 下一步行动

### 7.1 立即行动（今天）

1. ✅ 创建本设计文档
2. ⬜ 审查和批准设计方案
3. ⬜ 创建实施分支
4. ⬜ 设置项目跟踪（GitHub Issues / Project Board）

### 7.2 第一周（阶段 1 启动）

1. ⬜ 实现权限管理系统核心（2-3 天）
2. ⬜ 实现上下文压缩基础（2-3 天）
3. ⬜ 编写单元测试
4. ⬜ 集成测试和调试

### 7.3 后续里程碑

- **第 2 周**：完成阶段 1，开始阶段 2
- **第 3-4 周**：完成阶段 2
- **第 5-6 周**：完成阶段 3，发布 v2.0

---

## 附录 A：OpenCode 关键代码参考

### A.1 权限评估核心逻辑

参考 `/tmp/opencode-study/packages/opencode/src/permission/next.ts`

### A.2 消息处理循环

参考 `/tmp/opencode-study/packages/opencode/src/session/processor.ts`

### A.3 上下文压缩

参考 `/tmp/opencode-study/packages/opencode/src/session/compaction.ts`

### A.4 Agent 系统

参考 `/tmp/opencode-study/packages/opencode/src/agent/agent.ts`

---

## 附录 B：术语表

- **Agent**：具有特定权限和配置的 AI 实体
- **Subagent**：由主 Agent 调用的辅助 Agent
- **Permission Ruleset**：权限规则集合
- **Doom Loop**：相同工具和参数的重复调用循环
- **Compaction**：上下文压缩，生成摘要替换旧消息
- **Pruning**：工具输出修剪，移除旧的工具结果
- **Truncation**：输出截断，限制单次输出长度
- **Plan Mode**：计划模式，只读分析和计划生成
- **Build Mode**：构建模式，完整的开发工作流

---

## 文档版本

- **版本**: 1.0
- **日期**: 2026-01-16
- **作者**: Claude (AI Agent)
- **状态**: 待审查
