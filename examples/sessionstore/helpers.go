package sessionstore

import (
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

func cloneMeta(meta session.SessionMeta) session.SessionMeta {
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
