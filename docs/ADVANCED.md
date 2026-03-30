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

通过 Quick API 的 `Request`/`Response`/`ChainFields` 三个字段接入任意 HTTP API。
SDK 内部会自动将原始 HTTP 报文编译为 GenericProfile，无需手动构造内部结构。

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `Request` | `string` | 原始 HTTP 请求模板文本。`$$$` = 用户输入，`$$$SESSION_ID$$$` = 会话 ID，`$$$NAME$$$` = 链路字段占位 |
| `Response` | `string` | 代表性 HTTP 响应文本，SDK 自动推断流式协议、delta 路径和结束条件 |
| `ChainFields` | `[]generic.ChainField` | 多轮字段链路传递规则（可不传） |

### ChainFields 来源

ChainFields 描述"上一轮响应中的某个字段值 → 下一轮请求中的占位符"的传递关系。典型场景：

- **parent_message_id**：响应返回 `message_id`，下一轮请求需注入 `parent_message_id`
- **conversation_id**：首轮响应分配的会话 ID，后续轮次需回传

每条 `generic.ChainField` 包含 3 个子字段：

- `Placeholder`：请求体中的占位符（格式 `$$$NAME$$$`，如 `$$$PARENT_MSG$$$`）
- `ResponsePath`：从响应 JSON 中提取值的路径（如 `message_id`）
- `ExtractOnEvent`：仅从指定 SSE event 类型提取（如 `message_end`），可为空

获取方式：

1. **手动构造**：根据 API 文档确定字段映射关系
2. **自动推理导出**：通过 `generic.ExportToHTTPSpec()` 从多轮抓包推理结果中自动生成

### 基础用法（手动构造）

```go
qs, err := client.New().Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     "https://api.example.com/v1/chat",
    SessionMode: "remote_session",
    APIKey:      "your-token",
    Request: `POST /v1/chat HTTP/1.1
Authorization: Bearer your-token
Content-Type: application/json

{
  "prompt": "$$$",
  "session_id": "$$$SESSION_ID$$$"
}`,
    Response: `data: {"content":"你好","session_id":"sess-001"}`,
})
```

### 自动推理 → Quick（一步到位）

```go
// RawReasoning = 推理 + 导出，一步返回 RawHTTPSpec
spec, err := cli.RawReasoning(rawMultiRoundSpec)

// 直接传给 Quick
qs, err := cli.Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     spec.BaseURL,
    SessionMode: spec.Model,
    Request:     spec.Request,
    Response:    spec.Response,
    ChainFields: spec.ChainFields,
})
```

如需分步控制（如检查推理报告），也可手动调用底层 API：

```go
sess, inferred, err := cli.NewSessionFromHTTPMultiRound(rawMultiRoundSpec)
spec, err := generic.ExportToHTTPSpec(inferred, rawMultiRoundSpec)
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

