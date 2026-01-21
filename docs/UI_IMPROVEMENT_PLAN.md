# UI 交互改进计划

## 概述

本文档详细规划 gmain-agent 的 UI 交互改进，参照 Claude Code 的设计理念，打造专业的终端交互体验。

## 当前状态分析

### 现有 UI 组件

| 组件 | 文件 | 功能 | 状态 |
|------|------|------|------|
| Terminal | terminal.go | 基础 I/O | ⚠️ 基础 |
| Spinner | spinner.go | 加载动画 | ✅ 可用 |
| Markdown | markdown.go | MD 渲染 | ✅ 可用 |

### 现有问题

1. **输入体验差**：单行输入，无历史记录，无自动补全
2. **状态不可见**：没有状态栏显示当前 Agent、Token 使用等
3. **工具显示简陋**：工具调用没有折叠/展开，输出截断粗暴
4. **无权限确认 UI**：权限确认是简单的文本提示
5. **无快捷键**：缺少常用快捷键支持
6. **布局不响应**：没有适应终端大小变化

## 改进目标

参照 Claude Code，实现以下核心 UI 特性：

### Phase 1: 基础框架升级（核心）

1. **采用 BubbleTea 框架**
2. **实现完整的 TUI 布局**
3. **添加状态栏**
4. **改进输入区域**

### Phase 2: 交互增强

5. **改进工具调用显示**
6. **添加权限确认对话框**
7. **键盘快捷键支持**

### Phase 3: 体验优化

8. **主题系统**
9. **响应式布局**
10. **高级输入功能**

---

## Phase 1: 基础框架升级

### 1.1 采用 BubbleTea 框架

**BubbleTea** 是 Go 中最流行的 TUI 框架，Claude Code 也使用了类似的架构。

**新增依赖**：
```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textarea"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/lipgloss"
)
```

**核心架构**：
```
┌─────────────────────────────────────────────────────────────┐
│                         Header                               │
│  gmain-agent v0.3.0  │  Agent: build  │  Model: claude-4    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│                      Message Area                            │
│                      (Viewport)                              │
│                                                              │
│  User: 帮我创建一个 HTTP 服务器                               │
│                                                              │
│  Claude: 好的，我来帮你创建...                                │
│                                                              │
│  ▶ bash [running]                                            │
│    go build -o server ./cmd/server                          │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│                       Input Area                             │
│  > █                                                         │
├─────────────────────────────────────────────────────────────┤
│ Tokens: 2.3k/200k │ ↑↓ History │ Ctrl+C Cancel │ ? Help     │
└─────────────────────────────────────────────────────────────┘
```

**Model 结构**：
```go
// internal/ui/app.go

type AppModel struct {
    // 子组件
    viewport    viewport.Model   // 消息区域
    textarea    textarea.Model   // 输入区域
    spinner     spinner.Model    // 加载动画

    // 状态
    messages    []Message        // 消息历史
    agent       string           // 当前 Agent
    model       string           // 当前模型
    tokens      TokenStats       // Token 统计

    // 工具状态
    activeTool  *ToolExecution   // 当前执行的工具
    toolHistory []ToolExecution  // 工具执行历史

    // UI 状态
    width       int              // 终端宽度
    height      int              // 终端高度
    ready       bool             // 初始化完成
    loading     bool             // 正在加载
    showHelp    bool             // 显示帮助

    // 输入历史
    inputHistory []string
    historyIdx   int

    // 事件通道
    agentEvents chan AgentEvent
}
```

### 1.2 布局系统

**文件**: `internal/ui/layout.go`

```go
// 布局常量
const (
    HeaderHeight    = 1
    StatusBarHeight = 1
    InputAreaHeight = 3
    MinWidth        = 60
    MinHeight       = 20
)

// 计算各区域高度
func (m *AppModel) calculateLayout() {
    m.viewportHeight = m.height - HeaderHeight - StatusBarHeight - InputAreaHeight
    m.viewport.Height = m.viewportHeight
    m.viewport.Width = m.width
    m.textarea.SetWidth(m.width - 4)
}
```

