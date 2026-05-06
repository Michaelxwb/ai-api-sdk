# SDK 扩展能力可行性探索

> 基于完整调研与实测更新，涵盖所有 13 个 Provider 的多模态方案与单轮多步编排设计。

---

## 一、多模态适配（图片/附件/音频）

### 现状

- `base/types.go` — `Message.Content` 为 `string`，所有 Provider 的 `BuildRequest` 均假设纯文本
- 当前共 **13 个 Provider**（目录级）：`bailian`、`claude`、`coze`、`dify`、`fastgpt`、`gemini`、`generic`、`ollama`、`openai`（含 6 个 compat 注册）、`plugin`、`qianfan_app`、`ragflow`、`self_developed`
- Plugin Transport (`transport.go`) 已有 `coerceToString()` 支持 `interface{}`，有一定基础

### 推荐方案：并存方案（零破坏）

```go
// base/types.go 新增

// ContentPart 表示消息的一个内容块，用于多模态场景。
// Type 取值：
//   "text"  — 纯文本，填 Text
//   "image" — 图片，公网可达时填 URL；内网/离线时填 Data(base64) + MIMEType
//   "file"  — 文档/音频，填 URL 或 Data + MIMEType；部分 Provider 需要 FileName
type ContentPart struct {
    Type     string `json:"type"`
    Text     string `json:"text,omitempty"`
    URL      string `json:"url,omitempty"`       // 公网图片/文件 URL
    Data     string `json:"data,omitempty"`      // base64 编码字节（内网场景）
    MIMEType string `json:"mime_type,omitempty"` // 配合 Data，如 "image/jpeg"
    FileName string `json:"file_name,omitempty"` // FastGPT file_url 的 name 字段
}

// Message 并存方案：Content 保留旧行为；len(Parts)>0 时走多模态路径。
type Message struct {
    Role       string        `json:"role"`
    Content    string        `json:"content"`
    Parts      []ContentPart `json:"parts,omitempty"`
    Name       string        `json:"name,omitempty"`
    ToolCallID string        `json:"tool_call_id,omitempty"`
}
```

语义规则：
- `len(Parts) == 0` → 原有纯文本路径，所有现存调用方零改动
- `len(Parts) > 0` → 多模态路径，各 Provider 各自映射

### 内网场景结论（已实测）

百炼 App Responses API 实测：`image_url` 和 `file_url` 均接受 `data:MIME;base64,...` 格式。
百炼内部自动将 data URI 上传到 OSS，转为内部临时 URL 后再喂给模型。

| 内容类型 | 公网场景 | 内网场景 |
|---|---|---|
| 图片 | 填 `ContentPart.URL` | 填 `ContentPart.Data` + `MIMEType`（base64） |
| 小文件 PDF（<5MB） | 填 `ContentPart.URL` | 填 `ContentPart.Data` + `MIMEType` |
| 大文件/音频 | 填 `ContentPart.URL` | 需预上传或部署层解决（base64 请求体过大） |

### 13 个 Provider 多模态支持汇总

#### A 组 — 直接内联（`BuildRequest` 单步完成）

