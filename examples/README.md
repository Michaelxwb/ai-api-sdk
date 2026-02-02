# 示例代码

本目录提供完整的示例矩阵，覆盖以下维度：

- 对话模式：非流式（basic）/ 流式（streaming）
- 对话轮次：单轮（single-turn）/ 多轮（multi-turn）
- 会话存储（多轮）：Memory / File / SQLite / PostgreSQL / MySQL
- 凭证方式：YAML 配置 / 代码构建（programmatic）

## 示例矩阵

| 对话模式 | 对话轮次 | 存储方式 | 凭证方式 | 示例路径 |
| --- | --- | --- | --- | --- |
| 非流式 | 单轮 | - | YAML | `01-quickstart/main.go` |
| 流式 | 单轮 | - | YAML | `02-streaming/main.go` |
| 非流式 | 单轮 | - | 代码构建 | `03-programmatic-auth/main.go` |
| 非流式 | 单轮 | - | 环境变量/YAML | `dify/basic/main.go` |
| 流式 | 单轮 | - | 环境变量 | `dify/streaming/main.go` |
| 非流式 | 多轮 | Memory | YAML | `04-session-memory/basic/main.go` |
| 流式 | 多轮 | Memory | YAML | `04-session-memory/streaming/main.go` |
| 非流式 | 多轮 | Memory | 代码构建 | `04-session-memory/programmatic/main.go` |
| 非流式 | 多轮 | File | YAML | `05-session-file/basic/main.go` |
| 流式 | 多轮 | File | YAML | `05-session-file/streaming/main.go` |
| 非流式 | 多轮 | File | 代码构建 | `05-session-file/programmatic/main.go` |
| 非流式 | 多轮 | SQLite | YAML | `06-session-sqlite/basic/main.go` |
| 流式 | 多轮 | SQLite | YAML | `06-session-sqlite/streaming/main.go` |
| 非流式 | 多轮 | SQLite | 代码构建 | `06-session-sqlite/programmatic/main.go` |
| 非流式 | 多轮 | PostgreSQL | YAML | `07-session-postgres/basic/main.go` |
| 流式 | 多轮 | PostgreSQL | YAML | `07-session-postgres/streaming/main.go` |
| 非流式 | 多轮 | PostgreSQL | 代码构建 | `07-session-postgres/programmatic/main.go` |
| 非流式 | 多轮 | MySQL | YAML | `08-session-mysql/basic/main.go` |
| 流式 | 多轮 | MySQL | YAML | `08-session-mysql/streaming/main.go` |
| 非流式 | 多轮 | MySQL | 代码构建 | `08-session-mysql/programmatic/main.go` |
| 测试 | - | - | YAML | `09-connectivity-test/basic/main.go` |
| 测试 | - | - | 代码构建 | `09-connectivity-test/programmatic/main.go` |
| 非流式 | 单轮 | - | 代码构建 | `qwen-signature/main.go` |

## 快速索引

### 连通性测试
- [09-connectivity-test/basic](../examples/09-connectivity-test/basic/) - 基础测试（YAML 配置）
- [09-connectivity-test/programmatic](../examples/09-connectivity-test/programmatic/) - 代码构建凭证测试

## 运行示例前的准备

### 基础示例（无需额外依赖）
- 01-quickstart
- 02-streaming
- 03-programmatic-auth
- dify
- 04-session-memory
- 05-session-file
- 09-connectivity-test
- 10-custom-provider-huggingface
- qwen-signature

### 需要数据库驱动的示例

#### SQLite 示例
```bash
CGO_ENABLED=1 go run examples/06-session-sqlite/basic/main.go
```

#### PostgreSQL 示例
```bash
go run examples/07-session-postgres/basic/main.go
```

#### MySQL 示例
```bash
go run examples/08-session-mysql/basic/main.go
```

## 运行方式

1. 在 `config.example.yaml` 中填入真实凭证与 Provider 端点。
2. `cd` 到你要运行的示例目录。
3. 运行矩阵中列出的文件。

示例：

```bash
cd examples/01-quickstart
go run main.go
```

对于多轮示例：

```bash
cd examples/04-session-memory/basic
go run main.go
```

## 配置

`config.example.yaml` 包含：

- 认证存储设置（基于文件的 JSON；可选加密）
- Provider 定义（OpenAI/Claude/Gemini/vLLM 等）
- 由 provider `auth_ref` 引用的凭证定义

示例会通过 `../config.example.yaml` 读取配置，并将凭证存储在 `../credentials.json`（相对于示例目录）。

## 会话存储

实现位于 `examples/sessionstore/`。前置条件如下：

- Memory / File：无外部依赖
- SQLite：需要 CGO（用于 `github.com/mattn/go-sqlite3`）
- PostgreSQL：需要正在运行的数据库和有效的连接字符串
- MySQL：需要正在运行的数据库和有效的 DSN
- Redis：需要 Redis 实例

## 备注

- 流式示例会在响应到达时打印分片（打字机效果）。
- 多轮示例使用 `ChatSessionStreamSync`（非流式）或 `ChatSessionStream`（流式）。
- 代码构建示例在代码中构建凭证和 Provider 配置（不使用 YAML）。
