# 快速开始

本指南帮助你在 5 分钟内完成安装、配置并发起第一个请求。

## 1. 安装

```bash
go get github.com/Michaelxwb/ai-api-sdk
```

## 2. 创建配置文件

从示例配置复制一份：

```bash
cp examples/config.example.yaml config.yaml
```

最小化配置示例（仅演示结构）：

```yaml
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false

providers:
  - name: "openai"
    type: "openai"
    auth_ref: "openai_key"

credentials:
  - id: "openai_key"
    provider: "openai"
    auth_type: "bearer_token"
    access_token: "sk-your-api-key"
```

更多字段说明见 [配置指南](configuration.md)。

## 2.5 测试连通性（可选）

在发送正式请求前，可以先测试 Provider 配置和凭证是否有效：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Michaelxwb/ai-api-sdk/client"
    // ... 其他 import
)

func main() {
    // 创建客户端（同前面步骤）
    cli := client.NewClient(cfg, mgr)

    // 测试连通性
    result, err := cli.Test(context.Background(), "openai", &client.TestOptions{
        Model:   "gpt-4o-mini",
        Timeout: 10 * time.Second,
    })

    if err != nil {
        fmt.Printf("❌ 连通性测试失败: %v\n", err)
        return
    }

    fmt.Printf("✅ 连通性测试成功，延迟: %v\n", result.Latency)
}
```

**测试内容**：
- ✅ 网络连通性
- ✅ 凭证有效性
- ✅ 模型可用性

完整示例请参考 [examples/04-connectivity-test](../examples/04-connectivity-test/)

## 3. 第一个请求（非流式）

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"

    _ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
    cfg, _ := config.LoadConfig("config.yaml")
    authStore := auth.NewFileStore(cfg.Auth.Store.Path)
    mgr, _ := auth.NewManager(authStore, &auth.RoundRobinSelector{})
    for _, cred := range cfg.Credentials {
        mgr.Register(cred)
    }

    cli := client.NewClient(cfg, mgr)

    resp, err := cli.NewSession("openai").Chat(context.Background(), base.ChatRequest{
        Model:    "gpt-4o-mini",
        Messages: []base.Message{{Role: "user", Content: "Hello!"}},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Text)
}
```

## 4. 流式对话

```go
stream, err := cli.NewSession("openai").ChatStream(context.Background(), base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "用三句话介绍 Go"}},
})
if err != nil {
    log.Fatal(err)
}
for chunk := range stream {
    if chunk.Error != nil {
        log.Fatalf("stream error: %v", chunk.Error)
    }
    fmt.Print(chunk.Text)
}
```

如果你更喜欢同步返回，可直接使用 `Session.Chat()`。

## 5. 多轮对话（最小示例）

```go
import (
    "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
    "github.com/Michaelxwb/ai-api-sdk/session"
)

cli.SessionStore = sessionstore.NewMemoryStore()
cli.SessionConfig = client.SessionConfig{
    AutoCreate: true,
    TruncatePolicy: session.WindowPolicy{
        MaxMessages:      20,
        KeepSystemPrompt: true,
    },
}

sessionID := "user-001"

resp1, _ := cli.NewSession("openai", client.WithID(sessionID)).Chat(context.Background(), base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "介绍一下 Go 语言"}},
})

resp2, _ := cli.NewSession("openai", client.WithID(sessionID)).Chat(context.Background(), base.ChatRequest{
    Model:    "gpt-4o-mini",
    Messages: []base.Message{{Role: "user", Content: "它的并发模型是什么？"}},
})

fmt.Println(resp1.Text)
fmt.Println(resp2.Text)
```

多轮对话的完整教程见 [Session 完整指南](session-guide.md)。

## 6. 常见问题

- **配置文件加载失败**：确认 `config.yaml` 在当前目录，或传入绝对路径。
- **找不到凭证**：`providers[].auth_ref` 必须能在 `credentials` 中找到匹配的 `id`。
- **401 / 403**：检查 `access_token` 是否正确，或确认 provider 对应的 `auth_type`。
- **流式中断**：流式返回的 `StreamChunk.Error` 需要逐段处理。

## 相关文档
- [文档索引](README.md)
- [配置指南](configuration.md)
- [API 使用指南](api-guide.md)
- [Session 完整指南](session-guide.md)
- [示例代码](../examples/README.md)
