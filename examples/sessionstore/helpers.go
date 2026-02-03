package sessionstore

import (
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
)

// Helper functions for metadata management shared across store implementations

func updateMeta(meta *session.SessionMeta, sessionID string) {
	applyMeta(meta, sessionID, nil)
}

func applyMeta(meta *session.SessionMeta, sessionID string, updates *session.SessionMeta) {
	now := time.Now()
	if meta.ID == "" {
		meta.ID = sessionID
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	meta.UpdatedAt = now

	if updates == nil {
		return
	}

	if updates.Provider != "" {
		meta.Provider = updates.Provider
	}
	if updates.Model != "" {
		meta.Model = updates.Model
	}
	if updates.Attrs != nil {
		meta.Attrs = updates.Attrs
	}
}

func cloneSessionMeta(meta session.SessionMeta) session.SessionMeta {
	clone := meta
	if meta.Attrs != nil {
		attrs := make(map[string]any, len(meta.Attrs))
		for k, v := range meta.Attrs {
			attrs[k] = v
		}
		clone.Attrs = attrs
	}
	return clone
}

func cloneMessages(msgs []session.Message) []session.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]session.Message, len(msgs))
	copy(out, msgs)
	return out
}

func cloneMeta(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func cloneState(state *session.SessionState) *session.SessionState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.Messages = cloneMessages(state.Messages)
	cloned.Meta = cloneMeta(state.Meta)
	return &cloned
}

func cloneAttrs(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	return out
}

func metaFromState(state *session.SessionState) *session.SessionMeta {
	if state == nil {
		return nil
	}
	meta := &session.SessionMeta{
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

func normalizeMetaForSave(state *session.SessionState, existing *session.SessionMeta, now time.Time) *session.SessionMeta {
	meta := metaFromState(state)
	if meta == nil {
		meta = &session.SessionMeta{}
	}
	if meta.ID == "" {
		meta.ID = state.ID
	}
	if meta.Provider == "" && existing != nil {
		meta.Provider = existing.Provider
	}
	if meta.CreatedAt.IsZero() {
		if existing != nil && !existing.CreatedAt.IsZero() {
			meta.CreatedAt = existing.CreatedAt
		} else {
			meta.CreatedAt = now
		}
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = now
	}
	if existing != nil {
		if meta.Model == "" {
			meta.Model = existing.Model
		}
		if existing.Attrs != nil {
			if meta.Attrs == nil {
				meta.Attrs = cloneAttrs(existing.Attrs)
			} else {
				for k, v := range existing.Attrs {
					if _, exists := meta.Attrs[k]; !exists {
						meta.Attrs[k] = v
					}
				}
			}
		}
	}
	return meta
}

func applyStoredMeta(state *session.SessionState, meta *session.SessionMeta) {
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
		state.Meta[k] = stringifyMetaValue(v)
	}
}

func stringifyMetaValue(value any) string {
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

func isMessagePrefix(prefix, full []session.Message) bool {
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
