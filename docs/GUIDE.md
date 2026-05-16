# 使用指南

## 场景 1：单轮对话

### 流式输出（推荐）

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4",
})

ctx := context.Background()
ch, err := qs.SendText(ctx, "你好")
if err != nil {
    log.Fatal(err)
}

for chunk := range ch {
    if chunk.Error != nil {
        log.Fatal(chunk.Error)
    }
    fmt.Print(chunk.Text)
}
```

### 非流式输出

```go
import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4",
    Stream:   &[]bool{false}[0], // 显式禁用流式
})

ch, _ := qs.SendText(ctx, "你好")
chunk := <-ch // 只有一个完整响应
fmt.Println(chunk.Text)
```

## 场景 2：多轮对话

### local_history 模式（SDK 管理历史）

适用于：OpenAI、Claude、Gemini、Ollama 等标准 Chat API

```go
import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs := client.New().Quick(client.ProviderConfig{
    Provider:    "openai",
    APIKey:      "sk-xxx",
    Model:       "gpt-4",
    SessionMode: "local_history", // 可省略，自动推断
})

// 第一轮
ch, _ := qs.SendText(ctx, "我叫张三")
for chunk := range ch { fmt.Print(chunk.Text) }

// 第二轮（自动带上历史）
ch, _ = qs.SendText(ctx, "我叫什么？")
for chunk := range ch { fmt.Print(chunk.Text) }
```

### remote_session 模式（服务端管理会话）

适用于：Dify、RAGFlow

```go
import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs := client.New().Quick(client.ProviderConfig{
    Provider:    "dify",
    APIKey:      "app-xxx",
    SessionMode: "remote_session", // 可省略，自动推断
})

// SDK 自动管理 session_id，无需手动传递
ch, _ := qs.SendText(ctx, "第一个问题")
// ...
ch, _ = qs.SendText(ctx, "第二个问题")
```

## 场景 3：限制历史长度

防止上下文过长导致请求失败或费用暴涨：

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider:           "openai",
    APIKey:             "sk-xxx",
    Model:              "gpt-4",
    SessionMode:        "local_history",
    HistoryMaxMessages: 10,    // 最多保留 10 条历史
    HistoryMaxTokens:   4000,  // 历史 token 预算（按 len/4 估算）
})
```

## 场景 4：容错多轮对话

单轮失败不终止整个会话：

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4",
    OnError:  "continue", // 默认 "abort"
})

// 即使某一轮失败，后续轮次仍可继续
```

## 场景 5：OpenAI 兼容协议

接入兼容 OpenAI 协议的第三方服务：

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai_compat",
    APIKey:   "sk-xxx",
    BaseURL:  "http://10.6.193.48:30090/v1",
    Model:    "Qwen3-32B-FP8",
})
```

## 场景 6：Generic 适配器（接入任意 HTTP API）

当目标平台没有内置 Provider 时，使用 Generic 适配器——贴入原始 HTTP 报文即可接入。

### 方式一：手动构造 HTTP 报文

从 API 文档或浏览器抓包中复制请求/响应报文，用占位符标记动态字段：

- `$$$` — 用户输入
- `$$$SESSION_ID$$$` — 会话 ID（SDK 自动注入）
- `$$$NAME$$$` — 自定义链路字段（通过 ChainFields 声明）

```go
import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs, _ := client.New().Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     "https://api.example.com/v1/chat",
    SessionMode: "remote_session",
    APIKey:      "your-token",
    Request: "POST /v1/chat HTTP/1.1\n" +
        "Content-Type: application/json\n\n" +
        `{"prompt":"$$$","session_id":"$$$SESSION_ID$$$"}`,
    Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
        `{"content":"hello","session_id":"s1"}`,
})

ch, _ := qs.SendText(context.Background(), "你好")
for chunk := range ch {
    fmt.Print(chunk.Text)
}
```

### 方式二：从抓包自动推理（RawReasoning）

有 2-5 轮抓包数据时，SDK 自动识别字段语义、流式协议和链路传递关系：

