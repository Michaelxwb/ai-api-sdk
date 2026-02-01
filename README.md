# AI API SDK

> 统一 AI 模型接入 SDK，支持多平台认证、流式对话和会话管理

## 核心特性

- 🔐 **统一认证管理** - 多种认证方式，凭证加密存储
- 🌐 **多平台适配** - OpenAI、Claude、Gemini、Ollama 及兼容协议
- 💬 **流式优先** - 原生流式支持，打字机效果
- 🔄 **多轮对话** - 会话管理，支持多种存储方案
- ⚙️ **灵活配置** - YAML 配置或代码构建

## 安装

```bash
go get github.com/Michaelxwb/ai-api-sdk
```

**注意**：SDK 包含数据库驱动依赖（SQLite、PostgreSQL、MySQL、Redis），但仅在使用对应存储时才会编译进二进制。如果只使用内存或文件存储，这些依赖不会影响最终二进制大小。

## 快速开始

### 最简示例

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

    cli := client.NewClient(cfg, mgr)

    resp, err := cli.ChatStreamSync(context.Background(), "openai", base.ChatRequest{
        Model:    "gpt-4",
        Messages: []base.Message{{Role: "user", Content: "Hello!"}},
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.Text)
}
```

## 项目结构

```
ai-api-sdk/
├── provider/          # Provider 适配层
│   ├── base/         # 核心接口
│   ├── streaming/    # 流式基础设施
│   └── impls/        # Provider 实现
├── client/           # 统一客户端
├── auth/             # 认证管理
├── session/          # 会话管理
├── config/           # 配置加载
├── docs/             # 详细文档
└── examples/         # 完整示例
```

## 文档

- 📚 [完整文档索引](docs/README.md)
- 🚀 [快速开始指南](docs/quickstart.md)
- ⚙️ [配置说明](docs/configuration.md)
- 📖 [使用指南](docs/usage-guide.md)
- 🔧 [API 参考](docs/api-reference.md)
- 💡 [示例代码](examples/README.md)

## 示例场景

查看 [examples/](examples/) 目录，包含：
- 单轮对话（流式 / 非流式）
- 多轮对话（Memory / File / SQLite / PostgreSQL / MySQL）
- 代码构建凭证（无需配置文件）

完整示例矩阵请参考 [examples/README.md](examples/README.md)

## License

MIT License
