# Backend Logging

## Rules

- SDK core（`auth/client/config/provider/session`）禁止使用 `log`/`slog`/第三方 logger（zerolog/zap/logrus 等）。
- 核心链路的可观测性通过 error 返回值和结构化响应字段（`ChatResponse.Raw`、`StreamChunk`）承载，不通过日志副作用。
- 示例代码（`examples/`）可用 `log.Printf/log.Fatalf`，但禁止输出：API keys、token、完整凭证对象、主加密密钥、完整 HTTP 响应体。

## Patterns

### 诊断信息如何传递（SDK core 内）
| 机制 | 用途 |
|------|------|
| error 返回 | 所有失败以带前缀描述的 error 返回 |
| sentinel errors | `ErrSessionNotFound` 等允许调用方 `errors.Is()` 分支 |
| `ChatResponse.Raw` | 保留原始字节，供调用方调试用 |

### 示例代码日志约定
```go
log.Fatalf("Failed to load config: %v", err)      // 不可恢复启动错误
log.Printf("Error: %v", err)                       // 非致命错误
log.Printf("[ws] received reply_chunk, msgID=%s", msg.ID)   // 带组件前缀
log.Printf("[stream] SSE started, configID=%s", req.ConfigID)
```
组件前缀约定：`[ws]`（WebSocket）、`[stream]`（SSE streaming）。

### 调用方（使用 SDK 的业务应用）推荐
```go
resp, err := session.Chat(ctx, req)
if err != nil {
    slog.Error("chat failed", "provider", providerName, "error", err)
    return err
}
```
SDK 返回 error，调用方负责统一记录与脱敏。

## Anti-Patterns

- 在 SDK core 内为"方便调试"临时加入 `Printf` 并提交。
- 在日志中打印完整请求/响应 body（可能含 token、用户数据）。
- 同一错误既从 SDK 返回又在 SDK 内部打印（造成重复告警噪音）。
- 流式高频路径逐 chunk 打印日志（造成性能抖动）。
