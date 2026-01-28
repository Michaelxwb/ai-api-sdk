# AI API SDK

统一模型接入 SDK，提供多平台认证管理、请求适配和凭证安全存储能力。

## 特性

- **统一认证管理** - 支持 API Key / Bearer Token / OAuth / JWT 签名 / Cookie 等多种认证方式
- **多平台适配** - 内置 OpenAI、Claude、Gemini、Ollama 及通用 OpenAI 兼容协议支持
- **凭证安全存储** - AES-256-GCM 加密 + scrypt KDF，支持环境变量 / 密钥文件管理 master key
- **多凭证轮转** - Round-Robin / Priority 选择策略，故障自动冷却切换
- **可配置扩展** - 自定义路径、请求头、请求体字段，无需写代码即可接入非标准网关

## 项目结构

```
ai-api-sdk/
├── auth/                   # 认证核心
│   ├── credential.go       # 统一凭证模型
│   ├── strategy.go         # 认证策略（NoAuth/Bearer/ApiKey/OAuth/JWT）
│   ├── store.go            # 凭证持久化（加密文件存储）
│   ├── manager.go          # 凭证管理（选择、刷新、轮转）
│   └── selector.go         # 选择策略（RoundRobin/Priority）
├── provider/               # 平台适配
│   ├── spec.go             # ProviderSpec 接口定义
│   ├── registry.go         # Provider 注册表
│   ├── openai.go           # OpenAI（含 Moonshot/DeepSeek）
│   ├── claude.go           # Anthropic Claude
│   ├── gemini.go           # Google Gemini
│   ├── ollama.go           # Ollama
│   └── openai_compat.go    # 通用 OpenAI 兼容（vLLM/llama.cpp/TGI/自定义网关）
├── client/                 # 统一客户端
│   ├── client.go           # Client 封装（统一 Chat 入口）
│   └── transport.go        # AuthTransport（http.RoundTripper 认证注入）
├── config/                 # 配置
│   ├── config.go           # 配置结构体
│   └── loader.go           # YAML 配置加载
└── examples/               # 示例
    ├── config.example.yaml # 示例配置
    ├── main.go             # 本地模式示例（Chat）
    └── chatwith/
        └── main.go         # 平台集成模式示例（ChatWith）
```

## 快速开始

### 安装

```bash
go get github.com/Michaelxwb/ai-api-sdk
```

### 配置

创建 `cp config.example.yaml config.yaml`：

```yaml
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false

providers:
  # 本地推理（无认证）
  - name: "local_llm"
    type: "openai_compat"
    base_url: "http://127.0.0.1:8080/v1"
    auth_ref: "none"

  # 云端 API
  - name: "openai"
    type: "openai"
    auth_ref: "openai_key"

credentials:
  - id: "none"
    auth_type: "none"

  - id: "openai_key"
    provider: "openai"
    auth_type: "bearer_token"
    access_token: "sk-your-api-key"
```

### 使用

```go
package main

import (
    "context"
    "fmt"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
    cfg, _ := config.LoadConfig("config.yaml")

    store := auth.NewFileStore(cfg.Auth.Store.Path)
    store.Encrypted = false

    mgr, _ := auth.NewManager(store, &auth.RoundRobinSelector{})
    for _, cred := range cfg.Credentials {
        mgr.Register(cred)
    }

    cli := client.NewClient(cfg, mgr)

    resp, err := cli.Chat(context.Background(), "openai", provider.ChatRequest{
        Model:    "gpt-4o-mini",
        Messages: []provider.Message{{Role: "user", Content: "hello"}},
    })
    if err != nil {
        fmt.Printf("error: %v\n", err)
        return
    }
    fmt.Println(resp.Text)
}
```

### 平台集成模式（ChatWith）

适用于平台自行管理凭证（如数据库存储），无需 `config.yaml` 和 `Manager`，直接传入凭证调用模型：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/provider"

    _ "github.com/Michaelxwb/ai-api-sdk/provider" // 注册所有 provider
)

