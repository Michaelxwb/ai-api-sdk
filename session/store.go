package session

import (
	"context"
	"errors"
)

// SessionStore defines the minimal interface for session storage.
// Implementations must be concurrency-safe; the SDK does not add locking.
type SessionStore interface {
	Get(ctx context.Context, id string) (*SessionState, error)
	Save(ctx context.Context, state *SessionState) error
	Delete(ctx context.Context, id string) error
}

// SessionStoreAppender is an optional extension for message appends.
type SessionStoreAppender interface {
	Append(ctx context.Context, id string, msgs ...Message) error
}

// LegacySessionStore defines the previous message-based storage interface.
// Deprecated: prefer SessionStore.
type LegacySessionStore interface {
	GetMessages(ctx context.Context, sessionID string, opts GetOptions) ([]Message, error)
	AppendMessages(ctx context.Context, sessionID string, msgs []Message) error
}

// SessionStoreWithLifecycle is an optional extension for explicit lifecycle control.
// Deprecated: legacy interface for message-based stores.
type SessionStoreWithLifecycle interface {
	CreateSession(ctx context.Context, sessionID string, meta *SessionMeta) error
	DeleteSession(ctx context.Context, sessionID string) error
}

// SessionStoreWithMeta is an optional extension for reading/writing session metadata.
// Deprecated: legacy interface for message-based stores.
type SessionStoreWithMeta interface {
	GetMeta(ctx context.Context, sessionID string) (*SessionMeta, error)
	UpsertMeta(ctx context.Context, sessionID string, meta *SessionMeta) error
}

// SessionStoreWithVersion is an optional extension for optimistic concurrency control.
// Implementations should return ErrSessionConflict when the expected version mismatches.
// Deprecated: legacy interface for message-based stores.
type SessionStoreWithVersion interface {
	GetVersion(ctx context.Context, sessionID string) (int64, error)
	AppendMessagesWithVersion(ctx context.Context, sessionID string, expectedVersion int64, msgs []Message) (int64, error)
}

var (
	// ErrSessionNotFound indicates the requested session does not exist.
	ErrSessionNotFound = errors.New("session store: session not found")
	// ErrSessionConflict indicates an optimistic-locking conflict.
	ErrSessionConflict = errors.New("session store: version conflict")
	// ErrSessionClosed indicates the session has been closed for further writes.
	ErrSessionClosed = errors.New("session store: session closed")
	// ErrStoreUnavailable indicates the underlying store is unavailable.
	ErrStoreUnavailable = errors.New("session store: unavailable")
)