**区域划分**：
```
Total Height
    │
    ├── Header (1 行)
    │     └── 项目名 | Agent | Model | 时间
    │
    ├── Message Area (动态高度)
    │     └── Viewport 组件（可滚动）
    │           ├── 用户消息
    │           ├── AI 回复
    │           └── 工具执行块
    │
    ├── Input Area (3 行)
    │     └── TextArea 组件（多行输入）
    │
    └── Status Bar (1 行)
          └── Token | 快捷键提示 | 状态
```

### 1.3 状态栏设计

**文件**: `internal/ui/statusbar.go`

```go
type StatusBar struct {
    // Token 信息
    InputTokens  int
    OutputTokens int
    CacheTokens  int
    TotalTokens  int
    MaxTokens    int

    // 状态信息
    Agent       string
    Mode        string  // normal, plan, loading

    // 提示
    Hints       []string
}

func (s *StatusBar) View() string {
    // 左侧：Token 信息
    tokenInfo := fmt.Sprintf("Tokens: %s/%s",
        formatTokens(s.TotalTokens),
        formatTokens(s.MaxTokens))

    // 中间：快捷键提示
    hints := "↑↓ History │ Ctrl+C Cancel │ ? Help"

    // 右侧：Agent 状态
    agentBadge := s.renderAgentBadge()

    return lipgloss.JoinHorizontal(
        lipgloss.Left,
        tokenInfo,
        lipgloss.PlaceHorizontal(width, lipgloss.Center, hints),
        agentBadge,
    )
}

func (s *StatusBar) renderAgentBadge() string {
    var color lipgloss.Color
    switch s.Agent {
    case "build":
        color = lipgloss.Color("#3B82F6") // 蓝色
    case "plan":
        color = lipgloss.Color("#8B5CF6") // 紫色
    case "explore":
        color = lipgloss.Color("#10B981") // 绿色
    }

    return lipgloss.NewStyle().
        Background(color).
        Foreground(lipgloss.Color("#FFFFFF")).
        Padding(0, 1).
        Render(s.Agent)
}
```

**状态栏样式**：
```
┌──────────────────────────────────────────────────────────────┐
│ Tokens: 2.3k/200k (+1.2k cache) │ ↑↓ History │ Ctrl+C │ [build] │
└──────────────────────────────────────────────────────────────┘
```

### 1.4 输入区域增强

**文件**: `internal/ui/input.go`

**功能特性**：

1. **多行输入**
```go
textarea := textarea.New()
textarea.Placeholder = "Send a message..."
textarea.CharLimit = 10000
textarea.SetWidth(width)
textarea.SetHeight(3)
textarea.ShowLineNumbers = false
```

2. **输入历史**
```go
type InputManager struct {
    history    []string
    historyIdx int
    current    string  // 临时保存当前输入
}

func (m *InputManager) PrevHistory() string {
    if m.historyIdx > 0 {
        m.historyIdx--
        return m.history[m.historyIdx]
    }
    return ""
}

func (m *InputManager) NextHistory() string {
    if m.historyIdx < len(m.history)-1 {
        m.historyIdx++
        return m.history[m.historyIdx]
    }
    m.historyIdx = len(m.history)
    return m.current
}
```

3. **快捷键**
```go
func (m *AppModel) handleInputKey(msg tea.KeyMsg) tea.Cmd {
    switch msg.String() {
    case "enter":
        if msg.Alt {
            // Alt+Enter: 换行
            return m.textarea.InsertNewline()
        }
        // Enter: 发送
        return m.sendMessage()

    case "up":
        if m.textarea.Line() == 0 {
            // 在第一行按上箭头：历史记录
            return m.loadPrevHistory()
        }

    case "down":
        if m.textarea.Line() == m.textarea.LineCount()-1 {
            // 在最后一行按下箭头：历史记录
            return m.loadNextHistory()
        }

    case "ctrl+c":
        return m.cancelOperation()

    case "ctrl+l":
        return m.clearScreen()

    case "tab":
        return m.triggerCompletion()
    }

    return nil
}
```

**输入区域样式**：
```
┌──────────────────────────────────────────────────────────────┐
│ > 帮我创建一个 HTTP 服务器，要求：                            │
│   1. 监听 8080 端口                                          │
│   2. 提供 /health 健康检查接口█                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Phase 2: 交互增强

### 2.1 工具调用显示

**文件**: `internal/ui/toolblock.go`

**设计**：可折叠的工具执行块

```go
type ToolBlock struct {
    ID         string
    Name       string
    Input      string       // JSON 格式的输入
    Output     string
    Status     ToolStatus   // pending, running, success, error
    StartTime  time.Time
    EndTime    time.Time
    Expanded   bool         // 是否展开
}

