# 日志指南

> 本项目中日志的处理方式。

---

## 概述

**SDK 核心包不使用任何日志库。** 这是有意的设计决策。

作为 Go 库/SDK，日志由调用方负责。SDK 通过 error 返回值传递所有诊断信息，而不是 log 输出。

---

## SDK Core Packages (auth/, client/, config/, provider/, session/)

### 规则：不记录日志

这些包不得导入或使用任何日志库：
- 不使用 `log` 包
- 不使用 `slog` 包
- 不使用第三方 logger（zerolog, zap, logrus 等）

**原因**：库不应把日志选择强加给使用者。不同调用方可能使用不同的 log 框架、格式和输出目标。

### 诊断信息如何传递

| 机制 | 用途 |
|-----------|-------|
| Error 返回 | 所有失败都以带有描述性前缀信息的 `error` 值返回 |
| 哨兵错误 | 诸如 `ErrSessionNotFound` 之类的预期条件允许调用方分支逻辑 |
| 结构化响应 | `ChatResponse`, `StreamChunk` 携带全部数据，包括用于调试的 `Raw` 字节 |

---

## 示例代码 (examples/)

### 标准库 `log` 包

示例为简洁起见使用 Go 标准库 `log` 包：

```go
import "log"

// Fatal errors (startup failures)
log.Fatalf("Failed to load config: %v", err)

// Non-fatal errors (continue execution)
log.Printf("Error: %v", err)

// Informational output
log.Printf("plugin platform listening on http://localhost%s", server.Addr)
```

### 示例中的 log 级别约定

| 函数 | 用途 |
|----------|-------|
| `log.Fatalf()` | 不可恢复的启动错误（配置加载、连接失败） |
| `log.Printf()` | 运行消息和非致命错误 |
| `fmt.Printf()` | 面向用户的输出（聊天响应、测试结果） |

---

## 平台集成 (examples/plugin-platform/)

plugin-platform 示例服务器使用带前缀模式的 `log.Printf` 以便可追踪：

```go
log.Printf("[ws] received reply_chunk, msgID=%s", msg.ID)
log.Printf("[stream] starting SSE stream for configID=%s, pendingID=%s", req.ConfigID, pending.id)
```

**前缀约定**：使用 `[component]` 前缀便于过滤：
- `[ws]` — WebSocket messages
- `[stream]` — SSE streaming events

---

## 不要记录的内容

即使在示例代码中，也不要记录：
- API keys or access tokens
- 完整的凭证对象
- 主加密密钥
- 生产场景中的用户消息内容
- 完整的 HTTP 响应体（可能包含敏感数据）

---

## 新代码指南

### 如果添加到 SDK Core

不要添加日志。改为返回带描述的错误：

```go
// Bad
log.Printf("failed to resolve provider: %s", name)
return nil, err

// Good
return nil, fmt.Errorf("client: provider %s not configured", name)
```

### 如果添加示例/服务器代码

使用标准 `log` 包并遵循前缀约定：

```go
log.Printf("[component] action description, key=%s", value)
```

### 如果构建使用方应用

选择你偏好的日志库并封装 SDK 错误：

```go
resp, err := session.Chat(ctx, req)
if err != nil {
    slog.Error("chat failed", "provider", providerName, "error", err)
    return err
}
```
