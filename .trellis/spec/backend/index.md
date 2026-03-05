# 后端开发指南

> 该 Go SDK 项目（`github.com/Michaelxwb/ai-api-sdk`）的后端开发最佳实践。

---

## 概述

这是一个用于统一多供应商 AI 模型访问的 **Go SDK 库**。本 SDK 提供：
- 统一的认证管理（API Key、Bearer Token、OAuth、JWT、mTLS）
- 多供应商支持（OpenAI、Claude、Gemini、Ollama、Dify、Browser Plugin）
- 以流式为先的设计（SSE + NDJSON）
- 具备可插拔存储的会话管理

**核心原则**：SDK 定义 interface 和最小实现。存储驱动和平台服务器是 `examples/` 中的参考代码。

---

## 指南索引

| 指南 | 描述 | 状态 |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | 模块组织、依赖流向、命名约定 | 完成 |
| [Database Guidelines](./database-guidelines.md) | SessionStore interface、查询模式、schema 约定 | 完成 |
| [Error Handling](./error-handling.md) | 错误前缀约定、sentinel errors、包装模式 | 完成 |
| [Quality Guidelines](./quality-guidelines.md) | 代码标准、禁止模式、必需模式 | 完成 |
| [Logging Guidelines](./logging-guidelines.md) | SDK 核心不做 logging、示例 logging 约定 | 完成 |

---

## 快速参考

### 关键设计决策

1. **SDK 核心不做 logging** — 错误通过返回值传递
2. **小型 interfaces** — `SessionStore` 只有 3 个方法；扩展使用独立 interface
3. **Provider 自注册** — 通过 `init()` + 空白导入
4. **Functional options** — `WithStore()`、`WithID()`、`WithAutoID()` 模式
5. **优雅降级** — session store 故障不会阻塞聊天响应

### 错误前缀约定

| Package | 前缀 |
|---------|--------|
| `client` | `client:` |
| `auth` (Manager) | `auth manager:` |
| `auth` (Store) | `credential store:` |
| `session` | `session store:` |
| `provider/plugin` | `plugin:` |

### 禁止模式

- 在 SDK 核心 package 中记录日志
- 用 panic 代替返回错误
- 向稳定 interfaces 添加方法（使用可选的扩展 interfaces）
- 循环 package 依赖
- 从 SDK 核心导入 `examples/`

---

**语言**：所有文档都以 **中文** 编写。
