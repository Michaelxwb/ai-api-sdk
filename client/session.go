package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/Michaelxwb/ai-api-sdk/provider"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// ChatSession performs a multi-turn chat using stored session history.
// It merges stored messages with the new request, applies optional truncation,
// sends a stateless Chat request, then appends the new messages and response.
func (c *Client) ChatSession(ctx context.Context, providerName, sessionID string, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c == nil {
		return provider.ChatResponse{}, errors.New("client: nil client")
	}
	if c.SessionStore == nil {
		return provider.ChatResponse{}, errors.New("client: session store not configured")
	}
	if providerName == "" {
		return provider.ChatResponse{}, errors.New("client: missing provider name")
	}
	if sessionID == "" {
		return provider.ChatResponse{}, errors.New("client: missing session id")
	}

	maxRetry := c.SessionConfig.MaxConflictRetry
	if maxRetry < 0 {
		maxRetry = 0
	}

	for attempt := 0; attempt <= maxRetry; attempt++ {
		history, version, err := c.loadSessionHistory(ctx, providerName, sessionID, req.Model)
		if err != nil {
			return provider.ChatResponse{}, err
		}

		merged := make([]provider.Message, 0, len(history)+len(req.Messages))
		merged = append(merged, history...)
		merged = append(merged, req.Messages...)

		if c.SessionConfig.TruncatePolicy != nil {
			merged = c.SessionConfig.TruncatePolicy.Truncate(merged)
		}

		newReq := req
		newReq.Messages = merged

		resp, err := c.Chat(ctx, providerName, newReq)
		if err != nil {
			return resp, err
		}

		appendMsgs := make([]provider.Message, 0, len(req.Messages)+1)
		appendMsgs = append(appendMsgs, req.Messages...)
		appendMsgs = append(appendMsgs, provider.Message{Role: "assistant", Content: resp.Text})

		if err := c.appendSessionMessages(ctx, sessionID, version, appendMsgs); err != nil {
			c.fireStoreError(ctx, err)
			if errors.Is(err, session.ErrSessionConflict) && attempt < maxRetry {
				continue
			}
			return resp, err
		}

		if metaStore, ok := c.SessionStore.(session.SessionStoreWithMeta); ok {
			meta := &session.SessionMeta{ID: sessionID, Provider: providerName, Model: req.Model}
			_ = metaStore.UpsertMeta(ctx, sessionID, meta)
		}

		return resp, nil
	}

	return provider.ChatResponse{}, fmt.Errorf("client: session conflict after %d retries", maxRetry)
}

func (c *Client) loadSessionHistory(ctx context.Context, providerName, sessionID, model string) ([]provider.Message, int64, error) {
	store := c.SessionStore
	opts := session.GetOptions{}
	if policy := c.SessionConfig.TruncatePolicy; policy != nil {
		if withOpts, ok := policy.(interface{ Options() session.GetOptions }); ok {
			opts = withOpts.Options()
		}
	}

	msgs, err := store.GetMessages(ctx, sessionID, opts)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) && c.SessionConfig.AutoCreate {
			if lifecycle, ok := store.(session.SessionStoreWithLifecycle); ok {
				meta := &session.SessionMeta{ID: sessionID, Provider: providerName, Model: model}
				if err := lifecycle.CreateSession(ctx, sessionID, meta); err != nil && !errors.Is(err, session.ErrSessionConflict) {
					c.fireStoreError(ctx, err)
					return nil, 0, err
				}
			}
			msgs = nil
		} else {
			c.fireStoreError(ctx, err)
			return nil, 0, err
		}
	}

	var version int64
	if versioned, ok := store.(session.SessionStoreWithVersion); ok {
		v, err := versioned.GetVersion(ctx, sessionID)
		if err != nil {
			if errors.Is(err, session.ErrSessionNotFound) && c.SessionConfig.AutoCreate {
				return msgs, 0, nil
			}
			c.fireStoreError(ctx, err)
			return nil, 0, err
		}
		version = v
	}

	return msgs, version, nil
}

func (c *Client) appendSessionMessages(ctx context.Context, sessionID string, version int64, msgs []provider.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	if versioned, ok := c.SessionStore.(session.SessionStoreWithVersion); ok {
		_, err := versioned.AppendMessagesWithVersion(ctx, sessionID, version, msgs)
		return err
	}

	return c.SessionStore.AppendMessages(ctx, sessionID, msgs)
}

func (c *Client) fireStoreError(ctx context.Context, err error) {
	if c.SessionConfig.OnStoreError != nil && err != nil {
		c.SessionConfig.OnStoreError(ctx, err)
	}
}
