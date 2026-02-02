package session

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// LegacyStoreAdapter adapts a LegacySessionStore to the new SessionStore interface.
type LegacyStoreAdapter struct {
	store LegacySessionStore
}

// AdaptLegacyStore wraps a legacy store into the new SessionStore interface.
func AdaptLegacyStore(store LegacySessionStore) SessionStore {
	if store == nil {
		return nil
	}
	return &LegacyStoreAdapter{store: store}
}

// NewLegacyAdapter returns a concrete adapter for legacy stores.
func NewLegacyAdapter(store LegacySessionStore) *LegacyStoreAdapter {
	if store == nil {
		return nil
	}
	return &LegacyStoreAdapter{store: store}
}

// Get returns a SessionState built from legacy store data.
func (a *LegacyStoreAdapter) Get(ctx context.Context, id string) (*SessionState, error) {
	msgs, msgErr := a.store.GetMessages(ctx, id, GetOptions{})

	var meta *SessionMeta
	if metaStore, ok := a.store.(SessionStoreWithMeta); ok {
		m, err := metaStore.GetMeta(ctx, id)
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			return nil, err
		}
		meta = m
	}

	if msgErr != nil {
		if errors.Is(msgErr, ErrSessionNotFound) && meta != nil {
			return legacyStateFromMeta(id, nil, meta), nil
		}
		return nil, msgErr
	}

	state := &SessionState{
		ID:       id,
		Messages: cloneMessages(msgs),
	}
	if meta != nil {
		applyLegacyMeta(state, meta)
	}
	return state, nil
}

// Save writes the provided state into the legacy store.
func (a *LegacyStoreAdapter) Save(ctx context.Context, state *SessionState) error {
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

	metaStore, hasMeta := a.store.(SessionStoreWithMeta)
	if hasMeta {
		meta := legacyMetaFromState(state)
		if meta != nil {
			if err := metaStore.UpsertMeta(ctx, state.ID, meta); err != nil {
				return err
			}
		}
	}

	existing, err := a.store.GetMessages(ctx, state.ID, GetOptions{})
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			if lifecycle, ok := a.store.(SessionStoreWithLifecycle); ok {
				if err := lifecycle.CreateSession(ctx, state.ID, legacyMetaFromState(state)); err != nil && !errors.Is(err, ErrSessionConflict) {
					return err
				}
			}
			if len(state.Messages) == 0 {
				return nil
			}
			return a.store.AppendMessages(ctx, state.ID, state.Messages)
		}
		return err
	}

	if isMessagePrefix(existing, state.Messages) {
		delta := state.Messages[len(existing):]
		if len(delta) == 0 {
			return nil
		}
		return a.store.AppendMessages(ctx, state.ID, delta)
	}

	if lifecycle, ok := a.store.(SessionStoreWithLifecycle); ok {
		if err := lifecycle.DeleteSession(ctx, state.ID); err != nil && !errors.Is(err, ErrSessionNotFound) {
			return err
		}
		if err := lifecycle.CreateSession(ctx, state.ID, legacyMetaFromState(state)); err != nil && !errors.Is(err, ErrSessionConflict) {
			return err
		}
		if len(state.Messages) == 0 {
			return nil
		}
		return a.store.AppendMessages(ctx, state.ID, state.Messages)
	}

	return ErrSessionConflict
}

// Delete removes a session using legacy lifecycle hooks when available.
func (a *LegacyStoreAdapter) Delete(ctx context.Context, id string) error {
	if lifecycle, ok := a.store.(SessionStoreWithLifecycle); ok {
		return lifecycle.DeleteSession(ctx, id)
	}
	return ErrStoreUnavailable
}

// Append forwards to the legacy AppendMessages implementation.
func (a *LegacyStoreAdapter) Append(ctx context.Context, id string, msgs ...Message) error {
	if len(msgs) == 0 {
		return nil
	}
	return a.store.AppendMessages(ctx, id, msgs)
}

func legacyMetaFromState(state *SessionState) *SessionMeta {
	if state == nil {
		return nil
	}
	meta := &SessionMeta{
		ID:        state.ID,
		Provider:  state.Provider,
		CreatedAt: state.CreatedAt,
		UpdatedAt: state.UpdatedAt,
	}
	if len(state.Meta) == 0 {
		return meta
	}
	attrs := make(map[string]any, len(state.Meta))
	for k, v := range state.Meta {
		if k == "model" {
			meta.Model = v
			continue
		}
		attrs[k] = v
	}
	if len(attrs) > 0 {
		meta.Attrs = attrs
	}
	return meta
}

func applyLegacyMeta(state *SessionState, meta *SessionMeta) {
	if state == nil || meta == nil {
		return
	}
	state.Provider = meta.Provider
	state.CreatedAt = meta.CreatedAt
	state.UpdatedAt = meta.UpdatedAt
	if meta.Model == "" && len(meta.Attrs) == 0 {
		return
	}
	if state.Meta == nil {
		state.Meta = make(map[string]string, len(meta.Attrs)+1)
	}
	if meta.Model != "" {
		state.Meta["model"] = meta.Model
	}
	for k, v := range meta.Attrs {
		state.Meta[k] = toString(v)
	}
}

func legacyStateFromMeta(id string, msgs []Message, meta *SessionMeta) *SessionState {
	state := &SessionState{
		ID:       id,
		Messages: cloneMessages(msgs),
	}
	applyLegacyMeta(state, meta)
	return state
}

func isMessagePrefix(prefix, full []Message) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if prefix[i] != full[i] {
			return false
		}
	}
	return true
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

var (
	_ SessionStore         = (*LegacyStoreAdapter)(nil)
	_ SessionStoreAppender = (*LegacyStoreAdapter)(nil)
)
