# 目录结构

> 本 Go SDK 项目中后端代码的组织方式。

---

## 概览

这是一个用于统一多提供商 AI 模型访问的 **Go SDK library** (`github.com/Michaelxwb/ai-api-sdk`)。项目遵循 Go 的扁平包约定，清晰区分 core interfaces、实现与示例。

**关键原则**：SDK core 定义 interfaces 和最小实现。较重的实现（存储驱动、平台服务器）放在 `examples/` 里作为参考代码，而不是可导入的包。

---

## 目录布局

```
ai-api-sdk/
├── auth/                    # Authentication management
│   ├── credential.go        # Credential type, AuthType constants
│   ├── manager.go           # Manager: credential selection, refresh, cooldown
│   ├── selector.go          # Selector interface + RoundRobin/Priority impls
│   ├── store.go             # CredentialStore interface + FileStore (AES-256-GCM)
│   └── strategy.go          # AuthStrategy interface + all strategy impls
├── client/                  # Unified API client
│   ├── client.go            # Client struct, NewClient(), chatWith()
│   ├── session.go           # Session struct, Chat(), ChatStream(), options
│   ├── session_config.go    # SessionConfig (truncation, conflict retry)
│   ├── prepare.go           # Request preparation, credential resolution
│   ├── stream.go            # Stream helpers: chatWithStream(), collectStream()
│   ├── test.go              # Connectivity test: Test(), TestWith()
│   └── transport.go         # AuthTransport: HTTP RoundTripper with auth injection
├── config/                  # Configuration loading
│   ├── config.go            # Config struct, ProviderConfig, AuthConfig
│   └── loader.go            # LoadConfig() from YAML
├── provider/                # Provider abstraction layer
│   ├── base/                # Core interfaces (imported by all)
│   │   ├── types.go         # Message, ChatRequest, ChatResponse, Usage
│   │   ├── spec.go          # ProviderSpec interface
│   │   ├── transport.go     # ProviderTransportSpec interface
│   │   └── registry.go      # Global provider registry: Register(), Get(), List()
│   ├── streaming/           # Streaming infrastructure
│   │   ├── spec.go          # ProviderStreamSpec interface
│   │   ├── types.go         # StreamChunk, StreamProtocol
│   │   ├── config.go        # StreamConfig
│   │   ├── sse.go           # SSE parser
│   │   ├── ndjson.go        # NDJSON parser
│   │   ├── json_path.go     # JSON path extraction utility
│   │   └── utils.go         # Streaming utilities
│   ├── impls/               # Provider implementations (auto-registered via init())
│   │   ├── openai/          # OpenAI + compatible providers (moonshot, deepseek, dashscope, volcengine)
│   │   ├── claude/          # Anthropic Claude
│   │   ├── gemini/          # Google Gemini
│   │   ├── ollama/          # Ollama local models
│   │   ├── dify/            # Dify platform
│   │   └── plugin/          # Browser plugin provider
│   ├── plugin/              # Plugin WebSocket client
│   │   ├── client.go        # WebSocket Client
│   │   ├── config.go        # Plugin Config + validation
│   │   ├── types.go         # MessageType, Message, ElementLocator
│   │   ├── session.go       # Plugin Session adapter
│   │   └── transport.go     # Plugin transport layer
│   ├── provider.go          # Blank import aggregator (registers all providers)
│   └── compat.go            # Compatibility helpers
├── session/                 # Session management (interfaces only)
│   ├── store.go             # SessionStore interface + sentinel errors
│   ├── types.go             # SessionState, SessionMeta, GetOptions, Message alias
│   └── truncate.go          # TruncatePolicy interface + WindowPolicy
├── tools/                   # Development tools
│   └── config-generator/    # YAML config generator CLI
├── examples/                # Reference implementations (NOT imported by SDK)
│   ├── sessionstore/        # SessionStore implementations (Memory/File/SQLite/MySQL/PG/Redis)
│   ├── plugin-platform/     # Plugin platform HTTP+WS server
│   ├── 01-single-turn/      # Single-turn chat examples
│   ├── 02-multi-turn/       # Multi-turn chat with persistence
│   ├── 03-platform-integration/  # Platform integration
│   ├── 04-connectivity-test/     # Provider connectivity test
│   ├── 05-browser-plugin/        # Browser plugin usage
│   └── dify/                # Dify-specific examples
├── docs/                    # Documentation
├── go.mod
├── go.sum
└── README.md
```