type ToolStatus int

const (
    ToolStatusPending ToolStatus = iota
    ToolStatusRunning
    ToolStatusSuccess
    ToolStatusError
)

func (t *ToolBlock) View(width int) string {
    // 头部：工具名称 + 状态
    header := t.renderHeader()

    if !t.Expanded {
        return header
    }

    // 展开时显示详情
    var content strings.Builder
    content.WriteString(header)
    content.WriteString("\n")

    // 输入参数（语法高亮）
    if t.Input != "" {
        content.WriteString(t.renderInput())
        content.WriteString("\n")
    }

    // 输出结果
    if t.Output != "" {
        content.WriteString(t.renderOutput(width))
    }

    return boxStyle.Render(content.String())
}

func (t *ToolBlock) renderHeader() string {
    // 状态图标
    var icon string
    var iconColor lipgloss.Color

    switch t.Status {
    case ToolStatusPending:
        icon = "○"
        iconColor = lipgloss.Color("#6B7280")
    case ToolStatusRunning:
        icon = "◐" // 会有动画
        iconColor = lipgloss.Color("#3B82F6")
    case ToolStatusSuccess:
        icon = "✓"
        iconColor = lipgloss.Color("#10B981")
    case ToolStatusError:
        icon = "✗"
        iconColor = lipgloss.Color("#EF4444")
    }

    iconStyled := lipgloss.NewStyle().
        Foreground(iconColor).
        Render(icon)

    // 工具名称
    nameStyled := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#E5E7EB")).
        Render(t.Name)

    // 折叠指示器
    expandIcon := "▶"
    if t.Expanded {
        expandIcon = "▼"
    }

    // 执行时间
    var duration string
    if t.Status == ToolStatusRunning {
        duration = fmt.Sprintf("%.1fs", time.Since(t.StartTime).Seconds())
    } else if !t.EndTime.IsZero() {
        duration = fmt.Sprintf("%.1fs", t.EndTime.Sub(t.StartTime).Seconds())
    }

    return fmt.Sprintf("%s %s %s %s", expandIcon, iconStyled, nameStyled, duration)
}
```

**显示效果**：

```
# 折叠状态
▶ ✓ bash (0.3s)

# 展开状态
▼ ✓ bash (0.3s)
  ┌─ Input ─────────────────────────────────────────┐
  │ command: go build -o server ./cmd/server        │
  └─────────────────────────────────────────────────┘
  ┌─ Output ────────────────────────────────────────┐
  │ Build successful                                │
  └─────────────────────────────────────────────────┘

# 运行中状态
▼ ◐ bash (2.3s...)
  ┌─ Input ─────────────────────────────────────────┐
  │ command: go test ./...                          │
  └─────────────────────────────────────────────────┘
  ┌─ Output (streaming) ────────────────────────────┐
  │ === RUN   TestUserService                       │
  │ --- PASS: TestUserService (0.01s)               │
  │ █                                               │
  └─────────────────────────────────────────────────┘

# 错误状态
▼ ✗ bash (0.1s)
  ┌─ Input ─────────────────────────────────────────┐
  │ command: rm -rf /                               │
  └─────────────────────────────────────────────────┘
  ┌─ Error ─────────────────────────────────────────┐
  │ Permission denied: dangerous operation blocked  │
  └─────────────────────────────────────────────────┘
```

### 2.2 权限确认对话框

**文件**: `internal/ui/confirm.go`

```go
type ConfirmDialog struct {
    Title       string
    Message     string
    Details     string      // 详细信息（如命令内容）
    Options     []string    // ["Allow", "Deny", "Allow Always"]
    Selected    int
    Visible     bool
}

