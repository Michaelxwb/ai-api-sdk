package sessionstore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
)

// MemoryStore implements an in-memory session store for development/testing.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*memorySession
}

type memorySession struct {
	messages []session.Message
	meta     session.SessionMeta
	version  int64
	closed   bool
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*memorySession)}
}

// Get retrieves a session snapshot.
func (s *MemoryStore) Get(_ context.Context, sessionID string) (*session.SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	if entry.closed {
		return nil, session.ErrSessionClosed
	}

	state := &session.SessionState{
		ID:       sessionID,
		Messages: cloneMessages(entry.messages),
	}
	applyStoredMeta(state, &entry.meta)
	return state, nil
}

// Save writes a session snapshot.
func (s *MemoryStore) Save(_ context.Context, state *session.SessionState) error {
	if state == nil {
		return errors.New("session store: nil state")
	}
	if state.ID == "" {
		return errors.New("session store: missing session id")
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[state.ID]
	var existingMeta *session.SessionMeta
	if entry == nil {
		entry = &memorySession{}
		s.sessions[state.ID] = entry
	} else {
		existingMeta = &entry.meta
	}

	meta := normalizeMetaForSave(state, existingMeta, now)
	entry.messages = cloneMessages(state.Messages)
	entry.meta = *meta
	if entry.version == 0 {
		entry.version = 1
	} else {
		entry.version++
	}
	return nil
}

// Delete removes a session.
func (s *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	return s.DeleteSession(ctx, sessionID)
}

// Append appends messages to an existing session.
func (s *MemoryStore) Append(_ context.Context, sessionID string, msgs ...session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[sessionID]
	if entry == nil {
		entry = &memorySession{}
		s.sessions[sessionID] = entry
	}
	if entry.closed {
		return session.ErrSessionClosed
	}

	entry.messages = append(entry.messages, msgs...)
	entry.version++
	updateMeta(&entry.meta, sessionID)
	return nil
}

// CreateSession creates a new session with metadata.
func (s *MemoryStore) CreateSession(_ context.Context, sessionID string, meta *session.SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		return session.ErrSessionConflict
	}

	entry := &memorySession{}
	applyMeta(&entry.meta, sessionID, meta)
	s.sessions[sessionID] = entry
	return nil
}

// DeleteSession removes a session.
func (s *MemoryStore) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		return session.ErrSessionNotFound
	}
	delete(s.sessions, sessionID)
	return nil
}

// GetMeta returns session metadata.
func (s *MemoryStore) GetMeta(_ context.Context, sessionID string) (*session.SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	meta := cloneSessionMeta(entry.meta)
	return &meta, nil
}

// UpsertMeta updates or inserts metadata.
func (s *MemoryStore) UpsertMeta(_ context.Context, sessionID string, meta *session.SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[sessionID]
	if entry == nil {
		entry = &memorySession{}
		s.sessions[sessionID] = entry
	}
	applyMeta(&entry.meta, sessionID, meta)
	return nil
}

// GetVersion returns the current session version.
func (s *MemoryStore) GetVersion(_ context.Context, sessionID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return 0, session.ErrSessionNotFound
	}
	return entry.version, nil
}

// AppendMessagesWithVersion appends messages with optimistic locking.
func (s *MemoryStore) AppendMessagesWithVersion(_ context.Context, sessionID string, expectedVersion int64, msgs []session.Message) (int64, error) {
	if len(msgs) == 0 {
		return expectedVersion, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[sessionID]
	if entry == nil {
		if expectedVersion != 0 {
			return 0, session.ErrSessionConflict
		}
		entry = &memorySession{}
		s.sessions[sessionID] = entry
	}
	if entry.version != expectedVersion {
		return entry.version, session.ErrSessionConflict
	}
	if entry.closed {
		return entry.version, session.ErrSessionClosed
	}

	entry.messages = append(entry.messages, msgs...)
	entry.version++
	updateMeta(&entry.meta, sessionID)
	return entry.version, nil
}

var (
	_ session.SessionStore         = (*MemoryStore)(nil)
	_ session.SessionStoreAppender = (*MemoryStore)(nil)
)
