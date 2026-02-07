package client

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// chatWithStream 内部实现方法（仅供Session.ChatStream使用）
func (c *Client) chatWithStream(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
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
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(data), Op: "stream"}
	}
	return streamSpec.ParseStreamResponse(resp)
}

// chatWithStreamSync 内部实现方法（仅供内部使用）
func (c *Client) chatWithStreamSync(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error) {
	stream, err := c.chatWithStream(ctx, cred, pc, req)
	if err != nil {
		return base.ChatResponse{}, err
	}
	return collectStream(stream)
}

func collectStream(stream <-chan streaming.StreamChunk) (base.ChatResponse, error) {
	var fullText string
	var sessionID string
	var usage *base.Usage
	for chunk := range stream {
		if chunk.Error != nil {
			return base.ChatResponse{}, chunk.Error
		}
		fullText += chunk.Text
		if chunk.SessionID != "" {
			sessionID = chunk.SessionID
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	return base.ChatResponse{Text: fullText, SessionID: sessionID, Usage: usage}, nil
}

func collectStreamWithPartial(stream <-chan streaming.StreamChunk) (base.ChatResponse, error) {
	var fullText string
	var sessionID string
	var usage *base.Usage
	for chunk := range stream {
		if chunk.Text != "" {
			fullText += chunk.Text
		}
		if chunk.SessionID != "" {
			sessionID = chunk.SessionID
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Error != nil {
			return base.ChatResponse{Text: fullText, SessionID: sessionID, Usage: usage}, chunk.Error
		}
	}
	return base.ChatResponse{Text: fullText, SessionID: sessionID, Usage: usage}, nil
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
