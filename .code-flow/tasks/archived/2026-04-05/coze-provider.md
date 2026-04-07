# Tasks: Coze chat_v3 Provider

- **Source**: .claude/plans/sprightly-swimming-whistle.md
- **Created**: 2026-04-05
- **Updated**: 2026-04-05

## Proposal

接入 Coze (扣子) chat_v3 API 作为自定义协议 Provider，支持流式文本对话和 remote_session 多轮会话。Coze 使用非 OpenAI 兼容的私有协议，conversation_id 通过 URL query param 传递，SSE 事件类型丰富需手写解析。仅支持流式模式（非流式 API 需多次轮询，超出 ParseResponse 单次响应能力）。

---

## TASK-001: CozeSpec 核心结构 + BuildRequest + ParseResponse

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: plan#§4.1 创建 spec.go(L24-L43), plan#§2 关键协议特征(L7-L12), plan#§3 仅支持流式对话(L14-L20)

### Description

创建 `provider/impls/coze/spec.go`，实现 `ProviderSpec` 接口全部方法。

BuildRequest 关键映射：
- `req.Model` → `payload["bot_id"]`
- `req.Messages` 中最后一条 user 消息 → `payload["additional_messages"][0].content`
- `req.SessionID` → URL query param `?conversation_id=xxx`（不放 body）
- `opts.ExtraBody` → 透传 `user_id`/`custom_variables`/`meta_data` 等
- `payload["stream"]` 始终为 `true`
- `payload["user_id"]` 默认 `"sdk-user"`，可被 ExtraBody 覆盖

ParseResponse 返回明确错误：`"coze: non-streaming not supported, use ChatStream instead"`。

### Checklist

- [x] 定义 `CozeSpec struct{}` 并在 `init()` 中 `base.Register("coze", &CozeSpec{})`
- [x] `Name()` 返回 `"coze"`
- [x] `DefaultBaseURL()` 返回 `"https://api.coze.cn/v3"`
- [x] `SupportedAuthTypes()` 返回 `BearerToken, APIKey`
- [x] `AuthStrategyOverride`: APIKey → `BearerTokenStrategy`（同 Dify 模式）
- [x] `BuildRequest`: 构建 payload（bot_id/user_id/additional_messages/stream），conversation_id 放 URL query，合并 ExtraBody，设置 Content-Type 和 Accept headers
- [x] `ParseResponse`: 返回 `fmt.Errorf("coze: non-streaming not supported, use ChatStream instead")`
- [x] nil response 检查

### Log

- [2026-04-05] created (draft)
- [2026-04-05] started (in-progress)
- [2026-04-05] completed (done)

---

## TASK-002: ParseStreamResponse SSE 流式解析

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: plan#§4.2 创建 stream.go(L45-L64)

### Description

创建 `provider/impls/coze/stream.go`，手写 SSE 解析（Dify 模式），实现 `streaming.ProviderStreamSpec` 接口。

Coze SSE 使用 `event:` header 行标识事件类型，事件路由：

| SSE Event | 处理 |
|-----------|------|
| `conversation.chat.created` | 提取 `conversation_id` → SessionID（首轮关键） |
| `conversation.chat.in_progress` | 跳过 |
| `conversation.message.delta` (type=answer) | 提取 `content` → Text |
| `conversation.message.delta` (type!=answer) | 跳过（verbose_log/function_call 等） |
| `conversation.message.completed` | 跳过（已从 delta 累积） |
| `conversation.chat.completed` | 提取 usage → Done chunk |
| `conversation.chat.failed` | 提取 `last_error.msg` → Error chunk |
| `done` | 终止信号 |

Usage 映射：`input_count` → PromptTokens, `output_count` → CompletionTokens, `token_count` → TotalTokens。

### Checklist

- [x] 定义 `cozeEventData` 结构体（统一 SSE 事件 JSON 解析）
- [x] 实现 `ParseStreamResponse` 方法，goroutine 内 bufio.Reader 逐行读取
- [x] 实现 `handleCozeLine` — 解析 SSE 行（event:/data:/空行/注释），跟踪 currentEvent 和 dataLines
- [x] 实现 `handleCozeEvent` — 事件路由分发，8 种事件类型处理
- [x] conversation_id 通过指针跨 chunk 传递（从 chat.created 提取，后续 chunk 携带）
- [x] `sendStreamChunk` + `safeString` 辅助函数（复用 Dify 模式）
- [x] `var _ streaming.ProviderStreamSpec = (*CozeSpec)(nil)` 接口断言

### Log

- [2026-04-05] created (draft)
- [2026-04-05] started (in-progress)
- [2026-04-05] completed (done)

