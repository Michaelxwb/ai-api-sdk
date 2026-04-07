# AI API SDK

> 统一 AI 模型接入 SDK，一行代码接入 16+ 主流 AI 平台

## 特性

- 🚀 **极简接入** - Quick API 一行代码完成配置
- 🌐 **16+ 平台** - OpenAI、Claude、Gemini、Coze、Dify、FastGPT、RAGFlow 等
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

### 基模接入

| Provider | 说明 | 必填参数 | 可选参数 | 默认 BaseURL | SessionMode |
|----------|------|---------|---------|-------------|-------------|
| `ollama` | 本地/远程 Ollama | `Model`, `BaseURL` | `APIKey` | `127.0.0.1:11434` | 自动 `local_history` |
| `openai` | OpenAI GPT 系列 | `APIKey`, `Model`, `BaseURL` | — | `api.openai.com/v1` | 自动 `local_history` |
| `openai_compat` | OpenAI 兼容协议（第三方） | `APIKey`, `Model`, `BaseURL` | — | 无 | 自动 `local_history` |
| `deepseek` | DeepSeek | `APIKey`, `Model`, `BaseURL` | — | `api.deepseek.com/v1` | 自动 `local_history` |
| `moonshot` | Moonshot AI | `APIKey`, `Model`, `BaseURL` | — | `api.moonshot.cn/v1` | 自动 `local_history` |
| `dashscope` | 阿里云百炼 | `APIKey`, `Model`, `BaseURL` | — | `dashscope.aliyuncs.com/compatible-mode/v1` | 自动 `local_history` |
| `volcengine` | 火山引擎 | `APIKey`, `Model`, `BaseURL` | — | `ark.cn-beijing.volces.com/api/v3` | 自动 `local_history` |
| `qianfan` | 百度千帆（文心一言） | `APIKey`, `Model`, `BaseURL` | — | `qianfan.baidubce.com/v2` | 自动 `local_history` |
| `claude` | Anthropic Claude | `APIKey`, `Model`, `BaseURL` | — | `api.anthropic.com` | 自动 `local_history` |
| `gemini` | Google Gemini | `APIKey`, `Model`, `BaseURL` | — | `generativelanguage.googleapis.com` | 自动 `local_history` |

### 应用层接入

| Provider | 说明 | 必填参数 | 可选参数 | 默认 BaseURL | SessionMode |
|----------|------|---------|---------|-------------|-------------|
| `bailian_app` | 阿里百炼应用接入（Responses API） | `APIKey`, `BaseURL` | `Model`, `ExtraBody`, `Path` | 无（需填写应用 Endpoint） | 自动 `local_history` |
| `coze` | Coze 扣子（仅流式） | `APIKey`, `Model`(bot_id) | `BaseURL`, `ExtraBody`(`user_id`, `custom_variables`等) | `api.coze.cn/v3` | 自动 `remote_session` |
| `qianfan_app` | 百度千帆应用接入 | `APIKey`, `Model`(app_id) | `BaseURL`, `ExtraBody`(`end_user_id`, `file_ids`等) | `qianfan.baidubce.com/v2/app/conversation/runs` | 自动 `remote_session` |
| `dify` | Dify 平台 | `APIKey`, `BaseURL` | `Model`, `ExtraBody` | `api.dify.ai/v1` | 自动 `remote_session` |
| `ragflow` | RAGFlow（自动适配原生/OpenAI 兼容端点） | `APIKey`, `BaseURL`(完整 endpoint，含 `chat_id`) | `Model`, `ExtraBody` | 无 | 自动 `remote_session` |
| `fastgpt` | FastGPT | `APIKey`, `BaseURL`, `SessionMode` | `ExtraBody` | 无 | 需显式指定 |

### 通用适配

| Provider | 说明 | 必填参数 | 可选参数 | 默认 BaseURL | SessionMode |
|----------|------|---------|---------|-------------|-------------|
| `generic` | 通用适配器 | `BaseURL`, `SessionMode`, `Request`, `Response` | `ChainFields`, `APIKey` | 无 | 需显式指定 |

