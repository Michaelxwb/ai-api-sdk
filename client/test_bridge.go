package client

import (
	"context"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// NewSessionForTest creates a Session with internal fields for external test packages.
// This is NOT part of the public API.
func NewSessionForTest(c *Client, id, provider string, store session.SessionStore, convMode ConversationMode) *Session {
	return &Session{
		client:           c,
		id:               id,
		provider:         provider,
		store:            store,
		conversationMode: convMode,
	}
}

// ExposePrependStoredMessages exposes the unexported prependStoredMessages for testing.
func (s *Session) ExposePrependStoredMessages(ctx context.Context, msgs []base.Message) []base.Message {
	return s.prependStoredMessages(ctx, msgs)
}

// ExposeSaveState exposes the unexported saveState for testing.
func (s *Session) ExposeSaveState(ctx context.Context, req base.ChatRequest, resp base.ChatResponse) {
	s.saveState(ctx, req, resp)
}

// ExposeSessionFromInferred exposes the unexported sessionFromInferred for testing.
func (c *Client) ExposeSessionFromInferred(inferred *generic.InferredIntegration, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	return c.sessionFromInferred(inferred, opts...)
}
