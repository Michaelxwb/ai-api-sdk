# deepseek 平台配置文档

## 平台信息

- 名称: deepseek
- API 地址: https://api.deepseek.com
- 接口路径: /chat/completions
- 认证方式: Bearer Token

## 方式 1: YAML 配置

```yaml
# deepseek 平台配置（自定义网关示例）
# 说明：以下内容可直接保存为 config.yaml 使用

auth:
  store:
    type: file          # 凭证存储方式：file / db / memory
    path: "./credentials.json"
    encrypted: false

providers:
  - name: "deepseek"           # provider 名称（后续调用使用）
    type: "openai_compat"       # OpenAI 兼容协议
    base_url: "https://api.deepseek.com"   # API 根地址
    path: "/chat/completions"          # 接口路径
    auth_ref: "deepseek_cred"  # 关联凭证 ID

credentials:
  - id: "deepseek_cred"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_WITH_YOUR_TOKEN"  # 替换为真实 Token

```

## 方式 2: 内存构造（Go 代码）

```go
package main

import (
    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/config"
)

// deepseek 平台配置（内存构造方式）
// 注意：示例中的 Token / Cookie 为占位符，请替换为真实值。
func NewDeepseekConfig() *config.Config {
    return &config.Config{
        Auth: config.AuthConfig{
            Store: config.StoreConfig{
                Type:      "file",                // 凭证存储方式
                Path:      "./credentials.json",  // 凭证存储路径
                Encrypted: false,                  // 是否加密
            },
        },
        Providers: []config.ProviderConfig{
            {
                Name:    "deepseek",    // provider 名称
                Type:    "openai_compat",          // OpenAI 兼容协议
                BaseURL: "https://api.deepseek.com",          // API 根地址
                Path:    "/chat/completions",             // 接口路径
                AuthRef: "deepseek_cred", // 绑定凭证
            },
        },
        Credentials: []*auth.Credential{
            {
                ID:       "deepseek_cred", // 凭证 ID
                Provider: "openai_compat",           // Provider 类型
                AuthType: auth.AuthTypeBearerToken,
                AccessToken: "REPLACE_WITH_YOUR_TOKEN", // 替换为真实 Token
            },
        },
    }
}

```

## 使用示例

```go
package main

import (
    "context"

    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/client"
    "github.com/Michaelxwb/ai-api-sdk/config"
    "github.com/Michaelxwb/ai-api-sdk/provider/base"
)

func main() {
    // 方式一：从 YAML 加载配置
    cfg, _ := config.LoadConfig("config.yaml")

    // 方式二：从内存构造配置
    cfg = NewDeepseekConfig()

    // 初始化凭证管理器
    authStore := auth.NewFileStore(cfg.Auth.Store.Path)
    mgr, _ := auth.NewManager(authStore, &auth.RoundRobinSelector{})

    // 创建客户端
    cli := client.NewClient(cfg, mgr)

    // 发起请求
    resp, _ := cli.Chat(context.Background(), "deepseek", base.ChatRequest{
        Model: "deepseek-chat",
        Messages: []base.Message{
            {Role: "user", Content: "测试"},
        },
        Stream: false,
    })

    _ = resp
}
```

## 字段说明

- base_url: API 网关的根地址，通常是协议 + 域名，例如 https://example.com。
- path: 请求路径，默认是 /chat/completions；当网关路径不一致时需要覆盖。
- auth_type: 认证类型，支持 none / bearer_token / api_key；自定义 Header 使用 none + headers。
- headers: 自定义请求头（JSON），用于 Cookie、用户标识或网关签名等非标准认证信息。
- extra_body: 额外请求字段（JSON），会与标准请求体合并，例如 group / workspace / project。
- query_params: 追加到 URL 的 Query 参数（JSON），常用于租户或版本号。

## 注意事项

- 该平台返回流式响应（text/event-stream），如需非流式请在请求体中显式设置 stream=false。
