# Session API 文档

## 概述

Session 包提供了多轮对话的标准接口定义，支持应用层灵活实现存储方案（内存、文件、数据库等）。

## 核心接口

### SessionStore

所有存储实现必须实现的最小接口：

```go
type SessionStore interface {
    // GetMessages 获取会话历史消息
    GetMessages(ctx context.Context, sessionID string, opts GetOptions) ([]Message, error)

    // AppendMessages 追加新消息到会话
    AppendMessages(ctx context.Context, sessionID string, msgs []Message) error
}
```

**参数说明**：
- `sessionID`: 会话唯一标识符
- `opts`: 查询选项（最大消息数、Token 限制、是否保留系统提示）
- `msgs`: 要追加的消息列表

**错误类型**：
- `ErrSessionNotFound`: 会话不存在
- `ErrSessionConflict`: 并发冲突（乐观锁）
- `ErrSessionClosed`: 会话已关闭

### 可选扩展接口

#### SessionStoreWithLifecycle

支持会话生命周期管理：

```go
type SessionStoreWithLifecycle interface {
    SessionStore

    // CreateSession 创建新会话
    CreateSession(ctx context.Context, sessionID string, meta *SessionMeta) error

    // DeleteSession 删除会话
    DeleteSession(ctx context.Context, sessionID string) error

    // ListSessions 列出会话（可选前缀过滤）
    ListSessions(ctx context.Context, prefix string) ([]string, error)
}
```

#### SessionStoreWithMeta

支持会话元数据管理：

```go
type SessionStoreWithMeta interface {
    SessionStore

    // GetMeta 获取会话元数据
    GetMeta(ctx context.Context, sessionID string) (*SessionMeta, error)

    // UpsertMeta 更新或插入会话元数据
    UpsertMeta(ctx context.Context, sessionID string, meta *SessionMeta) error
}
```

**SessionMeta 结构**：
```go
type SessionMeta struct {
    ID        string         // 会话 ID
    Provider  string         // 提供者名称（如 "openai"）
    Model     string         // 模型名称（如 "gpt-4"）
    CreatedAt time.Time      // 创建时间
    UpdatedAt time.Time      // 更新时间
    Attrs     map[string]any // 自定义属性（业务数据）
}
```

#### SessionStoreWithVersion

支持乐观锁并发控制：

```go
type SessionStoreWithVersion interface {
    SessionStore

    // AppendMessagesWithVersion 基于版本号追加消息
    AppendMessagesWithVersion(ctx context.Context, sessionID string,
        msgs []Message, expectedVersion int64) (newVersion int64, err error)
}
```

## 数据结构

### Message

```go
type Message struct {
    Role    string // "user", "assistant", "system"
    Content string // 消息内容
}
```

### GetOptions

```go
type GetOptions struct {
    MaxMessages      int  // 最多返回消息数（0 表示无限制）
    MaxTokens        int  // 最大 Token 数（粗略估算，0 表示无限制）
    KeepSystemPrompt bool // 是否始终保留系统提示
}
```

## 截断策略

### TruncatePolicy 接口

```go
type TruncatePolicy interface {
    // Truncate 根据策略截断消息列表
    Truncate(messages []Message) []Message
}
```

### WindowPolicy 实现

保留最近 N 条消息，可选保留系统提示：

```go
type WindowPolicy struct {
    MaxMessages      int  // 最多保留消息数
    KeepSystemPrompt bool // 是否保留第一条系统消息
}
```

**示例**：
```go
policy := session.WindowPolicy{
    MaxMessages:      20,
    KeepSystemPrompt: true,
}

truncated := policy.Truncate(messages)
```

## Client 扩展

### 字段

```go
type Client struct {
    // ... 原有字段

    SessionStore  session.SessionStore  // 会话存储实现
    SessionConfig SessionConfig         // 会话配置
}
```

### SessionConfig

```go
type SessionConfig struct {
    AutoCreate       bool                      // 自动创建不存在的会话
    TruncatePolicy   session.TruncatePolicy    // 消息截断策略
    OnStoreError     func(error)               // 存储错误回调
    MaxConflictRetry int                       // 冲突重试次数（默认 3）
}
```

### ChatSession 方法

```go
func (c *Client) ChatSession(
    ctx context.Context,
    providerName string,
    sessionID string,
    req provider.ChatRequest,
) (provider.ChatResponse, error)
```

**执行流程**：
1. 从 SessionStore 获取历史消息
2. 合并历史与新消息
3. 可选截断（基于 TruncatePolicy）
4. 调用底层 Chat API
5. 保存新消息到 SessionStore
6. 支持乐观锁重试（如果实现了 WithVersion 接口）

