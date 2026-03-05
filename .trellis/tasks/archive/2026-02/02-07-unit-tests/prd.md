# Core Unit Tests

## Goal
为 SDK 核心包（auth、client、config、provider、streaming）添加完整的单元测试覆盖。

## Requirements
- 测试文件统一放在 `test/` 目录下，package 为 `test`
- 使用标准库 `testing` + `httptest`，不引入第三方测试框架
- 覆盖 happy path、边界条件、错误场景
- 每个测试用 `t.Run` 子测试组织

## Deliverables
- `test/auth_test.go` — Manager、FileStore、Strategy、Selector
- `test/client_test.go` — Errors、Transport、Session、Stream
- `test/config_test.go` — Loader、Config 解析
- `test/provider_test.go` — Registry、各 Provider Spec
- `test/streaming_test.go` — SSE、NDJSON 解析

## Acceptance Criteria
- [x] `go test ./test/...` 全部通过
- [x] 134 个测试用例，0 失败
- [x] 覆盖所有核心包的关键路径
