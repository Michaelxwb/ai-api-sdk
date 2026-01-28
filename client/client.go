package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-api-sdk/auth"
	"ai-api-sdk/config"
	"ai-api-sdk/provider"
)

// Client provides a unified API client.
type Client struct {
	HTTP    *http.Client
	AuthMgr *auth.Manager
	Config  *config.Config
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
func (c *Client) ChatWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req provider.ChatRequest) (provider.ChatResponse, error) {
	if pc == nil {
		return provider.ChatResponse{}, errors.New("client: missing provider config")
	}
	specName := pc.Type
	if specName == "" {
		specName = pc.Name
	}
	spec, ok := provider.Get(specName)
	if !ok {
		return provider.ChatResponse{}, fmt.Errorf("client: provider spec %s not registered", specName)
	}
	baseURL := pc.BaseURL
	if baseURL == "" {
		baseURL = spec.DefaultBaseURL()
	}

	strategy := auth.NewStrategyFromCredential(cred)
	if cred != nil {
		if override, ok := spec.AuthStrategyOverride(cred); ok {
			strategy = override
		}
	}

	opts := provider.BuildOptions{
		BaseURL:   baseURL,
		Path:      pc.Path,
		ExtraBody: pc.ExtraBody,
		Headers:   pc.Headers,
	}
	httpReq, err := spec.BuildRequest(ctx, opts, req)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	for k, v := range pc.Headers {
		httpReq.Header.Set(k, v)
	}

	transport := &AuthTransport{
		Base:     c.HTTP.Transport,
		Strategy: strategy,
		Cred:     cred,
	}
	httpClient := &http.Client{Transport: transport, Timeout: c.HTTP.Timeout}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return provider.ChatResponse{}, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
	}
	return spec.ParseResponse(resp)
}

// Chat sends a request using local config.yaml and auth manager.
// Suitable for CLI tools and standalone usage.
func (c *Client) Chat(ctx context.Context, providerName string, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c == nil || c.Config == nil {
		return provider.ChatResponse{}, errors.New("client: missing config")
	}
	pc := c.Config.FindProvider(providerName)
	if pc == nil {
		return provider.ChatResponse{}, fmt.Errorf("client: provider %s not configured", providerName)
	}
	specName := pc.Type
	if specName == "" {
		specName = pc.Name
	}
	spec, ok := provider.Get(specName)
	if !ok {
		return provider.ChatResponse{}, fmt.Errorf("client: provider spec %s not registered", specName)
	}
	baseURL := pc.BaseURL
	if baseURL == "" {
		baseURL = spec.DefaultBaseURL()
	}

	var cred *auth.Credential
	var strategy auth.AuthStrategy
	if c.AuthMgr != nil {
		if pc.AuthRef != "" {
			var err error
			cred, err = c.AuthMgr.Get(pc.AuthRef)
			if err != nil {
				return provider.ChatResponse{}, err
			}
			strategy = auth.NewStrategyFromCredential(cred)
		} else {
			var err error
			cred, strategy, err = c.AuthMgr.Resolve(spec.Name())
			if err != nil {
				return provider.ChatResponse{}, err
			}
		}
	}
	if cred != nil {
		if override, ok := spec.AuthStrategyOverride(cred); ok {
			strategy = override
		}
	}

	opts := provider.BuildOptions{
		BaseURL:   baseURL,
		Path:      pc.Path,
		ExtraBody: pc.ExtraBody,
		Headers:   pc.Headers,
	}
	httpReq, err := spec.BuildRequest(ctx, opts, req)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	for k, v := range pc.Headers {
		httpReq.Header.Set(k, v)
	}

	transport := &AuthTransport{
		Base:     c.HTTP.Transport,
		Strategy: strategy,
		Manager:  c.AuthMgr,
		Cred:     cred,
	}
	httpClient := &http.Client{Transport: transport, Timeout: c.HTTP.Timeout}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		if c.AuthMgr != nil && cred != nil {
			c.AuthMgr.MarkFailed(cred.ID)
		}
		return provider.ChatResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		if c.AuthMgr != nil && cred != nil {
			c.AuthMgr.MarkFailed(cred.ID)
		}
		return provider.ChatResponse{}, fmt.Errorf("client: status %d: %s", resp.StatusCode, string(data))
	}
	if c.AuthMgr != nil && cred != nil {
		c.AuthMgr.MarkSuccess(cred.ID)
	}
	return spec.ParseResponse(resp)
}
