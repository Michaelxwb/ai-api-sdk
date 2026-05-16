# Backend Platform Rules

## Rules

- 对外 API 变更必须向后兼容：优先新增 `SessionOption`/扩展 interface，不破坏既有默认行为（尤其 `Session` legacy 路径与 `HistoryAuto`）。
- 新 Provider 必须实现 `base.ProviderSpec`；支持流式时再实现 `streaming.ProviderStreamSpec`。
- Provider 必须在 `init()` 注册，并在 `provider/provider.go` 加入空白导入，保证统一发现机制。
- Quick API 与底层 Session API 的语义必须一致：会话模式、超时、错误策略不得相互矛盾。
- `remote_session`（session_id 注入 provider 侧）与 `local_history`（SDK 本地维护 history）语义必须严格区分。
- Generic 适配器推断状态（`auto_confirmed` / `pending_confirm` / `failed`）必须稳定向后兼容。
- 多模态消息双路径互斥：`base.Message.Content` 与 `base.Message.Parts` 不可同时生效。`len(Parts)==0` 走 `Content`（向后兼容纯文本）；`len(Parts)>0` 走 `Parts`，忽略 `Content`。所有 provider 的 `convertMessages*` 必须先判 `len(Parts)` 决定路径，禁止两者拼接。
- B 组（文件上传）provider 在 `BuildRequest` 中需要 API Key 单独发起上传请求，必须从 `opts.Credential.APIKey` 取；为 nil 或空时必须按 `<name>: API key required for file upload` 报错并提前返回，不得继续构造主请求。

## Patterns

### Provider 类型决策树

```
新 Provider 是否兼容 OpenAI Chat Completions 格式？
  ├─ 是 → 在 openai/spec.go init() 加一行：
  │       base.Register("<name>", NewOpenAICompatSpec("<name>", "<base-url>"))
  └─ 否 → 创建 provider/impls/<name>/ 目录：
          spec.go   → 实现 base.ProviderSpec
          stream.go → 实现 streaming.ProviderStreamSpec（若支持流式）
          func init() { base.Register("<name>", &MySpec{}) }
          provider/provider.go → import _ ".../provider/impls/<name>"
```

### 会话模式选择（基于 `client/conversation.go:ResolveConversationMode`）

```
Provider 的默认 ConversationMode：
  local_history（SDK 本地维护）：openai / claude / gemini / ollama / deepseek / moonshot / dashscope / volcengine / qianfan / openai_compat / bailian_app
  remote_session（provider 侧维护）：dify / ragflow / qianfan_app
  ""（必须调用方显式指定）：fastgpt / generic / plugin 等

如何选择？
  └─ provider 侧管理会话 ID（conversation_id/chat_id）→ ConversationModeRemoteSession
  └─ SDK 本地追加 history messages → ConversationModeLocalHistory
  └─ 不确定 → 看 ResolveConversationMode() 返回值；返回 "" 则必须显式传 WithConversationMode()
```

仅单轮，不需要历史：
```go
WithHistoryMode(HistoryNone)  // 不加载历史，但保存仍取决于是否配置 store
```

### 认证统一注入
所有 HTTP 认证通过 `AuthTransport`（`client/transport.go`）统一注入，Provider 实现中禁止重复处理认证：
```go
// AuthTransport.RoundTrip() 调用 cred.Strategy.Apply(req)
// 已实现策略：APIKey/Bearer/OAuth/JWT/CustomHeader/CustomQuery
// 已定义常量但无策略实现：AuthTypeBasic、AuthTypeMTLS（fall-through 到 NoAuth）
```

**注意**：`AuthTransport.Manager` 字段在两处构造代码（`client.go:chatWith`、`stream.go:chatWithStream`）中**均未设置**，因此 OAuth 401 自动重试逻辑（`transport.go:38`）实际上是死代码。如需 OAuth 刷新，需手动调用 `Manager.RefreshOAuth()`。

### 错误可提取结构（平台层统一治理）
```go
var ae *client.APIError
if errors.As(err, &ae) {
    switch ae.StatusCode {
    case 429: // rate limit
    case 401: // auth failure
    }
}
```

