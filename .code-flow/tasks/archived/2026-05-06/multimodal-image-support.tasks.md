# Tasks: 多模态图片支持

- **Source**: multimodal-image-support.design.md
- **Created**: 2026-05-09
- **Updated**: 2026-05-09

## Proposal

为 ai-api-sdk 扩展图片多模态输入能力，支持 11 个主流 AI 供应商的视觉模型。通过新增 `Message.Parts` 字段统一图片传输接口，SDK 内部自动处理不同供应商的差异（base64 内联 vs 文件上传），保持现有纯文本调用方零改动，降低集成复杂度 80%。

---

## TASK-001: 扩展数据结构

- **Status**: done
- **Priority**: P0
- **Depends**: 
- **Source**: multimodal-image-support.design.md#3.3.1 核心数据结构

### Description

修改 `provider/base/types.go`，在 `Message` 结构体中新增 `Parts []ContentPart` 字段，支持多模态内容块（文本+图片混排）。新增 `ContentPart` 结构体定义图片、文本等多模态内容的统一格式。

**关键变更**：
- `Message` 新增 `Parts []ContentPart` 字段（可选）
- 新增 `ContentPart` 结构体，包含 `Type`, `Text`, `Data`, `MIMEType` 字段
- 语义规则：`len(Parts)==0` 使用 `Content`，`len(Parts)>0` 使用 `Parts`

### Checklist

- [x] 在 `provider/base/types.go` 中定义 `ContentPart` 结构体
  - [x] 添加 `Type` 字段（`"text"` | `"image_url"`）
  - [x] 添加 `Text` 字段（文本内容）
  - [x] 添加 `Data` 字段（base64 图片数据）
  - [x] 添加 `MIMEType` 字段（image/png, image/jpeg 等）
- [x] 在 `Message` 结构体中添加 `Parts []ContentPart` 字段
  - [x] 使用 `json:"parts,omitempty"` 标签
  - [x] 添加注释说明向后兼容逻辑
- [x] 验证 JSON 序列化/反序列化正常
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-002: 图片格式校验

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001
- **Source**: multimodal-image-support.design.md#2.3.2 功能字段约束

### Description

新增 `provider/base/validate.go`，实现图片格式校验逻辑。在 SDK 核心层对 `Parts` 中的图片进行 MIME 类型白名单校验（PNG/JPEG/WEBP/GIF），提前拦截非法输入，返回明确错误提示。

**校验规则**：
- MIME 类型白名单：`image/png`, `image/jpeg`, `image/webp`, `image/gif`
- MIME 类型大小写不敏感
- 空 Data 返回 `ErrEmptyImageData`

### Checklist

- [x] 创建 `provider/base/validate.go` 文件
- [x] 定义错误类型
  - [x] `ErrUnsupportedImageFormat`（不支持的图片格式）
  - [x] `ErrEmptyImageData`（图片数据为空）
- [x] 实现 `ValidateContentParts(parts []ContentPart) error` 函数
  - [x] 遍历 `parts`，检查 `Type=="image_url"` 的条目
  - [x] 校验 `MIMEType` 是否在白名单内（大小写不敏感）
  - [x] 校验 `Data` 非空
- [x] 编写单元测试 `validate_test.go`
  - [x] 测试有效格式（PNG/JPEG/WEBP/GIF）
  - [x] 测试无效格式（BMP 等）
  - [x] 测试空 Data
  - [x] 测试大小写混合
- [x] 运行 `go test ./provider/base/...` 确保测试通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-003: 向后兼容处理

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: multimodal-image-support.design.md#3.3.1 语义规则

### Description

修改 `client/session.go` 的 `Chat` 方法，实现向后兼容逻辑。当 `len(Parts)==0` 时自动回退到使用 `Content` 字段，确保现有纯文本调用方零改动。在请求处理前调用校验函数验证 `Parts` 合法性。

**兼容逻辑**：
- `len(Parts)==0` → 使用 `Content`（老代码路径）
- `len(Parts)>0` → 使用 `Parts`（新代码路径）

