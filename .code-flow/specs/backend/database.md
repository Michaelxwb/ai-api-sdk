# Backend Database (Session Store)

## Rules

- SDK 核心层只定义存储抽象，不绑定数据库：`session.SessionStore` 是稳定契约（Get/Save/Delete）。
- `SessionStore` 实现必须并发安全；SDK 不额外加锁保护实现。
- `Get` 不存在时返回 `session.ErrSessionNotFound`；`Save` 执行 upsert；按场景使用 `ErrSessionConflict/ErrSessionClosed/ErrStoreUnavailable`。
- `Save` 必须保证原子性（事务或等价机制），避免 metadata 与 messages 部分成功。
- SQL 查询必须参数化；禁止字符串拼接构造 SQL。
- 事务内禁止外部网络调用，避免锁持有时间失控。

## Patterns

### SessionStore 接口契约
```go
type SessionStore interface {
    Get(ctx context.Context, id string) (*SessionState, error)
    Save(ctx context.Context, state *SessionState) error
    Delete(ctx context.Context, id string) error
}
// 可选扩展（不修改核心接口）
type SessionStoreAppender interface {
    Append(ctx context.Context, id string, msgs ...Message) error
}
var _ session.SessionStore = (*SQLiteStore)(nil) // 编译期验证
```

### Sentinel Errors 使用
```go
state, err := s.store.Get(ctx, s.id)
if err == nil && state != nil {
    historyMsgs = state.Messages
}
// ErrSessionNotFound → 继续（无历史）；其他错误忽略或走 OnStoreError 回调
```

### SQLite Save 模式（事务内全量替换）
```go
tx.BeginTx(ctx, nil); defer tx.Rollback()
// 1. Upsert session metadata (ON CONFLICT DO UPDATE)
// 2. DELETE FROM session_messages WHERE session_id = ?
// 3. INSERT all messages via prepared stmt
tx.Commit()
```

### 跨后端 Save 策略
| 后端 | 策略 |
|------|------|
| SQLite | 事务内全量替换消息 |
| PostgreSQL / MySQL | 前缀优化：仅增量插入新消息（降低写放大） |
| Memory | 内存中全量替换 |
| File | 写临时文件 + rename（原子落盘） |
| Redis | Pipeline：删除列表 + RPUSH 全部 |

### 表结构约定（SQL 后端）
```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL, model TEXT NOT NULL,
    created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,  -- Unix timestamp
    attrs TEXT  -- JSON string
);
CREATE TABLE IF NOT EXISTS session_messages (
    id INTEGER PRIMARY KEY,  -- Auto-increment（保序）
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL, content TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id, id);
```

### 连接池（PG/MySQL）
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

## Anti-Patterns

- 修改 `session.SessionStore` 核心接口以承载某个具体数据库特性（应走可选 interface）。
- `Save` 中分散执行多条写操作但不做事务控制（metadata/messages 可能不一致）。
- 读取结果集后不 `defer rows.Close()` 或不检查 `rows.Err()`。
- 时间戳存为字符串而非 Unix integer（比较/排序混乱）。
- 以为 `HistoryNone` 或 `remote_session` 会自动禁用持久化（保存取决于是否配置 store，与 history 模式无关）。
- FileStore 使用相对路径而不绝对路径（换目录执行后写入位置变化）。
- 以为 SDK 会设置 `SessionState.CreatedAt`：`client/session.go:saveState` 只设置 `UpdatedAt`，`CreatedAt` 为零值；store 实现（postgres/redis）负责在 upsert 时检测零值并设为 `now`。自定义 store 实现也必须处理此逻辑。
