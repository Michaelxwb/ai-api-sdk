package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
	_ "github.com/lib/pq"
)

// PostgresStore implements session.SessionStore using PostgreSQL database.
// Suitable for production deployments with high availability requirements.
//
// Features:
// - Full ACID transactions
// - Advanced indexing and query optimization
// - Native JSONB support for metadata
// - Connection pooling
// - High concurrency support
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL-based session store.
//
// Connection string format:
//   - "host=localhost port=5432 user=postgres password=secret dbname=sessions sslmode=disable"
//   - "postgres://user:password@localhost:5432/sessions?sslmode=disable"
//
// Example:
//
//	store, _ := NewPostgresStore("postgres://postgres:secret@localhost:5432/sessions?sslmode=disable")
func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Create tables if not exist
	schema := `
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
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
	ON session_messages(session_id, id);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &PostgresStore{db: db}, nil
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Get retrieves a full session state.
func (s *PostgresStore) Get(ctx context.Context, sessionID string) (*session.SessionState, error) {
	var meta session.SessionMeta
	var attrsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, created_at, updated_at, attrs
		FROM sessions
		WHERE id = $1
	`, sessionID).Scan(&meta.ID, &meta.Provider, &meta.Model, &meta.CreatedAt, &meta.UpdatedAt, &attrsJSON)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}

	if len(attrsJSON) > 0 {
		if err := json.Unmarshal(attrsJSON, &meta.Attrs); err != nil {
			return nil, err
		}
	}

	messages, err := s.fetchMessages(ctx, s.db, sessionID)
	if err != nil {
		return nil, err
	}

	state := &session.SessionState{
		ID:       sessionID,
		Messages: messages,
	}
	applyStoredMeta(state, &meta)
	return state, nil
}