### Checklist

- [x] 修改 `client/session.go` 的 `Chat` 方法
  - [x] 在发送请求前遍历 `req.Messages`
  - [x] 对每个 `Message`，检查 `len(Parts)`
  - [x] 如果 `len(Parts)==0` 且 `Content` 不为空，保持原有逻辑
  - [x] 如果 `len(Parts)>0`，调用 `base.ValidateContentParts(msg.Parts)`
  - [x] 校验失败返回错误
- [x] 同时修改 `ChatStream` 方法添加相同校验逻辑
- [x] 编写单元测试
  - [x] 测试纯文本路径（Parts 为空）
  - [x] 测试多模态路径（Parts 非空）
  - [x] 测试校验失败场景
- [x] 运行 `go test ./test/...` 确保测试通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-004: A组供应商 base64 内联路径

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 A组供应商映射

### Description

实现 A组供应商（OpenAI, FastGPT, Ollama, Bailian）的 `Parts` 到原生格式的映射逻辑。修改各供应商的 `BuildRequest` 方法，将 `Parts` 中的图片转换为 `data:{MIMEType};base64,{Data}` 格式，内联到请求体的 `content` 数组中。

**A组供应商**（共4个）：
- OpenAI
- FastGPT
- Ollama
- Bailian

**映射格式**：
```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "..."},
    {
      "type": "image_url",
      "image_url": {"url": "data:image/png;base64,..."}
    }
  ]
}
```

### Checklist

- [x] 修改 `provider/impls/openai/compat.go`
  - [x] 添加 `convertMessagesToOpenAI` 转换函数
  - [x] 修改 `BuildRequest` 方法使用转换后的 messages
  - [x] 遍历 `msg.Parts`
  - [x] `Type=="text"` → 添加 `{"type":"text", "text":...}`
  - [x] `Type=="image_url"` → 拼接 data URI，添加 `{"type":"image_url", "image_url":{"url":"data:..."}}`
  - [x] 支持多图（循环处理所有 image_url）
- [x] 修改 `provider/impls/fastgpt/spec.go`（复用 OpenAI 逻辑）
- [x] 修改 `provider/impls/ollama/spec.go`（复用 OpenAI 逻辑）
- [x] 修改 `provider/impls/bailian/spec.go`（在 toResponsesInput 中实现相同逻辑）
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

- [2026-05-09] created (draft)

---

## TASK-005-1: Dify 供应商文件上传

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 B组供应商映射 - Dify

### Description

实现 Dify 供应商的文件上传逻辑。新增 `provider/impls/dify/upload.go`，实现并发上传多张图片并获取 `file_id`，然后在 `BuildRequest` 中构造 `files` 数组。

**Dify 格式**：
```json
{
  "query": "描述图片",
  "files": [
    {
      "type": "image",
      "transfer_method": "local_file",
      "upload_file_id": "a1b2c3d4-xxxx"
    }
  ]
}
```

### Checklist

- [x] 创建 `provider/impls/dify/upload.go`
- [x] 实现 `uploadImages(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error)`
  - [x] 使用 `sync.WaitGroup` 并发上传多张图片
  - [x] base64 解码后构造 multipart form
  - [x] 调用 `/v1/files/upload` 上传文件
  - [x] 返回 `file_id` 列表
  - [x] 错误处理：上传失败返回明确错误
- [x] 修改 `provider/impls/dify/spec.go` 的 `BuildRequest`
  - [x] 检测 Parts 中是否有 image_url
  - [x] 从 Headers 获取 API Key
  - [x] 调用 `uploadImages` 获取 `file_ids`
  - [x] 构造 `files` 数组
  - [x] 合并到请求 payload
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-005-2: Coze 供应商文件上传

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 B组供应商映射 - Coze

### Description

实现 Coze 供应商的文件上传逻辑。Coze 使用 `additional_messages` 字段传递文件信息。

**Coze 格式**：
```json
{
  "additional_messages": [
    {
      "role": "user",
      "content": "[{\"type\":\"image\",\"file_url\":\"<file_id>\"}]",
      "content_type": "object_string"
    }
  ]
}
```

