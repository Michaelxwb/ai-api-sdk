package session

import (
	"context"
	"errors"
)

// SessionStore defines the minimal interface for conversation storage.
// Implementations must be concurrency-safe; the SDK does not add locking.
//
// GetMessages returns the message history for the session ID.
// If the session does not exist, it should return ErrSessionNotFound.
//
// AppendMessages appends new messages to the session.
// Implementations may implicitly create the session on append.
type SessionStore interface {
	GetMessages(ctx context.Context, sessionID string, opts GetOptions) ([]Message, error)
	AppendMessages(ctx context.Context, sessionID string, msgs []Message) error
}

// SessionStoreWithLifecycle is an optional extension for explicit lifecycle control.
type SessionStoreWithLifecycle interface {
	CreateSession(ctx context.Context, sessionID string, meta *SessionMeta) error
	DeleteSession(ctx context.Context, sessionID string) error
}

// SessionStoreWithMeta is an optional extension for reading/writing session metadata.
type SessionStoreWithMeta interface {
	GetMeta(ctx context.Context, sessionID string) (*SessionMeta, error)
	UpsertMeta(ctx context.Context, sessionID string, meta *SessionMeta) error
}

// SessionStoreWithVersion is an optional extension for optimistic concurrency control.
// Implementations should return ErrSessionConflict when the expected version mismatches.
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