```go
import (
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

cli := client.New()

// 构造多轮抓包数据（通常从 JSON 文件加载）
rawSpec := generic.RawHTTPMultiRoundSpec{
    BaseURL: "https://api.example.com/v1/chat",
    Rounds: []generic.RawHTTPRound{
        {
            Request:  "POST /v1/chat HTTP/1.1\nAuthorization: Bearer tok\nContent-Type: application/json\n\n" +
                `{"prompt":"hello","session_id":"","parent_msg":""}`,
            Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
                `{"content":"hi","session_id":"s1","msg_id":"m1"}`,
        },
        {
            Request:  "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n" +
                `{"prompt":"how?","session_id":"s1","parent_msg":"m1"}`,
            Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
                `{"content":"fine","session_id":"s1","msg_id":"m2"}`,
        },
    },
}

// 一步推理 + 导出
spec, _ := cli.RawReasoning(rawSpec)

// 直接传给 Quick
qs, _ := cli.Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     spec.BaseURL,
    SessionMode: spec.Model,
    Request:     spec.Request,
    Response:    spec.Response,
    ChainFields: spec.ChainFields,
})
```

`RawReasoning` 会自动：
- 识别输入字段（prompt）、会话字段（session_id）、链路字段（parent_msg → msg_id）
- 检测流式协议（SSE/JSON）和结束条件
- 生成带正确占位符的请求模板

### 带 ChainFields 的手动构造

当响应中有需要回传到下一轮请求的字段时（如 `message_id` → `parent_message_id`），使用 ChainFields：

```go
import "github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"

qs, _ := client.New().Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     "https://api.example.com/v1/chat",
    SessionMode: "remote_session",
    APIKey:      "your-token",
    Request: "POST /v1/chat HTTP/1.1\n" +
        "Content-Type: application/json\n\n" +
        `{"prompt":"$$$","session_id":"$$$SESSION_ID$$$","parent_msg":"$$$PARENT_MSG$$$"}`,
    Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
        `{"content":"hello","session_id":"s1","msg_id":"m1"}`,
    ChainFields: []generic.ChainField{
        {
            Placeholder:  "$$$PARENT_MSG$$$",  // 请求中的占位符
            ResponsePath: "msg_id",            // 从响应 JSON 提取
        },
    },
})
```

## 场景 7：连通性测试

测试配置是否正确。`Test()` 内部复用 `Send()` 主链路（流式模式），发送最小化探测消息 `"return 1"` 验证全链路可用性。探测会话无状态，不污染业务会话、不写入 Store，所有 Provider（包括仅支持流式的 Coze）均兼容。

```go
qs, _ := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4",
})

result, err := qs.Test(ctx)
if err != nil {
    log.Fatalf("连接失败: %v", err)
}
fmt.Printf("延迟: %v\n", result.Latency)
fmt.Printf("响应: %s\n", result.Response.Text)
```

## 场景 8：结构化输出（ResponseFormat）

强制模型以 JSON 格式返回响应，便于程序化解析。通过 `ProviderConfig.ResponseFormat` 设置，会自动应用到该会话的每次 `Send()` 调用。

### json_object 模式（推荐，兼容性最广）

```go
import (
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

qs, _ := client.New().Quick(client.ProviderConfig{
    Provider: "deepseek",
    APIKey:   "sk-xxx",
    Model:    "deepseek-chat",
    ResponseFormat: &base.ResponseFormat{
        Type: "json_object",
    },
})

ch, _ := qs.Send(ctx, []base.Message{
    {Role: "user", Content: `请以 JSON 返回北京天气，包含 city、temp_celsius、condition 字段。`},
})
```

### json_schema 模式（仅 OpenAI / Gemini / Ollama）

提供 JSON Schema 约束，模型严格按 schema 输出：

```go
qs, _ := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
    Model:    "gpt-4o",
    ResponseFormat: &base.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &base.JSONSchemaParam{
            Name:        "weather",
            Description: "天气信息",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "city":      map[string]any{"type": "string"},
                    "temp":      map[string]any{"type": "number"},
                    "condition": map[string]any{"type": "string"},
                },
                "required": []any{"city", "temp", "condition"},
            },
        },
    },
})
```