## 错误处理

### 标准错误

```go
var (
    ErrSessionNotFound = errors.New("session not found")
    ErrSessionConflict = errors.New("session conflict")
    ErrSessionClosed   = errors.New("session closed")
)
```

### 错误处理建议

```go
resp, err := cli.ChatSession(ctx, "openai", sessionID, req)
if err != nil {
    if errors.Is(err, session.ErrSessionNotFound) {
        // 会话不存在处理
    } else if errors.Is(err, session.ErrSessionConflict) {
        // 并发冲突处理（通常自动重试）
    }
    return err
}
```

## 最佳实践

### 1. Context 管理

每次 API 调用使用独立 context，避免超时累积：

```go
// ✅ 正确
ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
defer cancel1()
resp1, _ := cli.ChatSession(ctx1, "openai", sessionID, req1)

ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
defer cancel2()
resp2, _ := cli.ChatSession(ctx2, "openai", sessionID, req2)

// ❌ 错误：共享 context 导致超时累积
ctx := context.WithTimeout(context.Background(), 300*time.Second)
resp1, _ := cli.ChatSession(ctx, "openai", sessionID, req1)
resp2, _ := cli.ChatSession(ctx, "openai", sessionID, req2) // 剩余时间不足！
```

### 2. 速率限制

在连续请求之间添加延迟，避免触发 API 限流：

```go
resp1, _ := cli.ChatSession(ctx1, "openai", sessionID, req1)
time.Sleep(2 * time.Second) // 避免 429 错误
resp2, _ := cli.ChatSession(ctx2, "openai", sessionID, req2)
```

### 3. 存储选择

| 存储方案 | 适用场景 | 特点 |
|---------|---------|------|
| Memory  | 开发/测试 | 零配置，无持久化 |
| File    | 单机小规模 | JSON 文件，简单持久化 |
| SQLite  | 单机中规模（推荐） | ACID 事务，并发安全 |
| Redis   | 分布式高并发 | 高性能，支持 TTL |

### 4. 截断策略

根据 Provider Token 限制设置：

```go
// OpenAI GPT-4: 8K tokens
cli.SessionConfig.TruncatePolicy = session.WindowPolicy{
    MaxMessages:      20, // 约 4K tokens（平均每条 200 tokens）
    KeepSystemPrompt: true,
}

// Claude Opus: 200K tokens（可以保留更多历史）
cli.SessionConfig.TruncatePolicy = session.WindowPolicy{
    MaxMessages:      100,
    KeepSystemPrompt: true,
}
```

### 5. 元数据使用

利用 SessionMeta 存储业务信息：

```go
meta := &session.SessionMeta{
    ID:       sessionID,
    Provider: "openai",
    Model:    "gpt-4",
    Attrs: map[string]any{
        "user_id":     "user-123",
        "department":  "engineering",
        "cost_center": "prod-ai",
    },
}

if metaStore, ok := cli.SessionStore.(session.SessionStoreWithMeta); ok {
    metaStore.UpsertMeta(ctx, sessionID, meta)
}
```

## 并发安全

- **接口层**：SDK 不提供锁，由存储实现负责
- **内存存储**：使用 `sync.RWMutex`
- **文件存储**：文件级锁（不适合高并发）
- **SQLite 存储**：事务 + WAL 模式
- **Redis 存储**：原子操作 + Lua 脚本
- **乐观锁**：实现 `SessionStoreWithVersion` 接口

## 性能优化

### 1. 预估 Token 数

避免过度截断或超出限制：

```go
// 粗略估算：中文 1 字符 ≈ 2 tokens，英文 1 单词 ≈ 1.3 tokens
estimatedTokens := len(content) / 2 // 中文
```

### 2. 批量操作

单次调用追加多条消息：

```go
msgs := []session.Message{
    {Role: "user", Content: userInput},
    {Role: "assistant", Content: aiResponse},
}
store.AppendMessages(ctx, sessionID, msgs)
```

### 3. 缓存元数据

频繁访问的元数据可在应用层缓存：

```go
type CachedStore struct {
    session.SessionStore
    metaCache sync.Map // sessionID -> *SessionMeta
}
```

## 参考实现

SDK 提供了 4 种参考存储实现（位于 `examples/sessionstore/`）：

1. **memory.go** - 内存存储
2. **file.go** - JSON 文件存储
3. **sqlite.go** - SQLite 数据库存储（推荐）
4. **redis.go** - Redis 存储

详细使用示例参见 `examples/session-*/main.go`。
