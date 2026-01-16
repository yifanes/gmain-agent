# gmain-agent 功能特性

## 核心功能

### 1. 多 Agent 系统

gmain-agent 现在支持多个专门化的 AI Agents，每个都有不同的权限和用途。

#### 可用 Agents

**build** (构建 Agent) - 默认
- 完整开发权限：读写代码、执行命令、修改文件
- 用于实际的代码开发和实施
- 颜色: 蓝色

**plan** (计划 Agent)
- 只读权限 + 计划文件编辑
- 用于代码分析和制定实施计划
- 在 `.gmain-agent/plans/` 目录创建计划文档
- 颜色: 紫色

**explore** (探索 Agent) - Subagent
- 纯只读权限
- 快速代码库探索和搜索
- 由其他 Agent 调用
- 颜色: 绿色

### 2. 计划模式

#### 使用场景
当你需要实施复杂功能时，建议先进入计划模式：

1. **进入计划模式**: AI 会自动使用 `plan_enter` 工具
2. **分析阶段**: Plan Agent 分析代码库，理解需求
3. **制定计划**: 创建详细的实施计划文档
4. **退出计划模式**: 使用 `plan_exit` 工具切换回构建模式
5. **实施阶段**: Build Agent 按计划实施功能

#### 计划文件位置
```
.gmain-agent/
└── plans/
    ├── plan-20260116-150405.md
    └── plan-20260116-160520.md
```

#### 示例对话

```
用户: "我想添加用户认证功能，但这个功能比较复杂，请先帮我制定计划"

AI: "我将进入计划模式来分析需求并制定实施计划。"
    [使用 plan_enter 工具]
    [切换到 plan Agent]

Plan Agent: "现在我处于计划模式，让我分析当前的代码库..."
    [使用 read/grep/glob 工具探索代码]
    [编写计划文档]

Plan Agent: "计划已完成，现在切换到实施模式。"
    [使用 plan_exit 工具]
    [切换回 build Agent]

Build Agent: "根据计划，我将开始实施用户认证功能..."
    [执行实施步骤]
```

### 3. 任务委托 (Subagent 调用)

Build Agent 可以调用 Explore Agent 来处理探索性任务。

#### 自动触发
当 AI 需要快速搜索代码库时，它会自动使用 `task` 工具：

```
用户: "找出所有处理用户登录的代码"

Build Agent: [使用 task 工具调用 explore subagent]

Explore Agent: [快速搜索并返回结果]

Build Agent: "我找到了以下登录相关代码：..."
```

#### 工作原理

```
Build Agent
    │
    ├─> 调用 Task 工具
    │
    └─> Explore Agent (独立会话)
            │
            ├─> 使用 grep/glob/read
            │
            └─> 返回结果
```

### 4. 权限管理

每个 Agent 都有明确的权限范围，系统会自动检查和执行权限规则。

#### 权限规则示例

**Build Agent 权限**:
```
✓ read *                    - 允许读取任何文件
✓ write *                   - 允许写入任何文件
✓ edit *                    - 允许编辑任何文件
✓ bash ls, cat, grep        - 允许安全命令
? bash rm, mv, git push     - 需要确认的危险命令
✗ bash rm -rf /             - 绝对禁止的命令
```

**Plan Agent 权限**:
```
✓ read *                    - 允许读取任何文件
✓ write .gmain-agent/plans/* - 只能写计划文件
✗ write *                   - 禁止写其他文件
✗ edit *                    - 禁止编辑代码
✗ bash *                    - 禁止执行命令
```

**Explore Agent 权限**:
```
✓ read *                    - 允许读取
✓ glob *                    - 允许搜索文件
✓ grep *                    - 允许搜索内容
✗ write *                   - 禁止写入
✗ edit *                    - 禁止编辑
✗ bash *                    - 禁止命令
```

#### 权限拒绝示例

```
Plan Agent: [尝试使用 edit 工具]
System: "Permission denied: agent 'plan' is not allowed to use tool 'edit'"
```

### 5. 上下文管理

#### 输出截断
- 工具输出超过 30KB 时自动截断
- 完整输出保存到 `/tmp/gmain-agent/[session]/outputs/`
- Agent 会收到截断提示和文件路径

#### 工具结果修剪（未来功能）
- 自动修剪旧的工具输出以节省上下文
- 保护最近 2 轮对话
- 特殊工具（plan_enter, plan_exit）不被修剪

#### 会话压缩（未来功能）
- Token 使用量超过 80% 时自动压缩
- 生成对话摘要
- 保持会话连贯性

## 命令行使用

### 基本命令

```bash
# 启动交互模式
gmain-agent

# 直接执行命令
gmain-agent "请帮我创建一个 HTTP 服务器"

# 指定模型
gmain-agent -m claude-sonnet-4-20250514 "查找所有 TODO 注释"

# 启用日志
gmain-agent --enable-logging

# 查看版本
gmain-agent --version
```

### 交互模式命令