### Checklist

- [x] 创建 `provider/impls/coze/upload.go`
- [x] 实现 `uploadImages(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error)`
  - [x] 使用 `sync.WaitGroup` 并发上传
  - [x] 调用 Coze 文件上传接口
  - [x] 返回 `file_id` 列表
- [x] 修改 `provider/impls/coze/spec.go` 的 `BuildRequest`
  - [x] 调用 `uploadImages` 获取 `file_ids`
  - [x] 构造 `additional_messages` 格式
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-005-3: Qianfan 供应商文件上传

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 B组供应商映射 - Qianfan

### Description

实现 Qianfan 供应商的文件上传逻辑。Qianfan 使用 `file_ids` 数组传递文件。

**Qianfan 格式**：
```json
{
  "messages": [...],
  "file_ids": ["file-id-1", "file-id-2"]
}
```

### Checklist

- [x] 创建 `provider/impls/qianfan_app/upload.go`
- [x] 实现 `uploadImages(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error)`
  - [x] 使用 `sync.WaitGroup` 并发上传
  - [x] 调用 Qianfan 文件上传接口
  - [x] 返回 `file_id` 列表
- [x] 修改 `provider/impls/qianfan_app/spec.go` 的 `BuildRequest`
  - [x] 调用 `uploadImages` 获取 `file_ids`
  - [x] 填充 `file_ids` 数组
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-005-4: Moonshot 供应商文件上传

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 B组供应商映射 - Moonshot

### Description

实现 Moonshot 供应商的文件上传逻辑。Moonshot 使用 `ms://` 前缀的 file_id。

**Moonshot 格式**：
```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "..."},
        {"type": "image_url", "image_url": {"url": "ms://file-id"}}
      ]
    }
  ]
}
```

### Checklist

- [x] 创建 `provider/impls/openai/upload.go`（方案C：集成到 OpenAI 包）
- [x] 实现 `uploadImagesForMoonshot(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error)`
  - [x] 使用 `sync.WaitGroup` 并发上传
  - [x] 调用 `/v1/files` 上传文件
  - [x] 返回 `file_id` 列表
- [x] 修改 `provider/impls/openai/compat.go` 的 `BuildRequest`
  - [x] 检测 Moonshot 供应商并上传图片获取 `file_ids`
  - [x] 修改 `convertMessagesToOpenAI` 支持 `ms://` 前缀构造 image_url
  - [x] 添加详细备注说明方案C的设计决策
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done) - 采用方案C：集成到 OpenAI 兼容模式

---

## TASK-006: C组供应商 错误处理

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001, TASK-002, TASK-003
- **Source**: multimodal-image-support.design.md#3.3.2 C组供应商映射

### Description

实现 C组供应商（RAGFlow, DeepSeek, Generic）的图片检测和错误处理逻辑。修改各供应商的 `BuildRequest` 方法，检测到 `Parts` 包含 `image_url` 类型时返回明确错误，提示该供应商不支持图片输入。

**C组供应商**（共3个）：
- RAGFlow（纯文本 RAG 引擎）
- DeepSeek（无视觉模型）
- Generic（模板字符串系统）

**错误提示格式**：
```
"ragflow: image input not supported, provider only accepts text"
"deepseek: vision model not available, only text models supported"
"generic: multimodal content not supported in template mode, use text-only messages"
```

### Checklist

- [x] 修改 `provider/impls/ragflow/spec.go`
  - [x] 在 `BuildRequest` 开头检测 `Parts`
  - [x] 遍历 `msg.Parts`，发现 `Type=="image_url"` 返回错误
  - [x] 错误消息：`"ragflow: image input not supported, provider only accepts text"`
- [x] 修改 `provider/impls/openai/compat.go`（DeepSeek 使用 OpenAI 兼容模式）
  - [x] 添加图片检测逻辑（针对 DeepSeek）
  - [x] 错误消息：`"deepseek: vision model not available, only text models supported"`