### Functional Options 模式
```go
type SessionOption func(*Session)

func WithStore(store session.SessionStore) SessionOption { ... }
func WithTimeout(d time.Duration) SessionOption { ... }
func WithHistoryWindow(n int) SessionOption { ... }
func WithHistoryMode(mode HistoryMode) SessionOption { ... }
func WithConversationMode(mode ConversationMode) SessionOption { ... }
```

### ChainValues 工具调用消息转换（隐含行为）
当上一轮响应的 `ChainValues` 中包含 `$$$TOOL_CALL_ID$$$` 时，`local_history` 和 `remote_session` 模式下会自动将当前轮最后一条 `user` 消息的 Role 改为 `"tool"` 并设置 `ToolCallID`（用于 function call 续轮场景）。这是 Generic adapter 的特殊约定，普通 provider 无需关心。

### Legacy 模式（conversationMode == ""）
当 `WithConversationMode()` 未设置时走 `chatLegacy()`：**同时**注入 session_id 且加载本地历史——这是老版本行为，不推荐，会导致 session_id 在 local_history provider 中被无意注入。新代码应显式设置 ConversationMode。

### Quick() 校验语义
`Quick()` 返回 `(*QuickSession, error)`，在创建会话前执行两条校验：

```go
// 规则 1：provider 无内置 BaseURL 时，调用方必须显式提供
// 覆盖：fastgpt / ragflow / generic / plugin
if spec.DefaultBaseURL() == "" && strings.TrimSpace(cfg.BaseURL) == "" {
    return nil, fmt.Errorf("client: %s requires BaseURL", cfg.Provider)
}

// 规则 2：provider 支持多种会话模式时，调用方必须显式指定
// 覆盖：fastgpt / generic / plugin（ResolveConversationMode 返回 ""）
if ResolveConversationMode(cfg.Provider) == "" && cfg.SessionMode == "" {
    return nil, fmt.Errorf("client: %s requires explicit SessionMode (\"local_history\" or \"remote_session\")", cfg.Provider)
}
```

规则由现有接口推导，无需修改任何 provider 实现；未来新增 provider 只要正确实现 `DefaultBaseURL()` 和在 `ResolveConversationMode` 中注册，自动受保护。

### 模式推断集中化
默认会话模式推断集中在 `ResolveConversationMode`，避免在多个入口重复散落判断逻辑。

### 多模态 Provider 能力三组分类

新增带视觉能力的 provider 时，先按下表归类，再按对应模式实现：

```
A 组（base64 内联）：原生接受 data:image/*;base64,...
  代表：openai / openai_compat / fastgpt / ollama / bailian_app
  实现：在 convertMessages*() 中将 ContentPart.Data + MIMEType 拼为 data URI；
        Ollama 例外，content 仍为字符串，base64 走单独 images 数组。

B 组（先上传再引用 file_id）：图片走独立 multipart 上传换 file_id
  代表：dify / coze / qianfan_app / moonshot
  实现：provider/impls/<name>/upload.go 提供 uploadImages / uploadSingleImage；
        BuildRequest 中读取 opts.Credential.APIKey 调用上传，把返回的 file_id
        组装进 payload（Dify=files[]、Coze=object_string、Qianfan=file_ids、Moonshot=ms://）。

C 组（不支持图片）：协议本身无法承载图片输入
  代表：generic / ragflow / deepseek
  实现：BuildRequest 入口遍历 req.Messages，发现 Type=="image_url" 立即按
        "<name>: <reason>, only text models supported" 形式返回错误，不再继续。
```

## Anti-Patterns

- 在 provider 实现中加入 client 层状态管理逻辑（反向耦合）。
- 新增平台能力时绕开注册中心做硬编码分支（如在 client.go 里 `if provider == "xxx"`）。
- 为"快速支持某平台"破坏通用 `ChatRequest/ChatResponse` 统一模型。
- Generic 推断失败时静默降级到不确定行为；应返回明确错误与回退建议。
- 让 `examples/` 目录代码成为生产依赖路径。
- 直接修改 Provider registry 中已注册的 spec 对象（race condition 风险）。
