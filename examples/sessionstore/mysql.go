package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
	"github.com/go-sql-driver/mysql"
)

// MySQLStore implements session.SessionStore using MySQL database.
// Suitable for production deployments requiring proven relational database.
//
// Features:
// - ACID transactions
// - Replication support
// - Native JSON support (MySQL 5.7+)
// - Connection pooling
// - High availability options
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a new MySQL-based session store.
//
// Connection string format (DSN):
//   - "user:password@tcp(localhost:3306)/dbname?parseTime=true"
//   - "root:secret@tcp(127.0.0.1:3306)/sessions?parseTime=true&charset=utf8mb4"
//
// Important: parseTime=true is required for proper TIMESTAMP handling
//
// Example:
//
//	store, _ := NewMySQLStore("root:secret@tcp(localhost:3306)/sessions?parseTime=true")
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
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
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Create tables if not exist
	schema := `
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
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		INDEX idx_session_messages_session_id (session_id, id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &MySQLStore{db: db}, nil
}

// Close closes the database connection pool.
func (s *MySQLStore) Close() error {
	return s.db.Close()
}

// Get retrieves a full session state.
func (s *MySQLStore) Get(ctx context.Context, sessionID string) (*session.SessionState, error) {
	var meta session.SessionMeta
	var attrsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, created_at, updated_at, attrs
		FROM sessions
		WHERE id = ?
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
func (s *MySQLStore) Save(ctx context.Context, state *session.SessionState) error {
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
	err = tx.QueryRowContext(ctx, `SELECT provider, model, created_at, updated_at, attrs FROM sessions WHERE id = ?`, state.ID).Scan(&ep, &em, &ec, &eu, &ea)
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
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			provider = VALUES(provider),
			model = VALUES(model),
			updated_at = VALUES(updated_at),
			attrs = VALUES(attrs)
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

	if _, err := tx.ExecContext(ctx, "DELETE FROM session_messages WHERE session_id = ?", state.ID); err != nil {
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
func (s *MySQLStore) Delete(ctx context.Context, sessionID string) error {
	return s.DeleteSession(ctx, sessionID)
}

// Append appends messages to an existing session.
func (s *MySQLStore) Append(ctx context.Context, sessionID string, msgs ...session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE id = ?)", sessionID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return session.ErrSessionNotFound
	}

	if err := s.insertMessagesTx(ctx, tx, sessionID, msgs); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET updated_at = ? WHERE id = ?", time.Now(), sessionID); err != nil {
		return err
	}

	return tx.Commit()
}

type mysqlMessageQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (s *MySQLStore) fetchMessages(ctx context.Context, queryer mysqlMessageQueryer, sessionID string) ([]session.Message, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT role, content
		FROM session_messages
		WHERE session_id = ?
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

func (s *MySQLStore) insertMessagesTx(ctx context.Context, tx *sql.Tx, sessionID string, msgs []session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO session_messages (session_id, role, content, created_at)
		VALUES (?, ?, ?, ?)
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
func (s *MySQLStore) CreateSession(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
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
		VALUES (?, ?, ?, ?, ?, ?)
	`, sessionID, meta.Provider, meta.Model, meta.CreatedAt, meta.UpdatedAt, attrsJSON)

	if err != nil {
		// Check for duplicate key error
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			return session.ErrSessionConflict
		}
		return err
	}

	return nil
}

// DeleteSession removes a session and all its messages.
// Implements session.SessionStoreWithLifecycle interface.
func (s *MySQLStore) DeleteSession(ctx context.Context, sessionID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
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
func (s *MySQLStore) ListSessions(ctx context.Context, prefix string) ([]string, error) {
	var query string
	var args []interface{}

	if prefix == "" {
		query = "SELECT id FROM sessions ORDER BY updated_at DESC"
	} else {
		query = "SELECT id FROM sessions WHERE id LIKE ? ORDER BY updated_at DESC"
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
func (s *MySQLStore) GetMeta(ctx context.Context, sessionID string) (*session.SessionMeta, error) {
	var meta session.SessionMeta
	var attrsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, created_at, updated_at, attrs
		FROM sessions
		WHERE id = ?
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
func (s *MySQLStore) UpsertMeta(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
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
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			provider = VALUES(provider),
			model = VALUES(model),
			updated_at = VALUES(updated_at),
			attrs = VALUES(attrs)
	`, sessionID, meta.Provider, meta.Model, meta.CreatedAt, meta.UpdatedAt, attrsJSON)

	return err
}

// CleanupOldSessions deletes sessions older than the specified duration.
// Returns the number of sessions deleted.
func (s *MySQLStore) CleanupOldSessions(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE updated_at < ?
	`, cutoff)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

var (
	_ session.SessionStore         = (*MySQLStore)(nil)
	_ session.SessionStoreAppender = (*MySQLStore)(nil)
)
