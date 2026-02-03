package session

import (
	"time"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// Message is an alias to base.Message for session storage.
type Message = base.Message

// SessionState represents a full session snapshot.
type SessionState struct {
	ID        string            `json:"id"`
	Provider  string            `json:"provider"`
	Messages  []Message         `json:"messages"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// SessionMeta carries basic session metadata.
type SessionMeta struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"`
	Model     string         `json:"model"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}

// GetOptions provides hints for retrieving session history.
// Implementations may ignore these options or apply them for efficiency.
type GetOptions struct {
	MaxMessages      int  // maximum number of messages to return (keep last N)
	MaxTokens        int  // token budget hint (store-specific)
	KeepSystemPrompt bool // keep system prompts when truncating
}
