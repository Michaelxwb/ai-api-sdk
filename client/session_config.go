package client

import (
	"context"

	"github.com/Michaelxwb/ai-api-sdk/session"
)

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
}