### 各 Provider 兼容性

| Provider | `json_object` | `json_schema` | 说明 |
|----------|:---:|:---:|------|
| OpenAI | ✅ | ✅ | 原生支持，`json_schema` 支持 `strict` 模式 |
| DeepSeek / Moonshot / DashScope / 火山 / 千帆 | ✅ | ❌ | 仅 `json_object`，`json_schema` 返回 400 |
| Gemini | ✅ | ✅ | SDK 自动映射为 `responseMimeType` + `responseSchema` |
| Ollama | ✅ | ✅ | SDK 自动映射为 `format` 字段 |
| Claude | ❌ | ❌ | 不支持，设置后返回错误 |

> **最佳实践**：跨平台场景优先使用 `json_object` + 在 SystemPrompt 中描述 schema 约束，兼容性最好。`json_schema` 仅在确认目标平台支持时使用。

## 场景 9：多模态图像

SDK 通过 `base.Message.Parts` 统一表达图文混排，按 provider 能力自动选择上传方式。调用方一次编码，SDK 在 spec 层做差异化适配。

### 9.1 数据模型

```go
type ContentPart struct {
    Type     string // "text" | "image_url" | "video_url"(预留) | "audio_url"(预留)
    Text     string // Type=="text" 时使用
    Data     string // Type=="image_url" 时：base64 编码的图片字节
    MIMEType string // image/png | image/jpeg | image/webp | image/gif
}

type Message struct {
    Role    string
    Content string        // 纯文本兼容路径
    Parts   []ContentPart // 多模态路径（与 Content 互斥）
    // ...
}
```

**互斥语义**：`len(Parts)==0` 时走 `Content`（保持旧行为）；`len(Parts)>0` 时走 `Parts`，忽略 `Content`。

### 9.2 Provider 能力三组分类

| 组 | 上传方式 | Provider | 调用方零感知点 |
|---|---|---|---|
| A | base64 内联 data URI | `openai` / `openai_compat` / `fastgpt` / `ollama` / `bailian_app` | SDK 自动拼 `data:<mime>;base64,<data>` |
| B | 先 multipart 上传换 `file_id` | `dify` / `coze` / `qianfan_app` / `moonshot` | SDK 内部并发上传所有图片，再注入 `file_id`/`object_string` |
| C | 不支持图片 | `generic` / `ragflow` / `deepseek` | `BuildRequest` 入口直接拒绝，错误明确 |

### 9.3 单图最小示例

```go
package main

import (
    "context"
    "encoding/base64"
    "fmt"
    "os"

    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
    raw, _ := os.ReadFile("photo.png")
    encoded := base64.StdEncoding.EncodeToString(raw)

    qs, _ := client.New().Quick(client.ProviderConfig{
        Provider: "openai",
        APIKey:   "sk-xxx",
        Model:    "gpt-4o",
    })

    ch, _ := qs.Send(context.Background(), []base.Message{{
        Role: "user",
        Parts: []base.ContentPart{
            {Type: "text", Text: "请描述这张图片"},
            {Type: "image_url", Data: encoded, MIMEType: "image/png"},
        },
    }})
    for c := range ch { fmt.Print(c.Text) }
}
```

### 9.4 多图 + 多轮对话

A/B 组 Provider 都支持单条消息内多张图（顺序保留）。B 组的多图会被 SDK 并发上传。

```go
parts := []base.ContentPart{{Type: "text", Text: "请分别描述这些图片"}}
for _, path := range []string{"a.png", "b.jpg", "c.webp"} {
    raw, _ := os.ReadFile(path)
    parts = append(parts, base.ContentPart{
        Type:     "image_url",
        Data:     base64.StdEncoding.EncodeToString(raw),
        MIMEType: "image/" + filepath.Ext(path)[1:],
    })
}
ch, _ := qs.Send(ctx, []base.Message{{Role: "user", Parts: parts}})
```

