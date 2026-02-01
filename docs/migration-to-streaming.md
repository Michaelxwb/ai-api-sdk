# 迁移到流式优先架构

本文档说明如何从旧的非流式 API 迁移到新的“流式优先”架构，并给出典型代码示例与常见问题。

## 为什么选择流式优先

- **更低延迟**：首个 token 提前返回，提升交互体验
- **更好的可视化**：前端可直接呈现打字机效果
- **更少内存**：无需等全部返回后再处理
- **统一路径**：流式与非流式共用实现，减少重复逻辑

## API 对照表

| 旧 API | 新流式 API | 新非流式封装 |
|---|---|---|
| `ChatSession` | `ChatSessionStream` | `ChatSessionStreamSync` |
| `Chat` | `ChatStream` | `ChatStreamSync` |
| `ChatWith` | `ChatWithStream` | `ChatWithStreamSync` |

> 说明：`ChatSession` 已标记为 Deprecated，内部调用 `ChatSessionStreamSync`。

## 迁移示例

### 1. 多轮对话（旧 → 新）

**旧写法（非流式）：**

```go
resp, err := cli.ChatSession(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})
fmt.Println(resp.Text)
```

**新写法（流式）：**

```go
stream, err := cli.ChatSessionStream(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})
if err != nil {
    log.Fatal(err)
}
for chunk := range stream {
    if chunk.Error != nil {
        log.Fatal(chunk.Error)
    }
    fmt.Print(chunk.Text)
}
```

**新写法（非流式封装）：**

```go
resp, err := cli.ChatSessionStreamSync(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})
fmt.Println(resp.Text)
```

### 2. 单轮对话（旧 → 新）

**旧写法：**

```go
resp, _ := cli.Chat(ctx, "openai", req)
```

**新写法（流式）：**

```go
stream, _ := cli.ChatStream(ctx, "openai", req)
```

**新写法（非流式封装）：**

```go
resp, _ := cli.ChatStreamSync(ctx, "openai", req)
```

## 行为差异说明

- **Session 冲突**：流式过程中无法安全重试，因此 `ChatSessionStream` 遇到 `ErrSessionConflict` 会直接返回错误，不做重试。
- **保存时机**：流式结束后自动保存完整 assistant 回复到 `SessionStore`。
- **超时控制**：流式请求由 `context` 控制超时，不再依赖全局 HTTP Timeout。

## 常见问题

**Q1: 不想改太多代码怎么办？**

使用 `ChatSessionStreamSync` / `ChatStreamSync` / `ChatWithStreamSync`，可保持非流式使用方式。

**Q2: Provider 不支持流式怎么办？**

`ChatStream`/`ChatWithStream` 会返回错误。请使用非流式 API，或为该 provider 实现 `ProviderStreamSpec`。

**Q3: 如何处理流式错误？**

遍历 `StreamChunk` 时遇到 `chunk.Error != nil` 就停止处理并向上返回。

**Q4: 还能继续使用 `ChatSession` 吗？**

可以，但已标记为 Deprecated。建议尽快迁移到 `ChatSessionStreamSync`。

## 相关文档
- [文档索引](README.md)
- [使用指南](usage-guide.md)
- [流式解析架构](architecture.md#流式解析架构)
- [Session 教程](session-tutorial.md)
- [API 参考](api-reference.md)
