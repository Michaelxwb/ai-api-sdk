# 设计简报：统一 ResponseFormat 支持

## 目标
- 为 SDK 统一 `ChatRequest` 新增 `ResponseFormat` 字段，允许调用方控制模型输出格式（如强制 JSON）
- SDK 内部自动将统一参数映射为各 provider 原生格式，调用方无需感知差异
- Quick API 支持 session 级别设定，每次 Send 自动注入

## 非目标
- 不对不支持的 provider（Claude、Dify、Coze、FastGPT 等平台类）报错或警告，静默忽略
- 不做 ResponseFormat 值的合法性校验（如 json_schema 缺少 Schema），由目标 API 自行校验
- 不修改 Generic provider（调用方可通过 ExtraBody 自行注入）

## 接口设计

### 新增类型（`provider/base/types.go`）

```go
type ResponseFormat struct {
    Type       string           `json:"type"`                  // "text" | "json_object" | "json_schema"
    JSONSchema *JSONSchemaParam `json:"json_schema,omitempty"`
}

type JSONSchemaParam struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    Schema      map[string]any `json:"schema"`
    Strict      *bool          `json:"strict,omitempty"`
}
```

### ChatRequest 新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `ResponseFormat` | `*ResponseFormat` | 指针，nil 表示不设定（默认行为）。跟 Temperature/MaxTokens 同模式 |

### Quick API 入参（`client/quick.go` ProviderConfig）

| 字段 | 类型 | 说明 |
|------|------|------|
| `ResponseFormat` | `*base.ResponseFormat` | session 级别设定，Send() 每次自动注入到 ChatRequest |

### 调用方示例

```go
// Quick 路径 — json_object
qs, _ := client.Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4o",
    ResponseFormat: &base.ResponseFormat{Type: "json_object"},
})
ch, _ := qs.SendText(ctx, "返回 JSON 格式的研判结果")

// Quick 路径 — json_schema（Structured Outputs）
qs, _ := client.Quick(client.ProviderConfig{
    Provider: "openai",
    Model:    "gpt-4o",
    ResponseFormat: &base.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &base.JSONSchemaParam{
            Name: "judge_result",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "risk_level": map[string]any{"type": "string"},
                    "reason":     map[string]any{"type": "string"},
                },
                "required": []string{"risk_level", "reason"},
            },
            Strict: ptrBool(true),
        },
    },
})

// Session.Chat 路径 — 每次请求独立控制
resp, _ := session.Chat(ctx, base.ChatRequest{
    Model:          "gpt-4o",
    Messages:       messages,
    ResponseFormat: &base.ResponseFormat{Type: "json_object"},
})
```

## 技术方案

### Provider 映射规则

| Provider | SDK ResponseFormat | 实际发送到 API |
|----------|-------------------|---------------|
| **OpenAI Compat**（openai/deepseek/moonshot/dashscope/volcengine/qianfan/openai_compat） | 直接透传 | `"response_format": {"type":"json_object"}` 或含 `json_schema` |
| **Gemini** | json_object → 映射 | `"generationConfig": {"responseMimeType":"application/json"}` |
| **Gemini** | json_schema → 映射 | 同上 + `"responseSchema": <schema>` |
| **Ollama** | json_object → 映射 | `"format": "json"` |
| **Ollama** | json_schema → 映射 | `"format": <schema object>` |
| **Claude** | 忽略 | 不发送 |
| **平台类**（dify/coze/fastgpt/ragflow/qianfan_app/bailian/plugin/generic） | 忽略 | 不发送 |

### 变更范围

| 文件 | 改动 |
|------|------|
| `provider/base/types.go` | 新增类型定义 + ChatRequest 字段 |
| `provider/compat.go` | 两行 re-export 别名 |
| `client/quick.go` | ProviderConfig 加字段 + QuickSession 存储 + Send() 注入 |
| `provider/impls/openai/compat.go` | BuildRequest 透传 response_format |
| `provider/impls/gemini/spec.go` | BuildRequest 映射 generationConfig |
| `provider/impls/ollama/spec.go` | BuildRequest 映射 format |
| `test/provider_test.go` | boolPtr helper + 11 个测试 |

### 关键决策

- **ExtraBody 优先级**：ResponseFormat 逻辑放在 ExtraBody merge 之后，类型化字段覆盖 ExtraBody 同名 key（原因：显式设定应优先于兜底扩展字段）
- **Quick API 注入层级**：session 级别而非 per-request（原因：研判模型场景是整个 session 固定格式，跟 Model/Stream 注入方式一致，且不破坏 Send() 签名）
- **不支持的 provider 静默忽略**：不报错不警告（原因：SDK 核心禁止 log，报错会阻断调用方，忽略是最安全的降级策略）

## 约束条件

- SDK core 包禁止 log，诊断通过 error 返回
- 错误字符串必须带包前缀（如 `gemini:`）
- 向后兼容：指针字段零值为 nil，现有调用方无需任何改动
- 流式/非流式共用同一 BuildRequest，改动自动覆盖两条路径

## 验收标准

- [ ] `ChatRequest.ResponseFormat` 为 nil 时，所有 provider 行为不变（现有测试全通过）
- [ ] OpenAI compat 设 `json_object` → payload 含 `response_format.type=json_object`
- [ ] OpenAI compat 设 `json_schema` → payload 含完整 `response_format.json_schema` 结构
- [ ] Gemini 设 `json_object` → payload 含 `generationConfig.responseMimeType=application/json`
- [ ] Gemini 设 `json_schema` → payload 额外含 `generationConfig.responseSchema`
- [ ] Ollama 设 `json_object` → payload 含 `format=json`（字符串）
- [ ] Ollama 设 `json_schema` → payload 含 `format=<schema object>`（map）
- [ ] Claude 设 ResponseFormat → payload 不含 response_format 相关字段
- [ ] Quick API 设 `ProviderConfig.ResponseFormat` → Send() 发出的请求含对应格式
- [ ] `go test ./test/...` 全部通过
- [ ] `go vet ./...` 无警告
