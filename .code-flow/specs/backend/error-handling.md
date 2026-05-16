# Backend Error Handling

## Rules

- 所有错误字符串必须以 `"package: "` 前缀开头，标明来源包/组件：
  | Package | 前缀 |
  |---------|------|
  | `client` | `client:` |
  | `auth` (Manager) | `auth manager:` |
  | `auth` (Store) | `credential store:` |
  | `auth` (Selector) | `selector:` |
  | `auth` (JWT) | `jwt_sign:` |
  | `auth` (Strategy) | `auth:` |
  | `session` | `session store:` |
  | `config` | `config:` |
  | `provider/plugin` | `plugin:` |
  | Provider impls | `<name>:` 如 `fastgpt:`, `ragflow:`, `claude:`, `gemini:`, `ollama:`, `dify:`, `generic:` |
- 包装外部错误（标准库/第三方）必须用 `%w`，允许调用方 `errors.Is/As` 拆包。
- SDK 内部错误直接返回，不再包一层（避免双重前缀）。
- 禁止在 SDK core 包内记录日志；所有诊断通过 error 返回值传递给调用方。
- HTTP 非 2xx 响应必须转为带前缀错误，响应体截断至 4KB（`APIError.Body` 已处理）。
- Sentinel errors 只在调用方需用 `errors.Is()` 做分支时才定义；不要滥用。
- 不支持图片的 provider（C 组）必须在 `BuildRequest` 入口检测 `len(Parts)>0 && Type=="image_url"`，按自身前缀返回明确错误（不可走 `base.*` 通用 sentinel），便于调用方按 provider 分支：例 `generic: multimodal content not supported in template mode, use text-only messages`、`ragflow: image input not supported, provider only accepts text`、`deepseek: vision model not available, only text models supported`。

## Patterns

### 结构化错误类型（`client/errors.go`）
```go
// 可用 errors.As 提取
var ae *client.APIError
if errors.As(err, &ae) {
    fmt.Println(ae.StatusCode, ae.Body) // Body 已截断 4KB
}
```
三种错误类型：`APIError`（HTTP 非 2xx）、`ParseError`（响应解析失败）、`AuthError`（鉴权失败）。

### Sentinel Errors（`session/store.go`）
```go
var (
    ErrSessionNotFound  = errors.New("session store: session not found")
    ErrSessionConflict  = errors.New("session store: version conflict")
    ErrSessionClosed    = errors.New("session store: session closed")
    ErrStoreUnavailable = errors.New("session store: unavailable")
)
// 使用方式
if errors.Is(err, session.ErrSessionNotFound) { /* 创建新 session */ }
```

### Sentinel Errors（`provider/base/validate.go`，多模态校验）
```go
var (
    ErrUnsupportedImageFormat = errors.New("client: unsupported image format") // MIME 不在 PNG/JPEG/WEBP/GIF 白名单
    ErrEmptyImageData         = errors.New("client: empty image data")         // image_url 的 Data 字段为空
    ErrUnsupportedPartType    = errors.New("client: unsupported part type")    // Type 不在 knownPartTypes 白名单（拼错而非未实现）
)
// 调用方按语义分支
switch {
case errors.Is(err, base.ErrEmptyImageData):
    // 用户传了 image_url 但没填 Data → 数据缺失
case errors.Is(err, base.ErrUnsupportedImageFormat):
    // MIME 非白名单 → 数据格式问题
case errors.Is(err, base.ErrUnsupportedPartType):
    // Type 拼错（如 "image" 漏 "_url"）→ 调用方代码 bug
}
```
注意：`video_url` / `audio_url` 当前为预留占位，validate 放行，不在 `ErrUnsupportedPartType` 范围内。

### 优雅降级（SessionStore 失败不阻塞主响应）
```go
state, err := s.store.Get(ctx, s.id)
if err == nil && state != nil {
    historyMsgs = state.Messages
}
// 加载历史失败 → 继续，不报错给调用方

_ = s.store.Save(ctx, state) // Save 失败通过 OnStoreError 回调通知，不阻塞响应
```

### OAuth 401 单次重试（`client/transport.go`）
```go
if resp.StatusCode == http.StatusUnauthorized && cred.AuthType == auth.AuthTypeOAuth {
    _ = resp.Body.Close()
    if err := t.Manager.RefreshOAuth(ctx, t.Cred); err != nil {
        return nil, err  // 刷新失败直接返回
    }
    return base.RoundTrip(req) // 仅重试一次
}
```

### %w 包装示例
```go
return nil, fmt.Errorf("credential store: read failed: %w", err)   // 保留原因链
return nil, fmt.Errorf("client: provider %s not registered", name) // 内部错误无需 %w
```

## Anti-Patterns

- `_ = json.Unmarshal(...)` 静默丢弃解析错误（响应格式变化时调试极难）。
- 错误消息无前缀：`return errors.New("not found")` → 调用方无法定位来源包。
- 不必要地定义自定义错误类型（仅用 `fmt.Errorf` 即可的场景）。
- 不包装外部错误：`return fmt.Errorf("read failed")` 丢失原始 `err`（应加 `: %w`）。
- 在 SDK core 内 `log.Printf` 错误信息（应返回 error，让调用方决定如何处理）。
- 吞掉错误而无注释：`_ = someFunc()` 无任何说明（应注释说明为何可忽略）。
- 在错误消息中嵌入完整外部响应体（需截断至 4KB 防 OOM 与信息泄露）。

## Known Technical Debt

- `provider/impls/generic/rawpacket.go` 中 6 处内部错误缺少 `generic:` 前缀（如 `"response body is required"`、`"request text is empty..."`），违反上述规则，待修复。
- `client/transport.go:40` 的 OAuth 重试使用 `context.Background()` 而非调用方 ctx（会丢失调用方 deadline/cancel），待修复。但由于 `AuthTransport.Manager` 目前从未被设置（两处 `AuthTransport{...}` 构造均未填 Manager 字段），此分支实际不可达。
