# Backend Code Quality & Performance

## Rules

- SDK core 包（`auth/client/config/provider/session`）禁止写日志；诊断信息必须通过 error 返回。
- 错误信息必须包含明确前缀（如 `client:`、`auth manager:`、`session store:`），并对外部错误使用 `%w` 包装。详见 `error-handling.md`。
- 禁止在核心链路使用 `panic` 处理可恢复错误。
- 所有执行 I/O 的方法必须将 `context.Context` 作为第一个参数并传递 deadline；无 deadline 时自动包 5min context（流式链路）。
- 读取 **HTTP 响应体**必须限制大小：成功响应用 `io.LimitReader(4MB)`，错误消息截断至 4KB。注意：读取 SDK 调用方传入的入站请求体（如 `plugin/transport.go` 中）不受此约束。
- 共享状态必须并发安全：`sync.RWMutex`（读多写少）或 `sync.Mutex`；interface 注释说明并发要求。
- 可变数据在返回或存储前必须克隆：`Credential.Clone()`、`cloneMessages()`。
- 自定义 Header/Query 输入必须校验 CRLF 注入（`\r\n` 字符检测）。

## Patterns

### 结构化错误类型
```go
// errors.As 提取 HTTP 错误
var ae *client.APIError
if errors.As(err, &ae) { /* ae.StatusCode, ae.Body */ }
```
三种错误类型：`APIError`（HTTP 非 2xx）、`ParseError`、`AuthError`。

### context 传递
```go
func (s *Store) Get(ctx context.Context, id string) (*SessionState, error)
func (c *Client) chatWith(ctx context.Context, ...) (ChatResponse, error)
```

### 并发安全模式
```go
type Manager struct {
    mu    sync.RWMutex
    cache map[string]*Credential
}
func (m *Manager) get(key string) *Credential {
    m.mu.RLock(); defer m.mu.RUnlock()
    return m.cache[key]
}
```

### SessionConfig 接入状态
```go
// ✅ 已接入：OnStoreError（session.go:441/579/640/784）
cfg.OnStoreError = func(ctx context.Context, err error) { slog.Warn("store save failed", "err", err) }
// ✅ 已接入：HistoryWindow（session.go:469）
// ⏳ 未接入：MaxConflictRetry、TruncatePolicy、AutoCreate（已定义但未在 Chat/ChatStream 中使用）
// ⏳ 未接入：SessionConfig.OnError（已定义，session.go 保存到 Meta 但从未读取来控制错误策略）
// ⏳ 未接入：WithOnError/Session.onError（同上，仅序列化到 state.Meta["on_error"]，不影响实际行为）
```

### 历史窗口裁剪
```go
client.WithHistoryWindow(20) // 最多保留最近 20 条消息
// session.Truncate 支持消息数 + token 预算两种策略
```

### 流式解析安全
```go
// bufio.Scanner + 行长上限（1MB），防超长帧拖垮解析器
scanner.Buffer(make([]byte, 1<<20), 1<<20)
```

## Review Checklist

提交前逐项确认：

- [ ] SDK core 包中无 `log.*` / `slog.*` / 第三方 logger
- [ ] 错误信息遵循 `"package: description"` 前缀约定
- [ ] 外部错误使用 `%w` 包装；SDK 内部错误直接返回
- [ ] 所有 I/O 操作传入 `context.Context`
- [ ] `io.ReadAll` 替换为 `io.ReadAll(io.LimitReader(resp.Body, 4<<20))`
- [ ] 错误消息中的外部响应体已截断至 4KB
- [ ] 共享状态访问并发安全（锁覆盖读写路径）
- [ ] nil 参数能被优雅处理（提前返回，不 panic）
- [ ] 可变切片在存储/返回前已克隆
- [ ] `json.Unmarshal` 错误不被 `_ =` 静默丢弃
- [ ] 自定义 Header/Query 已校验 CRLF（`strings.ContainsAny(v, "\r\n")`）
- [ ] 新增 Provider 注册路径完整：`init()` + `provider/provider.go` 空白导入
- [ ] 未引入包循环依赖
- [ ] `defer` 覆盖资源清理：`rows.Close()`、`resp.Body.Close()`、`tx.Rollback()`
- [ ] WebSocket 连接设有 `ReadLimit`（4MB）和 ReadDeadline

## Anti-Patterns

- `_ = json.Unmarshal(...)` 静默丢弃解析错误。
- 在错误消息中拼接完整外部响应体（应截断至 4KB）。
- 请求链路中吞掉错误且无注释/无回调。
- OAuth 重试时未重置 `req.Body`（非幂等 body 二次发送行为不确定）。
- 在高频流式循环中做重计算或无界缓存（延迟与内存持续升高）。
- 滥用全局可变状态（provider registry 是唯一允许的例外）。
