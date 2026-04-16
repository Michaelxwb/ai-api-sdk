# Tasks: 统一 ResponseFormat 支持

- **Source**: .code-flow/tasks/2026-04-16/response-format.design.md
- **Created**: 2026-04-16
- **Updated**: 2026-04-16 (all tasks done)

## Proposal

为 SDK 的 `ChatRequest` 新增统一的 `ResponseFormat` 字段，让调用方通过一个入口控制模型输出格式（如强制 JSON），SDK 内部自动按 provider 差异映射为原生格式。解决调用方需要针对不同 provider 手动拼 ExtraBody 的问题，降低接入成本。Quick API 支持 session 级别设定，每次 Send 自动注入。

---

## TASK-001: 类型定义与 ChatRequest 字段

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: response-format.design.md#接口设计

### Description

新增 `ResponseFormat` 和 `JSONSchemaParam` 两个类型定义，在 `ChatRequest` 中新增 `*ResponseFormat` 指针字段（跟 Temperature/MaxTokens 同模式），并在 `provider/compat.go` 中新增 re-export 别名。这是所有后续任务的基础。

### Checklist
- [x] 在 `provider/base/types.go` 中定义 `ResponseFormat` 和 `JSONSchemaParam` 结构体
- [x] 在 `ChatRequest` 的 `MaxTokens` 和 `Stream` 之间新增 `ResponseFormat *ResponseFormat` 字段
- [x] 在 `provider/compat.go` 中新增 `type ResponseFormat = base.ResponseFormat` 和 `type JSONSchemaParam = base.JSONSchemaParam`

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)

---

## TASK-002: Quick API 注入

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: response-format.design.md#接口设计（Quick API 入参）

### Description

在 `client/quick.go` 的 `ProviderConfig` 中新增 `ResponseFormat` 字段，存入 `QuickSession`，在 `Send()` 构建 `ChatRequest` 时自动注入。跟 `Model`/`Stream` 的注入方式保持一致。

### Checklist
- [x] `ProviderConfig` 结构体新增 `ResponseFormat *base.ResponseFormat` 字段
- [x] `QuickSession` 结构体新增 `responseFormat *base.ResponseFormat` 字段
- [x] `Quick()` 构造 `QuickSession` 时传入 `cfg.ResponseFormat`
- [x] `Send()` 构建 `ChatRequest` 时设置 `ResponseFormat: qs.responseFormat`

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)

---

## TASK-003: OpenAI Compat 透传

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: response-format.design.md#技术方案（Provider 映射规则）

### Description

在 `provider/impls/openai/compat.go` 的 `BuildRequest` 中，ExtraBody merge 之后、`json.Marshal` 之前，将 `req.ResponseFormat` 直接透传为 `response_format` 字段。覆盖全部 7 个注册名（openai/deepseek/moonshot/dashscope/volcengine/qianfan/openai_compat）。

### Checklist
- [x] nil 检查后构建 `response_format` map，type 直接透传
- [x] JSONSchema 非 nil 时构建嵌套 `json_schema` 对象（含 name/schema/description/strict）
- [x] 放在 ExtraBody merge 之后，确保类型化字段优先于 ExtraBody 同名 key

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)

---

## TASK-004: Gemini 映射

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: response-format.design.md#技术方案（Provider 映射规则）

### Description

在 `provider/impls/gemini/spec.go` 的 `BuildRequest` 中，将 `ResponseFormat` 映射为 Gemini 原生的 `generationConfig` 格式。`json_object` → `responseMimeType: application/json`；`json_schema` 额外加 `responseSchema`。

### Checklist
- [x] `json_object` → 设置 `payload["generationConfig"]["responseMimeType"] = "application/json"`
- [x] `json_schema` → 同上 + 将 `JSONSchema.Schema` 设为 `responseSchema`
- [x] `text` 或未知类型 → switch default 不操作

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)

---

## TASK-005: Ollama 映射

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: response-format.design.md#技术方案（Provider 映射规则）

### Description

在 `provider/impls/ollama/spec.go` 的 `BuildRequest` 中，将 `ResponseFormat` 映射为 Ollama 原生的 `format` 字段。`json_object` → 字符串 `"json"`；`json_schema` → schema 对象。

### Checklist
- [x] `json_object` → `payload["format"] = "json"`（字符串）
- [x] `json_schema` + JSONSchema 非 nil → `payload["format"] = JSONSchema.Schema`（map）；JSONSchema 为 nil 时 fallback 到 `"json"`
- [x] `text` 或未知类型 → switch default 不操作

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)

---

## TASK-006: 测试覆盖

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003, TASK-004, TASK-005
- **Source**: response-format.design.md#验收标准

### Description

在 `test/provider_test.go` 中新增 `boolPtr` helper 和 11 个测试函数，覆盖所有验收标准：OpenAI compat（json_object/json_schema/nil/text）、Gemini（json_object/json_schema/nil）、Ollama（json_object/json_schema/nil）、Claude（忽略验证）。

### Checklist
- [x] 新增 `boolPtr(v bool) *bool` helper 函数
- [x] OpenAI compat 4 个测试：json_object 透传、json_schema 含完整参数、nil 无 key、text 透传
- [x] Gemini 3 个测试：json_object → responseMimeType、json_schema → responseSchema、nil 无 generationConfig
- [x] Ollama 3 个测试：json_object → format 字符串、json_schema → format map、nil 无 format key
- [x] Claude 1 个测试：设 ResponseFormat 后 payload 不含 response_format

### Log
- [2026-04-16] created (draft)
- [2026-04-16] started (in-progress)
- [2026-04-16] completed (done)
