# 质量指南

> 后端开发的代码质量标准。

---

## 概览

这是一个 Go SDK 库。质量标准优先关注 **API 稳定性**、**interface 清晰性**、以及 **零依赖核心**。SDK 目前还没有 lint 配置、没有单元测试，也没有 CI 流水线。

---

## 代码风格约定

### Go 惯用法

- 标准 gofmt 格式化
- 内部说明注释用中文，对外导出的 API 用英文注释
- 允许混合语言注释（例如面向中文市场功能的中文描述：Dify/Qwen）

### 导出 API 文档

```go
// Good: English doc comments for exported symbols
// NewClient creates a client with local config and auth manager (Session mode).
func NewClient(cfg *config.Config, mgr *auth.Manager) *Client { ... }

// Acceptable: Chinese comments for internal methods
// chatWith 内部实现方法（仅供Session.Chat使用，业务层请使用Session API）
func (c *Client) chatWith(...) { ... }
```

### interface 设计

- **小 interface**：`SessionStore` 只有 3 个方法（Get/Save/Delete）
- **可选扩展**：用独立的 interface 表达可选能力
  ```go
  // Core (required)
  type SessionStore interface { Get(); Save(); Delete() }
  // Optional extension
  type SessionStoreAppender interface { Append() }
  ```
- **编译期验证**：使用空白赋值验证 interface 实现
  ```go
  var _ session.SessionStore = (*SQLiteStore)(nil)
  var _ session.SessionStoreAppender = (*SQLiteStore)(nil)
  ```

### Functional Options 模式

SDK 使用 `With<Option>()` 模式进行可选配置：

```go
type SessionOption func(*Session)

func WithStore(store session.SessionStore) SessionOption { ... }
func WithID(id string) SessionOption { ... }
func WithAutoID() SessionOption { ... }
func WithHistoryMode(mode HistoryMode) SessionOption { ... }
```

### Provider 注册模式

Provider 通过 `init()` + 空白导入自注册：

```go
// In provider/impls/openai/spec.go
func init() {
    base.Register("openai", NewOpenAICompatSpec("openai", "https://api.openai.com/v1"))
}

// In provider/provider.go (aggregator)
import _ "github.com/Michaelxwb/ai-api-sdk/provider/impls/openai"
```

---

## 禁止模式

### 1. 在 SDK Core 中记录日志

**禁止** 在 `auth/`、`client/`、`config/`、`provider/`、`session/` 中添加 `log.*`、`slog.*` 或任何 logger 调用。

### 2. 触发 panic

**禁止** 在 SDK 代码中使用 `panic()`。必须返回 errors。

### 3. 全局可变状态（registry 除外）

Provider registry（`provider/base/registry.go`）是唯一允许的全局可变状态。不得新增其它全局可变状态。

### 4. 包循环依赖

依赖图必须是 DAG。依赖流向见 [directory-structure.md](./directory-structure.md)。

### 5. 破坏 SessionStore interface

`SessionStore` interface（Get/Save/Delete）是稳定契约。新能力必须通过独立可选 interface 增加，不能修改核心 interface。

### 6. SDK Core 中导入 examples/

`examples/` 不是 SDK 的一部分。Core 包绝不能从 `examples/` 导入。

### 7. 静默 JSON Unmarshal 失败

**禁止** 在 Provider `ParseResponse` 中使用 `_ = json.Unmarshal(...)`。必须返回错误。

### 8. 无限制读取 HTTP Response Body

**禁止** 在任何路径使用裸 `io.ReadAll(resp.Body)`。必须使用 `io.LimitReader` 限制读取量：
```go
// Good
data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MB

// Bad
data, err := io.ReadAll(resp.Body) // 恶意服务可导致 OOM
```

### 9. 错误消息包含完整响应体

**禁止** 将不受信任的外部响应体完整嵌入错误消息。截断到合理上限（4KB）：
```go
if len(body) > 4096 {
    body = body[:4096] + "...(truncated)"
}
```

### 10. Header/Query 注入

**禁止** 未校验就将用户可控值设为 HTTP Header。必须检查 `\r\n` 控制字符：
```go
if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
    return fmt.Errorf("auth: header injection detected in %q", name)
}
```

---

## 必需模式

### 1. 错误前缀约定

每个错误必须以 "package:" 前缀开头。详见 [error-handling.md](./error-handling.md)。

### 2. context 传递

所有执行 I/O 的方法必须将 `context.Context` 作为第一个参数：

```go
func (s *Store) Get(ctx context.Context, id string) (*SessionState, error)
func (c *Client) chatWith(ctx context.Context, ...) (ChatResponse, error)
```

### 3. 线程安全

- 共享状态使用 `sync.Mutex` / `sync.RWMutex`
- `auth.Manager` 中的凭证缓存使用 `sync.RWMutex`
- `client.Session` 中的 Session ID 使用 `sync.Mutex`
- Provider registry 使用 `sync.RWMutex`
- 在 interface 注释中说明并发要求：
  ```go
  // SessionStore ... Implementations must be concurrency-safe; the SDK does not add locking.
  ```

### 4. nil 安全

函数必须优雅处理 nil 输入：

```go
func (c *Credential) IsExpired(now time.Time) bool {
    if c == nil || c.ExpiresAt == nil {
        return false
    }
    // ...
}
```

### 5. 存储前先克隆

存储或返回可变数据前，必须进行克隆：

```go
func (c *Credential) Clone() *Credential { ... }
func cloneMessages(msgs []session.Message) []session.Message { ... }
```