**说明**：
- `BaseURL` 有默认值的 Provider 不传则直连官方 API，传入则切换到自建/代理地址
- `SessionMode`：`local_history` = SDK 管理历史；`remote_session` = 服务端管理会话。标注"自动"的无需手动指定
- `bailian_app`/`dify`/`ragflow`/`fastgpt` 的 `Model` 在平台侧配置，SDK 端可不传；`qianfan_app` 的 `Model` 填 app_id；`coze` 的 `Model` 填 bot_id
- `generic` 的 `Model` 语义由 `Request` 模板决定，不单独传
- `ragflow` 需在 `BaseURL` 中直接填写完整 endpoint，支持两种端点格式（SDK 自动识别）：
  - OpenAI 兼容端点：`/api/v1/chats_openai/{chat_id}/chat/completions`（推荐，需传 `Model`）
  - 原生端点：`/api/v1/chats/{chat_id}/completions`（使用 `question` 字段，支持 `session_id`）
- `fastgpt` 可通过 `ExtraBody` 传入 `detail`、`variables`

**通用可选参数**（所有 Provider 均支持）：`Stream`、`TimeoutSec`、`OnError`、`HistoryMaxMessages`、`HistoryMaxTokens`、`StartNewChat`、`AuthHeaders`、`QueryParams`

**连通性测试**：所有 Provider 均支持 `qs.Test(ctx)`，内部复用 `Send()` 主链路（流式模式），默认探测 prompt 为 `"return 1"`。探测会话无状态，不污染业务会话、不写入 Store。

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

## Provider 默认配置参考

以下是各 Provider 的默认 API 地址和推荐模型列表，可直接用于初始化配置：

### 基模

| # | 名称 | SDKProvider | API URL 示例 | 推荐模型 |
|---|------|-------------|-------------|---------|
| 1 | OpenAI | `openai` | `https://api.openai.com/v1` | gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o3-mini |
| 2 | Anthropic | `claude` | `https://api.anthropic.com` | claude-opus-4, claude-sonnet-4, claude-haiku-4 |
| 3 | Google Gemini | `gemini` | `https://generativelanguage.googleapis.com` | gemini-2.5-pro, gemini-2.5-flash, gemini-2.0-flash, gemini-2.0-flash-lite |
| 4 | DeepSeek | `deepseek` | `https://api.deepseek.com/v1` | deepseek-chat, deepseek-reasoner |
| 5 | Moonshot | `moonshot` | `https://api.moonshot.cn/v1` | moonshot-v1-8k, moonshot-v1-32k, moonshot-v1-128k |
| 6 | 阿里云 DashScope | `dashscope` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | qwen-max, qwen-plus, qwen-turbo, qwen3-235b-a22b, qwen3-30b-a3b |
| 7 | 火山引擎 | `volcengine` | `https://ark.cn-beijing.volces.com/api/v3` | doubao-pro-4k, doubao-lite-4k, doubao-pro-32k |
| 8 | 百度千帆 | `qianfan` | `https://qianfan.baidubce.com/v2` | ernie-4.5-8k, ernie-4.0-8k, ernie-3.5-8k, ernie-speed-8k |
| 9 | Ollama | `ollama` | `http://127.0.0.1:11434` | llama3.1, qwen2.5, deepseek-r1, gemma2, phi3 |

### 应用平台

| # | 名称 | SDKProvider | API URL 示例 | 说明 |
|---|------|-------------|-------------|------|
| 10 | Coze 扣子 | `coze` | `https://api.coze.cn/v3` | Model 填 bot_id；国际站用 `api.coze.com`；仅流式 |
| 11 | 阿里百炼应用 | `bailian_app` | `https://dashscope.aliyuncs.com/api/v2/apps/agent/{APP_ID}/compatible-mode/v1` | Model 可选，平台侧配置 |
| 12 | 百度千帆应用 | `qianfan_app` | `https://qianfan.baidubce.com/v2/app/conversation/runs` | Model 填 app_id |
| 13 | Dify | `dify` | `https://api.dify.ai/v1` | Model 在平台侧配置 |
| 14 | RAGFlow | `ragflow` | `http://{HOST}/api/v1/chats_openai/{CHAT_ID}/chat/completions` 或 `http://{HOST}/api/v1/chats/{CHAT_ID}/completions` | 自动识别端点格式；OpenAI 兼容端点需传 Model |
| 15 | FastGPT | `fastgpt` | `https://api.fastgpt.in/api/v1/chat/completions` | 需显式指定 SessionMode |

### 通用适配

| # | 名称 | SDKProvider | API URL 示例 | 说明 |
|---|------|-------------|-------------|------|
| 16 | 自定义接入 | `generic` | 无（需手动填写） | 贴入 HTTP 报文即可接入 |
| 17 | OpenAI 兼容 | `openai_compat` | 无（需手动填写） | 兼容 OpenAI 协议的第三方服务 |

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
