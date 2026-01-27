package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-sec-eval-sdk/auth"
	"ai-sec-eval-sdk/config"
	"ai-sec-eval-sdk/provider"
)

// Client provides a unified API client.
type Client struct {
	HTTP    *http.Client
	AuthMgr *auth.Manager
	Config  *config.Config
}

// NewClient creates a new client with default settings.
func NewClient(cfg *config.Config, mgr *auth.Manager) *Client {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	return &Client{HTTP: httpClient, AuthMgr: mgr, Config: cfg}
}

// Chat sends a unified chat request to a provider.
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

	httpReq, err := spec.BuildRequest(ctx, baseURL, req)
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
	defer resp.Body.Close()
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
