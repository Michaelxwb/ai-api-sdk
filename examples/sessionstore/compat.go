package sessionstore

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
	"github.com/redis/go-redis/v9"
)

// Compatibility config types used by existing examples.
type FileConfig struct {
	BaseDir  string
	FileName string
	Path     string
}

type SQLiteConfig struct {
	DSN string
}

type MySQLConfig struct {
	DSN string
}

type PostgreSQLConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	TTL      time.Duration
	Prefix   string
}

// NewMemory returns an in-memory store (compat wrapper).
func NewMemory() *MemoryStore {
	return NewMemoryStore()
}

// NewFile returns a file-backed store (compat wrapper).
func NewFile(cfg FileConfig) *FileStore {
	path := cfg.Path
	if path == "" {
		base := cfg.BaseDir
		if base == "" {
			base = "."
		}
		name := cfg.FileName
		if name == "" {
			name = "sessions.json"
		}
		path = filepath.Join(base, name)
	}
	store, err := NewFileStore(path)
	if err != nil {
		panic(fmt.Errorf("sessionstore: init file store: %w", err))
	}
	return store
}

// NewSQLite returns a SQLite-backed store (compat wrapper).
func NewSQLite(cfg SQLiteConfig) (*SQLiteStore, error) {
	return NewSQLiteStore(cfg.DSN)
}

// NewMySQL returns a MySQL-backed store (compat wrapper).
func NewMySQL(cfg MySQLConfig) (*MySQLStore, error) {
	return NewMySQLStore(cfg.DSN)
}

// NewPostgreSQL returns a PostgreSQL-backed store (compat wrapper).
func NewPostgreSQL(cfg PostgreSQLConfig) (*PostgresStore, error) {
	return NewPostgresStore(cfg.DSN)
}

// NewRedis returns a Redis-backed store (compat wrapper).
func NewRedis(cfg RedisConfig) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	store := NewRedisStore(rdb, cfg.TTL)
	if cfg.Prefix != "" {
		store.Prefix = cfg.Prefix
	}
	return store, nil
}

var _ session.SessionStore = (*MemoryStore)(nil)
var _ session.SessionStore = (*FileStore)(nil)
var _ session.SessionStore = (*SQLiteStore)(nil)
var _ session.SessionStore = (*MySQLStore)(nil)
var _ session.SessionStore = (*PostgresStore)(nil)
var _ session.SessionStore = (*RedisStore)(nil)