- [x] 修改 `provider/impls/generic/spec.go`
  - [x] 添加图片检测逻辑
  - [x] 错误消息：`"generic: multimodal content not supported in template mode, use text-only messages"`
- [x] 运行 `go build ./...` 确保编译通过

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## TASK-007: 编写测试代码

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-004, TASK-005-1, TASK-005-2, TASK-005-3, TASK-005-4, TASK-006
- **Source**: multimodal-image-support.design.md#7.2 测试代码

### Description

新增 `examples/11-multimodal-image/main.go` 测试脚本，采用**方案 B：通用函数 + 独立函数封装**。实现一个通用测试函数 `testMultimodalImages`，为每个供应商封装独立的测试函数，方便单独测试某个供应商。

**测试方案 B**：
- 一个通用函数 `testMultimodalImages(provider, imagePaths, query)`
- 11 个独立函数：`testOpenAI()`, `testDify()`, `testFastGPT()` 等
- 在 `main()` 中选择性调用（注释控制）

**测试覆盖**：
- 单图测试（每个供应商）
- 多图测试（A组和B组）
- 错误处理（C组预期返回错误）

### Checklist

- [x] 创建 `examples/11-multimodal-image/main.go`
- [x] 实现通用测试函数 `testMultimodalImages(provider, imagePaths, query)`
  - [x] 初始化 `client.New()`
  - [x] 配置 `ProviderConfig`（从环境变量读取 API Key）
  - [x] 实现 `getBaseURL(provider)` 辅助函数
  - [x] 实现 `getModel(provider)` 辅助函数
  - [x] 构造 `Parts` 数组（文本 + 图片）
  - [x] 调用 `qs.Send(ctx, msgs)`
  - [x] 打印流式输出
- [x] 实现独立测试函数（共11个 + 多图变体）
  - [x] **A组**：`testOpenAI()`, `testFastGPT()`, `testOllama()`, `testBailian()`, `testOpenAIMulti()`
  - [x] **B组**：`testDify()`, `testCoze()`, `testQianfan()`, `testMoonshot()` + 各自的 Multi 版本
  - [x] **C组**：`testRAGFlow()`, `testDeepSeek()`, `testGeneric()`（预期返回错误）
- [x] 实现 `main()` 函数
  - [x] 默认调用 `testOpenAI()`
  - [x] 其他函数注释，方便选择性测试
- [x] 实现辅助函数
  - [x] `readImageBase64(path) (string, string, error)` - 返回 base64 数据和 MIME 类型
  - [x] `printStream(ch <-chan streaming.StreamChunk) error`
  - [x] `getBaseURL(provider) string`
  - [x] `getModel(provider) string`
  - [x] `getAPIKey(provider) string`
- [x] 添加 README.md 说明
  - [x] 环境变量配置说明（11个供应商）
  - [x] 运行方式说明
  - [x] 供应商测试选择说明
  - [x] 故障排查指南
  - [x] 技术细节说明
- [x] 编译验证 `go build .`

### Log

- [2026-05-09] created (draft)
- [2026-05-09] completed (done)

---

## 执行建议

### 实施顺序

```
TASK-001 (数据结构)
    ↓
TASK-002 (校验) + TASK-003 (兼容) ← 可并行
    ↓
TASK-004 (A组) + TASK-005 (B组) + TASK-006 (C组) ← 可并行
    ↓
TASK-007 (测试)
```

### 关键里程碑

1. **M1**: 完成 TASK-001 → 数据结构定义完成
2. **M2**: 完成 TASK-002, TASK-003 → SDK 核心层完成
3. **M3**: 完成 TASK-004, TASK-005, TASK-006 → 所有供应商适配完成
4. **M4**: 完成 TASK-007 → 功能可测试验证

### 风险提示

- **TASK-005** 复杂度最高（4个供应商，文件上传逻辑），预留足够时间
- **TASK-007** 需要准备测试图片和配置各供应商的 API Key
- C组供应商测试需验证错误返回是否符合预期

---

**下一步**: 审阅任务，执行 `/cf-task:start multimodal-image-support` 开始实施
