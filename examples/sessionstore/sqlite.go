package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements session.SessionStore using SQLite database.
// It's ideal for single-machine deployments and local persistence.
//
// Features:
// - ACID transactions
// - Simple file-based storage
// - No external dependencies
// - Good for small to medium scale
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-based session store.
//
// dbPath examples:
//   - "./sessions.db" - relative path
//   - "/var/lib/app/sessions.db" - absolute path
//   - ":memory:" - in-memory database (for testing)
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	// Create tables if not exist
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		attrs TEXT
	);

	CREATE TABLE IF NOT EXISTS session_messages (
		id INTEGER PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_session_messages_session_id
	ON session_messages(session_id, id);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Get retrieves a session snapshot.
func (s *SQLiteStore) Get(ctx context.Context, sessionID string) (*session.SessionState, error) {
	meta, err := s.GetMeta(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
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

	state := &session.SessionState{
		ID:       sessionID,
		Messages: messages,
	}
	applyStoredMeta(state, meta)
	return state, nil
}

// Save writes a session snapshot.
func (s *SQLiteStore) Save(ctx context.Context, state *session.SessionState) error {
	if state == nil {
		return errors.New("session store: nil state")
	}
	if state.ID == "" {
		return errors.New("session store: missing session id")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingMeta *session.SessionMeta
	var provider, model string
	var createdAt, updatedAt int64
	var attrsJSON sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT provider, model, created_at, updated_at, attrs
		FROM sessions WHERE id = ?
	`, state.ID).Scan(&provider, &model, &createdAt, &updatedAt, &attrsJSON)
	if err == nil {
		meta := session.SessionMeta{
			ID:        state.ID,
			Provider:  provider,
			Model:     model,
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(updatedAt, 0),
		}
		if attrsJSON.Valid && attrsJSON.String != "" {
			_ = json.Unmarshal([]byte(attrsJSON.String), &meta.Attrs)
		}
		existingMeta = &meta
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now()
	meta := normalizeMetaForSave(state, existingMeta, now)

	attrsPayload := ""
	if meta.Attrs != nil {
		payload, err := json.Marshal(meta.Attrs)
		if err != nil {
			return err
		}
		attrsPayload = string(payload)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions(id, provider, model, created_at, updated_at, attrs)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			attrs = excluded.attrs
	`, state.ID, meta.Provider, meta.Model, meta.CreatedAt.Unix(), meta.UpdatedAt.Unix(), attrsPayload)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM session_messages WHERE session_id = ?", state.ID); err != nil {
		return err
	}

	if len(state.Messages) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO session_messages(session_id, role, content, created_at)
			VALUES(?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		createdAt := meta.UpdatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		createdAtUnix := createdAt.Unix()
		for _, msg := range state.Messages {
			if _, err := stmt.ExecContext(ctx, state.ID, msg.Role, msg.Content, createdAtUnix); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// Delete removes a session.
func (s *SQLiteStore) Delete(ctx context.Context, sessionID string) error {
	return s.DeleteSession(ctx, sessionID)
}

// Append appends messages to a session.
func (s *SQLiteStore) Append(ctx context.Context, sessionID string, msgs ...session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions(id, provider, model, created_at, updated_at)
		VALUES(?, '', '', ?, ?)
		ON CONFLICT(id) DO UPDATE SET updated_at = ?
	`, sessionID, now, now, now)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO session_messages(session_id, role, content, created_at)
		VALUES(?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range msgs {
		if _, err := stmt.ExecContext(ctx, sessionID, msg.Role, msg.Content, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CreateSession creates a new session with metadata.
// Implements session.SessionStoreWithLifecycle interface.
func (s *SQLiteStore) CreateSession(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	if meta == nil {
		meta = &session.SessionMeta{}
	}

	attrsJSON, _ := json.Marshal(meta.Attrs)
	now := time.Now().Unix()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, provider, model, created_at, updated_at, attrs)
		VALUES(?, ?, ?, ?, ?, ?)
	`, sessionID, meta.Provider, meta.Model, now, now, string(attrsJSON))

	if err != nil {
		return err
	}

	return nil
}

// DeleteSession deletes a session and all its messages.
// Implements session.SessionStoreWithLifecycle interface.
func (s *SQLiteStore) DeleteSession(ctx context.Context, sessionID string) error {
	// CASCADE will automatically delete messages
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return session.ErrSessionNotFound
	}

	return nil
}

// GetMeta retrieves session metadata.
// Implements session.SessionStoreWithMeta interface.
func (s *SQLiteStore) GetMeta(ctx context.Context, sessionID string) (*session.SessionMeta, error) {
	var meta session.SessionMeta
	var attrsJSON sql.NullString
	var createdAt, updatedAt int64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, created_at, updated_at, attrs
		FROM sessions WHERE id = ?
	`, sessionID).Scan(
		&meta.ID,
		&meta.Provider,
		&meta.Model,
		&createdAt,
		&updatedAt,
		&attrsJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}

	meta.CreatedAt = time.Unix(createdAt, 0)
	meta.UpdatedAt = time.Unix(updatedAt, 0)

	if attrsJSON.Valid && attrsJSON.String != "" {
		json.Unmarshal([]byte(attrsJSON.String), &meta.Attrs)
	}

	return &meta, nil
}

// UpsertMeta updates or inserts session metadata.
// Implements session.SessionStoreWithMeta interface.
func (s *SQLiteStore) UpsertMeta(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	if meta == nil {
		return errors.New("meta cannot be nil")
	}

	attrsJSON, _ := json.Marshal(meta.Attrs)
	now := time.Now().Unix()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, provider, model, created_at, updated_at, attrs)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			updated_at = excluded.updated_at,
			attrs = excluded.attrs
	`, sessionID, meta.Provider, meta.Model, now, now, string(attrsJSON))

	return err
}

// ListSessions returns all session IDs, optionally filtered by provider.
// This is a convenience method for admin/debugging.
func (s *SQLiteStore) ListSessions(ctx context.Context, provider string) ([]string, error) {
	query := "SELECT id FROM sessions"
	args := []interface{}{}

	if provider != "" {
		query += " WHERE provider = ?"
		args = append(args, provider)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, id)
	}

	return sessionIDs, rows.Err()
}

// CleanupOldSessions deletes sessions older than the specified duration.
// Useful for periodic cleanup tasks.
func (s *SQLiteStore) CleanupOldSessions(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Unix()

	result, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE updated_at < ?",
		cutoff,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Verify interface compliance at compile time
var (
	_ session.SessionStore         = (*SQLiteStore)(nil)
	_ session.SessionStoreAppender = (*SQLiteStore)(nil)
)
