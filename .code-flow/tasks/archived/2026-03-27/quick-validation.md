# Tasks: Quick() Provider 参数校验

- **Source**: 对话分析（FastGPT 两种会话模式调研 + Quick() 兼容性深度分析）
- **Created**: 2026-03-27
- **Updated**: 2026-03-27 (completed)

## Proposal

`Quick()` 当前对缺失必要参数（如 BaseURL、SessionMode）不做任何检查，导致错误延迟到 `Send()` 发出 HTTP 请求时才暴露，且报错信息来自 provider 内部，调用方难以定位是配置问题还是运行时问题。

通过利用 `ProviderSpec.DefaultBaseURL()` 和 `ResolveConversationMode()` 两个已有方法推导校验规则，无需改动任何 provider 实现，即可让 `Quick()` 在调用时立即返回清晰的配置错误，覆盖所有现有及未来 provider。

---

## TASK-001: 修改 Quick() 签名支持返回 error

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: client/quick.go

### Description

`Quick()` 当前签名为 `func (c *Client) Quick(cfg ProviderConfig) *QuickSession`，无法传递校验失败的错误。需改为 `(*QuickSession, error)`，为后续参数校验层提供错误传递通道。

同步修改 `QuickSession` 上的方法（`Send`、`SendText`、`Test`）无需改动，仅签名本身需变更。

### Checklist
- [x] 将 `client/quick.go` 中 `Quick()` 返回值改为 `(*QuickSession, error)`
- [x] 正常路径末尾改为 `return &QuickSession{...}, nil`
- [x] 确认 `go build ./...` 通过（编译报错即为需适配的调用点）

### Log
- [2026-03-27] created (draft)
- [2026-03-27] started (in-progress)
- [2026-03-27] completed (done)

---

## TASK-002: 在 Quick() 中增加 provider 参数校验

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: client/quick.go, client/conversation.go, provider/base/spec.go

### Description

在 `Quick()` 函数体前段，利用现有接口信息推导两条通用校验规则：

1. `spec.DefaultBaseURL() == "" && cfg.BaseURL == ""`
   → provider 无内置 BaseURL 且调用方未提供，立即报错
   → 自动覆盖：fastgpt、ragflow、generic、plugin 等

2. `ResolveConversationMode(provider) == "" && cfg.SessionMode == ""`
   → provider 支持多种会话模式，无法自动推断，调用方必须显式指定
   → 自动覆盖：fastgpt、generic 等

规则基于已有接口，**无需修改任何 provider 实现**，未来新增 provider 只要正确实现 `DefaultBaseURL()` 和在 `ResolveConversationMode` 中注册，自动获得校验。

错误消息格式遵循项目约定（`client:` 前缀）：
- `"client: fastgpt requires BaseURL"`
- `"client: fastgpt requires explicit SessionMode (\"local_history\" or \"remote_session\")"`

### Checklist
- [x] 在 `Quick()` 开头调用 `base.Get(cfg.Provider)` 获取 spec，未注册时返回 `fmt.Errorf("client: provider %s not registered", cfg.Provider)`
- [x] 增加 BaseURL 校验：`spec.DefaultBaseURL() == "" && strings.TrimSpace(cfg.BaseURL) == ""`
- [x] 增加 SessionMode 校验：`ResolveConversationMode(cfg.Provider) == "" && cfg.SessionMode == ""`
- [x] 在 `test/` 下补充校验行为的单元测试（fastgpt 缺 BaseURL、fastgpt 缺 SessionMode、openai 不触发校验）

### Log
- [2026-03-27] created (draft)
- [2026-03-27] completed (done)

---

## TASK-003: 更新 examples/ 中所有 Quick() 调用方

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001
- **Source**: examples/（18 处调用）

### Description

`Quick()` 签名变更后，以下 examples 需适配新的 `(*QuickSession, error)` 返回值。
examples 只作为参考示例，统一用 `log.Fatal(err)` 处理即可，无需复杂错误处理。

涉及文件（18 处）：
- `examples/01-single-turn/main.go` (2 处)
- `examples/02-multi-turn/main.go`
- `examples/03-platform-integration/main.go`
- `examples/04-connectivity-test/main.go`
- `examples/dify/01-single-turn/main.go`
- `examples/dify/02-multi-turn/main.go`
- `examples/dify/03-platform-integration/main.go`
- `examples/dify/04-connectivity-test/main.go`
- `examples/07-fastgpt/01-single-turn/main.go`
- `examples/07-fastgpt/02-multi-turn/main.go` (2 处)
- `examples/07-fastgpt/03-platform-integration/main.go`
- `examples/07-fastgpt/04-connectivity-test/main.go`
- `examples/08-ragflow/01-single-turn/main.go`
- `examples/08-ragflow/02-multi-turn/main.go`
- `examples/08-ragflow/03-platform-integration/main.go`
- `examples/08-ragflow/04-connectivity-test/main.go`

### Checklist
- [x] 将所有 `qs := cli.Quick(...)` 改为 `qs, err := cli.Quick(...); if err != nil { log.Fatal(err) }`
- [x] 确认 `go build ./examples/...` 通过
- [x] 顺手检查 fastgpt examples 是否已有显式 `SessionMode`，补充缺失的

### Log
- [2026-03-27] created (draft)
- [2026-03-27] completed (done)
