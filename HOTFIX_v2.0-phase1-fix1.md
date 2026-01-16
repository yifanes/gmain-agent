# 🔥 Hotfix v2.0-phase1-fix1

## 紧急修复：API 序列化错误

### 问题
在 v2.0-phase1 中，新增的内部字段 `Pruned` 和 `PrunedAt` 被错误地序列化发送到 API，导致 API 拒绝请求：

```
Error: messages.0.content.0.text.pruned_at: Extra inputs are not permitted
```

### 修复
将内部字段的 JSON 标签从 `json:"pruned,omitempty"` 改为 `json:"-"`：

```go
// Before (错误)
Pruned   bool      `json:"pruned,omitempty"`
PrunedAt time.Time `json:"pruned_at,omitempty"`

// After (正确)
Pruned   bool      `json:"-"`
PrunedAt time.Time `json:"-"`
```

### 影响
- **影响版本**: v2.0-phase1
- **严重程度**: 高（阻塞所有 API 调用）
- **影响功能**: 所有消息发送
- **修复时间**: 5 分钟

### 如何更新

#### 方式 1: 重新编译（推荐）
```bash
cd /Users/a1/code/gorepo/gmain-agent
git pull  # 如果使用 git
go build -o ~/bin/gmain-agent ./cmd/claude
```

#### 方式 2: 手动修改
编辑 `internal/api/messages.go`，在第 38-39 行：
```go
Pruned   bool      `json:"-"`
PrunedAt time.Time `json:"-"`
```

然后重新编译：
```bash
go build -o ~/bin/gmain-agent ./cmd/claude
```

### 验证

运行测试脚本：
```bash
go run test_serialization.go
```

应该看到：
```
✅ Serialized JSON: {"type":"text","text":"Hello, world!"}
✅ SUCCESS: Internal fields are not serialized
```

或直接运行 agent：
```bash
~/bin/gmain-agent
> hello
```

应该正常工作，不再报错。

### 根本原因

在设计上下文压缩功能时，我们为 `Content` 结构添加了内部跟踪字段，但忘记使用 `json:"-"` 标签来防止序列化。

### 预防措施

1. **代码审查**: 添加新字段时检查 JSON 标签
2. **测试覆盖**: 添加 API 序列化测试（已添加 `test_serialization.go`）
3. **文档更新**: 在设计文档中说明内部字段规范

### 相关文件

- ✅ `internal/api/messages.go` - 修复序列化标签
- ✅ `test_serialization.go` - 添加验证测试
- ✅ `BUGFIX.md` - 详细 bug 记录
- ✅ `UPGRADE_GUIDE.md` - 更新故障排查
- ✅ 本文档

### 版本信息

- **修复版本**: v2.0-phase1-fix1
- **修复日期**: 2026-01-16
- **修复者**: Claude (AI Agent)
- **测试状态**: ✅ 通过

### 更新日志

```
v2.0-phase1-fix1 (2026-01-16)
---------------------------
🔥 Hotfix: 修复 API 序列化错误
- 将 Content.Pruned 和 Content.PrunedAt 的 JSON 标签改为 json:"-"
- 添加序列化验证测试
- 更新文档

v2.0-phase1 (2026-01-16)
------------------------
✨ 新功能: 权限管理、上下文压缩、智能重试
- 实现细粒度权限控制系统
- 实现自动上下文压缩
- 实现智能重试机制
⚠️  已知问题: API 序列化错误（已在 fix1 中修复）
```

---

## 立即行动

如果您正在使用 v2.0-phase1，请**立即更新**到 v2.0-phase1-fix1：

```bash
cd /Users/a1/code/gorepo/gmain-agent
go build -o ~/bin/gmain-agent ./cmd/claude
```

然后重启您的 agent。

---

**状态**: ✅ 已修复并验证
**优先级**: 🔴 紧急
**影响**: 🔴 高（阻塞使用）
