# 数据库存储方案

本文档介绍如何使用 PostgreSQL、MySQL、SQLite 作为 Session 存储后端。

> **前提条件**：
> - 了解 SessionStore 接口：[Session API](session-api.md)
> - 了解多轮对话基础：[Session 教程](session-tutorial.md)

## 1. PostgreSQL 存储

### 1.1 安装和配置

**安装依赖**：
```bash
go get github.com/lib/pq
```

**连接字符串格式**：
```go
// 方式 1：URL 格式
"postgres://user:password@localhost:5432/sessions?sslmode=disable"

// 方式 2：Key-Value 格式
"host=localhost port=5432 user=postgres password=secret dbname=sessions sslmode=disable"
```

### 1.2 表结构

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    attrs JSONB
);

CREATE TABLE IF NOT EXISTS session_messages (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    name TEXT,
    tool_calls TEXT,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);
```

### 1.3 连接池配置

`NewPostgresStore` 内部默认配置连接池：
- `MaxOpenConns`: 25
- `MaxIdleConns`: 5
- `ConnMaxLifetime`: 5 minutes

如需调整，请修改 `examples/sessionstore/postgres.go` 中的配置。

### 1.4 使用示例

```go
import "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"

store, err := sessionstore.NewPostgresStore(
    "postgres://user:pass@localhost:5432/sessions?sslmode=disable",
)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

cli.SessionStore = store
```

### 1.5 常见问题

**Q: JSONB vs JSON 类型？**  
A: 默认使用 JSONB，查询与索引性能更好。

**Q: 如何调整连接池大小？**  
A: 修改 `examples/sessionstore/postgres.go` 中的连接池参数。

## 2. MySQL 存储

### 2.1 安装和配置

**安装依赖**：
```bash
go get github.com/go-sql-driver/mysql
```

**DSN 格式**：
```go
"user:password@tcp(localhost:3306)/sessions?parseTime=true&charset=utf8mb4"
```

### 2.2 表结构

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    provider VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    attrs JSON,
    INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS session_messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    name VARCHAR(255),
    tool_calls TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    INDEX idx_session_messages_session_id (session_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 2.3 连接池配置

`NewMySQLStore` 内部默认配置连接池：
- `MaxOpenConns`: 25
- `MaxIdleConns`: 5
- `ConnMaxLifetime`: 5 minutes

如需调整，请修改 `examples/sessionstore/mysql.go` 中的配置。

### 2.4 使用示例

```go
import "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"

dsn := "root:secret@tcp(localhost:3306)/sessions?parseTime=true&charset=utf8mb4"
store, err := sessionstore.NewMySQLStore(dsn)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

cli.SessionStore = store
```

### 2.5 常见问题

**Q: parseTime=true 的作用？**  
A: 自动将 MySQL TIMESTAMP 映射为 Go `time.Time`。

**Q: 字符集配置？**  
A: 建议使用 `utf8mb4`，支持完整 Unicode（包括 emoji）。

## 3. SQLite 存储

### 3.1 安装和配置

**安装依赖**：
```bash
go get github.com/mattn/go-sqlite3
```

**注意**：需要 CGO 支持。

### 3.2 表结构

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    attrs TEXT
);

CREATE TABLE IF NOT EXISTS session_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    name TEXT,
    tool_calls TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);
```

### 3.3 WAL 模式

SQLite 默认启用 WAL（Write-Ahead Logging）模式，会产生临时文件：
- `sessions.db-shm`
- `sessions.db-wal`

正常关闭通常会合并日志，异常退出可能残留。

**清理方法**：
```bash
# 方式 1：重新运行程序并正常退出
go run main.go

# 方式 2：手动删除（确保程序已停止）
rm sessions.db-shm sessions.db-wal
```

### 3.4 并发限制

SQLite 不适合高并发写入场景，建议：
- 单用户应用 ✅
- 低并发（<10 并发写） ✅
- 高并发（>100 并发写） ❌ 使用 PostgreSQL/MySQL

### 3.5 使用示例

```go
store, err := sessionstore.NewSQLiteStore("sessions.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

cli.SessionStore = store
```

## 4. 运维指南

### 4.1 索引优化

**PostgreSQL**：
```sql
-- 常见清理/排序场景可添加
CREATE INDEX IF NOT EXISTS idx_sessions_updated_at
ON sessions(updated_at);

-- 如果需要 JSONB 查询
CREATE INDEX IF NOT EXISTS idx_sessions_attrs
ON sessions USING GIN (attrs);
```

**MySQL**：
```sql
-- sessions 表已包含 idx_updated_at
SHOW INDEX FROM sessions;
```

### 4.2 备份恢复

**PostgreSQL**：
```bash
# 备份
pg_dump -U user -d sessions > backup.sql

# 恢复
psql -U user -d sessions < backup.sql
```

**MySQL**：
```bash
# 备份
mysqldump -u user -p sessions > backup.sql

# 恢复
mysql -u user -p sessions < backup.sql
```

**SQLite**：
```bash
# 备份
cp sessions.db sessions.db.bak
```

### 4.3 清理旧会话

```go
deleted, err := store.CleanupOldSessions(ctx, 30*24*time.Hour)
if err != nil {
    log.Fatal(err)
}
log.Printf("deleted %d sessions", deleted)
```

### 4.4 性能调优（数据库级别）

**PostgreSQL**：
- 定期 `VACUUM` / `ANALYZE` 以保持统计信息
- 关注慢查询与索引命中率

**MySQL**：
- 关注 InnoDB 缓冲池大小与慢查询日志
- 定期优化表与检查索引

## 相关文档

- [Session API](session-api.md) - SessionStore 接口
- [Session 教程](session-tutorial.md) - 存储方案选择
- [使用指南](usage-guide.md) - 基础配置
- [架构设计](architecture.md) - Session 层设计