// Save writes the provided session state.
func (s *PostgresStore) Save(ctx context.Context, state *session.SessionState) error {
	if state == nil {
		return errors.New("session store: nil state")
	}
	if state.ID == "" {
		return errors.New("session store: missing session id")
	}

	now := time.Now()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 查 existing meta 用于合并
	var existingMeta *session.SessionMeta
	var ep, em string
	var ec, eu time.Time
	var ea []byte
	err = tx.QueryRowContext(ctx, `SELECT provider, model, created_at, updated_at, attrs FROM sessions WHERE id = $1`, state.ID).Scan(&ep, &em, &ec, &eu, &ea)
	if err == nil {
		existingMeta = &session.SessionMeta{
			ID:        state.ID,
			Provider:  ep,
			Model:     em,
			CreatedAt: ec,
			UpdatedAt: eu,
		}
		if len(ea) > 0 {
			_ = json.Unmarshal(ea, &existingMeta.Attrs)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	meta := normalizeMetaForSave(state, existingMeta, now)
	var attrsJSON []byte
	if meta.Attrs != nil {
		attrsJSON, err = json.Marshal(meta.Attrs)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (id, provider, model, created_at, updated_at, attrs)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			updated_at = EXCLUDED.updated_at,
			attrs = EXCLUDED.attrs
	`, state.ID, meta.Provider, meta.Model, meta.CreatedAt, meta.UpdatedAt, attrsJSON)
	if err != nil {
		return err
	}

	existing, err := s.fetchMessages(ctx, tx, state.ID)
	if err != nil {
		return err
	}

	if isMessagePrefix(existing, state.Messages) {
		delta := state.Messages[len(existing):]
		if len(delta) > 0 {
			if err := s.insertMessagesTx(ctx, tx, state.ID, delta); err != nil {
				return err
			}
		}
		return tx.Commit()
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM session_messages WHERE session_id = $1", state.ID); err != nil {
		return err
	}
	if len(state.Messages) > 0 {
		if err := s.insertMessagesTx(ctx, tx, state.ID, state.Messages); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Delete removes a session and all its messages.
func (s *PostgresStore) Delete(ctx context.Context, sessionID string) error {
	return s.DeleteSession(ctx, sessionID)
}

// Append appends messages to an existing session.
func (s *PostgresStore) Append(ctx context.Context, sessionID string, msgs ...session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)", sessionID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return session.ErrSessionNotFound
	}

	if err := s.insertMessagesTx(ctx, tx, sessionID, msgs); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET updated_at = $1 WHERE id = $2", time.Now(), sessionID); err != nil {
		return err
	}

	return tx.Commit()
}

type postgresMessageQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (s *PostgresStore) fetchMessages(ctx context.Context, queryer postgresMessageQueryer, sessionID string) ([]session.Message, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT role, content
		FROM session_messages
		WHERE session_id = $1
		ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []session.Message
	for rows.Next() {
		var msg session.Message

		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *PostgresStore) insertMessagesTx(ctx context.Context, tx *sql.Tx, sessionID string, msgs []session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO session_messages (session_id, role, content, created_at)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, msg := range msgs {
		if _, err := stmt.ExecContext(ctx, sessionID, msg.Role, msg.Content, now); err != nil {
			return err
		}
	}
	return nil
}

// CreateSession creates a new session with metadata.
// Implements session.SessionStoreWithLifecycle interface.
func (s *PostgresStore) CreateSession(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	if meta == nil {
		meta = &session.SessionMeta{}
	}

	now := time.Now()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	meta.UpdatedAt = now
	meta.ID = sessionID

	var attrsJSON []byte
	if meta.Attrs != nil {
		var err error
		attrsJSON, err = json.Marshal(meta.Attrs)
		if err != nil {
			return err
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, provider, model, created_at, updated_at, attrs)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, sessionID, meta.Provider, meta.Model, meta.CreatedAt, meta.UpdatedAt, attrsJSON)

	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "pq: duplicate key value violates unique constraint \"sessions_pkey\"" {
			return session.ErrSessionConflict
		}
		return err
	}

	return nil
}

// DeleteSession removes a session and all its messages.
// Implements session.SessionStoreWithLifecycle interface.
func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", sessionID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return session.ErrSessionNotFound
	}

	return nil
}

// ListSessions returns all session IDs, optionally filtered by prefix.
// Implements session.SessionStoreWithLifecycle interface.
func (s *PostgresStore) ListSessions(ctx context.Context, prefix string) ([]string, error) {
	var query string
	var args []interface{}

	if prefix == "" {
		query = "SELECT id FROM sessions ORDER BY updated_at DESC"
	} else {
		query = "SELECT id FROM sessions WHERE id LIKE $1 ORDER BY updated_at DESC"
		args = append(args, prefix+"%")
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		sessions = append(sessions, id)
	}

	return sessions, rows.Err()
}

// GetMeta retrieves session metadata.
// Implements session.SessionStoreWithMeta interface.
func (s *PostgresStore) GetMeta(ctx context.Context, sessionID string) (*session.SessionMeta, error) {
	var meta session.SessionMeta
	var attrsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, created_at, updated_at, attrs
		FROM sessions
		WHERE id = $1
	`, sessionID).Scan(&meta.ID, &meta.Provider, &meta.Model, &meta.CreatedAt, &meta.UpdatedAt, &attrsJSON)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}

	if len(attrsJSON) > 0 {
		if err := json.Unmarshal(attrsJSON, &meta.Attrs); err != nil {
			return nil, err
		}
	}

	return &meta, nil
}

// UpsertMeta updates or inserts session metadata.
// Implements session.SessionStoreWithMeta interface.
func (s *PostgresStore) UpsertMeta(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	if meta == nil {
		return errors.New("meta cannot be nil")
	}

	meta.ID = sessionID
	meta.UpdatedAt = time.Now()

	var attrsJSON []byte
	if meta.Attrs != nil {
		var err error
		attrsJSON, err = json.Marshal(meta.Attrs)
		if err != nil {
			return err
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, provider, model, created_at, updated_at, attrs)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			updated_at = EXCLUDED.updated_at,
			attrs = EXCLUDED.attrs
	`, sessionID, meta.Provider, meta.Model, meta.CreatedAt, meta.UpdatedAt, attrsJSON)

	return err
}

// CleanupOldSessions deletes sessions older than the specified duration.
// Returns the number of sessions deleted.
func (s *PostgresStore) CleanupOldSessions(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE updated_at < $1
	`, cutoff)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

var (
	_ session.SessionStore         = (*PostgresStore)(nil)
	_ session.SessionStoreAppender = (*PostgresStore)(nil)
)