func main() {
    cli := client.New() // 轻量构造，不依赖 config.yaml

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // 凭证由平台从数据库等外部来源构造
    cred := &auth.Credential{
        ID:          "deepseek-from-db",
        Provider:    "deepseek",
        AuthType:    auth.AuthTypeBearerToken,
        AccessToken: "sk-xxx", // 平台从数据库读取后传入
    }
    pc := &config.ProviderConfig{
        Name:    "deepseek",
        Type:    "deepseek",
        BaseURL: "https://api.deepseek.com/v1",
    }

    resp, err := cli.ChatWith(ctx, cred, pc, provider.ChatRequest{
        Model:    "deepseek-chat",
        Messages: []provider.Message{{Role: "user", Content: "Hello"}},
    })
    if err != nil {
        fmt.Printf("error: %v\n", err)
        return
    }
    fmt.Println(resp.Text)
}
```

**两种模式对比：**

| | `Chat`（本地模式） | `ChatWith`（平台集成模式） |
|---|---|---|
| 构造方式 | `client.NewClient(cfg, mgr)` | `client.New()` |
| 凭证来源 | `config.yaml` + `Manager` 管理 | 调用方直接传入 `*auth.Credential` |
| 适用场景 | CLI 工具、独立程序 | SaaS 平台、多租户后端 |
| 凭证轮转 | 内置 Round-Robin / Priority | 由平台自行管理 |

完整示例见 [`examples/chatwith/main.go`](examples/chatwith/main.go)。

## 支持的平台

| 平台 | type 值 | 认证方式 | 说明 |
|---|---|---|---|
| OpenAI | `openai` | Bearer Token | 含 GPT 系列 |
| Claude | `claude` | API Key (`x-api-key`) / OAuth | Anthropic |
| Gemini | `gemini` | API Key (`x-goog-api-key`) / OAuth | Google |
| DeepSeek | `deepseek` | Bearer Token | OpenAI 兼容 |
| Moonshot | `moonshot` | Bearer Token | OpenAI 兼容 |
| Ollama | `ollama` | 无 / Bearer | 本地，`/api/chat` 格式 |
| vLLM | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| llama.cpp | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| HF TGI | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| 自定义网关 | `openai_compat` | 自定义 Headers / Cookie | 通过 `path` + `extra_body` + `headers` 配置 |

## 接入自定义网关

对于非标准 OpenAI 兼容网关（如 New API / One API），通过配置即可接入：

```yaml
providers:
  - name: "my_gateway"
    type: "openai_compat"
    base_url: "https://gateway.example.com/pg"
    path: "/chat/completions"           # 自定义 endpoint 路径
    auth_ref: "gateway_key"
    headers:                            # 自定义请求头
      X-Custom-User: "12345"
    extra_body:                         # 注入到请求体的额外字段
      group: "default"
      tenant: "team-a"
```

## 认证类型

| auth_type | 说明 | 适用场景 |
|---|---|---|
| `none` | 无认证 | 本地推理端点 |
| `bearer_token` | `Authorization: Bearer <token>` | OpenAI / vLLM / DeepSeek |
| `api_key` | 自定义 Header 注入 | Claude (`x-api-key`) / Gemini (`x-goog-api-key`) |
| `oauth` | OAuth2 + refresh token（401 自动刷新） | 需要 token 续期的平台 |
| `jwt_sign` | HMAC-SHA256 签名 JWT | 智谱 GLM |

## 凭证加密存储

生产环境建议启用加密：

```yaml
auth:
  store:
    type: file
    path: "./credentials.enc.json"
    encrypted: true
    encryption:
      enabled: true
      algo: "AES-256-GCM"
      kdf: "scrypt"
      kdf_params:
        n: 32768
        r: 8
        p: 1
        key_len: 32
      master_key_env: "AI_SEC_EVAL_MASTER_KEY"
```

```bash
export AI_SEC_EVAL_MASTER_KEY="your-secret-master-key"
```

## 新增平台

实现 `provider.ProviderSpec` 接口并注册即可：

```go
package provider

func init() {
    Register("my_platform", &MyPlatformSpec{})
}

type MyPlatformSpec struct{}

func (s *MyPlatformSpec) Name() string                    { return "my_platform" }
func (s *MyPlatformSpec) DefaultBaseURL() string          { return "https://api.example.com" }
func (s *MyPlatformSpec) SupportedAuthTypes() []auth.AuthType { ... }
func (s *MyPlatformSpec) BuildRequest(ctx context.Context, opts BuildOptions, req ChatRequest) (*http.Request, error) { ... }
func (s *MyPlatformSpec) ParseResponse(resp *http.Response) (ChatResponse, error) { ... }
func (s *MyPlatformSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) { ... }
```

## 依赖

- `golang.org/x/crypto` - scrypt KDF
- `gopkg.in/yaml.v3` - YAML 配置解析
- Go 标准库 - AES-GCM、HTTP、JSON

## License

MIT