---

## 模块组织

### 依赖流向（严格、无环）

```
provider/base  <──  provider/streaming
     ↑                    ↑
     │                    │
auth ←── config      provider/impls/*
     ↑       ↑            ↑
     │       │            │
     └── client ──────────┘
              ↑
              │
         session (interfaces only)
```

**规则**：
- `provider/base` 和 `session` 是叶子包 —— 它们不从本模块中导入任何内容
- `auth` 只导入标准库 + `golang.org/x/crypto`
- `config` 导入 `auth`（用于 `Credential` 类型）
- `client` 导入 `auth`、`config`、`provider/base`、`provider/streaming`、`session`
- `provider/impls/*` 导入 `provider/base`、`provider/streaming`、`auth`
- `examples/` 可以导入任何内容

### 新增 Provider

#### 方式一：OpenAI 兼容 Provider（推荐，若 API 兼容 OpenAI Chat Completions 格式）

仅需在 `provider/impls/openai/spec.go` 的 `init()` 中添加一行注册：

```go
base.Register("<name>", NewOpenAICompatSpec("<name>", "<base-url>"))
```

已通过此方式接入的 Provider：`openai`、`moonshot`、`deepseek`、`dashscope`、`volcengine`。

判断标准：
- 端点兼容 `/chat/completions` 路径
- 请求/响应格式兼容 OpenAI（`choices.0.message.content` / `choices.0.delta.content`）
- 流式使用 SSE 协议，结束标记为 `[DONE]`
- 认证使用 `Authorization: Bearer <token>`

#### 方式二：自定义 Provider（API 格式不兼容 OpenAI）

1. 创建 `provider/impls/<name>/` 目录
2. 添加 `spec.go` 实现 `base.ProviderSpec` interface
3. 添加 `stream.go` 实现 `streaming.ProviderStreamSpec`（若支持 streaming）
4. 通过 `init()` 注册：`base.Register("<name>", &MySpec{})`
5. 在 `provider/provider.go` 中添加空导入

### 新增 Session Store

Session stores 不属于 SDK core。请在你自己的包中实现 `session.SessionStore` interface。参考实现见 `examples/sessionstore/`。

---

## 命名规范

### 文件

| 模式 | 用途 |
|---------|-------|
| `spec.go` | Provider specification / interface 实现 |
| `types.go` | 类型定义与常量 |
| `stream.go` | 与 streaming 相关的逻辑 |
| `config.go` | 配置类型与校验 |
| `client.go` | Client struct 与核心方法 |
| `store.go` | 存储 interface 与实现 |
| `helpers.go` | 共享工具函数 |

### 类型与函数

- **Interfaces**：基于名词，例如 `ProviderSpec`、`SessionStore`、`AuthStrategy`、`Selector`
- **Structs**：描述性命名，例如 `Client`、`Session`、`Manager`、`StreamChunk`
- **Constructors**：`New<Type>()` 模式，例如 `NewClient()`、`NewMemoryStore()`
- **Option functions**：`With<Option>()` 模式，例如 `WithStore()`、`WithID()`、`WithAutoID()`
- **Constants**：对外导出使用 `CamelCase`，按类型分组，例如 `AuthTypeAPIKey`、`MsgReplyChunk`
- **Sentinel errors**：`Err<Domain><Issue>` 模式，例如 `ErrSessionNotFound`、`ErrSessionConflict`

### 包

- 尽量使用小写、单词：`auth`、`client`、`config`、`session`
- 多词使用扁平结构：`sessionstore`（不是 `session_store`）
- 实现包嵌套在父级之下：`provider/impls/openai`

---

## 示例

### 组织良好的模块：`auth/`

- 清晰分层：类型（`credential.go`）、逻辑（`manager.go`）、interfaces（`store.go`、`selector.go`）、策略（`strategy.go`）
- 每个文件单一职责
- 当足够简单时，interface 与实现放在同一包内

### 组织良好的模块：`provider/`

- `base/` 用于共享 interfaces（所有模块都导入）
- `streaming/` 用于 streaming 基础设施
- `impls/` 用于具体实现
- 通过 `init()` 自动注册 + 在 `provider.go` 中空导入
