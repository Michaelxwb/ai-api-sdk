package session

import (
	"context"
	"errors"
	"sync"
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

// NewMemoryStore returns a built-in in-memory session store. It is concurrency-safe.
func NewMemoryStore() SessionStore {
	return &memoryStore{data: make(map[string]*SessionState)}
}

type memoryStore struct {
	mu   sync.RWMutex
	data map[string]*SessionState
}

func (m *memoryStore) Get(_ context.Context, id string) (*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.data[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return state, nil
}

func (m *memoryStore) Save(_ context.Context, state *SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[state.ID] = state
	return nil
}

func (m *memoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
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
