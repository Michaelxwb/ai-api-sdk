# 数据库指南

> 会话存储的数据库模式与约定。

---

## 概览

SDK 核心仅为会话存储定义**接口**。它不包含任何数据库驱动或 ORM 库。

- 核心接口：`session.SessionStore`（Get/Save/Delete）
- 可选扩展：`session.SessionStoreAppender`（Append）
- 参考实现：`examples/sessionstore/`（Memory、File、SQLite、MySQL、PostgreSQL、Redis）

`go.mod` 中的数据库驱动**仅供示例使用**，SDK 核心不使用。

---

## 接口契约

### 核心接口（session/store.go）

```go
type SessionStore interface {
    Get(ctx context.Context, id string) (*SessionState, error)
    Save(ctx context.Context, state *SessionState) error
    Delete(ctx context.Context, id string) error
}
```

**规则**：
- 实现必须是并发安全的（SDK 不会加锁）
- 会话不存在时，`Get` 返回 `ErrSessionNotFound`
- `Save` 执行 upsert：不存在则创建，存在则更新
- 会话不存在时，`Delete` 返回 `ErrSessionNotFound`

### 可选扩展

```go
type SessionStoreAppender interface {
    Append(ctx context.Context, id string, msgs ...Message) error
}
```

### 哨兵错误

```go
ErrSessionNotFound  // Session does not exist
ErrSessionConflict  // Optimistic locking version mismatch
ErrSessionClosed    // Session closed for further writes
ErrStoreUnavailable // Storage backend unavailable
```

---

## 查询模式

### SQLite 参考实现（examples/sessionstore/sqlite.go）

**表结构**：
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at INTEGER NOT NULL,  -- Unix timestamp
    updated_at INTEGER NOT NULL,  -- Unix timestamp
    attrs TEXT                     -- JSON string
);

CREATE TABLE IF NOT EXISTS session_messages (
    id INTEGER PRIMARY KEY,        -- Auto-increment
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);
```

**Save 模式**（在事务内全量替换）：

```go
tx, err := s.db.BeginTx(ctx, nil)
defer tx.Rollback()

// 1. Upsert session metadata
_, err = tx.ExecContext(ctx, `
    INSERT INTO sessions(...) VALUES(...)
    ON CONFLICT(id) DO UPDATE SET ...
`, ...)

// 2. Delete all existing messages for this session
tx.ExecContext(ctx, "DELETE FROM session_messages WHERE session_id = ?", id)

// 3. Re-insert all messages
stmt, _ := tx.PrepareContext(ctx, "INSERT INTO session_messages(...) VALUES(...)")
for _, msg := range state.Messages {
    stmt.ExecContext(ctx, ...)
}

return tx.Commit()
```

**Get 模式**（元数据 + 消息）：

```go
// 1. Get session metadata
meta, err := s.GetMeta(ctx, sessionID)

