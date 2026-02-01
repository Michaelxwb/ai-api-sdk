# 配置指南

本指南介绍 `config.yaml` 的结构、Provider 配置、认证与加密存储。

## 配置文件结构

顶层结构包括：

- `auth`: 凭证存储与加密配置
- `providers`: Provider 实例列表
- `credentials`: 凭证列表（被 providers 引用）

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

## Provider 配置详解

```yaml
providers:
  - name: "openai"
    type: "openai"
    base_url: "https://api.openai.com/v1"   # 可选
    path: "/chat/completions"              # 可选
    auth_ref: "openai_key"
    headers:
      X-Custom-Header: "value"             # 可选
    extra_body:
      group: "default"                     # 可选
```

字段说明：

- `name`: Provider 实例名（调用 `Chat(ctx, name, ...)` 时使用）
- `type`: 内置 Provider 类型（如 `openai` / `claude` / `gemini` / `openai_compat`）
- `base_url`: 覆盖默认 API 地址（可选）
- `path`: 覆盖默认 endpoint（如 `/chat/completions`）
- `auth_ref`: 绑定凭证（对应 `credentials[].id`）
- `headers`: 追加到请求的自定义 Header
- `extra_body`: 注入到请求体的额外字段

### 支持的平台与类型

| 平台 | type 值 | 认证方式 | 说明 |
|---|---|---|---|
| OpenAI | `openai` | Bearer Token | GPT 系列 |
| Claude | `claude` | API Key / OAuth | Anthropic |
| Gemini | `gemini` | API Key / OAuth | Google |
| DeepSeek | `deepseek` | Bearer Token | OpenAI 兼容 |
| Moonshot | `moonshot` | Bearer Token | OpenAI 兼容 |
| Ollama | `ollama` | 无 / Bearer | 本地 `/api/chat` 格式 |
| vLLM | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| llama.cpp | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| HF TGI | `openai_compat` | 无 / Bearer | OpenAI 兼容 |
| 自定义网关 | `openai_compat` | 自定义 Headers / Cookie | 通过 `path` + `extra_body` + `headers` 配置 |

### 接入自定义网关

```yaml
providers:
  - name: "my_gateway"
    type: "openai_compat"
    base_url: "https://gateway.example.com/pg"
    path: "/chat/completions"
    auth_ref: "gateway_key"
    headers:
      X-Custom-User: "12345"
    extra_body:
      group: "default"
      tenant: "team-a"
```

## 认证配置

```yaml
auth:
  store:
    type: file
    path: "./credentials.json"
    encrypted: false
```

- `type`: 当前实现为 `file`
- `path`: 凭证文件路径（JSON）
- `encrypted`: 是否启用加密存储

## 凭证管理

凭证通过 `credentials` 列表声明，并由 `providers[].auth_ref` 引用：

```yaml
credentials:
  - id: "openai_key"
    provider: "openai"
    auth_type: "bearer_token"
    access_token: "sk-your-api-key"

  - id: "claude_key"
    provider: "claude"
    auth_type: "api_key"
    api_key: "sk-ant-xxx"
```

字段说明：

- `id`: 凭证 ID（Provider 通过 `auth_ref` 引用）
- `provider`: 建议与 `providers[].name` 一致；为空则对所有 Provider 可用
- `auth_type`: `none` / `bearer_token` / `api_key` / `oauth` / `jwt_sign`
- `access_token` / `api_key`: 认证信息
- `headers` / `query_params`: 自定义 Header/Query 注入
- `priority` / `disabled`: 选择策略与禁用控制

注意：如果 `credentials[].provider` 与 `providers[].name` 不一致，`Manager` 在解析凭证时可能无法匹配。需要按实例区分时请保持一致。

## 加密配置

生产环境建议启用加密存储：

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
      master_key_env: "AI_SDK_MASTER_KEY"
      # master_key_file: "/path/to/master.key"
```

```bash
export AI_SDK_MASTER_KEY="your-secret-master-key"
```

## 完整配置示例

以下示例来自 `examples/config.example.yaml`：

```yaml
auth:
  store:
    type: file
    path: "../credentials.json"
    encrypted: false
    encryption:
      enabled: false
      algo: "AES-256-GCM"
      kdf: "scrypt"
      kdf_params:
        n: 32768
        r: 8
        p: 1
        key_len: 32
      master_key_env: "AI_SDK_MASTER_KEY"

providers:
  - name: "vllm_local"
    type: "openai_compat"
    base_url: "https://integrate.api.nvidia.com/v1"
    auth_ref: "vllm_bearer"

  - name: "openai_cloud"
    type: "openai"
    auth_ref: "openai_key"

  - name: "claude_cloud"
    type: "claude"
    auth_ref: "claude_key"

  - name: "gemini_cloud"
    type: "gemini"
    auth_ref: "gemini_key"

  - name: "gateway"
    type: "openai_compat"
    base_url: "https://your-gateway.example.com/v1"
    path: "/chat/completions"
    auth_ref: "gateway_token"
    headers:
      X-Project: "demo"
    extra_body:
      group: "default"

  - name: "deepseek_cloud"
    type: "deepseek"
    auth_ref: "deepseek_key"

credentials:
  - id: "none"
    auth_type: "none"

  - id: "vllm_bearer"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE"

  - id: "openai_key"
    provider: "openai"
    auth_type: "bearer_token"
    access_token: "sk-REPLACE"

  - id: "claude_key"
    provider: "claude"
    auth_type: "api_key"
    api_key: "sk-ant-REPLACE"

  - id: "gemini_key"
    provider: "gemini"
    auth_type: "api_key"
    api_key: "AIza-REPLACE"

  - id: "gateway_token"
    provider: "openai_compat"
    auth_type: "bearer_token"
    access_token: "REPLACE_ME"

  - id: "deepseek_key"
    provider: "deepseek"
    auth_type: "bearer_token"
    access_token: "sk-REPLACE"
```

## 相关文档
- [文档索引](README.md)
- [快速开始](quickstart.md)
- [使用指南](usage-guide.md)
- [API 参考](api-reference.md)