| Provider | 图片 | 文件 | 音频 | 格式关键字 | 官网文档 |
|---|---|---|---|---|---|
| `openai` / compat 系列（moonshot / deepseek / dashscope / volcengine / qianfan） | ✅ base64/URL | ✅ base64 data URI | ❌（模型决定） | `content[].type: "image_url"` | [OpenAI Vision](https://platform.openai.com/docs/guides/vision) |
| `claude` | ✅ base64/URL | ✅ PDF base64 | ❌ | `content[].type: "image"/"document"` | [Claude Vision](https://docs.anthropic.com/en/docs/build-with-claude/vision) |
| `gemini` | ✅ base64/URL | ✅ | ✅（部分模型） | `parts[].inlineData / fileData` | [Gemini Vision](https://ai.google.dev/gemini-api/docs/vision) |
| `ollama` | ✅ base64 only | ❌ | ❌ | `message.images: []` | [Ollama Vision](https://ollama.com/blog/vision-models) |
| `bailian` | ✅ base64/URL（已实测） | ✅ base64/URL（已实测） | ⚠️ 需视觉模型 | `input_image` / `input_file` | [百炼多模态](https://help.aliyun.com/zh/model-studio/developer-reference/multimodal-input) |

> `deepseek` 目前无视觉模型，Parts 存在时仅取文本 Part，不报错。

#### B 组 — 需预上传（实现 `FileUploadSpec` 接口）

| Provider | 图片 | 文件 | 音频 | 上传端点 | 关键参数来源 | 官网文档 |
|---|---|---|---|---|---|---|
| `dify` | ✅ | ✅ | ✅ | `POST /files/upload` (multipart) | BaseURL | [Dify 文件上传](https://docs.dify.ai/zh-hans/guides/workflow/file-upload) |
| `coze` | ✅ | ✅ | ✅ | `POST /v1/files/upload` | Bearer Token | [Coze 文件 API](https://www.coze.cn/docs/developer_guides/api_upload_files) |
| `fastgpt`（文件） | ✅ base64/URL（图片） | ✅ presign 上传 | ❌ | `POST /core/chat/file/presignChatFilePostUrl` + PUT | `ExtraBody["appId"]` + `SessionID` | [FastGPT 多模态](https://doc.fastgpt.cn/zh-CN/openapi/intro) |
| `qianfan_app` | ✅ | ✅ | ✅ wav/mp3/m4a/amr | `POST /v2/app/conversation/file/upload` (multipart) | `req.Model`(app_id) + `req.SessionID` | [千帆文件上传](https://cloud.baidu.com/doc/qianfan-api/s/Nm7vrwe2k) |

#### C 组 — 不支持或透传

| Provider | 结论 | 原因 | 官网文档 |
|---|---|---|---|
| `ragflow` | ❌ Parts 时返回错误 | 官方 chat API 纯文本，无多模态字段 | [RAGFlow API](https://ragflow.io/docs/dev/http_api_reference) |
| `generic` | ❌ Parts 时返回错误 | 模板字符串替换系统，不支持结构化 Parts | — |
| `self_developed` | 透传 ExtraBody，SDK 不处理 | 自定义协议，无统一规范 | — |
| `plugin` | Parts 序列化为 JSON 字符串 | Transport 层 `coerceToString()` 已有支持 | — |

---

### 各 Provider 多模态能力与格式映射

#### A 组 — URL/base64 直接内联，`BuildRequest` 单步完成

**OpenAI Compat 系列**（openai / moonshot / dashscope / volcengine / qianfan / openai_compat）

上游 API 格式：
```json
"content": [
  {"type": "text", "text": "..."},
  {"type": "image_url", "image_url": {"url": "https://... 或 data:image/jpeg;base64,..."}}
]
```

- `deepseek`：目前无视觉模型，Parts 存在时忽略图片类型，仅取文本 Part
- 其余 compat Provider：透传 content 数组，模型支持情况取决于所选 model

`BuildRequest` 改动：`"messages": req.Messages` → 当 `Parts` 非空时，将 `Content` 字段值由 `string` 改为 `[]map[string]any`

---

**Claude**

上游 API 格式：
```json
"content": [
  {"type": "text", "text": "..."},
  {"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "..."}},
  {"type": "image", "source": {"type": "url", "url": "https://..."}}
]
```

文件（PDF）：`{"type": "document", "source": {"type": "base64", "media_type": "application/pdf", "data": "..."}}`

注意：Claude system prompt 已单独处理，多模态只影响 user/assistant messages。

---

**Gemini**

上游 API 格式（`contents[].parts[]`）：
```json
"parts": [
  {"text": "..."},
  {"inlineData": {"mimeType": "image/jpeg", "data": "base64..."}},
  {"fileData":  {"mimeType": "video/mp4",   "fileUri": "gs://..."}}
]
```

当前 `BuildRequest` 硬编码 `"parts": [{"text": m.Content}]`，需改为按 `Parts` 动态构建。

---

**Ollama**

上游 API 格式（`images` 是消息级字段，**非** content 数组）：
```json
{
  "role": "user",
  "content": "描述文本",
  "images": ["base64...", "base64..."]
}
```

**注意**：Ollama 的多模态格式与所有其他 Provider 不同，不使用 content 数组，需单独处理。
只支持图片，不支持文件。视觉能力依赖所用模型（如 llava、bakllava）。

---

**Bailian**（已实测 ✅）

上游 API 格式（Responses API，`input[]`）：
```json
{
  "role": "user",
  "content": [
    {"type": "input_text",  "text": "..."},
    {"type": "input_image", "image_url": "https://... 或 data:image/jpeg;base64,..."},
    {"type": "input_file",  "file_url":  "https://... 或 data:audio/mpeg;base64,..."}
  ]
}
```

类型名为 `input_text` / `input_image` / `input_file`，与 OpenAI 的 `text` / `image_url` 不同。
`toResponsesInput()` 当前将 `content` 序列化为字符串，需改为按 `Parts` 构建数组。

实测结论：base64 data URI 对 `input_image` 和 `input_file` 均有效，百炼自动中转 OSS。

---

#### B 组 — 需要预上传，`BuildRequest` 前有额外 HTTP 调用

**Dify**

图片流程：`POST /files/upload` (multipart) → 返回 `upload_file_id` → 注入请求体 `files` 字段

```json
{
  "query": "...",
  "files": [
    {"type": "image", "transfer_method": "local_file", "upload_file_id": "xxx"},
    {"type": "image", "transfer_method": "remote_url",  "url": "https://..."}
  ]
}
```

远端 URL 可直接用 `transfer_method: remote_url`，无需上传。
内网图片需先上传（`transfer_method: local_file`）。

---

**Coze**

当前 `BuildRequest` 硬编码 `content_type: "text"`，多模态需改为 `content_type: "object_string"`：

```json
{
  "additional_messages": [{
    "role": "user",
    "content_type": "object_string",
    "content": "[{\"type\":\"image\",\"file_id\":\"xxx\"},{\"type\":\"text\",\"text\":\"...\"}]"
  }]
}
```

图片需先通过 `POST /v1/files/upload` 上传获取 `file_id`。
远端 URL 可填 `file_url` 字段避免上传。

---

**FastGPT（文件类型）**

图片可直接 base64（OpenAI 兼容格式）：
```json
{"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,..."}}
```

文件（PDF/文档）需预上传：
```
POST /core/chat/file/presignChatFilePostUrl
  Body: {appId, chatId, filename}     ← appId 从 ExtraBody 取，chatId 从 SessionID 取
  → 返回预签名 PUT URL
PUT <presigned_url>                   ← 上传文件字节
→ 拿到 file key / 访问 URL
```

然后在 chat 请求中：
```json
{"type": "file_url", "name": "文件名", "url": "<访问URL>"}
```

**注意**：预签名上传接口需要 API Key 鉴权（Bearer Token，与 chat 端点相同）。

---

**qianfan_app**（已确认 ✅）

文件上传流程：`POST /v2/app/conversation/file/upload` (multipart) → 返回 `id` → 注入请求体 `file_ids` 字段

```
POST /v2/app/conversation/file/upload
  Content-Type: multipart/form-data
  Body: {
    file: <binary>,          ← 与 file_url 二选一，file 优先
    file_url: "https://...", ← 不可为内网地址
    app_id: "xxx",           ← 必填，从 req.Model 取
    conversation_id: "xxx"   ← 必填，从 req.SessionID 取（首轮不可上传文件）
  }
  → 返回 {id: "uuid", conversation_id: "...", request_id: "..."}
```

然后在 chat 请求中：
```json
{
  "query": "...",
  "app_id": "...",
  "conversation_id": "...",
  "file_ids": ["上传返回的id"]
}
```

支持格式：xlsx、json、jsonl、png、jpg、jpeg、pdf、wav、docx、csv、txt、dcm、gz、mha、tif、tiff、webp、heic、mp3、pcm、m4a、amr、zip（最大 100MB）。

**注意**：
- `file_url` 不接受内网地址，内网文件必须通过 `file` 字段 multipart 上传
- 文件与会话绑定，`conversation_id` 是必填项，因此首轮对话（无 SessionID）不能上传文件
- `app_id` 从 `req.Model` 字段取（与现有 BuildRequest 逻辑一致）

---

#### C 组 — 暂不支持或透传

| Provider | 处理方式 | 说明 |
|---|---|---|
| `ragflow` | Parts 存在时返回错误 | 官方 chat API 无多模态支持 |
| `self_developed` | 调用方自己处理，SDK 透传 ExtraBody | 自定义协议，无统一规范 |
| `generic` | Parts 存在时返回错误 | 模板系统基于字符串替换，不支持结构化 Parts |
| `plugin` | 已有 `coerceToString()`，Parts 转 JSON 字符串 | Transport 层已有一定基础 |

### 可选扩展接口设计（B 组用）

```go
// provider/base/types.go 新增（可选接口，B 组 Provider 实现）
// FileUploadSpec 在 BuildRequest 前执行文件上传，将 ContentPart.Data 转为 URL。
// 实现此接口的 Provider：dify、coze、fastgpt（文件类型）、qianfan_app。
type FileUploadSpec interface {
    UploadFiles(ctx context.Context, opts BuildOptions, msgs []Message) ([]Message, error)
}
```

`client/session.go` 插入 hook（在 `BuildRequest` 之前）：
```go
if uploader, ok := spec.(base.FileUploadSpec); ok {
    messages, err = uploader.UploadFiles(ctx, opts, messages)
    if err != nil {
        return chatResp, fmt.Errorf("client: file upload failed: %w", err)
    }
}
```

`UploadFiles` 将包含 `Data` 字节的 `ContentPart` 上传并回填 `URL`，清空 `Data`；
`BuildRequest` 此时只看 `URL`，保持单次 HTTP 契约。

### 改造范围汇总

| 层 | 文件 | 改动规模 | 说明 |
|---|---|---|---|
| 类型定义 | `base/types.go` | 小（+20行） | 新增 `ContentPart`、`FileUploadSpec`、`Message.Parts` |
| Client hook | `client/session.go` | 小（+10行） | 调 `BuildRequest` 前检查 `FileUploadSpec` |
| A 组 Provider | 6个文件 | 中（各+30行） | `BuildRequest` 加 Parts 分支 |
| B 组 Provider | 4个文件 | 大（各+80行） | 新增 `UploadFiles` + `BuildRequest` 调整 |
| C 组 Provider | 4个文件 | 小（各+5行） | 错误返回或透传 |

### 关键决策（已确认）

1. **`ContentPart` 设计**：用 `FileName string` 替代 `Metadata map[string]any`，静态类型更安全
2. **`deepseek` 无视觉**：Parts 存在时只取文本 Part，不报错
3. **预上传失败**：整个 Chat 请求返回错误，不降级纯文本（避免静默丢失内容）
4. **`generic` 不支持**：模板系统本质是字符串替换，Parts 存在时返回明确错误
5. **FastGPT `appId`**：存入 `ExtraBody["appId"]`，文档说明为必填配置项

### 实施顺序

1. `base/types.go` — `ContentPart` + `Message.Parts` + `FileUploadSpec`
2. A 组 6 个 Provider — `BuildRequest` Parts 映射
3. `client/session.go` — `FileUploadSpec` hook
4. B 组 4 个 Provider — `UploadFiles` + `BuildRequest`（dify / coze / fastgpt / qianfan_app）
5. C 组 — 错误返回或透传（ragflow / generic / self_developed / plugin）

### 结论

**可行性：高**。并存方案零破坏，Provider 层逐个适配，互不影响。
B 组预上传通过可选接口隔离，不污染 A 组和 C 组。

---

## 二、单轮内多步编排（Multi-Step Pipeline）

### 场景

用户发一次消息，SDK 内部需要串联调用多个接口才能得到最终答案。例如：
1. 调接口 A（发送问题）→ 拿到 `task_id`
2. 调接口 B（传入 `task_id`）→ 拿到最终答案

应用层只感知一次 `Chat()` 调用，多步编排由 SDK 内部完成。

### 现状

- **ChainField 机制**（`generic/profile.go:6-8`）：已支持**跨轮次**的字段传递（第 N 轮提取 → 第 N+1 轮注入），但不支持单轮内多步
- **Rounds**（`generic/rawpacket.go:34-41`）：用于推理/配置生成阶段的多轮抓包解析，不是运行时编排
- **GenericProfile** 只包含一组请求/响应配置，无 pipeline/steps 概念

### 需要新增的能力

#### 方案：GenericProfile 支持 Steps

```go
type GenericProfile struct {
    // 现有字段...

    // Steps 定义单轮内多步编排，为空时保持当前单步行为。
    // 每个 Step 是一组独立的请求/响应配置，步骤间通过 ChainField 传递数据。
    Steps []StepProfile `json:"steps,omitempty" yaml:"steps,omitempty"`
}

type StepProfile struct {
    Name     string            `json:"name"`    // 步骤名，如 "send_question", "get_answer"
    URL      string            `json:"url"`     // 可选，不同步骤可能调不同接口
    Method   string            `json:"method"`
    Headers  map[string]string `json:"headers"`
    Body     map[string]any    `json:"body"`    // 模板，支持 $$$PLACEHOLDER$$$
    Response StepResponse      `json:"response"`
}

type StepResponse struct {
    ExtractFields []ChainField  `json:"extract_fields"` // 提取值注入下一步
    IsFinal       bool          `json:"is_final"`        // 是否为最终步骤
    TextPath      string        `json:"text_path"`       // 最终文本提取路径
    Stream        StreamProfile `json:"stream"`
}
```

#### 运行时执行流程

```
GenericSpec.BuildRequest() / ParseResponse() 改造为：

if len(profile.Steps) == 0:
    // 现有单步逻辑，不变
else:
    stepValues := map[string]string{}
    for i, step := range profile.Steps:
        req := renderTemplate(step.Body, input, stepValues)
        resp := httpClient.Do(req)
        stepValues = extractFields(resp, step.Response.ExtractFields)
        if step.Response.IsFinal:
            return parseText(resp, step.Response.TextPath)
```

### 改造范围

| 层 | 影响 | 说明 |
|---|---|---|
| `generic/profile.go` | 中 | 新增 StepProfile 类型 |
| `generic/spec.go` BuildRequest/ParseResponse | 大 | 核心改造，支持多步执行 |
| `generic/raw.go` / `rawpacket.go` | 中 | 配置解析支持 steps |
| `generic/inference.go` | 大 | 推理器需识别多步模式 |
| `client/session.go` | 无 | 透传，不感知步骤 |
| 其他 Provider | 无 | 仅 Generic Adapter 需要 |

### 与现有 ChainField 的关系

- **ChainField**：跨轮次，第 N 轮 → 第 N+1 轮（保持不变）
- **StepProfile.ExtractFields**：单轮内，Step N → Step N+1（新增）
- 两者可复用同一个 `ChainField` 结构体，只是作用域不同

### 接入方式：显式声明 vs 自动推理

#### 显式声明（有 API 文档时）

```yaml
steps:
  - name: send_question
    url: https://api.example.com/v1/chat
    body: {"question": "$$$"}
    extract_fields:
      - placeholder: "$$$TASK_ID$$$"
        response_path: "data.task_id"
  - name: get_answer
    url: https://api.example.com/v1/result
    body: {"task_id": "$$$TASK_ID$$$"}
    is_final: true
    text_path: "data.answer"
```

准确率：100%，改造量集中在 GenericSpec 运行时执行逻辑。

#### 自动推理（抓包接入时）

现有推理器核心算法可复用：

| 已有能力 | 函数 | 行号 | 说明 |
|---|---|---|---|
| 值精确匹配 | `checkChainPattern()` | 507-545 | `round[i-1].resp == round[i].req` → 0.85 置信度 |
| 命名语义分析 | `scoreDynamicCandidate()` | 562-594 | `task_id`, `msg_id` 等关键字加分 |
| 首轮空值检测 | `checkSessionIDPattern()` | 448-451 | 强信号 |
| 置信度聚合 | `buildReport()` | 1006-1065 | 几何均值，自动确认阈值 0.85 |

扩展改动点：

| 函数 | 改动 |
|---|---|
| `parseRounds()` 行 135 | 支持每轮内多步 `[]StepPair` |
| `checkChainPattern()` 行 507 | 新增同轮内 `step[j-1].resp → step[j].req` 匹配 |
| `analyzeFields()` 行 224 | 嵌套循环：轮次 × 步骤 |
| `buildProfileFromFields()` 行 839 | 生成按步骤分组的链路规则 |

准确率预估：

| 条件 | 准确率 |
|---|---|
| 2 组样本 | ~70-75% |
| 3 组样本 | ~85-90% |
| 推理 + 用户确认（低于 0.85 阈值时提示） | ~95%+ |

### 关键决策点（实施时确认）

1. **流式支持**：多步中只有最后一步需要流式，还是中间步骤也可能是流式
2. **错误处理**：某个中间步骤失败时，是否需要回滚/重试
3. **超时分配**：总超时如何在各步骤间分配
4. **轮询模式**：是否需要支持"调接口 A → 轮询接口 B 直到完成"的异步任务模式
5. **步骤分组**：抓包场景下靠时间戳间隔还是 URL 差异来区分步骤

### 结论

**可行性：中-高**。
- 有 API 文档 → 显式声明，改造量小，仅改 GenericSpec 运行时逻辑
- 抓包接入 → 自动推理，核心算法可复用，主要新增步骤分组和同轮内值追溯
- 推荐**显式声明优先 + 推理兜底**组合，降低实施风险

---

## 总结

| 场景 | 可行性 | 改造范围 | 对现有 API 的影响 |
|---|---|---|---|
| 多模态 | 高 | `base/types.go` + 13 个 Provider | 零破坏（并存方案） |
| 单轮多步编排 | 中-高 | 局部（仅 Generic Adapter） | 零破坏（新增 Steps） |

两个能力可以独立实施，互不依赖。多模态建议优先实施（改造范围明确，各 Provider 互不影响）。