---

## 测试要求

### 当前状态

SDK 单测位于 `test/` 目录（外部测试包 `package test`）。

- 使用 Go 标准 `testing` + `httptest` 包，无第三方依赖
- 每个测试用 `t.Run` 子测试组织
- 运行命令：`go test ./test/...`

### 当前测试覆盖

| 测试文件 | 覆盖包 | 用例数 | 状态 |
|----------|--------|--------|------|
| `test/auth_test.go` | auth/ | Manager、FileStore（含加密）、5 种 Strategy、RoundRobin | ✅ |
| `test/client_test.go` | client/ | APIError/ParseError/AuthError、AuthTransport、Chat mock、Session、OnStoreError | ✅ |
| `test/config_test.go` | config/ | LoadConfig 正常/异常、YAML 字段映射、嵌套结构 | ✅ |
| `test/provider_test.go` | provider/ | Registry、6 个 Provider BuildRequest/ParseResponse/AuthStrategyOverride | ✅ |
| `test/streaming_test.go` | streaming/ | SSE、NDJSON、json_path、错误/EOF | ✅ |

**总计**：134 个用例，0 失败

### 待增加测试

| 包 | 优先级 | 关注点 |
|---------|----------|-------|
| `session/truncate.go` | 高 | WindowPolicy 及其边界情况 |
| URL 校验 | 高 | `base_url` SSRF 防护（安全审计 HIGH #1） |
| `auth/store.go` 加密降级 | 中 | `Encrypted=true` 时拒绝明文（安全审计 MEDIUM #1） |

---

## 代码评审检查清单

- [ ] SDK core 包中无日志
- [ ] 错误信息遵循前缀约定
- [ ] 所有 I/O 操作都传入 `context.Context`
- [ ] 共享状态访问是线程安全的
- [ ] nil 参数能被优雅处理
- [ ] 未引入循环依赖
- [ ] interface 新增为可选能力（独立 interface）
- [ ] Provider 通过 `init()` + 空白导入注册
- [ ] 使用 `defer` 做清理（Close、Rollback、Unlock）
- [ ] 可变数据在存储/返回前已克隆
- [ ] JSON unmarshal 错误不被静默丢弃
- [ ] HTTP response body 读取使用 `io.LimitReader`
- [ ] 错误消息中的外部数据已截断
- [ ] 自定义 Header/Query 值已校验（无 CRLF）
- [ ] 流式连接有超时保护（context deadline 或显式 timeout）
- [ ] WebSocket 连接设有 `ReadLimit`

---

## 已知技术债

1. ~~**无单元测试**~~ — ✓ 已修复：`test/` 目录下 5 个测试文件，134 个用例覆盖 auth/client/config/provider/streaming
2. ~~**SessionConfig 未使用**~~ — ✓ 已修复：`OnStoreError` 已接入 `Session.Chat()`/`ChatStream()`；`AutoCreate`、`TruncatePolicy`、`MaxConflictRetry` 尚未接入
3. ~~**静默 JSON 解析失败**~~ — ✓ 已修复：所有 Provider `ParseResponse` 现已检查 `json.Unmarshal` 错误
4. **OAuth 重试 body 问题** — `AuthTransport` 重试时未重置 `req.Body`
5. **生命周期 interface 缺口** — Store 实现在注释中引用了 "SessionStoreWithLifecycle/Meta"，但这些 interface 在 `session/` 中不存在

---

## 安全审计发现（2026-02-07）

### [HIGH] 高风险

| # | 位置 | 问题 | 修复状态 |
|---|------|------|----------|
| 1 | `client/prepare.go`、各 Provider `BuildRequest` | `base_url` 来自配置无校验，SSRF + 凭证外泄风险 | 不修复（SDK 定位为库，URL 校验由调用方负责） |
| 2 | `auth/manager.go:132-151` | OAuth `token_url` 未校验 scheme/域名，可明文泄露 refresh_token | 不修复（同上） |

### [MEDIUM] 中风险

| # | 位置 | 问题 | 修复状态 |
|---|------|------|----------|
| 1 | `auth/store.go` | ~~`Encrypted=true` 时仍接受明文 JSON~~ | ✓ 已修复：明文文件在加密模式下报错 |
| 2 | 各 Provider `ParseResponse` | ~~成功响应 `io.ReadAll` 无上限~~ | ✓ 已修复：统一 4MB `LimitReader` |
| 3 | `streaming/sse.go` | ~~SSE `ReadString('\n')` 无长度限制~~ | ✓ 已修复：改为 `bufio.Scanner` + 1MB 行上限 |
| 4 | `plugin/client.go` | ~~WebSocket 未设 `ReadLimit`/`ReadDeadline`~~ | ✓ 已修复：4MB ReadLimit + 可配置 ReadTimeout |
| 5 | `client/stream.go` | ~~流式 HTTP client `Timeout: 0`~~ | ✓ 已修复：无 deadline 时自动包 5min context |

### [LOW] 低风险

| # | 位置 | 问题 | 修复状态 |
|---|------|------|----------|
| 1 | `client/errors.go` | ~~`APIError` 包含完整响应 body~~ | ✓ 已修复：截断到 4KB |
| 2 | `examples/config.example.yaml` | ~~包含看似真实的 token~~ | ✓ 已修复：替换为占位符 |
| 3 | `auth/strategy.go` | ~~自定义 Header 无校验~~ | ✓ 已修复：CRLF 注入校验 |