// 2. Get messages ordered by insertion order
rows, err := s.db.QueryContext(ctx, `
    SELECT role, content FROM session_messages
    WHERE session_id = ? ORDER BY id ASC
`, sessionID)
```

---

## 命名约定

### 表名

| 表 | 用途 |
|-------|---------|
| `sessions` | 会话元数据（id, provider, model, timestamps, attrs） |
| `session_messages` | 归属于会话的消息 |

- 复数形式（`sessions`，而不是 `session`）
- Snake_case
- 具描述性，并以领域前缀开头（`session_messages`）

### 列名

- Snake_case：`session_id`、`created_at`、`updated_at`
- 时间戳存为 Unix 整数（不是 datetime 字符串）
- JSON 数据存为 TEXT（`attrs` 列）
- 外键按被引用表命名：`session_id`

### 索引名

- 格式：`idx_{table}_{columns}`
- 示例：`idx_session_messages_session_id`

---

## 事务模式

### 全量替换（Save）

Save 操作在单个事务内使用“删除全部 + 重新插入”的模式：

```go
tx.BeginTx(ctx, nil)
defer tx.Rollback()
// ... upsert metadata
// ... delete old messages
// ... insert new messages
tx.Commit()
```

**原因**：会话历史始终以完整快照保存，这简化了一致性保证。

### 乐观锁（MemoryStore）

MemoryStore 参考实现展示了乐观锁：

```go
func (s *MemoryStore) AppendMessagesWithVersion(ctx, id, expectedVersion, msgs) (int64, error) {
    if entry.version != expectedVersion {
        return entry.version, session.ErrSessionConflict
    }
    entry.messages = append(entry.messages, msgs...)
    entry.version++
    return entry.version, nil
}
```

### WAL 模式（SQLite）

SQLite 存储开启 WAL 以获得更好的并发读取性能：

```go
db.Exec("PRAGMA journal_mode=WAL")
```

---

## 元数据管理

### State ↔ Meta 转换

`helpers.go` 文件提供 `SessionState.Meta`（map[string]string）与 `SessionMeta`（结构化）之间的转换：

- `Meta` 中的 `model` 键映射到 `SessionMeta.Model`
- 其他键映射到 `SessionMeta.Attrs`
- `normalizeMetaForSave()` 会将新 state 与已有元数据合并（保留新 state 中未包含的字段）

---

## 常见错误

### 错误 1：未处理 ErrSessionNotFound

**不佳**：将“not found”当作致命错误

**良好**：检查并优雅处理（SDK 的 Session.Chat 就这样做）：
```go
state, err := s.store.Get(ctx, s.id)
if err == nil && state != nil {
    historyMsgs = state.Messages
}
// Not found → continue without history
```

### 错误 2：Save 未使用事务

**不佳**：在没有事务的情况下分别执行 UPDATE + DELETE + INSERT

**良好**：始终用事务包裹 Save 以确保原子性

### 错误 3：将时间戳存为字符串

**不佳**：`created_at TEXT` 且使用 `"2024-01-01T00:00:00Z"` 格式

**良好**：`created_at INTEGER` 且使用 Unix timestamp — 无时区歧义，比较高效

### 错误 4：未关闭 Rows

**不佳**：在 `QueryContext` 之后漏掉 `defer rows.Close()`

**良好**：检查错误后立即 defer close：
```go
rows, err := db.QueryContext(ctx, ...)
if err != nil { return nil, err }
defer rows.Close()
```

### 错误 5：破坏接口契约

**不佳**：向 `SessionStore` 接口添加方法

**良好**：创建一个新的可选接口（如 `SessionStoreAppender`）

---

## 跨后端差异

### Save 策略

| 后端 | 策略 |
|---------|----------|
| SQLite | 完整删除 + 重新插入所有消息 |
| PostgreSQL | 前缀优化：若已有消息是新消息的前缀，则仅插入增量 |
| MySQL | 前缀优化：与 PostgreSQL 相同 |
| Memory | 内存中全量替换 |
| File | 全量替换，原子文件重命名 |
| Redis | Pipeline：删除列表 + RPUSH 全部 |

### Append 行为

| 后端 | 自动创建会话？ |
|---------|----------------------|
| SQLite | 是（以空 provider/model 插入） |
| PostgreSQL | 否（要求会话已存在） |
| MySQL | 否（要求会话已存在） |
| Memory | 是 |
| File | 是 |
| Redis | 是（不做存在性检查） |

### 时间戳存储

| 后端 | 类型 |
|---------|------|
| SQLite | `INTEGER`（Unix seconds） |
| PostgreSQL | `TIMESTAMP` |
| MySQL | `TIMESTAMP` |
| Redis | meta JSON 中的 Unix seconds |

### JSON/Attrs 存储

| 后端 | 类型 |
|---------|------|
| SQLite | `TEXT`（JSON 字符串） |
| PostgreSQL | `JSONB` |
| MySQL | `JSON` |
| Redis | meta key 中的 JSON 字符串 |

### 连接池

PostgreSQL 与 MySQL 存储会配置连接池：

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

### 无迁移框架

所有 SQL 存储在启动时用 `CREATE TABLE IF NOT EXISTS` 创建表。没有版本化迁移系统。架构变更必须手动处理。
