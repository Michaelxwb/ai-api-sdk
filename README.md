# AI API SDK

> 统一 AI 模型接入 SDK，一行代码接入 14+ 主流 AI 平台

## 特性

- 🚀 **极简接入** - Quick API 一行代码完成配置
- 🌐 **14+ 平台** - OpenAI、Claude、Gemini、Dify、FastGPT、RAGFlow 等
- 💬 **流式优先** - 原生流式支持，实时响应
- 🔄 **会话管理** - 自动管理多轮对话历史
- 🔐 **灵活认证** - API Key、Bearer Token、自定义 Header
- 🔌 **通用适配** - Generic 适配器：贴入原始 HTTP 报文即可接入任意 API
- 🧠 **自动推理** - `RawReasoning()` 从抓包自动识别协议并生成接入配置

## 安装

```bash
go get github.com/Michaelxwb/ai-api-sdk
```

## 快速开始

### 单轮对话（流式）

```go
package main

import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/client"
    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
    qs := client.New().Quick(client.ProviderConfig{
        Provider: "openai",
        APIKey:   "sk-xxx",
        Model:    "gpt-4",
    })

    ch, _ := qs.SendText(context.Background(), "什么是 Rust 语言？")

    for chunk := range ch {
        if chunk.Error != nil {
            panic(chunk.Error)
        }
        fmt.Print(chunk.Text)
    }
}
```

### 多轮对话（自动管理历史）

```go
qs := client.New().Quick(client.ProviderConfig{
    Provider:    "openai",
    APIKey:      "sk-xxx",
    Model:       "gpt-4",
    SessionMode: "local_history", // 自动管理历史
})

// 第一轮
ch, _ := qs.SendText(ctx, "我叫张三")
for chunk := range ch { fmt.Print(chunk.Text) }

// 第二轮（自动带上历史）
ch, _ = qs.SendText(ctx, "我叫什么名字？")
for chunk := range ch { fmt.Print(chunk.Text) }
// 输出：你叫张三
```

## 支持的平台

| Provider | 说明 | SessionMode |
|----------|------|-------------|
| `openai` | OpenAI GPT 系列 | `local_history` |
| `claude` | Anthropic Claude | `local_history` |
| `gemini` | Google Gemini | `local_history` |
| `ollama` | 本地 Ollama | `local_history` |
| `deepseek` | DeepSeek | `local_history` |
| `moonshot` | Moonshot AI | `local_history` |
| `dashscope` | 阿里云百炼 | `local_history` |
| `volcengine` | 火山引擎 | `local_history` |
| `qianfan` | 百度千帆（文心一言） | `local_history` |
| `openai_compat` | OpenAI 兼容协议（第三方服务） | `local_history` |
| `dify` | Dify 平台 | `remote_session` |
| `ragflow` | RAGFlow | `remote_session` |
| `fastgpt` | FastGPT | 需显式指定 |
| `generic` | 通用适配器 | 需显式指定 |

**SessionMode 说明**：
- `local_history`：SDK 管理历史，适合标准 Chat API
- `remote_session`：服务端管理会话，SDK 传递 session_id

## 配置选项

### 基础配置

```go
client.ProviderConfig{
    Provider: "openai",        // 必填：平台标识
    APIKey:   "sk-xxx",        // API 密钥
    BaseURL:  "https://...",   // 覆盖默认 URL
    Model:    "gpt-4",         // 模型名称
    TimeoutSec: 60,            // 超时（秒），默认 60
}
```

### 高级选项

```go
client.ProviderConfig{
    // ... 基础配置 ...

    // 历史管理（local_history 模式）
    HistoryMaxMessages: 10,    // 最多保留 10 条历史
    HistoryMaxTokens:   4000,  // 历史 token 预算

    // 错误处理
    OnError: "continue",       // "abort"(默认) 或 "continue"

    // 强制独立会话
    StartNewChat: true,        // 每次调用不带历史
}
```

### Generic 适配器（接入任意 HTTP API）

贴入原始 HTTP 请求/响应报文即可接入：

```go
qs, _ := client.New().Quick(client.ProviderConfig{
    Provider:    "generic",
    BaseURL:     "https://api.example.com/v1/chat",
    SessionMode: "remote_session",
    APIKey:      "your-token",
    Request: "POST /v1/chat HTTP/1.1\n" +
        "Content-Type: application/json\n\n" +
        `{"prompt":"$$$","session_id":"$$$SESSION_ID$$$"}`,
    Response: `HTTP/1.1 200 OK\nContent-Type: application/json\n\n` +
        `{"content":"hello","session_id":"s1"}`,
})
```

如果有多轮抓包数据，可以用 `RawReasoning` 自动推理：

```go
// 一步完成推理 + 导出
spec, _ := cli.RawReasoning(rawMultiRoundSpec)

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

详见 [高级主题 - Generic 适配器](docs/ADVANCED.md#generic-适配器)。

## 更多示例

查看 [examples/](examples/) 目录：

- `01-single-turn/` - 单轮对话（流式/非流式）
- `02-multi-turn/` - 多轮对话（自动历史管理）
- `03-platform-integration/` - 平台集成示例
- `04-connectivity-test/` - 连通性测试
- `06-generic-raw/` - Generic 适配器（RawReasoning → Quick 完整闭环）
- `dify/`、`07-fastgpt/`、`08-ragflow/` - 特定平台示例

## 文档

- 📖 [使用指南](docs/GUIDE.md) - 分场景详细说明
- 🔧 [高级主题](docs/ADVANCED.md) - 自定义网关、认证、SessionStore
- 🧱 [设计文档](docs/internal/) - 内部架构设计

## License

MIT
