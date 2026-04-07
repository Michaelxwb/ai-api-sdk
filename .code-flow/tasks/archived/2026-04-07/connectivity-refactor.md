# Tasks: 连通性测试重构 — 复用 Send 主链路

- **Source**: .claude/plans/sprightly-swimming-whistle.md
- **Created**: 2026-04-07
- **Updated**: 2026-04-07



## Proposal

当前 `Test()` / `TestWith()` 维护了一条独立的测试执行链路，与 `QuickSession.Send()` 的业务链路是两条路径。修改 Chat/ChatStream 逻辑不会自动反映到 Test，存在"对话正常但测试失败"或反之的风险。本次重构删除 `client/test.go` 独立链路，Test 改为构造临时探测 QuickSession 后调用 `Send()`，实现真正的主链路复用，所有 Provider（包括仅流式的 Coze）均自动兼容，后续新增 Provider 零维护成本。

---

## TASK-001: 新建 connectivity.go — 类型定义 + 核心探测逻辑

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: plan#§3.1 新建 connectivity.go(L24-L58), plan#§3.4 临时探测 QuickSession 构造细节(L76-L122), plan#§3.5 stream 收集逻辑(L124-L147)

### Description

新建 `client/connectivity.go`，从 `client/test.go` 迁移 `TestOptions`、`TestResult` 类型定义和 `normalizeTestOptions`、`ensureTestContext` 辅助函数。实现 `Client.TestWith` 和 `Client.Test`，内部构造临时探测 `QuickSession`（`WithStore(nil)` + `WithHistoryMode(HistoryNone)` + `WithStartNewChat(true)` + `stream: true`），通过 `probeQS.Send()` 复用主链路。

### Checklist

- [x] 定义 `TestOptions` 结构体（Model, Timeout, Prompt），默认 Prompt 为 `"return 1"`
- [x] 定义 `TestResult` 结构体（Latency, Response）
- [x] 迁移 `normalizeTestOptions`（校验 Model，默认 Timeout=10s，默认 Prompt="return 1"）
- [x] 迁移 `ensureTestContext`（尊重调用方 deadline，否则用默认 timeout）
- [x] 实现 `Client.TestWith`：构造临时 QuickSession struct（`NewSessionWith` + 无状态选项），调用 `probeQS.Send(ctx, msgs)` 收集结果
- [x] 实现 `Client.Test`：从 config 构建 cred + pc 后委托 `TestWith`
- [x] 确认 `defaultTestTimeout = 10s` 常量迁移

### Log

- [2026-04-07] created (draft)
- [2026-04-07] started (in-progress)
- [2026-04-07] completed (done)

---

## TASK-002: 修改 quick.go — Test/TestText 方法

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001
- **Source**: plan#§3.2 修改 quick.go(L60-L70)

### Description

修改 `client/quick.go` 中的 `QuickSession.Test(ctx)` 方法，改为委托 `Client.TestWith`（connectivity.go 实现）。新增 `QuickSession.TestText(ctx, text)` 方法，支持自定义探测文本。

### Checklist

- [x] `QuickSession.Test(ctx)` 改为构造 `TestOptions{Model: qs.model}` 后调用 `qs.session.client.TestWith(ctx, qs.cred, qs.pc, &opt)`
- [x] Model 为空时回退为 `"test"`（保持现有行为）
- [x] 新增 `QuickSession.TestText(ctx, text)` — 同 Test 但 `TestOptions.Prompt = text`
- [x] 移除 quick.go 中对旧 test.go 的依赖（如有）

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)

---

## TASK-003: 删除 client/test.go

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-001, TASK-002
- **Source**: plan#§3.3 删除 test.go(L72-L74)

### Description

删除 `client/test.go` 整个文件。所有导出类型和方法已迁移到 `connectivity.go`，此文件不再需要。

### Checklist

- [x] 删除 `client/test.go`
- [x] `go build ./...` 编译通过
- [x] 确认无残留引用（grep `testStream`、`makeSession` 等旧内部函数名）

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)

---

## TASK-004: 清理旧测试文件 + 更新测试用例

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-003
- **Source**: plan#§4 测试更新(L149-L159)

### Description

清理 `test/client_test.go` 中依赖旧 `test.go` 路径的测试用例。更新 `TestBailianApp_QuickStoreAndTest` 和 `TestBailianApp_TestWith` 的 mock 服务器适配新流式路径。补充 `TestText` 和无状态验证用例。

### Checklist

- [x] 确认 `TestBailianApp_QuickStoreAndTest/quick_test_works_without_model` mock 返回 SSE 格式（已完成则验证）
- [x] 确认 `TestBailianApp_TestWith` mock 返回 SSE 格式（已完成则验证）
- [x] 补充 `TestQuickSession_TestText` 测试 — 自定义 prompt 通过 Send 链路发送
- [x] 补充无状态验证测试 — 确认探测会话不写入 SessionStore
- [x] 补充 `Client.Test` 向后兼容测试
- [x] 验证 bailian_app model 占位场景（Model 为空时用 "test"）
- [x] 验证 timeout 行为：默认 10s、ctx deadline 优先
- [x] `go test ./test/... -run TestBailianApp -v` 全部通过

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)

---

## TASK-005: 更新 README.md 文档

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-002
- **Source**: 新增（文档同步）

### Description

更新 `README.md` 中 Quick API 使用示例和连通性测试相关说明，反映 Test 复用 Send 主链路、默认 prompt 改为 `"return 1"`、新增 `TestText` 方法。

### Checklist

- [x] 更新"通用可选参数"或相关章节，说明 `Test()` / `TestText()` 方法
- [x] 如有连通性测试示例代码，更新默认 prompt 从 `"1"` 到 `"return 1"`
- [x] 确认 `examples/04-connectivity-test/` 目录引用与实际实现一致

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)

---

## TASK-006: 更新 docs/GUIDE.md 文档

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-002
- **Source**: 新增（文档同步）

### Description

更新 `docs/GUIDE.md` 使用指南中连通性测试章节，说明 `Test` / `TestText` 用法、探测会话无状态特性、全 Provider 流式兼容。

### Checklist

- [x] 更新连通性测试章节说明（Test 复用 Send 主链路）
- [x] 补充 `TestText` 方法的使用示例
- [x] 说明探测会话无状态特性（不污染业务会话、不写入 Store）

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)

---

## TASK-007: 全量验证 + 回归测试

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-004, TASK-005, TASK-006
- **Source**: plan#§5 验证(L161-L167)

### Description

运行全量测试确保重构无回归，验证 Quick API 兼容性。

### Checklist

- [x] `go build ./...` 编译通过（client/test.go 已删除）
- [x] `go test ./test/... -timeout 60s` 全量测试通过
- [x] `go vet ./...` 静态分析通过

### Log

- [2026-04-07] created (draft)
- [2026-04-07] completed (done)