---

## TASK-003: SDK 注册集成

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: plan#§4.3 修改 provider.go(L66-L71), plan#§4.4 修改 conversation.go(L73-L78)

### Description

将 Coze Provider 接入 SDK 注册体系和会话模式推断。

### Checklist

- [x] `provider/provider.go` 添加 `_ "github.com/Michaelxwb/ai-api-sdk/provider/impls/coze"` 空白导入
- [x] `client/conversation.go` 的 `ResolveConversationMode` switch 中，将 `"coze"` 加入 `remote_session` 分支：`case "dify", "ragflow", "qianfan_app", "coze":`

### Log

- [2026-04-05] created (draft)
- [2026-04-05] started (in-progress)
- [2026-04-05] completed (done)

---

## TASK-004: BuildRequest 单元测试

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001, TASK-003
- **Source**: plan#§4.5 BuildRequest 测试(L82-L90)

### Description

在 `test/coze_provider_test.go` 中编写 `TestCozeSpec_BuildRequest` 测试套件，复用 `mustGetSpec`、`decodeBodyMap` 等现有 test helpers。

### Checklist

- [x] `normal_streaming` — 验证 URL 为 `{baseURL}/chat`，body 含 bot_id/user_id/additional_messages/stream
- [x] `conversation_id_in_url_query` — SessionID="conv-123" 时，URL 含 `?conversation_id=conv-123`，body 无 conversation_id
- [x] `no_conversation_id_when_empty` — SessionID 为空时，URL 无 conversation_id query param
- [x] `bot_id_from_model` — Model="bot-abc" 映射为 body 中 `bot_id: "bot-abc"`
- [x] `user_id_default` — 无 ExtraBody 时默认 `"sdk-user"`
- [x] `user_id_override_via_extra_body` — ExtraBody 中 user_id 覆盖默认值
- [x] `stream_always_true` — req.Stream=false 时 body 仍有 `"stream": true`
- [x] `extra_body_passthrough` — custom_variables/meta_data 等 ExtraBody 字段透传

### Log

- [2026-04-05] created (draft)

---

## TASK-005: ParseStreamResponse + ParseResponse 测试

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-002, TASK-003
- **Source**: plan#§4.5 ParseResponse 测试(L92-L94), plan#§4.5 ParseStreamResponse 测试(L96-L104)

### Description

在 `test/coze_provider_test.go` 中编写流式解析和非流式错误返回的测试套件，复用 `collectChunks` 等现有 test helpers。

### Checklist

- [x] `TestCozeSpec_ParseResponse_not_supported` — 验证返回包含 "non-streaming not supported" 的错误
- [x] `TestCozeSpec_ParseResponse_nil` — nil response 返回错误
- [x] `TestCozeSpec_ParseStreamResponse_normal_delta_and_done` — 多个 message.delta(type=answer) + chat.completed + done，验证文本、usage、done
- [x] `TestCozeSpec_ParseStreamResponse_conversation_id_carried` — chat.created 中的 conversation_id 出现在后续所有 chunk 的 SessionID
- [x] `TestCozeSpec_ParseStreamResponse_only_answer_emits_text` — delta type=verbose_log 不产生文本，type=answer 正常产生
- [x] `TestCozeSpec_ParseStreamResponse_error_event` — chat.failed 携带 last_error，验证 error chunk
- [x] `TestCozeSpec_ParseStreamResponse_usage_mapping` — chat.completed 中 token_count/output_count/input_count 正确映射为 TotalTokens/CompletionTokens/PromptTokens
- [x] `TestCozeSpec_ParseStreamResponse_invalid_json` — 畸形 JSON data 行产生 error chunk
- [x] `TestCozeSpec_ParseStreamResponse_sse_comment_ignored` — 以 `:` 开头的行被忽略
- [x] `TestCozeSpec_ParseStreamResponse_nil` — nil response 返回错误

### Log

- [2026-04-05] created (draft)

---

## TASK-006: 全量测试验证

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-004, TASK-005
- **Source**: plan#§4.6 运行测试(L106-L110), plan#§8 Quick API 兼容性(L136-L140)

### Description

运行全量测试确保 Coze Provider 集成无回归，验证 Quick API 兼容性。

### Checklist

- [x] `go test ./test/... -run TestCoze -v` 全部通过（21/21）
- [x] `go test ./test/... -run TestQuickValidation -v` 无回归（9/9）
- [x] 确认 Quick API 最简调用 `ProviderConfig{Provider: "coze", APIKey: "pat-xxx", Model: "bot-xxx"}` 不报错

### Log

- [2026-04-05] created (draft)
