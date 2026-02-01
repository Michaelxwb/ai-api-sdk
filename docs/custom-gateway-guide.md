# 自定义网关配置指南
## 1. 概述
自定义网关是指实现了 OpenAI 兼容协议的第三方服务或私有网关，通常提供 `/chat/completions` 类似接口，但在路径、鉴权、请求体字段上存在差异。
为什么需要自定义网关配置：路径不一致、需要额外 Header/Cookie、请求体要注入自定义字段、以及多租户动态加载等。
适用场景包括：统一接入多个 OpenAI 兼容平台、企业内部模型网关、Web 控制台 Cookie 鉴权、SaaS 多租户配置管理。
## 2. 核心概念
### 2.1 Provider 配置（ProviderConfig）
Provider 用于描述“如何访问平台 API”，决定地址、路径、额外字段等差异。
```go
// ProviderConfig 配置 Provider 实例
// 字段会影响请求地址、Header 与请求体
// 来源：config/config.go
type ProviderConfig struct {
    Name      string            // provider 名称，调用时使用
    Type      string            // provider 类型，例如 openai / openai_compat
    BaseURL   string            // API 根地址，例如 https://api.xxx.com
    Path      string            // 自定义路径，例如 /pg/chat/completions
    AuthRef   string            // 绑定 Credential 的 ID
    Headers   map[string]string // 额外 Header（非认证也可放这里）
    ExtraBody map[string]any    // 额外请求体字段
}
```
关键点：
- Provider 解决“地址、路径、额外字段”的差异。
- Provider 通过 `AuthRef` 绑定 Credential。
### 2.2 凭证配置（Credential）
Credential 用于描述“如何认证”，支持多种认证策略。
```go
// Credential 统一认证结构
// 字段会被 AuthStrategy 使用并注入请求
type Credential struct {
    ID          string            // 凭证 ID
    Provider    string            // 指定 Provider 名称
    AuthType    AuthType          // none / bearer_token / api_key ...
    APIKey      string            // API Key
    AccessToken string            // Bearer Token
    Headers     map[string]string // 自定义 Header
    QueryParams map[string]string // 自定义 Query 参数
    Priority    int               // 优先级
    Disabled    bool              // 禁用开关
}
```
关键点：
- `AuthType` 决定使用哪种认证策略。
- `Headers` / `QueryParams` 可注入任意值（常用于自定义网关）。
### 2.3 认证类型（AuthType）
常用类型：
- `none`：不做标准认证，通常配合自定义 Header / Cookie。
- `bearer_token`：写入 `Authorization: Bearer <token>`。
- `api_key`：写入自定义 Header（可通过 metadata 控制）。
补充类型（SDK 已支持，本文不展开）：`oauth`、`jwt_sign`、`basic`、`mtls`。
### 2.4 配置优先级
请求构建过程（优先级从低到高）：
1. Provider 内置逻辑生成基础请求（Content-Type 等）。
2. ProviderConfig.Headers 写入请求 Header。
3. Credential.AuthType 写入标准认证（Bearer / API Key）。
4. Credential.Headers / QueryParams 再次注入（覆盖同名 Header）。
结论：Credential Header 优先级最高，`Authorization` 可能覆盖 Provider 同名 Header。
## 3. 配置方式对比
### 3.1 YAML 配置（适合开发/测试）
特点：可读性强、易调试、变更需重载配置；适合单租户或少量 Provider。
```yaml
# 本地 YAML 配置示例
# 适合开发/测试环境快速验证
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false
providers:
  - name: "gateway_dev"
    type: "openai_compat"
    base_url: "https://gateway.example.com"
    path: "/chat/completions"
    auth_ref: "gateway_dev_cred"
    extra_body:
      group: "default"
credentials:
  - id: "gateway_dev_cred"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_WITH_TOKEN"
```
### 3.2 内存构造（适合生产/动态场景）
特点：可动态组装配置、按租户注入 Token/Cookie、无需重启即可生效。
```go
// 内存构造方式示例
// 适合按租户动态生成配置
cfg := &config.Config{
    Providers: []config.ProviderConfig{
        {
            Name:    "gateway_runtime",
            Type:    "openai_compat",
            BaseURL: "https://gateway.example.com",
            Path:    "/chat/completions",
            AuthRef: "gateway_runtime_cred",
        },
    },
    Credentials: []*auth.Credential{
        {
            ID:          "gateway_runtime_cred",
            Provider:    "openai_compat",
            AuthType:    auth.AuthTypeBearerToken,
            AccessToken: "REPLACE_WITH_TOKEN",
        },
    },
}
```
### 3.3 数据库驱动（适合多租户）
特点：支持多租户动态加载、与后台管理系统联动、可统一治理与审计；实现思路：维护 `ai_providers` / `ai_credentials` 表，运行时加载并转换为 `config.Config`。
## 4. 字段详解
### 4.1 base_url（API 地址）
说明：`base_url` 是 API 根地址（协议 + 域名 + 可选前缀路径），SDK 会将其与 `path` 拼接；使用场景：网关地址非官方域名或内部反向代理。
```yaml
providers:
  - name: "gateway"
    type: "openai_compat"
    base_url: "https://gateway.example.com"   # 指向自定义网关
```
### 4.2 path（自定义路径）
说明：默认路径为 `/chat/completions`，不一致时需要显式配置；使用场景：网关前缀不同（如 `/pg/chat/completions`）或非标准路径。
```yaml
providers:
  - name: "gateway"
    type: "openai_compat"
    base_url: "https://gateway.example.com"
    path: "/pg/chat/completions"              # 覆盖默认路径
```
### 4.3 auth_type（认证类型）
说明：控制 SDK 采用哪种认证策略，并决定 Credential 字段的使用方式；使用场景：标准 Bearer Token、API Key Header、自定义 Header / Cookie。
```yaml
credentials:
  - id: "gateway_cred"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_WITH_TOKEN"
```
### 4.4 headers（自定义 Headers）
说明：可配置在 Provider 或 Credential；Provider 适合非认证固定 Header，Credential 适合认证相关 Header；使用场景：Cookie 鉴权、自定义用户头、网关签名 Header。
```yaml
credentials:
  - id: "gateway_cookie"
    provider: "openai_compat"
    auth_type: "none"                # 无标准认证
    headers:
      Cookie: "REPLACE_WITH_COOKIE"  # Cookie 认证
      New-Api-User: "REPLACE_WITH_USER_ID"
```
### 4.5 extra_body（额外请求字段）
说明：会与标准请求体合并，适合注入网关所需的扩展字段；使用场景：需要 `group` / `workspace` / `project` 等字段的平台。
```yaml
providers:
  - name: "gateway"
    type: "openai_compat"
    base_url: "https://gateway.example.com"
    path: "/chat/completions"
    auth_ref: "gateway_cred"
    extra_body:
      group: "default"              # 平台额外字段
      workspace: "team-a"
```
### 4.6 query_params（Query 参数）
说明：通过 Credential 配置，SDK 自动追加到 URL Query；使用场景：租户路由、版本号或网关分流参数。
```yaml
credentials:
  - id: "gateway_query"
    provider: "openai_compat"
    auth_type: "none"
    query_params:
      tenant: "team-a"               # Query 参数方式路由
      version: "2026-01"
```
## 5. 常见认证方式
### 5.1 Bearer Token（标准）
适用场景：大多数 OpenAI 兼容平台，Header 为 `Authorization: Bearer <token>`。
```yaml
credentials:
  - id: "bearer_cred"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_WITH_TOKEN"  # 替换为真实 Token
```
```go
// Bearer Token 认证（Go 示例）
cred := &auth.Credential{
    ID:          "bearer_cred",
    Provider:    "openai_compat",
    AuthType:    auth.AuthTypeBearerToken,
    AccessToken: "REPLACE_WITH_TOKEN", // 替换为真实 Token
}
```
### 5.2 API Key（Header）
适用场景：平台要求 `X-API-Key` 或自定义 Header 作为 API Key。
```yaml
credentials:
  - id: "api_key_cred"
    provider: "openai_compat"
    auth_type: "api_key"
    api_key: "REPLACE_WITH_API_KEY"    # 替换为真实 API Key
```
```go
// API Key 认证（Go 示例）
cred := &auth.Credential{
    ID:       "api_key_cred",
    Provider: "openai_compat",
    AuthType: auth.AuthTypeAPIKey,
    APIKey:   "REPLACE_WITH_API_KEY", // 替换为真实 API Key
    Metadata: map[string]any{
        "header_name":  "X-API-Key", // 自定义 Header 名称
        "header_prefix": "",         // 可选前缀
    },
}
```
### 5.3 Cookie（自定义 Headers）
适用场景：Web 控制台或代理接口使用 Cookie 会话。
```yaml
credentials:
  - id: "cookie_cred"
    provider: "openai_compat"
    auth_type: "none"
    headers:
      Cookie: "REPLACE_WITH_COOKIE"   # Cookie 放入自定义 Header
```
```go
// Cookie 认证（Go 示例）
cred := &auth.Credential{
    ID:       "cookie_cred",
    Provider: "openai_compat",
    AuthType: auth.AuthTypeNone,
    Headers: map[string]string{
        "Cookie": "REPLACE_WITH_COOKIE", // Cookie 认证
    },
}
```
### 5.4 自定义 Header（如 New-Api-User）
适用场景：网关要求固定用户标识或业务标识。
```yaml
credentials:
  - id: "user_header_cred"
    provider: "openai_compat"
    auth_type: "none"
    headers:
      New-Api-User: "REPLACE_WITH_USER_ID"   # 自定义 Header
```
### 5.5 无认证（公开 API）
适用场景：内部开放服务或无需认证的接口。
```yaml
credentials:
  - id: "public_cred"
    provider: "openai_compat"
    auth_type: "none"
```
## 6. 完整示例
### 6.1 elysiver 平台配置（YAML）
```yaml
# elysiver 平台配置（YAML）
# 说明：使用 Cookie + New-Api-User 作为认证
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false
providers:
  - name: "elysiver"
    type: "openai_compat"
    base_url: "https://elysiver.h-e.top"
    path: "/pg/chat/completions"
    auth_ref: "elysiver_cred"
    extra_body:
      group: "default"              # 平台需要的额外字段
credentials:
  - id: "elysiver_cred"
    provider: "openai_compat"
    auth_type: "none"              # 自定义 Header 认证
    headers:
      Cookie: "REPLACE_WITH_COOKIE"
      New-Api-User: "REPLACE_WITH_USER_ID"
```
### 6.2 elysiver 平台配置（内存构造）
```go
// elysiver 平台配置（内存构造）
// 注意：Cookie 与用户信息为占位符
cfg := &config.Config{
    Providers: []config.ProviderConfig{
        {
            Name:    "elysiver",
            Type:    "openai_compat",
            BaseURL: "https://elysiver.h-e.top",
            Path:    "/pg/chat/completions",
            AuthRef: "elysiver_cred",
            ExtraBody: map[string]any{
                "group": "default", // 平台额外字段
            },
        },
    },
    Credentials: []*auth.Credential{
        {
            ID:       "elysiver_cred",
            Provider: "openai_compat",
            AuthType: auth.AuthTypeNone,
            Headers: map[string]string{
                "Cookie": "REPLACE_WITH_COOKIE",
                "New-Api-User": "REPLACE_WITH_USER_ID",
            },
        },
    },
}
```
### 6.3 deepseek 平台配置（YAML）
```yaml
# deepseek 平台配置（YAML）
# 说明：标准 Bearer Token 认证
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false
providers:
  - name: "deepseek"
    type: "openai_compat"
    base_url: "https://api.deepseek.com"
    path: "/chat/completions"
    auth_ref: "deepseek_cred"
credentials:
  - id: "deepseek_cred"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_WITH_TOKEN"
```
### 6.4 deepseek 平台配置（内存构造）
```go
// deepseek 平台配置（内存构造）
// 注意：Token 为占位符
cfg := &config.Config{
    Providers: []config.ProviderConfig{
        {
            Name:    "deepseek",
            Type:    "openai_compat",
            BaseURL: "https://api.deepseek.com",
            Path:    "/chat/completions",
            AuthRef: "deepseek_cred",
        },
    },
    Credentials: []*auth.Credential{
        {
            ID:          "deepseek_cred",
            Provider:    "openai_compat",
            AuthType:    auth.AuthTypeBearerToken,
            AccessToken: "REPLACE_WITH_TOKEN",
        },
    },
}
```
### 6.5 OpenAI 兼容平台配置（通用模板）
```yaml
# OpenAI 兼容平台通用配置
# 可用于 vLLM / llama.cpp / HF TGI 等
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false
providers:
  - name: "openai_compat_generic"
    type: "openai_compat"
    base_url: "https://gateway.example.com"
    path: "/chat/completions"
    auth_ref: "compat_cred"
    extra_body:
      project: "demo"               # 平台额外字段
credentials:
  - id: "compat_cred"
    provider: "openai_compat"
    auth_type: "api_key"
    api_key: "REPLACE_WITH_API_KEY"
```
### 6.6 OpenAI 兼容平台配置（内存构造模板）
```go
// OpenAI 兼容平台通用配置（内存构造）
// 适合快速复制并修改
cfg := &config.Config{
    Providers: []config.ProviderConfig{
        {
            Name:    "openai_compat_generic",
            Type:    "openai_compat",
            BaseURL: "https://gateway.example.com",
            Path:    "/chat/completions",
            AuthRef: "compat_cred",
            ExtraBody: map[string]any{
                "project": "demo",
            },
        },
    },
    Credentials: []*auth.Credential{
        {
            ID:       "compat_cred",
            Provider: "openai_compat",
            AuthType: auth.AuthTypeAPIKey,
            APIKey:   "REPLACE_WITH_API_KEY",
            Metadata: map[string]any{
                "header_name": "X-API-Key",
            },
        },
    },
}
```
## 7. 数据库表结构
### 7.1 ai_providers 表设计
```sql
-- Provider 配置表
-- 用于存储多租户下的 Provider 实例
CREATE TABLE ai_providers (
  id            BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id     VARCHAR(64)  NOT NULL,        -- 租户 ID
  name          VARCHAR(64)  NOT NULL,        -- Provider 名称
  type          VARCHAR(32)  NOT NULL,        -- Provider 类型
  base_url      VARCHAR(255) NOT NULL,        -- API 根地址
  path          VARCHAR(255) NOT NULL,        -- 接口路径
  auth_ref      VARCHAR(64)  NOT NULL,        -- 关联 Credential ID
  headers_json  TEXT         NULL,            -- JSON: Provider.Headers
  extra_body    TEXT         NULL,            -- JSON: Provider.ExtraBody
  created_at    DATETIME     NOT NULL,
  updated_at    DATETIME     NOT NULL
);
```
### 7.2 ai_credentials 表设计
```sql
-- Credential 配置表
-- 用于存储多租户下的认证信息
CREATE TABLE ai_credentials (
  id             BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id      VARCHAR(64)  NOT NULL,       -- 租户 ID
  cred_id        VARCHAR(64)  NOT NULL,       -- Credential ID
  provider       VARCHAR(64)  NOT NULL,       -- Provider 类型或名称
  auth_type      VARCHAR(32)  NOT NULL,       -- 认证类型
  api_key        TEXT         NULL,           -- API Key
  access_token   TEXT         NULL,           -- Bearer Token
  headers_json   TEXT         NULL,           -- JSON: Credential.Headers
  query_json     TEXT         NULL,           -- JSON: Credential.QueryParams
  priority       INT          DEFAULT 0,      -- 选择优先级
  disabled       TINYINT      DEFAULT 0,      -- 是否禁用
  created_at     DATETIME     NOT NULL,
  updated_at     DATETIME     NOT NULL
);
```
### 7.3 从数据库动态加载配置的代码
```go
// 从数据库动态加载 Provider 与 Credential
// 说明：示例仅展示核心逻辑，需替换为实际 DAO
func LoadConfigFromDB(tenantID string) (*config.Config, error) {
    // 1. 查询 Provider 列表
    providers, err := queryProviders(tenantID)
    if err != nil {
        return nil, err
    }
    // 2. 查询 Credential 列表
    creds, err := queryCredentials(tenantID)
    if err != nil {
        return nil, err
    }
    // 3. 组装 Config
    cfg := &config.Config{
        Providers:   providers,
        Credentials: creds,
    }
    return cfg, nil
}
// queryProviders 查询 ai_providers 并转换为 ProviderConfig
func queryProviders(tenantID string) ([]config.ProviderConfig, error) {
    // TODO: 使用 SQL/ORM 查询表 ai_providers
    // TODO: headers_json / extra_body JSON 需要反序列化
    return []config.ProviderConfig{}, nil
}
// queryCredentials 查询 ai_credentials 并转换为 Credential
func queryCredentials(tenantID string) ([]*auth.Credential, error) {
    // TODO: 使用 SQL/ORM 查询表 ai_credentials
    // TODO: headers_json / query_json JSON 需要反序列化
    return []*auth.Credential{}, nil
}
```
## 8. 最佳实践
- 敏感信息加密存储：Token / Cookie 建议放在加密存储或 KMS 中。
- Cookie 过期处理：建议设置过期检测与自动刷新流程。
- 配置缓存策略：数据库配置建议加缓存，避免每次请求查询数据库；多平台统一管理：为每个平台定义统一命名规范与默认模型。
- 配置隔离：多租户场景下使用 tenant_id 做隔离。
- 凭证轮换：定期轮换 Token，并设置 `priority` 支持平滑切换。
- 监控与告警：对 401/403 等鉴权失败进行监控。
## 9. 常见问题
### Q: 如何处理 Cookie 过期？
A: 建议将 Cookie 与过期时间一起存储，并在每次请求前判断是否过期；若已过期则触发登录或刷新流程并更新 Credential。
### Q: 如何支持 HMAC 签名认证？
A: 在业务层生成签名后写入 `Credential.Headers`，或使用 `metadata` 存储签名参数，再由业务逻辑注入 Header。
### Q: extra_body 和 headers 的区别？
A: `extra_body` 合并到请求体 JSON；`headers` 写入 HTTP Header，通常用于认证或网关路由信息。
### Q: 如何实现多凭证轮询？
A: 使用 `auth.RoundRobinSelector` 作为选择器，并通过 `priority` / `disabled` 做轮询与灰度。
## 10. 小结
- 自定义网关配置的核心是 `base_url + path + auth_type`。
- `extra_body` 与 `headers` 可解决非标准平台差异。
- 多租户场景建议使用数据库驱动并配合缓存策略。
