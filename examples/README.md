# 示例代码

本目录提供统一的示例结构，涵盖：

- 单轮 / 多轮对话
- SessionStore 持久化（Memory / File / SQLite）
- 平台集成（NewSessionWith）
- 连通性测试（Test / TestWith）
- Dify 专用示例（conversation_id 自动管理）

## 目录结构

```
examples/
├── 01-single-turn/               # 单轮对话（HistoryNone）
├── 02-multi-turn/                # 多轮对话（HistoryAuto）
├── 03-platform-integration/      # NewSessionWith 平台集成示例
├── 04-connectivity-test/         # 连通性测试示例
├── 05-browser-plugin/            # 浏览器插件接入示例（统一 Session.Chat / ChatStream）
├── dify/
│   ├── 01-single-turn/           # Dify 单轮（conversation_id 自动提取）
│   ├── 02-multi-turn/            # Dify 多轮（conversation_id 自动复用）
│   ├── 03-platform-integration/  # Dify NewSessionWith 示例
│   └── 04-connectivity-test/     # Dify 连通性测试
├── plugin-platform/              # 浏览器插件平台服务示例
├── config.example.yaml
└── sessionstore/                 # 辅助实现（保留）
```

## 运行前准备

1. 在 `examples/config.example.yaml` 中配置可用的 Provider 与凭证。
2. 建议从仓库根目录运行示例（路径更清晰）：

```bash
go run ./examples/01-single-turn
```

（示例内部会自动尝试定位 `config.example.yaml`，因此也可在示例目录中 `go run main.go`。）

## 示例说明

### 01-single-turn
- 展示单轮对话的 7 个场景（无 Store / Memory / File / SQLite / MySQL / PostgreSQL / Redis）。
- 使用 `HistoryNone`（仅持久化，不加载历史）。
- MySQL / PostgreSQL / Redis 默认注释（需自行配置）。

### 02-multi-turn
- 展示多轮对话的 7 个场景。
- 使用 `HistoryAuto`，自动加载历史。
- 无 Store 时演示手动拼接历史。

### 单轮隔离（StartNewChat）
- **用途**：多次单轮对话互不依赖（不加载历史、不保存历史、不复用 SessionID）。
- 注意：不是所有 Provider 都识别 `session_id`，因此**单轮隔离应使用 `StartNewChat`**。

```go
resp, err := cli.NewSession(
    "openai",
    client.WithStartNewChat(true),
).Chat(ctx, base.ChatRequest{
    Messages:     []base.Message{{Role: "user", Content: "你好"}},
    StartNewChat: true,
})
```

### 03-platform-integration
- 展示 `NewSessionWith` 的平台集成用法。
- 适合从数据库/配置中心读取凭证的场景。

### 04-connectivity-test
- `Test()`：本地配置模式
- `TestWith()`：平台集成模式

### plugin-platform
- 浏览器插件平台服务（WebSocket + 控制台）
- 提供定位与消息转发能力

### 05-browser-plugin
- 使用浏览器插件接入（统一 Session.Chat / ChatStream）
- 依赖 `examples/plugin-platform` 服务与浏览器插件
- 需要提供 locators JSON（可从 `plugin-platform` 控制台导出）
- 支持 `-new`（单轮隔离）与 `-stream`（流式输出）

### Dify 示例
- Provider 为 `dify`
- `conversation_id` 由服务端生成，SDK 自动提取并复用
- 多轮示例展示了自动管理效果

## SessionStore 说明

- SessionStore 参考实现位于 `examples/sessionstore/`。
- Memory / File / SQLite / MySQL / PostgreSQL / Redis 均提供完整示例实现，可按需启用或复制到业务代码中。

## SQLite 依赖

SQLite 示例依赖 `github.com/mattn/go-sqlite3`，需启用 CGO：

```bash
CGO_ENABLED=1 go run ./examples/01-single-turn
```
