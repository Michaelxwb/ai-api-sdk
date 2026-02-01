package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// ChatWithStream sends a streaming request using caller-provided credential and provider config.
func (c *Client) ChatWithStream(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	req.Stream = true

	prep, err := c.prepareChatWithRequest(ctx, cred, pc, req)
	if err != nil {
		return nil, err
	}

	streamSpec, ok := prep.spec.(streaming.ProviderStreamSpec)
	if !ok {
		return nil, fmt.Errorf("client: provider spec %s does not support streaming", prep.spec.Name())
	}

	transport := &AuthTransport{
		Base:     c.HTTP.Transport,
		Strategy: prep.strategy,
		Cred:     prep.cred,
	}
	streamClient := &http.Client{Transport: transport, Timeout: 0}
	resp, err := streamClient.Do(prep.httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
	}
	return streamSpec.ParseStreamResponse(resp)
}

// ChatStream sends a streaming request using local config.yaml and auth manager.
func (c *Client) ChatStream(ctx context.Context, providerName string, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	resolved, err := c.resolveChatInputs(providerName)
	if err != nil {
		return nil, err
	}

	stream, err := c.ChatWithStream(ctx, resolved.cred, resolved.pc, req)
	if err != nil {
		if c.AuthMgr != nil && resolved.cred != nil {
			c.AuthMgr.MarkFailed(resolved.cred.ID)
		}
		return nil, err
	}
	if c.AuthMgr != nil && resolved.cred != nil {
		c.AuthMgr.MarkSuccess(resolved.cred.ID)
	}
	return stream, nil
}

// ChatSessionStream performs a multi-turn chat in streaming mode.
func (c *Client) ChatSessionStream(ctx context.Context, providerName, sessionID string, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	if c == nil {
		return nil, errors.New("client: nil client")
	}
	if c.SessionStore == nil {
		return nil, errors.New("client: session store not configured")
	}
	if providerName == "" {
		return nil, errors.New("client: missing provider name")
	}
	if sessionID == "" {
		return nil, errors.New("client: missing session id")
	}

	history, version, err := c.loadSessionHistory(ctx, providerName, sessionID, req.Model)
	if err != nil {
		return nil, err
	}

	merged := make([]base.Message, 0, len(history)+len(req.Messages))
	merged = append(merged, history...)
	merged = append(merged, req.Messages...)
	if c.SessionConfig.TruncatePolicy != nil {
		merged = c.SessionConfig.TruncatePolicy.Truncate(merged)
	}

	newReq := req
	newReq.Messages = merged

	stream, err := c.ChatStream(ctx, providerName, newReq)
	if err != nil {
		return nil, err
	}

	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)

		appendMsgs := make([]base.Message, 0, len(req.Messages)+1)
		appendMsgs = append(appendMsgs, req.Messages...)

		var fullText string
		var streamErr error

		for chunk := range stream {
			out <- chunk
			if chunk.Error != nil {
				streamErr = chunk.Error
				continue
			}
			if chunk.Text != "" {
				fullText += chunk.Text
			}
		}

		if streamErr != nil {
			return
		}

		appendMsgs = append(appendMsgs, base.Message{Role: "assistant", Content: fullText})
		if err := c.appendSessionMessages(ctx, sessionID, version, appendMsgs); err != nil {
			c.fireStoreError(ctx, err)
			if errors.Is(err, session.ErrSessionConflict) {
				sendSessionStreamError(ctx, out, err)
				return
			}
			sendSessionStreamError(ctx, out, err)
			return
		}

		if metaStore, ok := c.SessionStore.(session.SessionStoreWithMeta); ok {
			meta := &session.SessionMeta{ID: sessionID, Provider: providerName, Model: req.Model}
			_ = metaStore.UpsertMeta(ctx, sessionID, meta)
		}
	}()

	return out, nil
}

// ChatWithStreamSync collects a streaming response into a single ChatResponse.
func (c *Client) ChatWithStreamSync(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error) {
	stream, err := c.ChatWithStream(ctx, cred, pc, req)
	if err != nil {
		return base.ChatResponse{}, err
	}
	return collectStream(stream)
}

// ChatStreamSync collects a streaming response into a single ChatResponse.
func (c *Client) ChatStreamSync(ctx context.Context, providerName string, req base.ChatRequest) (base.ChatResponse, error) {
	stream, err := c.ChatStream(ctx, providerName, req)
	if err != nil {
		return base.ChatResponse{}, err
	}
	return collectStream(stream)
}

// ChatSessionStreamSync collects a session streaming response into a single ChatResponse.
func (c *Client) ChatSessionStreamSync(ctx context.Context, providerName, sessionID string, req base.ChatRequest) (base.ChatResponse, error) {
	stream, err := c.ChatSessionStream(ctx, providerName, sessionID, req)
	if err != nil {
		return base.ChatResponse{}, err
	}
	return collectStream(stream)
}

func collectStream(stream <-chan streaming.StreamChunk) (base.ChatResponse, error) {
	var fullText string
	for chunk := range stream {
		if chunk.Error != nil {
			return base.ChatResponse{}, chunk.Error
		}
		fullText += chunk.Text
	}
	return base.ChatResponse{Text: fullText}, nil
}

func collectStreamWithPartial(stream <-chan streaming.StreamChunk) (base.ChatResponse, error) {
	var fullText string
	for chunk := range stream {
		if chunk.Text != "" {
			fullText += chunk.Text
		}
		if chunk.Error != nil {
			return base.ChatResponse{Text: fullText}, chunk.Error
		}
	}
	return base.ChatResponse{Text: fullText}, nil
}

func sendSessionStreamError(ctx context.Context, out chan<- streaming.StreamChunk, err error) {
	if err == nil {
		return
	}
	select {
	case out <- streaming.StreamChunk{Error: err, Done: true}:
	case <-ctx.Done():
	}
}
