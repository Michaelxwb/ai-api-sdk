# Backend Retrieval Map

> AI 导航地图：快速定位 Go SDK 结构与关键链路，在修改代码前读此文件。

## Purpose

`ai-api-sdk` 是统一多 AI 平台接入的 Go SDK（库，非服务端应用）：
- 统一认证（API Key / Bearer / OAuth / JWTSign / 自定义 Header+Query）
- 统一请求与响应结构（`provider/base`）
- 流式优先（SSE / NDJSON）
- 本地历史与远端会话双模式（`local_history` / `remote_session`）
- 可插拔会话存储（接口 `session/`，实现示例 `examples/sessionstore/`）

## Architecture

- Language: Go `1.23`
- Core packages: `auth`, `client`, `config`, `provider`, `session`
- Streaming infra: `provider/streaming`
- Provider registry: `provider/base/registry.go` + `provider/provider.go` 空白导入
- Session persistence contract: `session.SessionStore`
- Tests: `test/`（外部包黑盒测试）

## Key Files

| File | Purpose |
|------|---------|
| `client/client.go` | SDK 主入口：`New()`、`NewSession*()`、聊天主链路 |
| `client/session.go` | 多轮会话核心：`remote_session` / `local_history` / legacy 行为 |
| `client/quick.go` | Quick API（扁平参数 + 自动模式推断） |
| `client/errors.go` | `APIError` / `ParseError` / `AuthError` + 错误体截断（4KB） |
| `client/transport.go` | `AuthTransport`：HTTP RoundTripper + auth 注入（Manager 字段未接入，OAuth 401 重试为死代码） |
| `session/store.go` | `SessionStore` 契约 + sentinel errors + 内置 memory store |
| `session/truncate.go` | 历史窗口裁剪（消息数 + token 预算） |
| `provider/base/spec.go` | `ProviderSpec` 抽象接口 |
| `provider/base/types.go` | 统一 `ChatRequest` / `ChatResponse` / `Message` |
| `provider/base/registry.go` | Provider 注册表（`Register` / `Get` / `List`） |
| `provider/provider.go` | Provider 聚合注册入口（空白导入触发 init） |
| `provider/impls/generic/inference.go` | Generic 自动推断：`InferIntegration*` 函数 |
| `provider/impls/generic/template.go` | Generic 模板适配器（自定义请求/响应映射） |
| `provider/impls/generic/raw.go` | Generic 原始 HTTP 适配器 |
| `provider/impls/generic/profile.go` | Generic profile 管理 |
| `auth/manager.go` | 凭证选择、冷却、OAuth 刷新 |
| `auth/store.go` | 凭证持久化（可选 AES-256-GCM） |
| `examples/sessionstore/*` | SessionStore Memory/File/SQLite/MySQL/PG/Redis 参考实现 |

## Module Map

```text
ai-api-sdk/
├── auth/                     # Credential / Strategy / Manager / Store
├── client/                   # Session API / Quick API / transport / test bridge
├── config/                   # YAML 配置结构与加载
├── provider/
│   ├── base/                 # Provider 抽象与注册表
│   ├── streaming/            # SSE / NDJSON / json_path
│   ├── impls/
│   │   ├── openai/           # OpenAI + 兼容（moonshot/deepseek/dashscope/volcengine）
│   │   ├── claude/           # Anthropic Claude
│   │   ├── gemini/           # Google Gemini
│   │   ├── ollama/           # Ollama 本地模型
│   │   ├── fastgpt/          # FastGPT（自定义协议，非 openai compat）
│   │   ├── ragflow/          # RAGFlow（自定义协议，remote_session 模式）
│   │   ├── dify/             # Dify 平台（remote_session 模式）
│   │   ├── generic/          # 通用适配器（template + raw + 多轮自动推断）
│   │   └── plugin/           # 浏览器插件 provider
│   └── plugin/               # 浏览器插件 WS 客户端
├── session/                  # SessionStore 接口、状态模型、truncate
├── examples/                 # 参考实现（sessionstore demo + 提供商示例）
└── test/                     # SDK 黑盒测试
```

## Data Flow

### 1. 标准配置模式（`NewClient` + `NewSession`）

```text
Config/AuthMgr → client.NewClient → Session.Chat/ChatStream
→ resolveChatInputs → ProviderSpec.BuildRequest
→ AuthTransport 注入鉴权 → HTTP 请求
→ ProviderSpec.ParseResponse / ParseStreamResponse
→ (可选) SessionStore.Save
```

### 2. Quick 模式（`client.New().Quick(...)`）

```text
ProviderConfig(扁平参数)
→ Quick() 组装 Credential + ProviderConfig + SessionOption
→ 自动推断 ConversationMode
→ QuickSession.Send/SendText
→ Session.Chat/ChatStream
```

### 3. Generic 自动推断模式

```text
RawHTTPMultiRoundSpec / MultiRoundSpec
→ generic.InferIntegration*()
→ client.NewSessionFrom*()
→ 动态注册 generic spec
→ 正常 Session 链路
```

## Navigation Guide

| 任务 | 看哪里 |
|------|--------|
| 新增 Provider（OpenAI 兼容） | `provider/impls/openai/spec.go` init()，加一行 `Register` |
| 新增 Provider（自定义协议） | `provider/impls/<name>/spec.go` + `stream.go` + `init()` + `provider/provider.go` 空白导入 |
| 新增会话行为 | `client/session.go` 模式分支，避免破坏 legacy `HistoryAuto` 行为 |
| 新增存储能力 | 保持 `session.SessionStore` 最小接口；扩展走独立可选 interface |
| 调整错误模型 | `client/errors.go` + 调用链 `%w` 包装，保持 `errors.Is/As` 可用 |
| 排查多轮问题 | `client/session.go`、`provider/impls/generic/*`、`test/conversation_test.go` |
| 调整认证逻辑 | `auth/manager.go`、`client/transport.go`（OAuth 401 重试逻辑） |
| 排查流式问题 | `provider/streaming/`、具体 provider `stream.go`、`test/streaming_test.go` |
