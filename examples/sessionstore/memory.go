package sessionstore

import (
	"context"
	"sync"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// MemoryStore implements an in-memory session store for development/testing.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*memorySession
}

type memorySession struct {
	messages []base.Message
	meta     session.SessionMeta
	version  int64
	closed   bool
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*memorySession)}
}

// GetMessages retrieves messages for a session.
func (s *MemoryStore) GetMessages(_ context.Context, sessionID string, opts session.GetOptions) ([]base.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	if entry.closed {
		return nil, session.ErrSessionClosed
	}

	msgs := append([]base.Message(nil), entry.messages...)
	if opts.MaxMessages > 0 {
		policy := session.WindowPolicy{
			MaxMessages:      opts.MaxMessages,
			KeepSystemPrompt: opts.KeepSystemPrompt,
		}
		msgs = policy.Truncate(msgs)
	}
	return msgs, nil
}

// AppendMessages appends messages to a session.
func (s *MemoryStore) AppendMessages(_ context.Context, sessionID string, msgs []base.Message) error {
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
	meta := cloneMeta(entry.meta)
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
func (s *MemoryStore) AppendMessagesWithVersion(_ context.Context, sessionID string, expectedVersion int64, msgs []base.Message) (int64, error) {
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
	_ session.LegacySessionStore        = (*MemoryStore)(nil)
	_ session.SessionStoreWithLifecycle = (*MemoryStore)(nil)
	_ session.SessionStoreWithMeta      = (*MemoryStore)(nil)
	_ session.SessionStoreWithVersion   = (*MemoryStore)(nil)
)
