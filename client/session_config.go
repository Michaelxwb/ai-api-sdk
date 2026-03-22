package client

import (
	"context"

	"github.com/Michaelxwb/ai-api-sdk/session"
)

// HistoryWindow defines dual thresholds for local history truncation.
type HistoryWindow struct {
	// MaxMessages is the maximum number of history messages to include.
	// Zero means no message count limit.
	MaxMessages int
	// MaxTokens is the approximate token budget for history messages.
	// Token estimation uses len(content)/4 per message. Zero means no token limit.
	MaxTokens int
}

// SessionConfig controls multi-turn session behavior.
type SessionConfig struct {
	// AutoCreate indicates whether missing sessions should be created automatically
	// when the store supports explicit lifecycle management.
	AutoCreate bool

	// TruncatePolicy applies message truncation before sending a request.
	TruncatePolicy session.TruncatePolicy

	// OnStoreError is invoked when a session store operation returns an error.
	// The error is still returned to the caller unless retried successfully.
	OnStoreError func(context.Context, error)

	// MaxConflictRetry controls retry attempts on ErrSessionConflict.
	// Zero means no retry; negative values are treated as zero.
	MaxConflictRetry int

	// OnError defines the error handling strategy for multi-turn conversations.
	// Default is OnErrorAbort.
	OnError OnErrorStrategy

	// HistoryWindow defines truncation thresholds for local_history mode.
	// When set, loaded history is truncated before being sent.
	HistoryWindow HistoryWindow
}
