# Tasks: qianfan_app Provider

- **Source**: .claude/plans/humming-munching-toast.md
- **Created**: 2026-04-02
- **Updated**: 2026-04-04

## Proposal

接入百度千帆应用层 API（`/v2/app/conversation/runs`），新增 `qianfan_app` provider。该 API 与已有的 `qianfan` 基模 provider（OpenAI 兼容协议）不同：请求体为 `app_id` + `query` 格式，会话由服务端通过 `conversation_id` 管理，属于 `remote_session` 模式。首轮不传 `conversation_id` 时服务端自动创建，无需调用独立的新建会话接口。

---

## TASK-001: Provider 核心实现 (spec.go)

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: .claude/plans/humming-munching-toast.md#§Implementation.1(L22-L44)

### Description

新建 `provider/impls/qianfan_app/spec.go`，实现 `base.ProviderSpec` 接口。

- `BuildRequest`: POST `/v2/app/conversation/runs`
  - `app_id` 从 `req.Model` 取（跳过 `"test"` 占位符）
  - `query` 从最后一条 role=user 的 message 提取
  - `conversation_id` 从 `req.SessionID` 注入（仅非空时）
  - `stream` 从 `req.Stream`
  - 合并 `opts.ExtraBody`（`end_user_id`、`file_ids` 等）
- `ParseResponse`: 解析 `answer`→Text、`conversation_id`→SessionID、`usage`→Usage
- `AuthStrategyOverride`: APIKey → BearerToken
- 默认 BaseURL: `https://qianfan.baidubce.com/v2/app/conversation/runs`
- path: 空（URL 不含动态段，app_id 在请求体里，无需拆分 path）

### Checklist

- [x] 定义 `QianfanAppSpec` struct（name/defaultBaseURL）
- [x] `init()` 注册 `base.Register("qianfan_app", ...)`
- [x] `BuildRequest` 构造请求体（app_id/query/conversation_id/stream/ExtraBody）
- [x] `ParseResponse` 解析 answer + conversation_id + usage
- [x] `AuthStrategyOverride` APIKey→Bearer 映射
- [x] URL 拼接内联（path 为空，无需 joinEndpoint）
- [x] 错误前缀统一为 `"qianfan_app: ..."`

### Log

- [2026-04-02] created (draft)
- [2026-04-02] started (in-progress)
- [2026-04-02] completed (done)

---

## TASK-002: 流式解析实现 (stream.go)

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: .claude/plans/humming-munching-toast.md#§Implementation.2(L46-L65)

### Description

新建 `provider/impls/qianfan_app/stream.go`，实现 `streaming.ProviderStreamSpec`。

千帆 SSE 格式：
```
data: {"answer":"增量文本","conversation_id":"xxx","is_completion":false}
data: {"answer":"最后一段","conversation_id":"xxx","is_completion":true,"usage":{...}}
```

用 SSEParser + 自定义 extractor 解析 `answer` 增量文本，wrap channel 提取 `conversation_id`→`StreamChunk.SessionID` 和 `usage`→`StreamChunk.Usage`。

### Checklist

- [x] 定义 `StreamConfig`（Protocol=SSE，DeltaPaths=["answer"]，DonePath="is_completion"，DoneValue="true"）
- [x] `ParseStreamResponse` 用 SSEParser 解析
- [x] Wrap channel：从每帧 Raw 提取 `conversation_id`→SessionID
- [x] Wrap channel：从 done 帧提取 `usage`→Usage
- [x] 编译期断言 `var _ streaming.ProviderStreamSpec = (*QianfanAppSpec)(nil)`

### Log

- [2026-04-02] created (draft)
- [2026-04-02] started (in-progress)
- [2026-04-02] completed (done)

---

## TASK-003: Provider 注册与会话模式配置

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: .claude/plans/humming-munching-toast.md#§Implementation.3-4(L67-L80)

### Description

将 `qianfan_app` 注册到全局 provider 列表，并配置会话模式为 `remote_session`。

### Checklist

- [x] `provider/provider.go`: 添加 `_ "github.com/Michaelxwb/ai-api-sdk/provider/impls/qianfan_app"` blank import
- [x] `client/conversation.go`: `ResolveConversationMode` 的 `remote_session` case 加入 `"qianfan_app"`

### Log

- [2026-04-02] created (draft)
- [2026-04-02] started (in-progress)
- [2026-04-02] completed (done)

---

## TASK-004: 测试覆盖

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: .claude/plans/humming-munching-toast.md#§Tests(L82-L99)

### Description

为 qianfan_app provider 补齐单元测试和集成测试，覆盖请求构造、响应解析、流式解析、鉴权、Quick API 多轮会话。

### Checklist

- [x] `test/provider_test.go`: `TestQianfanAppSpec_BuildRequest`（URL/app_id/query/conversation_id/stream/ExtraBody）
- [x] `test/provider_test.go`: `TestQianfanAppSpec_BuildRequest_SkipsTestModel`（model="test" 时不传 app_id）
- [x] `test/provider_test.go`: `TestQianfanAppSpec_ParseResponse`（answer+conversation_id+usage、invalid JSON、nil）
- [x] `test/provider_test.go`: `TestQianfanAppSpec_ParseStreamResponse`（增量文本+conversation_id+usage+done、nil）
- [x] `test/provider_test.go`: `TestQianfanAppSpec_AuthStrategyOverride`（APIKey→Bearer）
- [x] `test/conversation_test.go`: 追加 qianfan_app→remote_session + stream=true 断言
- [x] `test/quick_validation_test.go`: qianfan_app 有默认 BaseURL 可省略校验
- [x] `test/client_test.go`: `TestQianfanApp_QuickMultiTurn`（httptest stub 验证 conversation_id 第二轮回传）

### Log

- [2026-04-02] created (draft)
- [2026-04-04] started (in-progress)
- [2026-04-04] completed (done)

---

## TASK-005: 文档更新

- **Status**: done
- **Priority**: P2
- **Depends**: TASK-003
- **Source**: .claude/plans/humming-munching-toast.md#§文档(L101-L104)

### Description

更新用户面向文档，在应用层接入表格中增加 qianfan_app 条目，并提供配置示例。

### Checklist

- [x] `README.md`: 应用层接入表格增加 `qianfan_app` 行（Provider/说明/必填参数/可选参数/默认BaseURL/SessionMode）
- [x] `examples/config.example.yaml`: 增加 qianfan_app provider + credential 配置示例

### Log

- [2026-04-02] created (draft)
- [2026-04-04] started (in-progress)
- [2026-04-04] completed (done)