多轮对话时，SessionStore 会原样保留每轮 `Parts`（含 base64），续轮自动带回历史图片。**注意**：长会话 + 多图会显著放大 store 体积与上下文长度，必要时通过 `HistoryMaxMessages` 裁剪。

### 9.5 B 组上传的隐式要求

```go
// B 组（Dify/Coze/Qianfan/Moonshot）需要 APIKey 来发起上传请求
qs, _ := client.New().Quick(client.ProviderConfig{
    Provider: "dify",
    APIKey:   "app-xxx",  // ← 缺失时返回 "dify: API key required for file upload"
    BaseURL:  "https://api.dify.ai/v1",
})
```

`qianfan_app` 在首轮带图时若无 `SessionID`，SDK 会先 `createConversation` 拿 ID 再上传文件；从第二轮起复用从响应中拿回的 `conversation_id`（remote_session 默认行为，无需手动管理）。

### 9.6 多模态连通性探测

`Test()` 支持图像探测，用于验证模型可见性而不污染业务会话：

```go
import "github.com/Michaelxwb/ai-api-sdk/client"

result, err := qs.Test(ctx, &client.TestOptions{
    Parts: []base.ContentPart{
        {Type: "text", Text: "请计算图中公式，仅返回结果"},
        {Type: "image_url", Data: encoded, MIMEType: "image/png"},
    },
    Timeout: 30 * time.Second,
})
```

`Parts` 优先级高于 `Prompt`，两者同时提供时 `Prompt` 被忽略。

### 9.7 错误处理

```go
import (
    "errors"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
)

_, err := qs.Send(ctx, msgs)
switch {
case errors.Is(err, base.ErrEmptyImageData):
    // image_url 的 Data 为空 —— 数据缺失
case errors.Is(err, base.ErrUnsupportedImageFormat):
    // MIME 不在 PNG/JPEG/WEBP/GIF 白名单 —— 数据格式
case errors.Is(err, base.ErrUnsupportedPartType):
    // Type 拼错（如 "image" 漏 "_url"）—— 调用方代码 bug
}
```

C 组 Provider 直接返回带 provider 前缀的错误，方便日志定位：

```
generic:  multimodal content not supported in template mode, use text-only messages
ragflow:  image input not supported, provider only accepts text
deepseek: vision model not available, only text models supported
```

### 9.8 约束与注意事项

- **支持格式**：PNG / JPEG / WEBP / GIF（白名单严格校验，大小写不敏感）。BMP / SVG 等被拒。
- **Data 字段**：必须是 base64 编码后的字符串（不含 `data:` 前缀，SDK 自动拼装）。
- **预留类型**：`video_url` / `audio_url` 在数据结构上预留，validate 放行但所有 provider 暂未实现，传入会被静默丢弃，**目前请勿使用**。
- **大图建议**：单图 base64 后超过几 MB 会显著拉长请求时间和上下文 token；如有大尺寸需求，优先用 B 组 provider（文件上传）。
- **历史持久化**：`Parts` 会写入 SessionStore，多轮多图场景请评估存储成本。
- **会话模式**：B 组里 `qianfan_app` 必须保持默认 `remote_session`，强行切到 `local_history` 会让 SDK 每轮重新创建会话。

完整可运行的多平台示例见 `examples/11-multimodal-image/`，覆盖 A/B/C 三组的全部 11 个 provider。

## 特定平台示例

### 百度千帆（文心一言）

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "qianfan",
    APIKey:   "your-qianfan-api-key",
    Model:    "ernie-4.0-8k",
})
```

### Dify

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "dify",
    APIKey:   "app-xxx",
    Model:    "dify-model",
})
```

### FastGPT

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider:    "fastgpt",
    APIKey:      "fastgpt-xxx",
    BaseURL:     "https://fastgpt.example.com/api",
    SessionMode: "local_history", // 必须显式指定
    ExtraBody: map[string]any{
        "detail":    true,
        "variables": map[string]string{"key": "value"},
    },
})
```

### RAGFlow

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "ragflow",
    APIKey:   "ragflow-xxx",
    BaseURL:  "https://ragflow.example.com/api/v1/chats_openai/your-chat-id/chat/completions",
})
```
