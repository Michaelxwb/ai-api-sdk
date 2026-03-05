# 错误处理

> 本 Go SDK 项目如何处理错误。

---

## 概览

本 SDK 遵循 Go 习惯用法的错误处理：错误作为值返回，内部绝不记录日志。SDK 是一个库——由调用方决定如何处理错误。

**关键原则**：SDK 使用 **sentinel errors**（用于预期情况）与 **fmt.Errorf wrapping**（用于意外失败）的混合方式。不定义任何自定义错误类型/structs。

---

## 错误类型

### Sentinel Errors（session/store.go）

```go
var (
    ErrSessionNotFound  = errors.New("session store: session not found")
    ErrSessionConflict  = errors.New("session store: version conflict")
    ErrSessionClosed    = errors.New("session store: session closed")
    ErrStoreUnavailable = errors.New("session store: unavailable")
)
```

这些是 SDK 中 **唯一** 的 sentinel errors。使用 `errors.Is()` 进行检查：

```go
state, err := store.Get(ctx, id)
if errors.Is(err, session.ErrSessionNotFound) {
    // expected: session doesn't exist yet
}
```

### 包级错误变量（provider/plugin/）

```go
var errNotConnected = errors.New("plugin: websocket not connected")
```

仅在包内使用的未导出 sentinel errors。

### 临时错误

多数错误以内联方式通过 `fmt.Errorf` 或 `errors.New` 创建：

```go
return nil, fmt.Errorf("client: provider spec %s not registered", specName)
return nil, errors.New("client: missing credential or provider config")
return nil, fmt.Errorf("auth manager: no credentials for provider %s", provider)
```

---

## 错误处理模式

### 模式 1：错误前缀约定

所有错误遵循 "package: description" 格式：

| 包 | 前缀 | 示例 |
|---------|--------|---------|
| `client` | `client:` | "client: status 404: ..." |
| `auth` (Manager) | `auth manager:` | "auth manager: no credentials for provider X" |
| `auth` (Store) | `credential store:` | "credential store: read failed: ..." |
| `session` | `session store:` | "session store: session not found" |
| `provider/plugin` | `plugin:` | "plugin: websocket not connected" |
| `auth` (JWT) | `jwt_sign:` | "jwt_sign: missing secret" |

### 模式 2：使用 %w 的错误包装

在 `auth/store.go` 中用于包装 OS/crypto 错误：

```go
return nil, fmt.Errorf("credential store: read failed: %w", err)
return nil, fmt.Errorf("credential store: scrypt failed: %w", err)
```

这允许调用方解包并检查底层原因。

### 模式 3：HTTP 状态码错误

在 `client/` 中将非 2xx 响应转换为错误：

```go
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    data, _ := io.ReadAll(resp.Body)
    return base.ChatResponse{}, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
}
```

### 模式 4：优雅降级（忽略非关键错误）

SessionStore 失败不会阻塞主响应：

```go
// In Session.Chat():
state, err := s.store.Get(ctx, s.id)
if err == nil && state != nil {
    historyMsgs = state.Messages
}
// Failed to load history → continue without it

_ = s.store.Save(ctx, state) // Save failure doesn't affect response
```

### 模式 5：nil 防护返回

接收 nil 参数的函数在安全时会提前返回且不报错：

```go
func (s BearerTokenStrategy) Apply(req *http.Request) error {
    if req == nil {
        return nil  // No-op, not an error
    }
    // ...
}
```

### 模式 6：OAuth 在 401 时重试一次

`AuthTransport.RoundTrip()` 对 OAuth 凭证在 401 时只重试一次：

```go
if resp.StatusCode == http.StatusUnauthorized && cred.AuthType == auth.AuthTypeOAuth {
    _ = resp.Body.Close()
    if err := t.Manager.RefreshOAuth(ctx, t.Cred); err != nil {
        return nil, err
    }
    return base.RoundTrip(req) // retry once
}
```

---

## 错误传播规则

1. **SDK core → 调用方**：错误直接返回，绝不吞掉（模式 4 除外）
2. **SDK 内不记录日志**：SDK 是库；核心包内 **必须不** 调用 `log.*`
3. **包装外部错误**：使用 `%w` 包装来自标准库/第三方的错误
4. **不包装内部错误**：SDK 内部错误直接返回
5. **始终加前缀**：每个错误字符串都以包/组件名开头

---

## API 错误响应

SDK 不定义 HTTP API 端点。来自 AI providers 的 HTTP 错误会被转换为 Go 错误：

```go
// Format: "client: status {code}: {response body}"
fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
```

调用方如有需要可从错误消息中解析状态码。

---

## 常见错误

### 错误 1：在 SDK core 内记录错误日志

**错误**：在 `auth/`、`client/` 等处添加 `log.Printf("error: %v", err)`

**正确**：返回错误，让调用方记录日志。

### 错误 2：无注释吞掉错误

**错误**：`_ = someFunc()` 没有任何说明

**正确**：`_ = s.store.Save(ctx, state) // Save failure doesn't affect response`

### 错误 3：不必要地创建自定义错误类型

**错误**：为简单场景定义 `type ProviderError struct { ... }`

**正确**：按前缀约定使用 `fmt.Errorf`。只有在调用方需要用 `errors.Is()` 检查条件时才创建 sentinel errors。

### 错误 4：不包装外部错误

**错误**：`return fmt.Errorf("credential store: read failed")`（丢失原始错误）

**正确**：`return fmt.Errorf("credential store: read failed: %w", err)`

### 错误 5：静默忽略 JSON 解析错误

若干 provider spec 在 `ParseResponse` 中使用 `_ = json.Unmarshal(...)`，当 API 响应结构变化时会返回空/部分结果而不是错误。这会使调试非常困难。

**错误**：`_ = json.Unmarshal(body, &result)`（静默失败）

**正确**：返回错误，让调用方知道响应格式不正确。

### 错误 6：未使用的 SessionConfig 错误钩子

`client.SessionConfig` 定义了 `OnStoreError` 回调和 `MaxConflictRetry`。`OnStoreError` 已接入 `Session.Chat()`/`ChatStream()`，store 保存失败时会调用回调。`MaxConflictRetry` 尚未接入。

---

## 已知反模式（技术债）

1. **OAuth 重试未重置 body**：`AuthTransport.RoundTrip()` 在 401 后重试时没有回退 `req.Body`，对非幂等 body 可能失败
2. ~~**RefreshOAuth 使用 context.Background()**~~ — ✓ 已修复：使用 30s 超时 HTTP client
3. ~~**%w 使用不一致**~~ — ✓ 已修复：`auth/manager.go`、`config/loader.go`、`client/transport.go` 已统一使用 `%w`
4. ~~**HTTP 状态错误不结构化**~~ — ✓ 已修复：使用 `APIError` 结构体，支持 `errors.As` 提取
5. ~~**APIError 信息泄露风险**~~ — ✓ 已修复：Body 截断到 4KB，超出部分追加 `...(truncated)`
6. ~~**成功响应无 LimitReader**~~ — ✓ 已修复：所有 Provider `ParseResponse` 统一使用 `io.LimitReader(4MB)`
