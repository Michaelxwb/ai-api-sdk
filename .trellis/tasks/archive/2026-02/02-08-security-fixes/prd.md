# Security Audit Fixes (M+L)

## Goal
修复安全审计中发现的 MEDIUM 和 LOW 级别问题（H1/H2 SSRF 校验已排除）。

## Requirements

### MEDIUM 优先级

#### M1: 加密降级防护（auth/store.go）
- `Encrypted=true` 时，`Load()` 如果检测到明文 JSON 数组应返回错误
- 只在 `s.Encrypted == true` 时生效，`Encrypted=false` 不受影响
- 错误消息：`"credential store: encrypted mode enabled but file is not encrypted"`

#### M2: ParseResponse 读取上限（5 个 Provider spec）
- 将 `ParseResponse` 中的 `io.ReadAll(resp.Body)` 改为 `io.ReadAll(io.LimitReader(resp.Body, 4<<20))`（4MB）
- 影响文件：openai/compat.go, claude/spec.go, gemini/spec.go, ollama/spec.go, dify/spec.go
- plugin/spec.go 也需要检查

#### M3: SSE 行长度限制（streaming/sse.go）
- 将 `ReadString('\n')` 改为 `bufio.Scanner` 并设置 `scanner.Buffer(buf, 1<<20)`（1MB 行上限）
- 超过阈值时返回错误 chunk

#### M4: WebSocket ReadLimit/ReadDeadline（plugin/client.go）
- 在 `readLoop()` 中设置 `conn.SetReadLimit(4 << 20)`（4MB）
- 在每次 ReadJSON 前设置 `conn.SetReadDeadline`（基于 config 中的超时）
- 对 ping/pong 保活不做要求（当前场景不需要）

#### M5: Stream HTTP 超时可配置（client/stream.go）
- 当前 `Timeout: 0`（无限），改为使用 `c.HTTP.Timeout` 或独立的 `StreamTimeout` 配置
- 如果 context 已有 deadline，尊重 context；否则使用默认 5 分钟
- 不改公共 API 签名

### LOW 优先级

#### L1: APIError body 截断（client/errors.go）
- `APIError.Body` 字段截断到 4KB
- 在创建 APIError 时截断，不改 struct 定义
- 截断时追加 "...(truncated)" 提示

#### L2: 示例 token 替换（examples/config.example.yaml）
- 将看似真实的 token 替换为明显的占位符
- `sk-b7B0x0APKfOnkW8sBU92ztOFzMW3I4W4iabNP9Hc04eavImH` → `sk-YOUR-TOKEN-HERE`
- `app-lwrLMC2HKTjz1jDLq2T02i4l` → `app-YOUR-DIFY-KEY-HERE`

#### L3: Header 注入校验（auth/strategy.go）
- 在 `CustomHeaderStrategy.Apply` 中校验 header name/value 不含控制字符（\r\n）
- 校验失败返回错误，不静默忽略

## Acceptance Criteria
- [ ] `go build ./...` 通过
- [ ] `go vet ./...` (SDK core) 通过
- [ ] `go test ./test/...` 全部通过（现有 134 个用例不能挂）
- [ ] 无公共 API 签名变更
- [ ] 无 interface 变更
