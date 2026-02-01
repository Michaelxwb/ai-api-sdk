# Session API 参考

本文档仅包含接口定义与参数说明。使用场景和最佳实践请参考 [Session 教程](session-tutorial.md)。

## 1. 核心接口

<a id="sessionstore-接口"></a>
### 1.1 SessionStore 接口

```go
type SessionStore interface {
    GetMessages(ctx context.Context, sessionID string, opts GetOptions) ([]Message, error)
    AppendMessages(ctx context.Context, sessionID string, msgs []Message) error
}
```

**方法说明**：
- `GetMessages`：获取会话历史消息；会话不存在时应返回 `ErrSessionNotFound`。
- `AppendMessages`：追加新消息；实现可选择在追加时隐式创建会话。

**参数详解**：
- `sessionID`：会话唯一标识。
- `opts`：拉取选项（可选），实现可忽略或用于优化查询。
- `msgs`：待追加的消息列表（`Message` 为 `base.Message` 的别名）。

### 1.2 SessionStoreWithMeta 接口

```go
type SessionStoreWithMeta interface {
    GetMeta(ctx context.Context, sessionID string) (*SessionMeta, error)
    UpsertMeta(ctx context.Context, sessionID string, meta *SessionMeta) error
}
```

**方法说明**：
- `GetMeta`：读取会话元数据。
- `UpsertMeta`：更新或插入会话元数据。

### 1.3 SessionStoreWithVersion 接口

```go
type SessionStoreWithVersion interface {
    GetVersion(ctx context.Context, sessionID string) (int64, error)
    AppendMessagesWithVersion(ctx context.Context, sessionID string, expectedVersion int64, msgs []Message) (int64, error)
}
```

**方法说明**：
- `GetVersion`：获取当前会话版本号。
- `AppendMessagesWithVersion`：基于期望版本号追加消息；冲突时返回 `ErrSessionConflict`。

### 1.4 SessionStoreWithLifecycle 接口

```go
type SessionStoreWithLifecycle interface {
    CreateSession(ctx context.Context, sessionID string, meta *SessionMeta) error
    DeleteSession(ctx context.Context, sessionID string) error
}
```

**方法说明**：
- `CreateSession`：显式创建会话。
- `DeleteSession`：删除会话及其历史。
该接口不包含读写方法，通常与 `SessionStore` 组合实现。

## 2. 截断策略

<a id="truncatepolicy-接口"></a>
### 2.1 TruncatePolicy 接口

```go
type TruncatePolicy interface {
    Truncate(messages []Message) []Message
}
```

### 2.2 WindowPolicy 实现

```go
type WindowPolicy struct {
    MaxMessages      int
    KeepSystemPrompt bool
}
```

- `MaxMessages`：最多保留消息数（<=0 表示不限制）。
- `KeepSystemPrompt`：是否保留前置系统消息。

`WindowPolicy` 还实现了 `Options() GetOptions`，用于向存储提供取数提示。

### 2.3 自定义截断策略

实现 `Truncate(messages []Message)` 即可；如需提示存储层优化，可额外实现 `Options() GetOptions`。

## 3. 配置结构

### 3.1 SessionConfig

```go
type SessionConfig struct {
    AutoCreate       bool
    TruncatePolicy   session.TruncatePolicy
    OnStoreError     func(context.Context, error)
    MaxConflictRetry int
}
```

**字段说明**：
- `AutoCreate`：会话不存在时是否自动创建（存储需支持生命周期接口）。
- `TruncatePolicy`：消息截断策略。
- `OnStoreError`：存储错误回调（错误仍会返回给调用方）。
- `MaxConflictRetry`：`ErrSessionConflict` 重试次数。

### 3.2 GetOptions

```go
type GetOptions struct {
    MaxMessages      int
    MaxTokens        int
    KeepSystemPrompt bool
}
```

**字段说明**：
- `MaxMessages`：最多返回消息数（0 表示无限制）。
- `MaxTokens`：Token 预算提示（实现可忽略）。
- `KeepSystemPrompt`：截断时是否保留系统提示。

### 3.3 SessionMeta

```go
type SessionMeta struct {
    ID        string
    Provider  string
    Model     string
    CreatedAt time.Time
    UpdatedAt time.Time
    Attrs     map[string]any
}
```

## 4. 错误类型

- `ErrSessionNotFound`
- `ErrSessionConflict`
- `ErrSessionClosed`
- `ErrStoreUnavailable`

## 相关文档

- [Session 教程](session-tutorial.md) - 使用场景和最佳实践
- [数据库存储](session-database.md) - 存储实现
- [API 参考](api-reference.md) - 完整 API 索引
