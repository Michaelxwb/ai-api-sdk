# Backend Directory Structure

## Rules

- 本仓库是 **Go SDK 库**，不是服务端应用；`auth/client/config/provider/session` 是核心包，`examples/` 仅作参考实现，不可被核心包导入。
- 依赖方向保持无环（DAG）：`provider/base` 与 `session` 是叶子包，不反向依赖上层业务包。
  - 依赖序：`provider/base` ← `auth` ← `config` ← `client`；`provider/impls/*` 只导入 `provider/base` + `streaming` + `auth`
- Provider 实现统一放在 `provider/impls/<name>/`，不得把具体平台逻辑散落到 `client/`。
- 会话存储契约只定义在 `session/store.go`；数据库驱动与存储实现放 `examples/sessionstore/`。
- 新增对外能力优先走 `client/` API 与 `SessionOption`，避免在调用方暴露内部细节结构。

## Patterns

### Provider 注册路径（必须完整）
```
provider/impls/<name>/spec.go   → func init() { base.Register("<name>", ...) }
provider/provider.go            → import _ ".../provider/impls/<name>"
```

### OpenAI 兼容 Provider（推荐方式）
满足以下条件时，直接在 `provider/impls/openai/spec.go` 的 `init()` 加一行注册，无需新建目录：
- 端点兼容 `/chat/completions` 路径
- 响应结构兼容 `choices[0].message.content` / `choices[0].delta.content`
- 流式使用 SSE + `[DONE]` 结束标记
- 认证为 `Authorization: Bearer <token>`

已有：`openai`、`moonshot`、`deepseek`、`dashscope`、`volcengine`

**注意**：`fastgpt` 和 `ragflow` 虽然 API 格式相近，但有自定义协议细节（会话 ID 字段、错误结构等），各自使用独立 `provider/impls/<name>/` 目录实现，而非 OpenAI compat 一行注册。

### 文件职责分工
| 文件 | 职责 |
|------|------|
| `spec.go` | Provider 协议实现，`BuildRequest`/`ParseResponse` |
| `stream.go` | 流式解析逻辑 |
| `types.go` | 类型定义与常量 |
| `config.go` | 配置结构与校验 |
| `store.go` | 存储 interface + 实现 |
| `helpers.go` | 共享工具函数 |

### 两层 API 设计
- `client/quick.go`：扁平参数 + 自动模式推断（Quick API，入门友好）
- `client/session.go`：细粒度控制（Session API，生产用）
- `SessionOption`（`WithStore/WithTimeout/WithHistoryWindow/WithOnError`）扩展行为，构造器签名保持稳定

### 可选扩展 interface 模式
```go
// Core（稳定，不动）
type SessionStore interface { Get(); Save(); Delete() }
// 可选扩展（新能力走独立 interface）
type SessionStoreAppender interface { Append() }
```

### 编译期 interface 验证
```go
var _ session.SessionStore = (*SQLiteStore)(nil)
```

## Anti-Patterns

- 在 `examples/` 中实现功能后被核心包 import 复用。
- 在 `client/` 内硬编码 provider 特殊分支，而不是下沉到 `provider/impls/<name>`。
- 把认证、流式解析、会话持久化写在同一文件形成"大文件耦合"。
- 为临时需求修改 `session.SessionStore` 核心方法签名，破坏现有实现兼容性。
- 在根目录或核心包新增无归属的临时脚本/实验代码。
- 新增 Provider 时只写 `spec.go` 而忘记 `provider/provider.go` 空白导入（导致 Provider 不生效）。
