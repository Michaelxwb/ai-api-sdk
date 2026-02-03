# Session Guide

本指南合并并更新了 Session 相关内容，统一为“Session 对象模式”。它覆盖 Session 的核心概念、SessionStore 接口、数据库存储方案、截断策略、最佳实践与故障排查。

## 1. 概述

Session 对象是对话的统一入口：单轮、多轮、流式、非流式都通过 `Session.Chat()` / `Session.ChatStream()` 完成。

- **Session 对象模式**：每次对话都从 `Client.NewSession(...)` 开始创建 Session。
- **HistoryMode**：控制是否自动加载历史。
- **SessionStore**：可选的存储后端，用于历史持久化和会话恢复。

## 2. 快速开始

### 2.1 最小示例（无存储）

```go
resp, err := cli.NewSession("openai").Chat(ctx, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.Text)
```

### 2.2 多轮对话（使用 SessionStore）

```go
import "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"

store, err := sessionstore.NewSQLiteStore("sessions.db")
if err != nil {
    log.Fatal(err)
}

defer store.Close()

sess := cli.NewSession(
    "openai",
    client.WithStore(store),
    client.WithHistoryMode(client.HistoryAuto),
    client.WithAutoID(),
)

resp1, _ := sess.Chat(ctx, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})
resp2, _ := sess.Chat(ctx, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "它的并发模型是什么？"}},
})

fmt.Println(resp1.Text)
fmt.Println(resp2.Text)
```

你也可以把 `store` 设为默认：`cli.SessionStore = store`，后续 `NewSession` 会自动继承。

### 2.3 会话恢复

```go
existingID := "user-001:topic-a"

sess := cli.NewSession(
    "openai",
    client.WithStore(store),
    client.WithID(existingID),
    client.WithHistoryMode(client.HistoryAuto),
)
```

## 3. 核心概念（Session 对象、HistoryMode、SessionStore）

### 3.1 Session 对象

Session 是对话状态的载体，内部负责：
1) 生成/维护 SessionID
2) 加载历史（HistoryAuto）
3) 合并本轮消息并发起请求
4) 将历史写回 SessionStore

**注意**：Session 实例不是并发安全的。并发对话请创建多个 Session 或按 SessionID 分片。

**SessionID 说明**：
- 非 Dify Provider：当 `store != nil` 且未显式指定 ID 时，首次请求会自动生成 UUID。
- Dify Provider：会话 ID 通常来自响应（如 `conversation_id`），可通过 `WithID()` 恢复历史。

### 3.2 HistoryMode

```go
type HistoryMode int

const (
    HistoryAuto HistoryMode = iota // 自动加载历史
    HistoryNone                    // 不加载历史，仅持久化
)
```

- **HistoryAuto**：多轮对话场景，自动从 SessionStore 读取历史并合并请求（默认值）。
- **HistoryNone**：单轮审计/记录场景，不加载历史，但仍会持久化本轮消息（store != nil）。

### 3.3 SessionStore

SessionStore 是存储后端的最小接口。Session 会在 `HistoryAuto` 下调用 `Get` 读取历史，并在请求结束后调用 `Save` 写回完整快照。

SessionStore 需要 **并发安全**，SDK 不会额外加锁。

当前实现对存储错误采取降级策略：`Get` / `Save` 失败不会阻断对话请求。建议在 Store 实现中记录日志与告警。

## 4. API 参考（SessionStore 接口定义）

```go
// session.Message is an alias to base.Message.
type Message = base.Message

// SessionState represents a full session snapshot.
type SessionState struct {
    ID        string
    Provider  string
    Messages  []Message
    CreatedAt time.Time
    UpdatedAt time.Time
    Meta      map[string]string
}

// SessionStore defines the minimal interface for session storage.
type SessionStore interface {
    Get(ctx context.Context, id string) (*SessionState, error)
    Save(ctx context.Context, state *SessionState) error
    Delete(ctx context.Context, id string) error
}

// SessionStoreAppender is an optional extension for message appends.
type SessionStoreAppender interface {
    Append(ctx context.Context, id string, msgs ...Message) error
}
```

Session 对象当前只调用 `Get` / `Save`；`Delete` 和 `SessionStoreAppender` 适用于业务侧清理或自定义封装场景。

**常见错误**（来自 `session` 包）：
- `ErrSessionNotFound`
- `ErrSessionConflict`
- `ErrSessionClosed`
- `ErrStoreUnavailable`

## 5. 高级用法（数据库存储、截断策略）

### 5.1 数据库存储

SDK 自带示例存储实现（位于 `examples/sessionstore`），支持 PostgreSQL / MySQL / SQLite。推荐在生产环境使用数据库存储。

**初始化示例**：

```go
import "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"

store, err := sessionstore.NewPostgresStore(
    "postgres://user:password@localhost:5432/sessions?sslmode=disable",
)
if err != nil {
    log.Fatal(err)
}

defer store.Close()

cli.SessionStore = store
```

**表结构（核心字段）**：

PostgreSQL：
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    attrs JSONB
);

CREATE TABLE IF NOT EXISTS session_messages (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);
```

MySQL：
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    provider VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    attrs JSON,
    INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS session_messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    INDEX idx_session_messages_session_id (session_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

SQLite：
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    attrs TEXT
);

CREATE TABLE IF NOT EXISTS session_messages (
    id INTEGER PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);
```

**清理旧会话（示例实现）**：

```go
deleted, err := store.CleanupOldSessions(ctx, 30*24*time.Hour)
if err != nil {
    log.Fatal(err)
}
log.Printf("deleted %d sessions", deleted)
```

### 5.2 截断策略

长对话需要控制历史长度以避免 Token 膨胀。SDK 提供 `session.TruncatePolicy` / `session.WindowPolicy` 作为工具，你可以在**请求构造阶段**或**存储层**应用截断：

**在请求前截断（自管历史）**：
```go
policy := session.WindowPolicy{MaxMessages: 30, KeepSystemPrompt: true}
req.Messages = policy.Truncate(req.Messages)
```

**在存储层截断**：
- `SessionStore.Get` 返回最近 N 条消息
- `SessionStore.Save` 写入时只保留最近 N 条

## 6. 最佳实践

- SessionID 设计：`userID + 业务场景/主题`，避免不同任务的上下文混淆。
- 主题切换时新建 Session，防止旧上下文干扰。
- 多并发场景使用不同 SessionID，避免同一 Session 被并发写入。
- 使用 `HistoryNone` 做单轮审计；使用 `HistoryAuto` 做多轮对话。
- 对数据库存储启用必要索引，定期清理旧会话。
- 为每次请求设置 `context.WithTimeout`，避免存储或网络阻塞。

## 7. 故障排查

- **Session Not Found**：确认 `SessionID` 是否正确；新会话使用 `WithAutoID()` 或显式 `WithID()`。
- **历史未加载**：检查 `HistoryMode` 是否为 `HistoryAuto`；确认 `SessionStore.Get` 能返回历史。
- **消息未持久化**：在 `SessionStore.Save` 中添加日志与告警；确认数据库连接与写权限。
- **性能问题**：历史过长时使用截断策略；数据库加索引并观察慢查询。
- **Dify 会话 ID 缺失**：确保上游响应包含 `SessionID`（如 `conversation_id`），否则无法恢复会话。
