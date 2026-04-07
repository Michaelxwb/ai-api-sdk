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
