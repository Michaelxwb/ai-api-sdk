# 使用指南

本指南覆盖常见调用方式和最佳实践。

## 单轮对话（非流式）

### 本地模式（配置文件 + Manager）

```go
resp, err := cli.Chat(ctx, "openai", base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "hello"}},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.Text)
```

### 平台集成模式（直接传入凭证）

```go
resp, err := cli.ChatWith(ctx, cred, pc, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "hello"}},
})
```

## 连通性测试

在正式使用前，可以通过连通性测试快速验证配置是否正确。

### 测试方法

**方式 1：使用 YAML 配置**：
```go
result, err := cli.Test(ctx, "openai", &client.TestOptions{
    Model:     "gpt-4o-mini",
    Timeout:   10 * time.Second,
    MaxTokens: 1,
    Prompt:    "test",
})

if err != nil {
    log.Printf("测试失败: %v", err)
} else {
    log.Printf("测试成功，延迟: %v", result.Latency)
}
```

**方式 2：使用代码构建凭证**：
```go
result, err := cli.TestWith(ctx, cred, providerConfig, &client.TestOptions{
    Model:   "gpt-4o-mini",
    Timeout: 10 * time.Second,
})
```

### 测试选项

```go
type TestOptions struct {
    Model     string        // 必填：要测试的模型名称
    Timeout   time.Duration // 可选：超时时间，默认 10s
    MaxTokens int           // 可选：最大 token 数，默认 1
    Prompt    string        // 可选：测试 prompt，默认 "1"
}
```

### 测试结果

```go
type TestResult struct {
    Latency  time.Duration // 请求延迟
    Response ChatResponse  // 原始响应
}
```

### 应用场景

- **配置验证**：部署前验证 Provider 配置
- **凭证测试**：检查 API Key 是否有效
- **健康检查**：定期监控 AI 服务可用性
- **延迟监控**：测量不同 Provider 的响应速度

### 完整示例

参考：[examples/09-connectivity-test](../examples/09-connectivity-test/)

## 多轮对话（基础）

多轮对话会自动读取历史并追加新消息：

```go
cli.SessionStore = sessionstore.NewMemoryStore()
cli.SessionConfig.AutoCreate = true

sessionID := "user-001"
resp, err := cli.ChatSessionStreamSync(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "第一轮"}},
})
```

> `ChatSession()` 已标记为 Deprecated，推荐使用 `ChatSessionStreamSync()`。

## 流式对话

流式接口返回 `<-chan streaming.StreamChunk`，可逐段输出。

### 单轮对话（流式）

```go
stream, err := cli.ChatStream(ctx, "openai", base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "用三句话介绍 Go"}},
})
if err != nil {
    log.Fatal(err)
}
for chunk := range stream {
    if chunk.Error != nil {
        log.Fatalf("stream error: %v", chunk.Error)
    }
    fmt.Print(chunk.Text)
}
```

### 多轮对话（流式）

```go
stream, err := cli.ChatSessionStream(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "继续"}},
})
for chunk := range stream {
    if chunk.Error != nil {
        log.Fatalf("stream error: %v", chunk.Error)
    }
    fmt.Print(chunk.Text)
}
```

如果你更喜欢同步返回，使用 `ChatStreamSync()` / `ChatSessionStreamSync()`。

## 错误处理

- **请求错误**：检查 `err` 返回值（HTTP 错误、鉴权失败、网络中断）。
- **流式错误**：流式接口的错误在 `StreamChunk.Error` 中逐段传递。
- **会话错误**：存储层可能返回 `ErrSessionNotFound` / `ErrSessionConflict`。

## 最佳实践

1. **Context 管理**：每次请求创建独立 `context.WithTimeout`。
2. **速率限制**：高频请求间添加延迟（如 1~2 秒）。
3. **截断策略**：设置 `WindowPolicy` 防止历史过长。
4. **存储选型**：开发环境用 Memory，生产推荐 SQLite/PostgreSQL。

## 相关文档
- [文档索引](README.md)
- [快速开始](quickstart.md)
- [Session 教程](session-tutorial.md)
- [Session API](session-api.md)
- [流式迁移指南](migration-to-streaming.md)
- [API 参考](api-reference.md)
