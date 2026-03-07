package client

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
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

// New creates a lightweight client for platform integration (Session API).
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// NewClient creates a client with local config and auth manager (Session mode).
func NewClient(cfg *config.Config, mgr *auth.Manager) *Client {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	return &Client{HTTP: httpClient, AuthMgr: mgr, Config: cfg}
}

// NewSession 创建新会话
func (c *Client) NewSession(provider string, opts ...SessionOption) *Session {
	sess := &Session{
		client:   c,
		provider: provider,
		mode:     HistoryAuto, // 默认自动加载历史
		store:    c.SessionStore,
	}

	for _, opt := range opts {
		opt(sess)
	}

	return sess
}

// NewSessionWith creates a session using caller-provided credential and provider config.
// This is designed for platform integration where credentials are managed externally.
func (c *Client) NewSessionWith(cred *auth.Credential, pc *config.ProviderConfig, opts ...SessionOption) *Session {
	provider := ""
	if pc != nil {
		if pc.Name != "" {
			provider = pc.Name
		} else if pc.Type != "" {
			provider = pc.Type
		}
	}
	if provider == "" && cred != nil {
		provider = cred.Provider
	}

	sess := &Session{
		client:   c,
		provider: provider,
		cred:     cred,
		pc:       pc,
		mode:     HistoryAuto,
		store:    c.SessionStore,
	}

	for _, opt := range opts {
		opt(sess)
	}

	return sess
}

// chatWith 内部实现方法（仅供Session.Chat使用，业务层请使用Session API）
func (c *Client) chatWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (base.ChatResponse, error) {
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
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		bodyStr := truncateAPIErrorBody(string(data))
		return base.ChatResponse{}, &APIError{StatusCode: resp.StatusCode, Body: bodyStr, Op: "chat"}
	}
	return prep.spec.ParseResponse(resp)
}

// NewSessionFromHTTPSpec 从业务层五字段原始 HTTP 报文格式创建会话。
// 相较于 NewSessionFromRaw，调用方无需了解 SDK 内部的 RawIntegrationSpec 结构，
// 只需传入从浏览器/抓包工具获取的原始请求和响应文本即可。
func (c *Client) NewSessionFromHTTPSpec(spec generic.RawHTTPSpec, opts ...SessionOption) (*Session, error) {
	raw, err := generic.ParseHTTPSpec(spec)
	if err != nil {
		return nil, err
	}
	return c.NewSessionFromRaw(raw, opts...)
}

// NewSessionFromRaw creates a session from a raw integration spec for non-standard API providers.
// It compiles the spec into a GenericProfile, extracts credentials, and configures conversation mode.
func (c *Client) NewSessionFromRaw(raw generic.RawIntegrationSpec, opts ...SessionOption) (*Session, error) {
	compiled, err := generic.ParseRawIntegration(raw)
	if err != nil {
		return nil, err
	}

	// Register a per-instance GenericSpec with the compiled profile
	spec := generic.NewGenericSpec(compiled.Profile)

	// Create a unique spec name to avoid collisions
	specName := "generic"
	if raw.Name != "" {
		specName = "generic_" + raw.Name
	}
	base.Register(specName, spec)

	pc := &config.ProviderConfig{
		Name:    specName,
		Type:    specName,
		BaseURL: compiled.BaseURL,
		Headers: compiled.ExtraHeaders,
	}

	// Determine conversation mode
	var convMode ConversationMode
	switch compiled.Profile.Conversation.Mode {
	case "remote_session":
		convMode = ConversationModeRemoteSession
	case "local_history":
		convMode = ConversationModeLocalHistory
	}

	sess := c.NewSessionWith(compiled.Credential, pc, opts...)
	sess.conversationMode = convMode

	return sess, nil
}
