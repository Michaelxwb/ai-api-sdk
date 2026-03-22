# FastGPT 专用 Provider 实现

## Goal
在现有 SDK Provider 体系内实现 FastGPT 专用 Provider，覆盖 FastGPT 对话接口的请求构建、响应解析与流式处理差异，并保持与 SDK 统一约定的错误处理、配置映射与会话模式一致。

## Requirements
- 新增 FastGPT ProviderSpec + ProviderStreamSpec（provider/impls/fastgpt/ 下）
- BuildRequest 映射：messages/stream/extra_body；默认 path=/api/v1/chat/completions；Authorization: Bearer
- responseChatItemId 未传时生成 UUID；variables 映射到请求体
- 会话模式：local_history 不传 chatId 且拼接完整历史；remote_session 传 chatId 且不拼接历史
- ParseResponse：非流式取 choices[0].message.content；detail=true 解析 responseData tokens/usage
- Stream 解析：SSE data 行；event=answer/fastAnswer 产出文本增量；event=error 返回错误；event 为空按 answer；[DONE] 结束；跳过空行与 retry 行；忽略非 answer 事件
- 配置映射：base_url/path/headers/extra_body
- 错误前缀遵循 SDK 约定；核心包禁止日志；JSON 解析错误不静默；响应体读取使用 LimitReader 4MB
- 通过 init() 注册并在 provider/provider.go 添加空导入
- 测试：补充 test/ 下用例，覆盖 BuildRequest/ParseResponse/Stream 的关键路径；go test ./test/... 通过

## Acceptance Criteria
- provider type "fastgpt" 可注册并被 client 使用
- stream/detail 四种组合中关键路径可解析（至少覆盖 stream true/false + detail true/false）
- event=error 返回错误，[DONE] 正常结束
- responseChatItemId 默认生成 UUID
- go test ./test/... 通过

## Technical Notes
- 设计文档：docs/internal/design-fastgpt-chat-integration.md
- 遵循 .trellis/spec/backend/error-handling.md 与 quality-guidelines.md 约束
- 若涉及会话历史拼接/模式开关，优先复用现有 Session/HistoryWindow 机制；必要时在实现阶段再细化
