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
