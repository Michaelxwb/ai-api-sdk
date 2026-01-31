package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/provider"
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
		name TEXT,
		tool_calls TEXT,
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

// GetMessages retrieves message history for a session.
// Implements session.SessionStore interface.
func (s *PostgresStore) GetMessages(ctx context.Context, sessionID string, opts session.GetOptions) ([]provider.Message, error) {
	query := `
		SELECT role, content, name
		FROM session_messages
		WHERE session_id = $1
		ORDER BY id ASC
	`

	if opts.MaxMessages > 0 {
		// Get last N messages
		query = `
			SELECT role, content, name
			FROM session_messages
			WHERE session_id = $1
			ORDER BY id DESC
			LIMIT $2
		`
	}

	var rows *sql.Rows
	var err error

	if opts.MaxMessages > 0 {
		rows, err = s.db.QueryContext(ctx, query, sessionID, opts.MaxMessages)
	} else {
		rows, err = s.db.QueryContext(ctx, query, sessionID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []provider.Message
	for rows.Next() {
		var msg provider.Message
		var name sql.NullString

		if err := rows.Scan(&msg.Role, &msg.Content, &name); err != nil {
			return nil, err
		}

		if name.Valid {
			msg.Name = name.String
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse if we got last N messages
	if opts.MaxMessages > 0 && len(messages) > 0 {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}

	// Handle KeepSystemPrompt
	if opts.KeepSystemPrompt && len(messages) > 0 {
		if messages[0].Role == "system" {
			// System prompt already first, do nothing
		} else {
			// Try to find system prompt in full history
			var systemPrompt *provider.Message
			row := s.db.QueryRowContext(ctx, `
				SELECT role, content, name
				FROM session_messages
				WHERE session_id = $1 AND role = 'system'
				ORDER BY id ASC
				LIMIT 1
			`, sessionID)

			var msg provider.Message
			var name sql.NullString
			if err := row.Scan(&msg.Role, &msg.Content, &name); err == nil {
				if name.Valid {
					msg.Name = name.String
				}
				systemPrompt = &msg
			}

			if systemPrompt != nil {
				messages = append([]provider.Message{*systemPrompt}, messages...)
			}
		}
	}

	return messages, nil
}

// AppendMessages adds new messages to a session.
// Implements session.SessionStore interface.
func (s *PostgresStore) AppendMessages(ctx context.Context, sessionID string, msgs []provider.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Ensure session exists
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)", sessionID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return session.ErrSessionNotFound
	}

	// Insert messages
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO session_messages (session_id, role, content, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, msg := range msgs {
		var name interface{} = sql.NullString{}
		if msg.Name != "" {
			name = msg.Name
		}

		if _, err := stmt.ExecContext(ctx, sessionID, msg.Role, msg.Content, name, now); err != nil {
			return err
		}
	}

	// Update session timestamp
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET updated_at = $1 WHERE id = $2", now, sessionID); err != nil {
		return err
	}

	return tx.Commit()
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