```bash
/help       - 显示帮助
/clear      - 清空对话历史
/exit       - 退出程序
/quit       - 退出程序
/model      - 显示当前模型
```

## 配置

配置文件位置: `~/.gmain-agent/config.yaml`

```yaml
# API 配置
api_key: "your-anthropic-api-key"
base_url: "https://api.anthropic.com"  # 可选

# 模型配置
model: "claude-sonnet-4-20250514"
max_tokens: 8192

# 认证方式
auth_type: "x-api-key"  # 或 "bearer"
```

## 示例工作流

### 示例 1: 添加新功能

```
用户: "我想添加一个用户认证系统，包括注册、登录、登出功能"

AI: "这是一个复杂的功能，让我先制定一个详细计划。"
    [使用 plan_enter]

Plan Agent: [分析现有代码]
    ✓ 读取 auth 相关文件
    ✓ 理解当前架构
    ✓ 创建实施计划
    [使用 plan_exit]

Build Agent: "计划已完成，现在开始实施..."
    ✓ 创建 auth 模块
    ✓ 实现注册功能
    ✓ 实现登录功能
    ✓ 添加测试
    ✓ 更新文档
```

### 示例 2: 代码探索

```
用户: "这个项目中的 HTTP 路由是如何定义的？"

Build Agent: [使用 task 工具调用 explore]

Explore Agent: [快速搜索]
    ✓ 搜索 "router", "route", "http"
    ✓ 查找路由定义文件
    ✓ 分析路由模式

Build Agent: "我找到了路由定义，位于以下文件..."
```

### 示例 3: Bug 修复

```
用户: "用户登录时偶尔会超时，帮我找出原因"

Build Agent: [使用 task 工具搜索登录代码]

Explore Agent: [找到登录相关代码]

Build Agent: [读取登录代码]
    "我发现了问题：数据库连接没有设置超时..."

    [修复代码]
    ✓ 添加连接超时
    ✓ 添加重试逻辑
    ✓ 更新测试
```

## 高级功能（开发中）

### Token 和成本追踪
- 实时显示 Token 使用量
- 计算 API 调用成本
- Session 统计信息

### Skill 系统
- 定义可复用的技能
- 技能组合和调用
- 社区共享技能

### Session 管理
- Session 分支和回滚
- 快照保存和恢复
- Session 导出和导入

## 常见问题

### Q: 如何知道当前使用的是哪个 Agent？

A: AI 的回答会体现其当前的权限和角色。你也可以从颜色编码看出（如果 UI 支持）。

### Q: 为什么 AI 拒绝执行某些操作？

A: 这是权限系统在工作。Plan Agent 只能读代码和写计划，不能修改代码。如果需要修改代码，需要切换回 Build Agent。

### Q: 计划模式什么时候使用？

A: 对于复杂功能（需要修改多个文件，影响多个模块）建议使用计划模式。简单任务可以直接执行。

### Q: Explore Agent 有什么限制？

A: Explore Agent 只能读取和搜索代码，不能修改。它被限制在最多 10 步操作内，适合快速探索任务。

### Q: 如何查看完整的工具输出？

A: 如果输出被截断，AI 会告诉你完整输出的保存位置（通常在 `/tmp/gmain-agent/` 下）。

## 开发者信息

### 项目结构

```
gmain-agent/
├── cmd/claude/           # 主程序入口
├── internal/
│   ├── agent/           # Agent 核心逻辑
│   ├── agentregistry/   # Agent 注册表
│   ├── api/             # API 客户端
│   ├── permission/      # 权限系统
│   ├── compaction/      # 上下文压缩
│   ├── retry/           # 重试机制
│   ├── tools/           # 工具实现
│   ├── config/          # 配置管理
│   ├── logger/          # 日志系统
│   └── ui/              # 终端 UI
├── examples/            # 示例程序
└── docs/                # 文档
```

### 扩展开发

#### 添加新的 Agent

```go
// 在 internal/agentregistry/builtin.go 中添加

func MyCustomAgent() AgentInfo {
    return AgentInfo{
        Name:        "custom",
        Description: "My custom agent",
        Mode:        ModePrimary,
        Native:      true,
        Temperature: 0,
        Permission:  customPermissions(),
        SystemPrompt: "Your custom prompt...",
        Color:       "#ff5733",
    }
}

func customPermissions() permission.Ruleset {
    return permission.Ruleset{
        Rules: []permission.Rule{
            {Permission: "read", Pattern: "*", Action: permission.ActionAllow},
            // ... more rules
        },
        DefaultAsk: false,
    }
}
```

#### 添加新的工具

```go
// 在 internal/tools/ 中创建新文件

type MyTool struct {
    // fields
}

func NewMyTool() *MyTool {
    return &MyTool{}
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "What this tool does..."
}

func (t *MyTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            // parameter definitions
        },
    }
}

func (t *MyTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
    // implementation
}
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可

MIT License

## 参考

本项目基于 [opencode](https://github.com/anomalyco/opencode) 的核心设计理念开发。
