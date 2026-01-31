# 数据库存储方案

本文档介绍如何使用 PostgreSQL 和 MySQL 作为多轮对话的持久化存储。

## 目录

- [PostgreSQL 存储](#postgresql-存储)
- [MySQL 存储](#mysql-存储)
- [存储方案对比](#存储方案对比)
- [数据库初始化](#数据库初始化)
- [性能优化](#性能优化)

## PostgreSQL 存储

### 特点

- **高级特性**：原生 JSONB 支持，强大的索引能力
- **并发性能**：MVCC 并发控制，读写不互斥
- **扩展性**：支持分区表、物化视图、全文搜索
- **适用场景**：需要复杂查询、高并发读写、大规模数据

### 安装 PostgreSQL

#### Docker 快速启动

```bash
docker run -d \
  --name postgres-sessions \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=sessions \
  -p 5432:5432 \
  postgres:16-alpine

# 验证连接
docker exec -it postgres-sessions psql -U postgres -d sessions -c 'SELECT version();'
```

#### 本地安装

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Ubuntu/Debian
sudo apt-get install postgresql-16

# 创建数据库
createdb sessions
```

### 使用示例

```go
package main

import (
    "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
)

func main() {
    // 连接字符串格式
    connStr := "postgres://postgres:secret@localhost:5432/sessions?sslmode=disable"

    store, err := sessionstore.NewPostgresStore(connStr)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // 使用 store 进行多轮对话...
}
```

### 连接字符串格式

**标准格式**：
```
postgres://user:password@host:port/dbname?sslmode=disable
```

**参数说明**：
- `user`: 数据库用户（默认 `postgres`）
- `password`: 用户密码
- `host`: 主机地址（默认 `localhost`）
- `port`: 端口号（默认 `5432`）
- `dbname`: 数据库名
- `sslmode`: SSL 模式（`disable`/`require`/`verify-full`）

**示例**：
```go
// 本地开发（无 SSL）
connStr := "postgres://postgres:secret@localhost:5432/sessions?sslmode=disable"

// 生产环境（启用 SSL）
connStr := "postgres://appuser:complex_pass@db.example.com:5432/sessions?sslmode=require"

// Unix socket 连接
connStr := "postgres:///sessions?host=/var/run/postgresql"
```

### 数据库管理

```bash
# 连接数据库
psql -U postgres -d sessions

# 查看所有会话
SELECT id, provider, model, created_at, updated_at FROM sessions;

# 查看会话消息
SELECT session_id, role, LEFT(content, 50) as preview
FROM session_messages
WHERE session_id = 'xxx'
ORDER BY id;

# 查看会话元数据（JSONB）
SELECT id, attrs->>'user_id' as user_id, attrs->>'department' as department
FROM sessions;

# 统计信息
SELECT
    COUNT(DISTINCT session_id) as total_sessions,
    COUNT(*) as total_messages,
    AVG(LENGTH(content)) as avg_message_length
FROM session_messages;

# 清空所有数据
TRUNCATE TABLE session_messages, sessions CASCADE;
```

## MySQL 存储

### 特点

- **成熟稳定**：广泛使用的关系型数据库
- **易于部署**：丰富的托管服务选择
- **复制支持**：主从复制、组复制
- **适用场景**：传统应用、需要主从复制、云托管服务

### 安装 MySQL

#### Docker 快速启动

```bash
docker run -d \
  --name mysql-sessions \
  -e MYSQL_ROOT_PASSWORD=secret \
  -e MYSQL_DATABASE=sessions \
  -p 3306:3306 \
  mysql:8.0

# 验证连接
docker exec -it mysql-sessions mysql -uroot -psecret -e 'SELECT version();'
```

#### 本地安装

```bash
# macOS
brew install mysql@8.0
brew services start mysql@8.0

# Ubuntu/Debian
sudo apt-get install mysql-server

# 创建数据库
mysql -u root -p -e 'CREATE DATABASE sessions CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
```

### 使用示例

```go
package main

import (
    "github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
)

func main() {
    // DSN 格式
    dsn := "root:secret@tcp(localhost:3306)/sessions?parseTime=true&charset=utf8mb4"

    store, err := sessionstore.NewMySQLStore(dsn)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // 使用 store 进行多轮对话...
}
```

### DSN 格式

**标准格式**：
```
user:password@tcp(host:port)/dbname?param1=value1&param2=value2
```

**必需参数**：
- `parseTime=true` - 自动解析 TIMESTAMP 类型

**推荐参数**：
- `charset=utf8mb4` - 支持完整 Unicode（包括 emoji）
- `loc=Local` - 使用本地时区

**示例**：
```go
// 本地开发
dsn := "root:secret@tcp(localhost:3306)/sessions?parseTime=true&charset=utf8mb4"

// 生产环境
dsn := "appuser:complex_pass@tcp(db.example.com:3306)/sessions?parseTime=true&charset=utf8mb4&timeout=30s"

// Unix socket 连接
dsn := "user:password@unix(/var/run/mysqld/mysqld.sock)/sessions?parseTime=true"
```

### 数据库管理

```bash
# 连接数据库
mysql -u root -p sessions

# 查看所有会话
SELECT id, provider, model, created_at, updated_at FROM sessions;

# 查看会话消息
SELECT session_id, role, LEFT(content, 50) as preview
FROM session_messages
WHERE session_id = 'xxx'
ORDER BY id;

# 查看会话元数据（JSON）
SELECT id,
       JSON_EXTRACT(attrs, '$.user_id') as user_id,
       JSON_EXTRACT(attrs, '$.department') as department
FROM sessions;

# 统计信息
SELECT
    COUNT(DISTINCT session_id) as total_sessions,
    COUNT(*) as total_messages,
    AVG(LENGTH(content)) as avg_message_length
FROM session_messages;

# 清空所有数据
TRUNCATE TABLE session_messages;
TRUNCATE TABLE sessions;
```

## 存储方案对比

| 特性 | SQLite | PostgreSQL | MySQL | Redis |
|-----|--------|------------|-------|-------|
| **部署复杂度** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **并发性能** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **查询能力** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |
| **数据持久化** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **水平扩展** | ❌ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **运维成本** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **内存占用** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| **磁盘占用** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

### 选择建议

**SQLite**：
- ✅ 单机应用
- ✅ 会话数 < 10 万
- ✅ 并发请求 < 50/s
- ✅ 零配置快速开发

**PostgreSQL**：
- ✅ 需要复杂查询（JSON 查询、全文搜索）
- ✅ 高并发读写（> 1000 QPS）
- ✅ 大规模数据（> 100 万会话）
- ✅ 需要高级特性（分区表、物化视图）

**MySQL**：
- ✅ 传统 Web 应用
- ✅ 需要主从复制
- ✅ 云托管服务（RDS、Aurora）
- ✅ 团队熟悉 MySQL 生态

**Redis**：
- ✅ 临时对话（带 TTL）
- ✅ 极高并发（> 10000 QPS）
- ✅ 分布式部署
- ✅ 已有 Redis 基础设施

## 数据库初始化

### PostgreSQL 初始化脚本

```sql
-- 创建数据库
CREATE DATABASE sessions
    WITH ENCODING 'UTF8'
    LC_COLLATE = 'en_US.UTF-8'
    LC_CTYPE = 'en_US.UTF-8';

\c sessions

-- 会话表（自动由 Go 代码创建）
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    attrs JSONB
);

-- 消息表
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

-- 索引
CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
ON session_messages(session_id, id);

CREATE INDEX IF NOT EXISTS idx_sessions_updated_at
ON sessions(updated_at);

-- GIN 索引加速 JSONB 查询
CREATE INDEX IF NOT EXISTS idx_sessions_attrs
ON sessions USING GIN (attrs);

-- 创建应用用户
CREATE USER appuser WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE sessions TO appuser;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO appuser;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO appuser;
```

### MySQL 初始化脚本

```sql
-- 创建数据库
CREATE DATABASE sessions
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE sessions;

-- 会话表（自动由 Go 代码创建）
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    provider VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    attrs JSON,
    INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 消息表
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

-- 创建应用用户
CREATE USER 'appuser'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON sessions.* TO 'appuser'@'localhost';
FLUSH PRIVILEGES;
```

## 性能优化

### PostgreSQL 优化

#### 1. 连接池配置

```go
store, _ := sessionstore.NewPostgresStore(connStr)

// 自定义连接池（如果需要）
store.db.SetMaxOpenConns(100)      // 最大连接数
store.db.SetMaxIdleConns(10)       // 空闲连接数
store.db.SetConnMaxLifetime(time.Hour)  // 连接最大生存时间
```

#### 2. 查询优化

```sql
-- 分析慢查询
EXPLAIN ANALYZE
SELECT * FROM session_messages WHERE session_id = 'xxx';

-- 查看索引使用情况
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
ORDER BY idx_scan;

-- VACUUM 和 ANALYZE
VACUUM ANALYZE sessions;
VACUUM ANALYZE session_messages;
```

#### 3. 批量操作

```go
// 批量插入消息（事务内）
tx, _ := store.db.Begin()
stmt, _ := tx.Prepare(`
    INSERT INTO session_messages (session_id, role, content, created_at)
    VALUES ($1, $2, $3, $4)
`)
for _, msg := range messages {
    stmt.Exec(sessionID, msg.Role, msg.Content, time.Now())
}
stmt.Close()
tx.Commit()
```

### MySQL 优化

#### 1. InnoDB 配置

```ini
# my.cnf
[mysqld]
innodb_buffer_pool_size = 2G        # 缓冲池大小（设为物理内存 50-80%）
innodb_log_file_size = 512M         # 日志文件大小
innodb_flush_log_at_trx_commit = 2  # 提交策略（2 = 每秒刷盘，性能更好）
innodb_flush_method = O_DIRECT      # 绕过 OS 缓存
```

#### 2. 查询优化

```sql
-- 分析查询性能
EXPLAIN
SELECT * FROM session_messages WHERE session_id = 'xxx';

-- 查看索引使用情况
SHOW INDEX FROM session_messages;

-- 优化表
OPTIMIZE TABLE sessions;
OPTIMIZE TABLE session_messages;
```

#### 3. 分区表（大规模数据）

```sql
-- 按月份分区消息表
ALTER TABLE session_messages
PARTITION BY RANGE (YEAR(created_at) * 100 + MONTH(created_at)) (
    PARTITION p202401 VALUES LESS THAN (202402),
    PARTITION p202402 VALUES LESS THAN (202403),
    PARTITION p202403 VALUES LESS THAN (202404),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);
```

### 通用优化建议

1. **定期清理旧数据**
   ```go
   // 每天清理 30 天前的会话
   store.CleanupOldSessions(ctx, 30*24*time.Hour)
   ```

2. **监控数据库指标**
   - 连接数
   - QPS（每秒查询数）
   - 慢查询日志
   - 磁盘使用率

3. **备份策略**
   - PostgreSQL: `pg_dump sessions > backup.sql`
   - MySQL: `mysqldump sessions > backup.sql`
   - 定期自动备份 + 异地存储

4. **读写分离**（高负载场景）
   - 主库：写操作（AppendMessages）
   - 从库：读操作（GetMessages）

## 故障排查

### 常见问题

#### PostgreSQL

**问题 1：连接被拒绝**
```
error: dial tcp: connect: connection refused
```
解决方案：
```bash
# 检查 PostgreSQL 是否运行
pg_isready

# 检查监听地址（postgresql.conf）
listen_addresses = '*'  # 或具体 IP

# 检查防火墙
sudo ufw allow 5432
```

**问题 2：权限错误**
```
error: permission denied for table sessions
```
解决方案：
```sql
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO appuser;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO appuser;
```

#### MySQL

**问题 1：字符集错误**
```
error: Incorrect string value
```
解决方案：
```sql
-- 确保使用 utf8mb4
ALTER DATABASE sessions CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
ALTER TABLE sessions CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

**问题 2：连接超时**
```
error: dial tcp: i/o timeout
```
解决方案：
```go
// DSN 中添加超时参数
dsn := "user:pass@tcp(host:3306)/sessions?parseTime=true&timeout=30s&readTimeout=30s&writeTimeout=30s"
```

## 下一步

- 查看 [API 文档](session-api.md) 了解接口定义
- 查看 [使用教程](session-tutorial.md) 了解更多场景
- 运行 `examples/session-postgres/` 和 `examples/session-mysql/` 示例代码
