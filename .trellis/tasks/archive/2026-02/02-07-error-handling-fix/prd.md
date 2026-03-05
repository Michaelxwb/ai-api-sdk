# Error Handling Audit Fix

## Goal

修复 SDK 错误处理审计中发现的 13 个 WARNING 和 3 个 INFO 问题。按 P0 → P1 → P2 优先级分批实施。

## Requirements

### P0 — 必须修复

#### 1. ParseResponse 必须检查 json.Unmarshal 错误（WARNING #8）

5 个 Provider 的 `ParseResponse` 忽略 `json.Unmarshal` 错误，返回空 Text 而非 error。

**文件**:
- `provider/impls/openai/compat.go` — ParseResponse
- `provider/impls/claude/spec.go` — ParseResponse
- `provider/impls/gemini/spec.go` — ParseResponse
- `provider/impls/ollama/spec.go` — ParseResponse
- `provider/impls/plugin/spec.go` — ParseResponse

**修复**: 将 `_ = json.Unmarshal(...)` 改为检查错误并返回 `fmt.Errorf("<provider>: parse response failed: %w", err)`。

#### 2. 定义结构化错误类型体系（INFO #1）

全 SDK 无自定义 error type。需要定义基础错误类型让调用方能用 `errors.Is` / `errors.As` 做结构化处理。

**文件**: 新建 `client/errors.go`

**定义**:
```go
// APIError 表示 API 返回的 HTTP 错误
type APIError struct {
    StatusCode int
    Body       string
    Op         string // 操作描述，如 "chat", "stream"
}

// AuthError 表示认证相关错误
type AuthError struct {
    Op  string // 操作描述
    Err error  // 底层错误
}

// ParseError 表示响应解析错误
type ParseError struct {
    Provider string
    Err      error
}
```

### P1 — 应该修复

#### 3. HTTP 错误用 %w 包装，添加操作上下文（WARNING #1, #2）

**文件**:
- `client/client.go:101` — chatWith 返回 transport 错误时加上下文
- `client/stream.go:37` — chatWithStream 同上
- `client/transport.go:25,38` — AuthTransport 错误加上下文

**修复**: 用 `fmt.Errorf("client: <op>: %w", err)` 包装，并改用 `&APIError{}` 返回 HTTP 错误。

#### 4. io.ReadAll 加 io.LimitReader 限制大小（WARNING #9）

**文件**:
- `client/client.go:105`
- `client/stream.go:40`

**修复**: `io.ReadAll(io.LimitReader(resp.Body, 1<<20))` 限制为 1MB。

#### 5. config/loader.go 裸返回错误加上下文（WARNING #4）

**文件**: `config/loader.go:11,16`

**修复**: `fmt.Errorf("config: load %s: %w", path, err)`

#### 6. RefreshOAuth 裸返回底层错误（WARNING #3）

**文件**: `auth/manager.go:145,151,164`

**修复**: 用 `fmt.Errorf("auth manager: refresh oauth: %w", err)` 包装。

#### 7. Auth strategy Apply 错误被忽略（WARNING #7）

**文件**: `client/transport.go:29,42`

**修复**: 检查 `CustomHeaderStrategy.Apply()` 返回值，失败时返回 error。

### P2 — 可以改善

#### 8. OAuth refresh 用 http.DefaultClient 无超时（WARNING #12）

**文件**: `auth/manager.go:151`

**修复**: 使用传入的 ctx 创建带超时的 request，替换 `http.DefaultClient.Do(req)` 为 `(&http.Client{Timeout: 30 * time.Second}).Do(req)`。

#### 9. Session 历史持久化错误通过回调通知（WARNING #5）

**文件**: `client/session.go:186,308`

**修复**: 检查 `SessionConfig.OnStoreError` 是否已设置，若设置则调用。保持当前降级行为不变。

#### 10. Plugin readLoop 丢弃 ReadJSON 错误详情（WARNING #13）

**文件**: `provider/plugin/client.go:123`

**修复**: 将 ReadJSON 错误传递给 handler 或 close reason。

#### 11. 非 2xx 响应读取 body 时错误被忽略（WARNING #6）

**文件**: `client/client.go:105`, `client/stream.go:40`

**修复**: 此处已用 LimitReader 改进（P1 #4），body 读取失败时提供空字符串即可，影响很小。

#### 12. safeSend 静默吞掉 panic（WARNING #10）

**文件**: `provider/plugin/client.go:307-312`

**修复**: recover 后记录到 error channel 或加注释说明为什么需要 recover。

#### 13. Plugin 流式管道 writeEvent 失败被忽略（WARNING #11）

**文件**: `provider/plugin/transport.go:210,214,221`

**修复**: writeEvent 失败时通过 error chunk 通知调用方。

## Acceptance Criteria

- [ ] 5 个 Provider ParseResponse 检查 json.Unmarshal 错误
- [ ] 定义 APIError / AuthError / ParseError 结构化错误类型
- [ ] HTTP 错误返回 APIError（可用 errors.As 提取）
- [ ] io.ReadAll 加 LimitReader 限制 1MB
- [ ] config/loader.go 加路径上下文
- [ ] RefreshOAuth 错误加上下文包装
- [ ] Auth strategy Apply 错误不再被忽略
- [ ] OAuth refresh 使用带超时的 HTTP client
- [ ] Session store 错误通过 OnStoreError 回调通知
- [ ] `go build ./...` 编译通过
- [ ] 不改变任何公开 API 签名（向后兼容）

## Technical Notes

- 所有修改必须向后兼容：不改变函数签名、不删除已导出的 API
- 新错误类型定义在 `client/errors.go`
- 每个错误类型实现 `Error() string` 和 `Unwrap() error`
- 保持现有错误前缀约定（"package: description"）
- Session store 降级行为不变，只增加可选回调
