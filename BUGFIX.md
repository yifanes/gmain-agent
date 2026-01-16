# Bug 修复记录

## Bug #1: API 拒绝 pruned 字段

### 问题描述
**日期**: 2026-01-16
**严重程度**: 高（阻塞使用）

**错误信息**:
```
Error: stream error: {"type":"error","error":"messages.0.content.0.text.pruned_at: Extra inputs are not permitted"}
```

**重现步骤**:
1. 启动 gmain-agent
2. 发送任何消息
3. API 返回错误

### 根本原因

在 `internal/api/messages.go` 中，我们为 `Content` 结构体添加了用于上下文压缩的内部字段：
```go
type Content struct {
    // ... 原有字段

    // Compaction support
    Pruned   bool      `json:"pruned,omitempty"`
    PrunedAt time.Time `json:"pruned_at,omitempty"`
}
```

这些字段被序列化并发送到 API，但 API 不认识这些字段，导致请求被拒绝。

### 解决方案

将这些内部字段的 JSON 标签改为 `json:"-"`，使其在序列化时被忽略：

```go
type Content struct {
    // ... 原有字段

    // Compaction support (internal use only, not sent to API)
    Pruned   bool      `json:"-"` // 是否已被修剪
    PrunedAt time.Time `json:"-"` // 修剪时间
}
```

**修改文件**: `internal/api/messages.go:38-39`

### 验证

编译并测试：
```bash
go build -o ~/bin/gmain-agent ./cmd/claude
~/bin/gmain-agent
```

现在应该可以正常工作了。

### 预防措施

1. **设计原则**: 内部元数据字段应该始终使用 `json:"-"` 标签
2. **测试建议**: 添加 API 序列化测试，确保发送的 JSON 符合 API schema
3. **文档更新**: 在 `UPGRADE_GUIDE.md` 中添加此说明

### 影响范围

- **影响版本**: v2.0-phase1 初始版本
- **修复版本**: v2.0-phase1-fix1
- **影响功能**: 所有 API 调用
- **修复耗时**: 5 分钟

### 相关代码

**Before**:
```go
Pruned   bool      `json:"pruned,omitempty"`
PrunedAt time.Time `json:"pruned_at,omitempty"`
```

**After**:
```go
Pruned   bool      `json:"-"`
PrunedAt time.Time `json:"-"`
```

### 经验教训

1. **序列化审查**: 添加新字段时，必须考虑是否应该序列化
2. **API 兼容性**: 内部字段不应该暴露给外部 API
3. **测试覆盖**: 需要添加 API 请求/响应的序列化测试

---

## 修复清单

- [x] 修改 `Content` 结构体的 JSON 标签
- [x] 重新编译项目
- [x] 安装到 `~/bin/gmain-agent`
- [x] 创建 Bug 修复文档
- [ ] 添加序列化测试（可选）
- [ ] 更新 `UPGRADE_GUIDE.md`

---

**修复者**: Claude (AI Agent)
**修复时间**: 2026-01-16
**版本**: v2.0-phase1-fix1
