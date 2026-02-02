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

func legacyAdapter(store session.LegacySessionStore) *session.LegacyStoreAdapter {
	return session.NewLegacyAdapter(store)
}

func legacyGet(store session.LegacySessionStore, ctx context.Context, id string) (*session.SessionState, error) {
	adapter := legacyAdapter(store)
	if adapter == nil {
		return nil, session.ErrStoreUnavailable
	}
	return adapter.Get(ctx, id)
}

func legacySave(store session.LegacySessionStore, ctx context.Context, state *session.SessionState) error {
	adapter := legacyAdapter(store)
	if adapter == nil {
		return session.ErrStoreUnavailable
	}
	return adapter.Save(ctx, state)
}

func legacyDelete(store session.LegacySessionStore, ctx context.Context, id string) error {
	adapter := legacyAdapter(store)
	if adapter == nil {
		return session.ErrStoreUnavailable
	}
	return adapter.Delete(ctx, id)
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *MemoryStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

func (s *FileStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *FileStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *FileStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *SQLiteStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

func (s *MySQLStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *MySQLStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *MySQLStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

func (s *PostgresStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *PostgresStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

func (s *RedisStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return legacyGet(s, ctx, id)
}

func (s *RedisStore) Save(ctx context.Context, state *session.SessionState) error {
	return legacySave(s, ctx, state)
}

func (s *RedisStore) Delete(ctx context.Context, id string) error {
	return legacyDelete(s, ctx, id)
}

var _ session.SessionStore = (*MemoryStore)(nil)
var _ session.SessionStore = (*FileStore)(nil)
var _ session.SessionStore = (*SQLiteStore)(nil)
var _ session.SessionStore = (*MySQLStore)(nil)
var _ session.SessionStore = (*PostgresStore)(nil)
var _ session.SessionStore = (*RedisStore)(nil)
