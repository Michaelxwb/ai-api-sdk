# Generic Adapter 重构实现

## Goal

按照 `docs/internal/design-generic-adapter-integration.md`（V2.5）完成 SDK 的跨层重构，
解决 `s.id` 语义冲突、`isDifyProvider` 特判、历史窗口未落地、流式 SessionID 丢失等问题。

## Requirements

### REQ-001 显式会话模式（ConversationMode）
- 新增 `ConversationMode` 类型（`remote_session` / `local_history`）
- `Session.Chat()` / `Session.ChatStream()` 按 mode 分支执行
- mode 校验为**惰性校验**（第二轮发起时触发），`Test()`/`TestWith()` 不受影响

### REQ-002 单一 SessionID 语义修正
- `remote_session`：首轮不注入 session_id；从响应提取后持久化；续轮注入
- `local_history`：应用端传入 session_id；查本地历史拼接；不传给目标模型
- 删除 `isDifyProvider()` / `isDifyName()` 特判函数

### REQ-003 非标准 API Raw Spec 接入
- 新增 `RawIntegrationSpec` / `CompiledIntegration` 数据结构
- 实现 `ParseRawIntegration()` 编译函数
- 实现 `Client.NewSessionFromRaw()` 入口
- URL 合成：缺 scheme 补 `https://`

### REQ-004 Generic Adapter 模板驱动
- 新增 `GenericProfile` 数据结构
- `{{input}}` 渲染前做 JSON 字符串转义
- SSE 解析：剥离 `data:` 前缀、识别 `[DONE]`、跳过空行和 `retry:` 行
- HTTP 响应自动处理 `Content-Encoding: gzip`

### REQ-005 鉴权标准化提取
- 实现 `ExtractCredential(headers map[string]string)` 函数
- 优先级：Bearer > Basic > Cookie/X-* > query params
- 冲突（Bearer + Basic 同时存在）直接报错
- 日志输出脱敏（Authorization/Cookie 仅保留前 4 字符 + `****`）

### REQ-006 OnError 策略可配置
- 新增 `OnErrorStrategy` 类型（`continue` / `abort`）
- 默认值：`abort`
- 在 `SessionConfig` 中新增该字段

### REQ-007 HistoryWindow 落地
- 扩展 `HistoryWindow`（MaxMessages + MaxTokens 双阈值）
- `local_history` 模式加载历史后裁剪再发送
- 落地 `session/truncate.go` 中已定义但未调用的 `TruncatePolicy`

### REQ-008 流式 SessionID 立即持久化
- `ChatStream()` goroutine 内：首个含 SessionID 的 chunk → 立即 `store.Save`
- 流中断时 SessionID 已持久化，不丢失
- 错误 body 读取（`stream.go:L57`）加 gzip 解压

### REQ-009 存储归档结构更新
- `SessionState.ID` 语义明确为单一 SessionID
- 移除 `Meta["remote_session_id"]` 冗余字段
- `Meta["mode"]` 记录会话模式

### REQ-010 自动化测试（test/ 目录）
- 每个 REQ 对应至少一个测试场景
- 覆盖：happy path、边界值、错误场景
- 流式解析测试需构造真实 SSE/NDJSON mock 响应

### REQ-011 示例代码更新
- 补充 `remote_session` / `local_history` 模式示例
- 补充 `NewSessionFromRaw` 非标准接入示例
- 受影响旧示例（dify、plugin 等）同步修改

## Acceptance Criteria

- [ ] `isDifyProvider()` 函数已删除
- [ ] `ConversationMode` 枚举已定义，Chat/ChatStream 按 mode 分支
- [ ] `ParseRawIntegration()` 可将 Raw Spec 编译为 GenericProfile
- [ ] `ExtractCredential()` 正确提取 Bearer/Basic/Cookie/query 鉴权
- [ ] `{{input}}` 模板渲染含引号/换行时 JSON 合法
- [ ] SSE 解析正确处理 `data:` / `[DONE]` / 空行 / `retry:`
- [ ] gzip 响应自动解压
- [ ] HistoryWindow MaxMessages + MaxTokens 双阈值生效
- [ ] 流中断后 SessionID 持久化不丢失
- [ ] `go test ./test/...` 全部通过
- [ ] 示例代码可正常运行

## Technical Notes

- 设计文档：`docs/internal/design-generic-adapter-integration.md`（V2.5）
- 重构顺序：数据结构层 → Session 执行层 → Provider 适配层 → 测试/示例/文档
- `Test()`/`TestWith()` 豁免 mode 校验，无需修改
- 不保持旧行为兼容（非兼容重构）
- SDK 核心不做 logging，错误通过返回值传递
