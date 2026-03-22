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

## 场景 6：连通性测试

测试配置是否正确：

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider: "openai",
    APIKey:   "sk-xxx",
})

result, err := qs.Test(ctx)
if err != nil {
    log.Fatalf("连接失败: %v", err)
}
fmt.Printf("延迟: %v\n", result.Latency)
```

## 特定平台示例

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
    BaseURL:  "https://ragflow.example.com/api",
    ExtraBody: map[string]any{
        "chat_id": "your-chat-id",
    },
})
```

