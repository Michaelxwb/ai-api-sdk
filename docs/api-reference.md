# API 参考

本文档提供 SDK 核心 API 的速览。完整实现与注释可参考源码。

## 包导入说明

常用类型来自以下包：

```go
import (
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
    "github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)
```

## Client API

### 构造函数

```go
cli := client.NewClient(cfg, mgr) // 配置文件 + Manager
cli := client.New()               // 轻量构造（平台集成）
```

### Chat / ChatWith

```go
func (c *Client) Chat(ctx context.Context, providerName string, req base.ChatRequest) (base.ChatResponse, error)
func (c *Client) ChatWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error)
```

- `Chat` 使用配置文件中的 Provider + Manager 解析凭证
- `ChatWith` 直接传入凭证与 Provider 配置，适合平台集成

### ChatStream / ChatStreamSync

```go
func (c *Client) ChatStream(ctx context.Context, providerName string, req base.ChatRequest) (<-chan streaming.StreamChunk, error)
func (c *Client) ChatStreamSync(ctx context.Context, providerName string, req base.ChatRequest) (base.ChatResponse, error)
```

- `ChatStream` 返回流式结果
- `ChatStreamSync` 自动聚合流式输出并返回 `ChatResponse`

### ChatSession / ChatSessionStream / ChatSessionStreamSync

```go
func (c *Client) ChatSession(ctx context.Context, providerName, sessionID string, req base.ChatRequest) (base.ChatResponse, error)
func (c *Client) ChatSessionStream(ctx context.Context, providerName, sessionID string, req base.ChatRequest) (<-chan streaming.StreamChunk, error)
func (c *Client) ChatSessionStreamSync(ctx context.Context, providerName, sessionID string, req base.ChatRequest) (base.ChatResponse, error)
```

- `ChatSession` 已标记为 Deprecated，推荐使用流式接口及其 Sync 封装

### 平台集成（流式）

```go
func (c *Client) ChatWithStream(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (<-chan streaming.StreamChunk, error)
func (c *Client) ChatWithStreamSync(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error)
```

### 1.4 连通性测试

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

**参数说明**：
- `providerName` - Provider 名称（来自配置文件）
- `cred` - 凭证对象
- `pc` - Provider 配置
- `opt` - 测试选项（TestOptions）

**返回值**：
- `TestResult` - 包含延迟和响应
- `error` - 测试失败时的错误

**TestOptions**：
```go
type TestOptions struct {
    Model     string        // 必填：要测试的模型名称
    Timeout   time.Duration // 可选：超时时间，默认 10s
    MaxTokens int           // 可选：最大 token 数，默认 1
    Prompt    string        // 可选：测试 prompt，默认 "1"
}
```

**TestResult**：
```go
type TestResult struct {
    Latency  time.Duration // 请求延迟
    Response ChatResponse  // 原始响应
}
```

**使用示例**：
```go
result, err := cli.Test(ctx, "openai", &client.TestOptions{
    Model:   "gpt-4o-mini",
    Timeout: 10 * time.Second,
})
```

## Provider API

### ProviderSpec 接口

```go
type ProviderSpec interface {
    Name() string
    DefaultBaseURL() string
    SupportedAuthTypes() []auth.AuthType
    BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error)
    ParseResponse(resp *http.Response) (base.ChatResponse, error)
    AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool)
}
```

### ProviderStreamSpec 接口

```go
type ProviderStreamSpec interface {
    ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error)
}
```

## Session API

多轮对话 API，管理会话历史和上下文。

### 3.1 核心方法

```go
// 非流式多轮对话
func (c *Client) ChatSessionStreamSync(
    ctx context.Context,
    providerName string,
    sessionID string,
    req base.ChatRequest,
) (base.ChatResponse, error)

// 流式多轮对话
func (c *Client) ChatSessionStream(
    ctx context.Context,
    providerName string,
    sessionID string,
    req base.ChatRequest,
) (<-chan streaming.StreamChunk, error)
```

**详细 API 说明**请参考：[Session API 参考](session-api.md)

### 3.2 SessionStore 接口

Session 存储接口定义，详见 [Session API 参考](session-api.md#sessionstore-接口)。

### 3.3 TruncatePolicy 接口

消息截断策略接口，详见 [Session API 参考](session-api.md#truncatepolicy-接口)。

## Auth API

### Manager

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
    ID          string
    Provider    string
    AuthType    AuthType
    APIKey      string
    AccessToken string
    RefreshToken string
    ExpiresAt   *time.Time
    Headers     map[string]string
    QueryParams map[string]string
    Priority    int
    Disabled    bool
    Metadata    map[string]any
}
```

### Strategy

```go
type AuthStrategy interface {
    Apply(req *http.Request) error
}
```

内置策略：`NoAuth` / `BearerTokenStrategy` / `ApiKeyHeaderStrategy` / `OAuthStrategy` / `JWTSignStrategy` / `CustomHeaderStrategy`。

## 相关文档
- [文档索引](README.md)
- [快速开始](quickstart.md)
- [配置指南](configuration.md)
- [使用指南](usage-guide.md)
- [Session API](session-api.md)
