# 高级主题

## 自定义 SessionStore

SDK 提供 `session.SessionStore` 接口，可自定义存储实现。

### 接口定义

```go
type SessionStore interface {
    Get(ctx context.Context, id string) (*SessionState, error)
    Save(ctx context.Context, state *SessionState) error
    Delete(ctx context.Context, id string) error
}
```

### 使用自定义 Store

```go
type MyStore struct {
    // 你的存储实现
}

func (s *MyStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
    // 实现逻辑
}

func (s *MyStore) Save(ctx context.Context, state *session.SessionState) error {
    // 实现逻辑
}

func (s *MyStore) Delete(ctx context.Context, id string) error {
    // 实现逻辑
}

// 使用
qs := client.New().Quick(client.ProviderConfig{
    Provider:    "openai",
    APIKey:      "sk-xxx",
    SessionMode: "local_history",
    Store:       &MyStore{},
})
```

参考实现：`examples/sessionstore/`（Memory/File/SQLite/MySQL/PostgreSQL/Redis）

## 自定义认证

### 自定义 Header

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "generic",
    AuthHeaders: map[string]string{
        "X-API-Key": "your-key",
        "X-Custom":  "value",
    },
})
```

### 自定义 Query 参数

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "generic",
    QueryParams: map[string]string{
        "api_key": "your-key",
        "version": "v1",
    },
})
```

## Generic 适配器

接入任意 HTTP API：

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "generic",
    BaseURL:  "https://api.example.com",
    Path:     "/chat",
    GenericProfile: map[string]any{
        "request_template": map[string]any{
            "messages_path": "input.messages",
            "model_path":    "model",
        },
        "response_path": "output.text",
        "stream_path":   "data.content",
    },
})
```

详见：`docs/internal/design-generic-adapter-integration.md`

## 高级 Session 选项

### 访问底层 Session

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
})

// 获取底层 Session 对象
sess := qs.Session()

// 使用 Session 的高级方法
resp, err := sess.Chat(ctx, base.ChatRequest{
    Model:    "gpt-4",
    Messages: []base.Message{{Role: "user", Content: "Hello"}},
})
```

### 使用旧 API（不推荐）

如果需要更底层的控制，可以使用 `NewSessionWith`：

```go
cred := auth.NewCredential("openai", "sk-xxx")
pc := &config.ProviderConfig{Name: "openai", Type: "openai"}

sess := client.New().NewSessionWith(cred, pc,
    client.WithConversationMode(client.ConversationModeLocalHistory),
    client.WithStore(session.NewMemoryStore()),
    client.WithTimeout(60*time.Second),
)
```

但推荐使用 Quick API，更简洁且功能完整。

