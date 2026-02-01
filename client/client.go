package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// Client provides a unified API client.
type Client struct {
	HTTP    *http.Client
	AuthMgr *auth.Manager
	Config  *config.Config

	// SessionStore enables multi-turn conversation persistence.
	SessionStore session.SessionStore
	// SessionConfig controls session behaviors such as auto-create and truncation.
	SessionConfig SessionConfig
}

// New creates a lightweight client for platform integration (ChatWith mode).
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// NewClient creates a client with local config and auth manager (Chat mode).
func NewClient(cfg *config.Config, mgr *auth.Manager) *Client {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	return &Client{HTTP: httpClient, AuthMgr: mgr, Config: cfg}
}

// ChatWith sends a request using caller-provided credential and provider config.
// This is the primary interface for platform integration — the platform manages
// credentials in its own database and passes them directly to the SDK.
func (c *Client) ChatWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error) {
	prep, err := c.prepareChatWithRequest(ctx, cred, pc, req)
	if err != nil {
		return base.ChatResponse{}, err
	}

	transport := &AuthTransport{
		Base:     c.HTTP.Transport,
		Strategy: prep.strategy,
		Cred:     prep.cred,
	}
	httpClient := &http.Client{Transport: transport, Timeout: c.HTTP.Timeout}
	resp, err := httpClient.Do(prep.httpReq)
	if err != nil {
		return base.ChatResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return base.ChatResponse{}, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
	}
	return prep.spec.ParseResponse(resp)
}

// Chat sends a request using local config.yaml and auth manager.
// Suitable for CLI tools and standalone usage.
func (c *Client) Chat(ctx context.Context, providerName string, req base.ChatRequest) (base.ChatResponse, error) {
	resolved, err := c.resolveChatInputs(providerName)
	if err != nil {
		return base.ChatResponse{}, err
	}
	pc := resolved.pc
	spec := resolved.spec

	baseURL := pc.BaseURL
	if baseURL == "" {
		baseURL = spec.DefaultBaseURL()
	}
	opts := base.BuildOptions{
		BaseURL:   baseURL,
		Path:      pc.Path,
		ExtraBody: pc.ExtraBody,
		Headers:   pc.Headers,
	}
	httpReq, err := spec.BuildRequest(ctx, opts, req)
	if err != nil {
		return base.ChatResponse{}, err
	}
	for k, v := range pc.Headers {
		httpReq.Header.Set(k, v)
	}

	transport := &AuthTransport{
		Base:     c.HTTP.Transport,
		Strategy: resolved.strategy,
		Manager:  c.AuthMgr,
		Cred:     resolved.cred,
	}
	httpClient := &http.Client{Transport: transport, Timeout: c.HTTP.Timeout}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		if c.AuthMgr != nil && resolved.cred != nil {
			c.AuthMgr.MarkFailed(resolved.cred.ID)
		}
		return base.ChatResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		if c.AuthMgr != nil && resolved.cred != nil {
			c.AuthMgr.MarkFailed(resolved.cred.ID)
		}
		return base.ChatResponse{}, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
	}
	if c.AuthMgr != nil && resolved.cred != nil {
		c.AuthMgr.MarkSuccess(resolved.cred.ID)
	}
	return spec.ParseResponse(resp)
}
