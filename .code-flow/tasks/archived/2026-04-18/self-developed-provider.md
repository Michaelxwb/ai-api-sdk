# Tasks: self_developed Provider（自研多轮对话供应商接入）

- **Source**: .code-flow/tasks/2026-04-18/self_developed-jwt-provider-v2.design.md
- **Created**: 2026-04-20
- **Updated**: 2026-04-20

## Proposal

新增自研多轮对话供应商（`self_developed`），支持通过 JWT RS256 签名认证调用 lg.exe 平台的多轮对话接口。SDK 层职责：JWT token 透传、HTTP 请求组装、响应原样返回；业务层负责 JWT 生成和响应解析。

---

## TASK-001: 调整 jwtutil 包（与设计对齐）

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: self_developed-jwt-provider-v2.design.md#JWT 实现方案

### Description

现有 `pkg/jwtutil/jwt.go` 存在自定义 `data` 参数和 `UserID`/`Role` 字段，与设计文档不一致。简化实现，移除不必要的扩展。

### Checklist

- [x] 移除 `Claims` 结构体的 `UserID` 和 `Role` 字段
- [x] `GenerateToken` 函数签名改为 `GenerateToken(config SignConfig) (string, error)`
- [x] 移除函数内的 `data` 参数处理逻辑
- [x] 确保 `go build ./pkg/jwtutil/...` 编译通过

### Log

- [2026-04-20] created (draft)
- [2026-04-20] started (in-progress)
- [2026-04-20] completed (done)

---

## TASK-002: 实现 self_developed ProviderSpec

- **Status**: done
- **Priority**: P0
- **Depends**:
- **Source**: self_developed-jwt-provider-v2.design.md#Provider 实现

### Description

新建 `provider/impls/self_developed/spec.go`，实现 `ProviderSpec` 接口：
- `BuildRequest`: 从 `ExtraBody` 组装请求体，附加 `AuthHeaders`（含 JWT）
- `ParseResponse`: 原样返回 JSON 字符串，不解析业务字段
- `SupportsVision`: 返回 `false`
- `ParseStreamResponse`: 返回"不支持流式"错误

### Checklist

- [x] 创建 `provider/impls/self_developed/` 目录
- [x] 实现 `spec.go`：定义 `MultiroundSpec`，注册 `ProviderName = "self_developed"`
- [x] 实现 `BuildRequest`：URL 拼接（`baseURL + path`）、ExtraBody 序列化、附加 Headers
- [x] 实现 `ParseResponse`：读取响应体、检查状态码、原样返回 JSON 字符串
- [x] 创建 `stream.go`：占位文件，流式返回"不支持"错误
- [x] 确保 `go build ./provider/impls/self_developed/...` 编译通过

### Log

- [2026-04-20] created (draft)
- [2026-04-20] started (in-progress)
- [2026-04-20] completed (done)

---

## TASK-003: 注册 self_developed Provider

- **Status**: done
- **Priority**: P0
- **Depends**: TASK-002
- **Source**: self_developed-jwt-provider-v2.design.md#Provider 注册

### Description

在 `provider/provider.go` 添加空白导入 `self_developed`，触发 `init()` 注册。

### Checklist

- [x] 在 `provider/provider.go` 添加 `_ "github.com/Michaelxwb/ai-api-sdk/provider/impls/self_developed"`
- [x] 确保导入按字母序排列
- [x] 确保 `go build ./...` 编译通过

### Log

- [2026-04-20] created (draft)
- [2026-04-20] started (in-progress)
- [2026-04-20] completed (done)

---

## TASK-004: 编写单元测试

- **Status**: done
- **Priority**: P1
- **Depends**: TASK-001, TASK-002
- **Source**: self_developed-jwt-provider-v2.design.md#质量实现方案

### Description

为 jwtutil 和 self_developed provider 编写单元测试，验证 JWT 生成、响应解析、错误处理等关键路径。

### Checklist

- [x] `pkg/jwtutil/jwt_test.go`：测试 `GenerateToken` 成功路径、无效私钥路径
- [x] `provider/impls/self_developed/spec_test.go`：测试 `BuildRequest` URL 拼接和 Headers 附加
- [x] `provider/impls/self_developed/spec_test.go`：测试 `ParseResponse` 200 响应原样返回
- [x] `provider/impls/self_developed/spec_test.go`：测试 `ParseResponse` 非 200 状态码返回 `APIError`
- [x] 确保 `go test ./pkg/jwtutil/... ./provider/impls/self_developed/...` 全部通过

### Log

- [2026-04-20] created (draft)
- [2026-04-21] completed (done)