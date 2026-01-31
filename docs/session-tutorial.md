# 多轮对话使用教程

本教程将指导你如何在 ai-api-sdk 中使用多轮对话功能。

## 目录

- [快速入门](#快速入门)
- [存储方案选择](#存储方案选择)
- [常见场景](#常见场景)
- [故障排查](#故障排查)

## 快速入门

### 1. 最简单的示例（内存存储）

适合快速测试和开发：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
    "github.com/Michaelxwb/ai-api-sdk/provider"
    "github.com/Michaelxwb/ai-api-sdk/session"
)

func main() {
    // 1. 加载配置
    cfg, _ := config.LoadConfig("config.yaml")

    // 2. 初始化认证
    authStore := auth.NewFileStore(cfg.Auth.Store.Path)
    mgr, _ := auth.NewManager(authStore, &auth.RoundRobinSelector{})
    for _, cred := range cfg.Credentials {
        mgr.Register(cred)
    }

    // 3. 创建客户端并配置会话存储
    cli := client.NewClient(cfg, mgr)
    cli.SessionStore = sessionstore.NewMemoryStore()
    cli.SessionConfig = client.SessionConfig{
        AutoCreate: true,
        TruncatePolicy: session.WindowPolicy{
            MaxMessages:      20,
            KeepSystemPrompt: true,
        },
    }

    sessionID := "my-session-001"

    // 4. 第一轮对话
    ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel1()

    resp1, _ := cli.ChatSession(ctx1, "openai", sessionID, provider.ChatRequest{
        Model:    "gpt-4",
        Messages: []provider.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
    })
    fmt.Println("回复 1:", resp1.Text)

    // 5. 第二轮对话（自动携带历史）
    ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel2()

    resp2, _ := cli.ChatSession(ctx2, "openai", sessionID, provider.ChatRequest{
        Model:    "gpt-4",
        Messages: []provider.Message{{Role: "user", Content: "它的并发模型是什么？"}},
    })
    fmt.Println("回复 2:", resp2.Text)
}
```

### 2. 持久化示例（SQLite）

适合生产环境和需要持久化的场景：

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
    "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
    // 1. 创建 SQLite 存储
    store, err := sessionstore.NewSQLiteStore("./sessions.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // 2. 配置客户端
    cfg, _ := config.LoadConfig("config.yaml")
    authStore := auth.NewFileStore(cfg.Auth.Store.Path)
    mgr, _ := auth.NewManager(authStore, &auth.RoundRobinSelector{})
    for _, cred := range cfg.Credentials {
        mgr.Register(cred)
    }

    cli := client.NewClient(cfg, mgr)
    cli.SessionStore = store
    cli.SessionConfig.AutoCreate = true

    // 3. 多轮对话
    sessionID := "persistent-session"

    ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel1()
    cli.ChatSession(ctx1, "openai", sessionID, provider.ChatRequest{
        Model:    "gpt-4",
        Messages: []provider.Message{{Role: "user", Content: "第一个问题"}},
    })

    // 延迟避免速率限制
    time.Sleep(2 * time.Second)

    ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
    defer cancel2()
    cli.ChatSession(ctx2, "openai", sessionID, provider.ChatRequest{
        Model:    "gpt-4",
        Messages: []provider.Message{{Role: "user", Content: "第二个问题"}},
    })

    // 4. 程序重启后恢复会话
    messages, _ := store.GetMessages(context.Background(), sessionID, session.GetOptions{})
    log.Printf("恢复了 %d 条历史消息", len(messages))
}
```

## 存储方案选择

### 内存存储（Memory）

**适用场景**：
- 开发和测试
- 单次运行的脚本
- 不需要持久化的临时对话

**优点**：
- 零配置，开箱即用
- 性能最高

**缺点**：
- 程序重启后丢失数据
- 不适合生产环境

**代码示例**：
```go
cli.SessionStore = sessionstore.NewMemoryStore()
```

### 文件存储（File）

**适用场景**：
- 单机部署
- 小规模应用（会话数 < 1000）
- 需要简单持久化

**优点**：
- 零依赖（只需文件系统）
- 可读性好（JSON 格式）
- 易于备份和迁移

**缺点**：
- 并发性能差（文件锁）
- 大文件加载慢

**代码示例**：
```go
store, _ := sessionstore.NewFileStore("./sessions.json")
defer store.Close()
cli.SessionStore = store
```

### SQLite 存储（推荐）

**适用场景**：
- 单机或小规模分布式部署
- 中等规模应用（会话数 < 100万）
- 需要 ACID 事务和并发安全

**优点**：
- 零配置数据库（嵌入式）
- 支持事务和索引
- 并发安全（WAL 模式）
- 性能优秀

**缺点**：
- 不适合大规模分布式

**代码示例**：
```go
store, _ := sessionstore.NewSQLiteStore("./sessions.db")
defer store.Close()
cli.SessionStore = store
```

**管理命令**：
```bash
# 查看会话列表
sqlite3 sessions.db "SELECT id, provider, model, created_at FROM sessions"

# 查看消息内容
sqlite3 sessions.db "SELECT role, content FROM messages WHERE session_id='xxx'"

# 清空所有会话
sqlite3 sessions.db "DELETE FROM sessions; DELETE FROM messages"
```

### Redis 存储

**适用场景**：
- 分布式部署
- 高并发场景（QPS > 1000）
- 需要 TTL 自动过期

**优点**：
- 高性能（内存存储）
- 分布式友好
- 支持 TTL 自动清理

**缺点**：
- 需要独立部署 Redis
- 内存成本高

**代码示例**：
```go
import "github.com/redis/go-redis/v9"

rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

store := sessionstore.NewRedisStore(rdb, 3600*time.Second) // 1 小时 TTL
cli.SessionStore = store
```

## 常见场景

### 场景 1：客服聊天机器人

每个用户一个会话，长期保留历史：

```go
// 用户登录时创建或恢复会话
userID := "user-12345"
sessionID := fmt.Sprintf("customer-%s", userID)

// 查询历史消息显示
ctx := context.Background()
history, _ := store.GetMessages(ctx, sessionID, session.GetOptions{
    MaxMessages: 10, // 只显示最近 10 条
})

// 用户发送新消息
ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
defer cancel()

resp, _ := cli.ChatSession(ctx, "openai", sessionID, provider.ChatRequest{
    Model:    "gpt-4",
    Messages: []provider.Message{{Role: "user", Content: userInput}},
})

// 返回回复给用户
sendToUser(resp.Text)
```

### 场景 2：临时对话（阅后即焚）

使用内存存储或 Redis TTL：

```go
// 内存存储（程序重启清空）
cli.SessionStore = sessionstore.NewMemoryStore()

// 或 Redis 短期 TTL
store := sessionstore.NewRedisStore(rdb, 300*time.Second) // 5 分钟过期
```

### 场景 3：多用户并发

SQLite 或 Redis 确保并发安全：

```go
// SQLite 自动处理并发（WAL 模式）
store, _ := sessionstore.NewSQLiteStore("./sessions.db")

// 多个 goroutine 同时访问不同会话
for i := 0; i < 10; i++ {
    go func(userID int) {
        sessionID := fmt.Sprintf("user-%d", userID)
        ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
        defer cancel()

        cli.ChatSession(ctx, "openai", sessionID, provider.ChatRequest{
            Model:    "gpt-4",
            Messages: []provider.Message{{Role: "user", Content: "hello"}},
        })
    }(i)
}
```

### 场景 4：会话元数据管理

追踪用户信息和业务数据：

```go
// 创建会话时设置元数据
meta := &session.SessionMeta{
    ID:       sessionID,
    Provider: "openai",
    Model:    "gpt-4",
    Attrs: map[string]any{
        "user_id":    "user-123",
        "user_name":  "张三",
        "department": "销售部",
        "created_by": "web-app",
    },
}

if metaStore, ok := store.(session.SessionStoreWithMeta); ok {
    metaStore.UpsertMeta(ctx, sessionID, meta)
}

// 后续查询元数据
meta, _ := metaStore.GetMeta(ctx, sessionID)
userName := meta.Attrs["user_name"].(string)
```

### 场景 5：定期清理旧会话

使用 SQLite 的扩展方法：

```go
sqliteStore, ok := store.(*sessionstore.SQLiteStore)
if ok {
    // 清理 30 天前的会话
    deleted, _ := sqliteStore.CleanupOldSessions(ctx, 30*24*time.Hour)
    log.Printf("清理了 %d 个旧会话", deleted)
}

// 或通过定时任务
ticker := time.NewTicker(24 * time.Hour)
go func() {
    for range ticker.C {
        sqliteStore.CleanupOldSessions(context.Background(), 30*24*time.Hour)
    }
}()
```

### 场景 6：流式对话

配合 ChatWithStream 实现流式多轮对话：

```go
// 先获取历史
history, _ := store.GetMessages(ctx, sessionID, session.GetOptions{})

// 合并新消息
allMessages := append(history, provider.Message{
    Role:    "user",
    Content: userInput,
})

// 流式调用
stream, _ := cli.ChatWithStream(ctx, "openai", provider.ChatRequest{
    Model:    "gpt-4",
    Messages: allMessages,
})

var fullResponse string
for chunk := range stream {
    fmt.Print(chunk.Text)
    fullResponse += chunk.Text
}

// 保存对话历史
store.AppendMessages(ctx, sessionID, []provider.Message{
    {Role: "user", Content: userInput},
    {Role: "assistant", Content: fullResponse},
})
```

## 故障排查

### 问题 1：Context Deadline Exceeded

**原因**：多次请求共享同一个 context 导致超时累积。

**解决方案**：每次 ChatSession 调用创建独立 context。

```go
// ❌ 错误
ctx := context.WithTimeout(context.Background(), 300*time.Second)
resp1, _ := cli.ChatSession(ctx, ...)
resp2, _ := cli.ChatSession(ctx, ...) // 超时！

// ✅ 正确
ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
defer cancel1()
resp1, _ := cli.ChatSession(ctx1, ...)

ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
defer cancel2()
resp2, _ := cli.ChatSession(ctx2, ...)
```

### 问题 2：429 Too Many Requests

**原因**：短时间内连续请求触发 API 速率限制。

**解决方案**：在请求之间添加延迟。

```go
resp1, _ := cli.ChatSession(ctx1, ...)
time.Sleep(2 * time.Second) // 延迟 2 秒
resp2, _ := cli.ChatSession(ctx2, ...)
```

### 问题 3：Session Not Found

**原因**：会话不存在且 AutoCreate 未启用。

**解决方案**：启用自动创建或手动创建会话。

```go
// 方案 1：启用自动创建
cli.SessionConfig.AutoCreate = true

// 方案 2：手动创建
if lifecycle, ok := store.(session.SessionStoreWithLifecycle); ok {
    lifecycle.CreateSession(ctx, sessionID, &session.SessionMeta{
        ID:       sessionID,
        Provider: "openai",
        Model:    "gpt-4",
    })
}
```

### 问题 4：消息历史过长

**原因**：未配置截断策略，历史消息超出 Token 限制。

**解决方案**：设置合理的截断策略。

```go
cli.SessionConfig.TruncatePolicy = session.WindowPolicy{
    MaxMessages:      20,          // 最多 20 条消息
    KeepSystemPrompt: true,        // 保留系统提示
}
```

### 问题 5：并发冲突

**原因**：多个请求同时修改同一会话（文件存储常见）。

**解决方案**：
1. 使用 SQLite 或 Redis（并发安全）
2. 或实现乐观锁重试

```go
cli.SessionConfig.MaxConflictRetry = 3 // 冲突时重试 3 次
```

### 问题 6：文件存储锁定

**原因**：文件存储在高并发下锁竞争。

**解决方案**：迁移到 SQLite 或 Redis。

```go
// 从文件存储迁移到 SQLite
sqliteStore, _ := sessionstore.NewSQLiteStore("./sessions.db")

// 导入历史数据（一次性任务）
fileStore, _ := sessionstore.NewFileStore("./sessions.json")
sessions, _ := fileStore.ListSessions(ctx, "")
for _, sid := range sessions {
    msgs, _ := fileStore.GetMessages(ctx, sid, session.GetOptions{})
    sqliteStore.AppendMessages(ctx, sid, msgs)
}
```

## 性能调优

### 1. 合理设置超时时间

根据提供者和模型调整：

```go
// 快速模型（如 GPT-3.5）
ctx := context.WithTimeout(context.Background(), 60*time.Second)

// 慢速模型（如 GPT-4, Claude Opus）
ctx := context.WithTimeout(context.Background(), 300*time.Second)
```

### 2. 批量操作

单次追加多条消息：

```go
msgs := []provider.Message{
    {Role: "user", Content: "问题 1"},
    {Role: "assistant", Content: "回答 1"},
    {Role: "user", Content: "问题 2"},
    {Role: "assistant", Content: "回答 2"},
}
store.AppendMessages(ctx, sessionID, msgs)
```

### 3. 预加载历史

频繁访问的会话可缓存历史：

```go
type SessionCache struct {
    mu    sync.RWMutex
    cache map[string][]provider.Message
}

func (c *SessionCache) Get(sessionID string) []provider.Message {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache[sessionID]
}
```

### 4. 定期维护

清理旧数据减小数据库体积：

```go
// SQLite 定期 VACUUM
sqliteStore.CleanupOldSessions(ctx, 30*24*time.Hour)

// 然后手动执行
// sqlite3 sessions.db "VACUUM"
```

## 下一步

- 查看 [API 文档](session-api.md) 了解详细接口
- 查看 `examples/` 目录查看完整示例
- 根据需求实现自定义存储（实现 SessionStore 接口）
