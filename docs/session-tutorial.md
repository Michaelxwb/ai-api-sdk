# Session 多轮对话教程

本教程聚焦多轮对话的高级用法、最佳实践和故障排查。基础入门请先阅读 [基础使用](usage-guide.md)。

> 注意：`ChatSession` 已标记为 Deprecated，推荐使用 `ChatSessionStream` / `ChatSessionStreamSync`。详见 [流式迁移指南](migration-to-streaming.md)。

## 1. 多轮对话核心概念

### 1.1 会话生命周期

一次多轮对话通常包含以下阶段：

- **创建/恢复会话**：通过 `AutoCreate` 自动创建，或使用 `SessionStoreWithLifecycle` 显式创建。
- **加载历史**：`SessionStore.GetMessages` 拉取历史消息。
- **合并与截断**：将历史与新消息合并，并应用截断策略。
- **写回存储**：将本轮输入与模型回复追加到存储中。

### 1.2 上下文管理策略

- **SessionID 设计**：建议用 `userID + 场景/主题` 组合，避免不同任务混淆。
- **上下文隔离**：主题切换时新建 Session，避免旧上下文干扰。
- **元数据驱动**：将用户属性/业务字段放入 `SessionMeta.Attrs`，便于检索与运营分析。

### 1.3 消息截断策略

- **WindowPolicy**：保留最近 N 条消息，必要时保留系统提示。
- **GetOptions 提示**：`TruncatePolicy` 可提供 `GetOptions`，提示存储层按需返回。

```go
cli.SessionConfig.TruncatePolicy = session.WindowPolicy{
    MaxMessages:      30,
    KeepSystemPrompt: true,
}
```

## 2. 高级使用场景

### 2.1 主题切换和上下文隔离

为同一用户的不同主题创建独立会话：

```go
sessionID := fmt.Sprintf("user-%s:%s", userID, topicID)
resp, err := cli.ChatSessionStreamSync(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: userInput}},
})
```

### 2.2 长会话管理（大量历史消息）

- 使用截断策略控制历史长度。
- 必要时将旧历史摘要为“系统消息”写回，降低 Token 压力。
- 推荐使用流式接口降低长响应等待时间：

```go
stream, err := cli.ChatSessionStream(ctx, "openai", sessionID, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: userInput}},
})
for chunk := range stream {
    if chunk.Error != nil {
        break
    }
    fmt.Print(chunk.Text)
}
```

### 2.3 并发会话处理

- **多用户并发**：为每个用户使用独立 SessionID。
- **同一会话并发**：使用 `SessionStoreWithVersion` + `MaxConflictRetry` 处理冲突。

```go
cli.SessionConfig.MaxConflictRetry = 3
```

## 3. 最佳实践

### 3.1 性能优化

- 每次请求使用独立 `context.WithTimeout`。
- 合理配置 `WindowPolicy`，避免历史过长。
- 选择合适的存储实现（详情见 [数据库存储](session-database.md)）。

### 3.2 错误处理

- `ErrSessionConflict`：重试或返回提示。
- `ErrSessionNotFound`：开启 `AutoCreate` 或显式创建会话。
- `OnStoreError`：统一记录与上报存储错误。

```go
cli.SessionConfig.OnStoreError = func(ctx context.Context, err error) {
    log.Printf("session store error: %v", err)
}
```

### 3.3 并发控制

- 实现 `SessionStoreWithVersion` 以支持乐观锁。
- 高并发环境避免文件存储，优先 SQLite / Redis / 数据库。
- 分布式场景确保 SessionID 分区清晰，避免热键。

## 4. 实战案例

### 4.1 客服机器人（主题切换）

- 用户问题按主题拆分 Session（订单、退款、物流）。
- 每个主题独立历史，避免上下文污染。

### 4.2 代码助手（长会话）

- 长对话使用 `WindowPolicy` 控制历史。
- 定期摘要旧记录，保留关键上下文。

### 4.3 多用户聊天应用（并发）

- 每个房间或用户一个 Session。
- 使用支持并发的存储 + 乐观锁重试。

## 5. 故障排查

### 5.1 常见问题

- **Session 冲突**：启用 `MaxConflictRetry` 或实现版本控制。
- **消息丢失**：检查 `SessionStore` 错误回调与持久化配置。
- **性能问题**：启用截断策略，或迁移至更高性能存储。
- **Session Not Found**：开启 `AutoCreate` 或显式创建会话。

### 5.2 调试技巧

- 记录 `sessionID` 与请求 ID 便于排查。
- 开启 `OnStoreError` 日志。
- 使用 `GetMessages` 验证历史读取是否正确。

## 相关文档

- [基础使用](usage-guide.md) - 多轮对话快速入门
- [Session API](session-api.md) - API 详细说明
- [数据库存储](session-database.md) - 存储方案选择
- [架构设计](architecture.md) - Session 层设计
- [流式迁移指南](migration-to-streaming.md) - 流式优先迁移