func (d *ConfirmDialog) View(width int) string {
    if !d.Visible {
        return ""
    }

    // 标题
    title := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FBBF24")).
        Render("⚠ " + d.Title)

    // 消息
    message := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#E5E7EB")).
        Render(d.Message)

    // 详情（代码块样式）
    details := lipgloss.NewStyle().
        Background(lipgloss.Color("#1F2937")).
        Foreground(lipgloss.Color("#9CA3AF")).
        Padding(0, 1).
        Render(d.Details)

    // 选项按钮
    var buttons []string
    for i, opt := range d.Options {
        style := lipgloss.NewStyle().
            Padding(0, 2).
            MarginRight(1)

        if i == d.Selected {
            style = style.
                Background(lipgloss.Color("#3B82F6")).
                Foreground(lipgloss.Color("#FFFFFF")).
                Bold(true)
        } else {
            style = style.
                Background(lipgloss.Color("#374151")).
                Foreground(lipgloss.Color("#9CA3AF"))
        }

        buttons = append(buttons, style.Render(opt))
    }

    buttonRow := lipgloss.JoinHorizontal(lipgloss.Left, buttons...)

    // 组合
    content := lipgloss.JoinVertical(
        lipgloss.Left,
        title,
        "",
        message,
        "",
        details,
        "",
        buttonRow,
        "",
        dimStyle.Render("← → Select │ Enter Confirm │ Esc Cancel"),
    )

    // 对话框边框
    dialog := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#FBBF24")).
        Padding(1, 2).
        Width(min(width-4, 60)).
        Render(content)

    // 居中显示
    return lipgloss.Place(width, 0, lipgloss.Center, lipgloss.Top, dialog)
}

