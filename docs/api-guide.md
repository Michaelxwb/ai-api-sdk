# API 指南

本文档合并了 API 参考与使用指南，统一以 `Session` 对象作为调用入口，覆盖基础用法、核心 API、连通性测试与最佳实践。

## 1. 概述

SDK 的核心调用流程：

1. 创建 `Client`
2. 通过 `Client` 创建 `Session`
3. 使用 `Session.Chat()` 或 `Session.ChatStream()` 完成对话
4. 需要时进行连通性测试与错误处理

## 2. 快速开始（基础示例）

下面示例使用本地配置模式（配置文件 + Manager），通过 `Session` 发起一次非流式对话：

```go
import (
    "context"
    "log"

    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
)

func main() {
    ctx := context.Background()

    // 配置文件 + Manager（本地配置模式）
    cli := client.NewClient(cfg, mgr)

    resp, err := cli.NewSession("openai").Chat(ctx, base.ChatRequest{
        Model:    "gpt-4o-mini",
        Messages: []base.Message{{Role: "user", Content: "hello"}},
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Println(resp.Text)
}
```

> `cfg` / `mgr` 的构建方式请参考配置指南与认证说明。

## 3. Client API（NewSession, NewSessionWith）

### 构造函数

```go
cli := client.NewClient(cfg, mgr) // 配置文件 + Manager
cli := client.New()               // 轻量构造（平台集成）
```

### NewSession / NewSessionWith

```go
func (c *Client) NewSession(provider string, opts ...SessionOption) *Session
func (c *Client) NewSessionWith(cred *auth.Credential, pc *config.ProviderConfig, opts ...SessionOption) *Session
```

- `NewSession` 使用配置文件中的 Provider + Manager 解析凭证，并创建会话对象
- `NewSessionWith` 直接传入凭证与 Provider 配置，适合平台集成

## 4. Session API（Chat, ChatStream, ID）

`Session` 负责管理会话上下文与多轮对话，统一通过以下方法调用：

```go
// 非流式对话
func (s *Session) Chat(ctx context.Context, req base.ChatRequest) (base.ChatResponse, error)

// 流式对话
func (s *Session) ChatStream(ctx context.Context, req base.ChatRequest) (<-chan streaming.StreamChunk, error)

// 获取 SessionID
func (s *Session) ID() string
```

### 流式示例

```go
stream, err := cli.NewSession("openai").ChatStream(ctx, base.ChatRequest{
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

### 多轮对话示例（基础）

```go
cli.SessionStore = sessionstore.NewMemoryStore()
cli.SessionConfig.AutoCreate = true

sessionID := "user-001"
resp, err := cli.NewSession("openai", client.WithID(sessionID)).Chat(ctx, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "第一轮"}},
})
```

## 5. 连通性测试（Test, TestWith）

在正式使用前，可以通过连通性测试快速验证配置是否正确。

### Test / TestWith

```go
// 本地配置模式测试
func (c *Client) Test(
    ctx context.Context,
    providerName string,
    opt *TestOptions,
) (TestResult, error)

// 平台集成模式测试
func (c *Client) TestWith(
    ctx context.Context,
    cred *auth.Credential,
    pc *config.ProviderConfig,
    opt *TestOptions,
) (TestResult, error)
```

### TestOptions

```go
type TestOptions struct {
    Model     string        // 必填：要测试的模型名称
    Timeout   time.Duration // 可选：超时时间，默认 10s
    MaxTokens int           // 可选：最大 token 数，默认 1
    Prompt    string        // 可选：测试 prompt，默认 "1"
}
```

### TestResult

```go
type TestResult struct {
    Latency  time.Duration // 请求延迟
    Response ChatResponse  // 原始响应
}
```

### 使用示例

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

## 6. 配置与认证

SDK 支持两种典型使用模式：

- **本地配置模式**：使用配置文件 + `Manager` 解析凭证，通过 `NewSession(provider)` 创建会话
- **平台集成模式**：直接传入 `Credential` 与 `ProviderConfig`，通过 `NewSessionWith(cred, pc)` 创建会话

平台集成模式示例（省略 import）：

```go
resp, err := cli.NewSessionWith(cred, pc).Chat(ctx, base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "hello"}},
})
```

### Auth Manager

```go
func NewManager(store CredentialStore, selector Selector) (*Manager, error)
func (m *Manager) Resolve(provider string) (*Credential, AuthStrategy, error)
func (m *Manager) Register(cred *Credential)
func (m *Manager) List() []*Credential
func (m *Manager) MarkFailed(id string)
func (m *Manager) MarkSuccess(id string)
func (m *Manager) RefreshOAuth(ctx context.Context, cred *Credential) error
```

### Credential

```go
type Credential struct {
    ID           string
    Provider     string
    AuthType     AuthType
    APIKey       string
    AccessToken  string
    RefreshToken string
    ExpiresAt    *time.Time
    Headers      map[string]string
    QueryParams  map[string]string
    Priority     int
    Disabled     bool
    Metadata     map[string]any
}
```

### AuthStrategy

```go
type AuthStrategy interface {
    Apply(req *http.Request) error
}
```

内置策略：`NoAuth` / `BearerTokenStrategy` / `ApiKeyHeaderStrategy` / `OAuthStrategy` / `JWTSignStrategy` / `CustomHeaderStrategy`。

## 7. 错误处理

- **请求错误**：检查 `err` 返回值（HTTP 错误、鉴权失败、网络中断）。
- **流式错误**：流式接口的错误在 `StreamChunk.Error` 中逐段传递。
- **会话错误**：存储层可能返回 `ErrSessionNotFound` / `ErrSessionConflict`。

## 8. 最佳实践

1. **Context 管理**：每次请求创建独立 `context.WithTimeout`。
2. **速率限制**：高频请求间添加延迟（如 1~2 秒）。
3. **截断策略**：设置 `WindowPolicy` 防止历史过长。
4. **存储选型**：开发环境用 Memory，生产推荐 SQLite/PostgreSQL。

## 相关文档

- [文档索引](README.md)
- [快速开始](quickstart.md)
- [配置指南](configuration.md)
- [使用指南](usage-guide.md)
- [Session API](session-api.md)
- [流式迁移指南](migration-to-streaming.md)
- [API 参考](api-reference.md)
