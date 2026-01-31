package sessionstore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/Michaelxwb/ai-api-sdk/provider"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// FileStore implements a simple JSON file-based session store.
// It is intended for local persistence and small datasets.
type FileStore struct {
	mu       sync.Mutex
	path     string
	sessions map[string]*fileSession
}

type fileSession struct {
	Messages []provider.Message  `json:"messages"`
	Meta     session.SessionMeta `json:"meta"`
	Version  int64               `json:"version"`
}

type fileData struct {
	Sessions map[string]*fileSession `json:"sessions"`
}

// NewFileStore loads or creates a JSON-backed store at the given path.
func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:     path,
		sessions: make(map[string]*fileSession),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// GetMessages retrieves message history for a session.
func (s *FileStore) GetMessages(_ context.Context, sessionID string, opts session.GetOptions) ([]provider.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	msgs := append([]provider.Message(nil), entry.Messages...)
	if opts.MaxMessages > 0 {
		policy := session.WindowPolicy{
			MaxMessages:      opts.MaxMessages,
			KeepSystemPrompt: opts.KeepSystemPrompt,
		}
		msgs = policy.Truncate(msgs)
	}
	return msgs, nil
}

// AppendMessages appends messages and persists to disk.
func (s *FileStore) AppendMessages(_ context.Context, sessionID string, msgs []provider.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[sessionID]
	if entry == nil {
		entry = &fileSession{}
		s.sessions[sessionID] = entry
	}

	entry.Messages = append(entry.Messages, msgs...)
	entry.Version++
	updateMeta(&entry.Meta, sessionID)
	return s.save()
}

// CreateSession creates a new session with metadata.
func (s *FileStore) CreateSession(_ context.Context, sessionID string, meta *session.SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		return session.ErrSessionConflict
	}

	entry := &fileSession{}
	applyMeta(&entry.Meta, sessionID, meta)
	s.sessions[sessionID] = entry
	return s.save()
}

// DeleteSession deletes a session and persists.
func (s *FileStore) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		return session.ErrSessionNotFound
	}
	delete(s.sessions, sessionID)
	return s.save()
}

// GetMeta returns session metadata.
func (s *FileStore) GetMeta(_ context.Context, sessionID string) (*session.SessionMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	meta := cloneMeta(entry.Meta)
	return &meta, nil
}

// UpsertMeta updates or inserts metadata.
func (s *FileStore) UpsertMeta(_ context.Context, sessionID string, meta *session.SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.sessions[sessionID]
	if entry == nil {
		entry = &fileSession{}
		s.sessions[sessionID] = entry
	}
	applyMeta(&entry.Meta, sessionID, meta)
	return s.save()
}

// GetVersion returns the current session version.
func (s *FileStore) GetVersion(_ context.Context, sessionID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return 0, session.ErrSessionNotFound
	}
	return entry.Version, nil
}

// AppendMessagesWithVersion appends messages with optimistic locking.
func (s *FileStore) AppendMessagesWithVersion(_ context.Context, sessionID string, expectedVersion int64, msgs []provider.Message) (int64, error) {
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
		entry = &fileSession{}
		s.sessions[sessionID] = entry
	}
	if entry.Version != expectedVersion {
		return entry.Version, session.ErrSessionConflict
	}

	entry.Messages = append(entry.Messages, msgs...)
	entry.Version++
	updateMeta(&entry.Meta, sessionID)
	if err := s.save(); err != nil {
		return entry.Version, err
	}
	return entry.Version, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var parsed fileData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	if parsed.Sessions == nil {
		parsed.Sessions = make(map[string]*fileSession)
	}
	s.sessions = parsed.Sessions
	return nil
}

func (s *FileStore) save() error {
	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	payload, err := json.MarshalIndent(fileData{Sessions: s.sessions}, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

var (
	_ session.SessionStore              = (*FileStore)(nil)
	_ session.SessionStoreWithLifecycle = (*FileStore)(nil)
	_ session.SessionStoreWithMeta      = (*FileStore)(nil)
	_ session.SessionStoreWithVersion   = (*FileStore)(nil)
)