func (d *ConfirmDialog) HandleKey(msg tea.KeyMsg) (bool, string) {
    switch msg.String() {
    case "left", "h":
        d.Selected = max(0, d.Selected-1)
    case "right", "l":
        d.Selected = min(len(d.Options)-1, d.Selected+1)
    case "enter":
        return true, d.Options[d.Selected]
    case "esc":
        return true, "Cancel"
    case "y":
        return true, "Allow"
    case "n":
        return true, "Deny"
    case "a":
        return true, "Allow Always"
    }
    return false, ""
}
```

**显示效果**：
```
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│  ⚠ Permission Required                                       │
│                                                              │
│  The agent wants to execute a shell command:                 │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ rm -rf ./build/*                                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  [Allow]  [Deny]  [Allow Always]                            │
│                                                              │
│  ← → Select │ Enter Confirm │ Esc Cancel                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 快捷键系统

**文件**: `internal/ui/keybindings.go`

```go
type KeyBinding struct {
    Key         string
    Description string
    Action      func() tea.Cmd
}

var GlobalKeyBindings = []KeyBinding{
    {"ctrl+c", "Cancel current operation", cancelOperation},
    {"ctrl+l", "Clear screen", clearScreen},
    {"ctrl+d", "Exit", exitProgram},
    {"?", "Show/hide help", toggleHelp},
    {"esc", "Close dialog / Cancel", handleEscape},
}

var InputKeyBindings = []KeyBinding{
    {"enter", "Send message", sendMessage},
    {"alt+enter", "New line", insertNewline},
    {"up", "Previous history / Move up", handleUp},
    {"down", "Next history / Move down", handleDown},
    {"tab", "Autocomplete", triggerComplete},
    {"ctrl+u", "Clear input", clearInput},
    {"ctrl+w", "Delete word", deleteWord},
}

var ViewportKeyBindings = []KeyBinding{
    {"up/k", "Scroll up", scrollUp},
    {"down/j", "Scroll down", scrollDown},
    {"pgup", "Page up", pageUp},
    {"pgdown", "Page down", pageDown},
    {"home/g", "Go to top", goToTop},
    {"end/G", "Go to bottom", goToBottom},
}

var ToolBlockKeyBindings = []KeyBinding{
    {"enter", "Toggle expand/collapse", toggleExpand},
    {"c", "Copy output", copyOutput},
    {"o", "Open in editor", openInEditor},
}
```

**帮助面板**：
```go
func (m *AppModel) renderHelpPanel() string {
    // 分组显示快捷键
    sections := []struct {
        Title    string
        Bindings []KeyBinding
    }{
        {"Global", GlobalKeyBindings},
        {"Input", InputKeyBindings},
        {"Navigation", ViewportKeyBindings},
        {"Tool Blocks", ToolBlockKeyBindings},
    }

    var content strings.Builder
    for _, section := range sections {
        content.WriteString(headerStyle.Render(section.Title))
        content.WriteString("\n")
        for _, kb := range section.Bindings {
            key := lipgloss.NewStyle().
                Width(12).
                Foreground(lipgloss.Color("#60A5FA")).
                Render(kb.Key)
            desc := lipgloss.NewStyle().
                Foreground(lipgloss.Color("#9CA3AF")).
                Render(kb.Description)
            content.WriteString(fmt.Sprintf("  %s %s\n", key, desc))
        }
        content.WriteString("\n")
    }

    return boxStyle.Render(content.String())
}
```

**帮助面板效果**：
```
┌─ Keyboard Shortcuts ────────────────────────────────────────┐
│                                                              │
│ Global                                                       │
│   Ctrl+C      Cancel current operation                       │
│   Ctrl+L      Clear screen                                   │
│   Ctrl+D      Exit                                           │
│   ?           Show/hide help                                 │
│                                                              │
│ Input                                                        │
│   Enter       Send message                                   │
│   Alt+Enter   New line                                       │
│   ↑/↓         History navigation                             │
│   Tab         Autocomplete                                   │
│                                                              │
│ Navigation                                                   │
│   ↑/↓ or j/k  Scroll up/down                                │
│   PgUp/PgDn   Page up/down                                   │
│   Home/End    Go to top/bottom                               │
│                                                              │
│                              Press ? to close                │
└──────────────────────────────────────────────────────────────┘
```

---

## Phase 3: 体验优化

### 3.1 主题系统

**文件**: `internal/ui/theme.go`

```go
type Theme struct {
    Name        string

    // 背景色
    Background  lipgloss.Color
    Foreground  lipgloss.Color

    // 强调色
    Primary     lipgloss.Color
    Secondary   lipgloss.Color
    Accent      lipgloss.Color

    // 状态色
    Success     lipgloss.Color
    Warning     lipgloss.Color
    Error       lipgloss.Color
    Info        lipgloss.Color

    // Agent 颜色
    BuildAgent   lipgloss.Color
    PlanAgent    lipgloss.Color
    ExploreAgent lipgloss.Color

    // 边框
    Border      lipgloss.Color
    BorderDim   lipgloss.Color

    // 文本
    TextPrimary   lipgloss.Color
    TextSecondary lipgloss.Color
    TextDim       lipgloss.Color
}

var DarkTheme = Theme{
    Name:        "dark",
    Background:  lipgloss.Color("#0D1117"),
    Foreground:  lipgloss.Color("#C9D1D9"),
    Primary:     lipgloss.Color("#58A6FF"),
    Secondary:   lipgloss.Color("#8B949E"),
    Accent:      lipgloss.Color("#F78166"),
    Success:     lipgloss.Color("#3FB950"),
    Warning:     lipgloss.Color("#D29922"),
    Error:       lipgloss.Color("#F85149"),
    Info:        lipgloss.Color("#58A6FF"),
    BuildAgent:  lipgloss.Color("#58A6FF"),
    PlanAgent:   lipgloss.Color("#A371F7"),
    ExploreAgent: lipgloss.Color("#3FB950"),
    Border:      lipgloss.Color("#30363D"),
    BorderDim:   lipgloss.Color("#21262D"),
    TextPrimary:   lipgloss.Color("#C9D1D9"),
    TextSecondary: lipgloss.Color("#8B949E"),
    TextDim:       lipgloss.Color("#484F58"),
}

var LightTheme = Theme{
    Name:        "light",
    Background:  lipgloss.Color("#FFFFFF"),
    Foreground:  lipgloss.Color("#24292F"),
    Primary:     lipgloss.Color("#0969DA"),
    Secondary:   lipgloss.Color("#57606A"),
    // ... 其他颜色
}

var DraculaTheme = Theme{
    Name:        "dracula",
    Background:  lipgloss.Color("#282A36"),
    Foreground:  lipgloss.Color("#F8F8F2"),
    Primary:     lipgloss.Color("#BD93F9"),
    // ... 其他颜色
}

// 主题管理器
type ThemeManager struct {
    current *Theme
    themes  map[string]*Theme
}

func NewThemeManager() *ThemeManager {
    tm := &ThemeManager{
        themes: make(map[string]*Theme),
    }
    tm.Register(&DarkTheme)
    tm.Register(&LightTheme)
    tm.Register(&DraculaTheme)
    tm.SetTheme("dark")
    return tm
}

func (tm *ThemeManager) SetTheme(name string) {
    if theme, ok := tm.themes[name]; ok {
        tm.current = theme
    }
}
```

### 3.2 响应式布局

**文件**: `internal/ui/responsive.go`

```go
func (m *AppModel) handleResize(msg tea.WindowSizeMsg) tea.Cmd {
    m.width = msg.Width
    m.height = msg.Height

    // 重新计算布局
    m.calculateLayout()

    // 更新子组件
    m.viewport.Width = m.width
    m.viewport.Height = m.viewportHeight
    m.textarea.SetWidth(m.width - 4)

    // 重新渲染消息（处理换行）
    m.reformatMessages()

    return nil
}

func (m *AppModel) calculateLayout() {
    // 最小尺寸检查
    if m.width < MinWidth || m.height < MinHeight {
        m.showSizeWarning = true
        return
    }
    m.showSizeWarning = false

    // 计算各区域高度
    headerHeight := 1
    statusBarHeight := 1
    inputHeight := 3

    // 消息区域高度 = 总高度 - 固定区域
    m.viewportHeight = m.height - headerHeight - statusBarHeight - inputHeight

    // 如果高度太小，隐藏某些元素
    if m.height < 30 {
        // 简化状态栏
        m.compactStatusBar = true
    }

    // 如果宽度太小，调整布局
    if m.width < 80 {
        // 隐藏某些信息
        m.hideTokenDetails = true
    }
}
```

### 3.3 高级输入功能

**文件**: `internal/ui/autocomplete.go`

**命令自动补全**：
```go
type Autocomplete struct {
    suggestions []string
    selected    int
    visible     bool
    prefix      string
}

// 内置命令补全
var commands = []string{
    "/help",
    "/clear",
    "/exit",
    "/model",
    "/agent",
    "/history",
    "/export",
    "/compact",
    "/tokens",
}

func (a *Autocomplete) GetSuggestions(input string) []string {
    if !strings.HasPrefix(input, "/") {
        return nil
    }

    var matches []string
    for _, cmd := range commands {
        if strings.HasPrefix(cmd, input) {
            matches = append(matches, cmd)
        }
    }
    return matches
}

func (a *Autocomplete) View() string {
    if !a.visible || len(a.suggestions) == 0 {
        return ""
    }

    var items []string
    for i, s := range a.suggestions {
        style := lipgloss.NewStyle().Padding(0, 1)
        if i == a.selected {
            style = style.Background(lipgloss.Color("#3B82F6"))
        }
        items = append(items, style.Render(s))
    }

    return lipgloss.JoinVertical(lipgloss.Left, items...)
}
```

**显示效果**：
```
> /he█
  ┌──────────────────┐
  │ /help            │  ← 选中
  │ /history         │
  └──────────────────┘
```

---

## 实现计划

### 优先级排序

| 优先级 | 功能 | 文件 | 预计代码量 |
|--------|------|------|-----------|
| P0 | BubbleTea 框架迁移 | app.go, model.go | ~500 行 |
| P0 | 基础布局 | layout.go | ~200 行 |
| P0 | 状态栏 | statusbar.go | ~150 行 |
| P0 | 输入区域增强 | input.go | ~300 行 |
| P1 | 工具块显示 | toolblock.go | ~400 行 |
| P1 | 权限确认对话框 | confirm.go | ~200 行 |
| P1 | 快捷键系统 | keybindings.go | ~150 行 |
| P2 | 主题系统 | theme.go | ~200 行 |
| P2 | 响应式布局 | responsive.go | ~150 行 |
| P2 | 自动补全 | autocomplete.go | ~200 行 |
| **总计** | | | **~2450 行** |

### 开发阶段

**Phase 1（核心 - 3 天）**：
1. 安装 BubbleTea 相关依赖
2. 创建 AppModel 和基础架构
3. 实现布局系统
4. 实现状态栏
5. 迁移现有功能

**Phase 2（增强 - 2 天）**：
1. 实现工具块组件
2. 实现确认对话框
3. 添加快捷键支持
4. 整合到主程序

**Phase 3（优化 - 2 天）**：
1. 实现主题系统
2. 添加响应式支持
3. 实现自动补全
4. 测试和优化

---

## 新增文件结构

```
internal/ui/
├── app.go           # BubbleTea 主应用
├── model.go         # 数据模型定义
├── update.go        # 更新逻辑（处理消息）
├── view.go          # 视图渲染
├── layout.go        # 布局计算
├── statusbar.go     # 状态栏组件
├── input.go         # 输入区域组件
├── toolblock.go     # 工具执行块组件
├── confirm.go       # 确认对话框组件
├── keybindings.go   # 快捷键定义
├── theme.go         # 主题系统
├── responsive.go    # 响应式布局
├── autocomplete.go  # 自动补全
├── messages.go      # 消息渲染
├── help.go          # 帮助面板
├── spinner.go       # 加载动画（已有，可能需更新）
├── markdown.go      # Markdown 渲染（已有）
└── terminal.go      # 基础终端操作（已有，可能需更新）
```

---

## 效果预览

### 完整界面

```
┌─────────────────────────────────────────────────────────────────┐
│  gmain-agent v0.3.0  │  claude-sonnet-4  │  2024-01-16 14:32  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  You: 帮我创建一个 HTTP 服务器                                    │
│                                                                 │
│  Claude: 好的，我来帮你创建一个简单的 HTTP 服务器。               │
│                                                                 │
│  ▼ ✓ write (0.1s)                                               │
│    ┌─ Input ───────────────────────────────────────────┐        │
│    │ file_path: /Users/a1/project/main.go              │        │
│    └───────────────────────────────────────────────────┘        │
│    ┌─ Output ──────────────────────────────────────────┐        │
│    │ File created successfully                          │        │
│    └───────────────────────────────────────────────────┘        │
│                                                                 │
│  ▶ ◐ bash (2.3s...)                                             │
│                                                                 │
│  文件已创建。现在我来运行测试...                                   │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  > █                                                             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ Tokens: 2.3k (+1.2k) │ ↑↓ History │ ? Help │     [build]        │
└─────────────────────────────────────────────────────────────────┘
```

### 权限确认

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│           ┌───────────────────────────────────────────┐         │
│           │  ⚠ Permission Required                    │         │
│           │                                           │         │
│           │  The agent wants to execute:              │         │
│           │                                           │         │
│           │  ┌─────────────────────────────────────┐  │         │
│           │  │ rm -rf ./node_modules               │  │         │
│           │  └─────────────────────────────────────┘  │         │
│           │                                           │         │
│           │  [Allow]  [Deny]  [Always Allow]          │         │
│           │                                           │         │
│           │  y Allow │ n Deny │ a Always │ Esc Cancel │         │
│           └───────────────────────────────────────────┘         │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  > █                                                             │
├─────────────────────────────────────────────────────────────────┤
│ Tokens: 2.3k (+1.2k) │ ↑↓ History │ ? Help │     [build]        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 技术依赖

### 新增依赖

```go
// go.mod 新增
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/bubbles v0.17.1
    github.com/charmbracelet/lipgloss v0.9.1  // 已有
    github.com/charmbracelet/glamour v0.6.0   // 已有
    github.com/atotto/clipboard v0.1.4        // 剪贴板支持
    github.com/muesli/reflow v0.3.0           // 文本重排
)
```

### 安装命令

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/atotto/clipboard@latest
go get github.com/muesli/reflow@latest
```

---

## 总结

本计划将 gmain-agent 的 UI 从基础终端输出升级为专业的 TUI 应用，主要改进：

### 核心价值

1. **专业的交互体验**：媲美 Claude Code 的终端 UI
2. **更好的可视化**：工具执行状态一目了然
3. **安全的权限控制**：清晰的权限确认界面
4. **高效的输入方式**：历史记录、自动补全、快捷键

### 预期效果

- 用户体验提升 200%
- 操作效率提升 50%
- 更清晰的状态反馈
- 更安全的交互流程

### 风险控制

1. **渐进式迁移**：保留现有 API，逐步替换
2. **回退机制**：可通过 `--simple` 参数使用简单模式
3. **充分测试**：每个组件独立测试

---

**下一步**：确认计划后开始 Phase 1 实现。
