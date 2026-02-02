# 自定义认证与签名指南

本文档面向需要接入**非标准认证**（签名、动态 Token、私有网关等）的开发者，说明当前 SDK 的认证机制、可扩展点，以及千问（通义）“签名错误”问题的定位与解决思路。

## 1. 当前认证机制速览

### 1.1 认证策略接口
- 接口：`auth.AuthStrategy`（`auth/strategy.go`）
- 生效时机：每次请求发送前，由 `client.AuthTransport` 调用 `Apply` 注入认证头/参数（`client/transport.go`）。

### 1.2 当前内置 AuthType
`auth/credential.go` 中定义的 `AuthType`：
- `none`
- `api_key`
- `bearer_token`
- `oauth`
- `basic`（仅类型，未内置策略）
- `mtls`（仅类型，未内置策略）
- `jwt_sign`

`auth/strategy.go` 目前已实现：
- `BearerTokenStrategy`
- `ApiKeyHeaderStrategy`
- `CustomHeaderStrategy`
- `OAuthStrategy`
- `JWTSignStrategy`

### 1.3 Provider 级认证重写
`provider/base/spec.go` 提供 `AuthStrategyOverride`：
- 允许 Provider 根据具体协议/厂商定制认证逻辑。
- 目前 `openai_compat`、`claude`、`gemini`、`ollama` 已使用该能力实现 API Key 头或 Bearer 逻辑。

### 1.4 动态签名支持现状
- **支持动态**：`AuthStrategy.Apply` 每次请求都会执行，可在内部生成时间戳/随机数并计算签名。
- **未内置通用签名**：没有现成的 HMAC/AKSK 统一签名策略，需要新增策略实现。
- **Headers/QueryParams**：`Credential.Headers` / `Credential.QueryParams` 是静态注入，无法自动更新。

## 2. 千问签名错误的根因定位

你遇到的错误：
```
status 403: {"status":1,"code":"EX015","msg":"签名错误","data":{}}
```

对应的生成配置（`generated/qwen-config.md`）有以下特征：
- `auth_type: none`，且仅注入静态 Header / Cookie。
- `Clt-Acs-Sign`、`Clt-Acs-Reqt`、`nonce`、`timestamp` 等字段是**固定值**。

这意味着：
1. 该接口显然要求**动态签名**（时间戳 + nonce + 签名）。
2. 生成器将抓包得到的签名/时间戳/nonce“固化”到了配置里，导致签名立即过期。
3. 服务器验证签名不匹配，返回 `EX015`。

结论：这不是简单的 Bearer Token 问题，而是**非标准签名鉴权**导致的失败。

## 3. 官方可用路径（优先推荐）

如果目标是“稳定可用的千问 API”，建议改用 **DashScope 官方 API**（OpenAI 兼容模式），其认证方式是标准 API Key Bearer Token。官方文档明确了兼容模式的 base URL 与 API Key 认证方式，且不需要签名算法。

> 这可以直接绕开 `chat2.qianwen.com` 这种网页端内部接口的签名校验。

## 4. 三种可行方案设计（A/B/C）

### 方案 A：扩展 AuthStrategy（通用签名）
**适用**：HMAC、AK/SK 这类常见签名协议。

**改动点**：
- 新增 `auth/strategy_signature.go`
- 在 `auth/credential.go` 增加 `AuthTypeSignature` + `AccessKey/SecretKey` 字段
- 在 `auth/strategy.go` 的 `NewStrategyFromCredential` 里注册新策略

**优点**：
- 配置化，无需业务代码改动
- 统一入口、易维护

**缺点**：
- 不同平台签名规则差异大，需要设计灵活的签名 DSL 或插件

### 方案 B：RequestInterceptor（复杂自定义）
**适用**：复杂签名、动态 Token、多步骤认证。

**改动点**：
- 客户端增加 `RequestInterceptor` 接口
- 在 `client.Do` 流程中执行拦截器

**优点**：
- 极高自由度，完全自定义
- 不污染核心认证逻辑

**缺点**：
- 需要用户写代码
- 配置化能力弱

### 方案 C：专用 Provider
**适用**：协议与 OpenAI 完全不同的厂商。

**改动点**：
- 新建 `provider/impls/qwen`
- 在 `BuildRequest` 内部实现签名

**优点**：
- 定制化最强，控制力最大

**缺点**：
- 维护成本最高

## 5. 混合方案（最佳实践）

建议组合：
1. **通用签名** → AuthStrategy
2. **复杂逻辑** → RequestInterceptor
3. **特殊平台** → 专用 Provider

## 6. 实施优先级（建议）

**阶段 1（最短闭环）**
- 新增 `SignatureAuthStrategy`
- `Credential` 增加 `AccessKey/SecretKey` + `Metadata` 支持

**阶段 2（增强扩展性）**
- 引入 `RequestInterceptor`
- 支持配置化挂载拦截器

**阶段 3（按需定制）**
- 为常见平台实现专用 Provider

## 7. 快速修复路线

### 路线 1：切换官方 API（推荐）
- 使用 DashScope OpenAI 兼容 API
- 认证仅需 API Key（Bearer Token）

### 路线 2：保留 chat2.qianwen.com（高风险）
- 需自行抓包确定签名算法
- 动态生成 `nonce/timestamp/sign`
- Cookie / 设备指纹可能会过期

## 8. 示例

- **官方 API 示例**：`examples/qwen-signature/main.go`（默认模式）
- **Web 签名模板**：同文件内 `QWEN_MODE=web` 分支（需要你补齐真实签名算法）

## 9. 常见排查清单

- 时间戳单位：秒还是毫秒
- 签名字段排序是否一致
- 签名串是否包含 body 或 body hash
- Header 名称与大小写是否严格匹配
- nonce 生成是否符合服务端规则
- Cookie / session token 是否过期

---

如需进一步扩展认证，请结合 `auth/strategy.go`、`client/transport.go` 与 `provider/base/spec.go` 的扩展点。
