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
	// If context already carries a deadline (set by Session.Chat timeout),
	// disable http.Client.Timeout to avoid a shorter hard cap overriding it.
	// This mirrors chatWithStream which always uses Timeout: 0.
	timeout := c.HTTP.Timeout
	if _, ok := ctx.Deadline(); ok {
		timeout = 0
	}
	httpClient := &http.Client{Transport: transport, Timeout: timeout}
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

// NewSessionFromMultiRound creates a session from a MultiRoundSpec using auto-inference.
// If inference status is "auto_confirmed" or "pending_confirm", creates and returns a session.
// If inference status is "failed", returns an error with fallback suggestion.
func (c *Client) NewSessionFromMultiRound(spec generic.MultiRoundSpec, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	inferred, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		return nil, nil, err
	}

	return c.sessionFromInferred(inferred, opts...)
}

// NewSessionFromHTTPMultiRound creates a session from raw HTTP request/response packets (2~5 rounds).
// SDK will parse raw packets into MultiRoundSpec internally, then run auto-inference.
func (c *Client) NewSessionFromHTTPMultiRound(spec generic.RawHTTPMultiRoundSpec, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	multi, err := generic.ParseHTTPMultiRoundSpec(spec)
	if err != nil {
		return nil, nil, err
	}
	return c.NewSessionFromMultiRound(multi, opts...)
}

// RawReasoning performs multi-round inference on raw HTTP packets and exports the result
// as a RawHTTPSpec suitable for passing directly to Quick().
//
// Usage:
//
//	spec, err := cli.RawReasoning(rawMultiRoundSpec)
//	qs, err := cli.Quick(client.ProviderConfig{
//	    Provider:    "generic",
//	    BaseURL:     spec.BaseURL,
//	    SessionMode: spec.Model,
//	    Request:     spec.Request,
//	    Response:    spec.Response,
//	    ChainFields: spec.ChainFields,
//	})
func (c *Client) RawReasoning(spec generic.RawHTTPMultiRoundSpec) (*generic.RawHTTPSpec, error) {
	_, inferred, err := c.NewSessionFromHTTPMultiRound(spec)
	if err != nil {
		return nil, fmt.Errorf("client: raw reasoning inference failed: %w", err)
	}
	exported, err := generic.ExportToHTTPSpec(inferred, spec)
	if err != nil {
		return nil, fmt.Errorf("client: raw reasoning export failed: %w", err)
	}
	return exported, nil
}

// NewSessionFromMultiRoundWithConfig creates a session with custom inference thresholds.
func (c *Client) NewSessionFromMultiRoundWithConfig(spec generic.MultiRoundSpec, cfg generic.InferenceConfig, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	inferred, err := generic.InferIntegrationByMultiRoundWithConfig(spec, cfg)
	if err != nil {
		return nil, nil, err
	}

	return c.sessionFromInferred(inferred, opts...)
}

// NewSessionFromTwoRound is a backward-compatible entry point for 2-round inference.
func (c *Client) NewSessionFromTwoRound(spec generic.TwoRoundSpec, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	inferred, err := generic.InferIntegrationByTwoRound(spec)
	if err != nil {
		return nil, nil, err
	}

	return c.sessionFromInferred(inferred, opts...)
}

// sessionFromInferred creates a session from an InferredIntegration result.
// Returns (session, inferred, nil) on auto_confirmed and pending_confirm.
// pending_confirm is preserved in inferred.Report.Status for caller-side warning display.
// Returns (nil, inferred, error) on failed.
func (c *Client) sessionFromInferred(inferred *generic.InferredIntegration, opts ...SessionOption) (*Session, *generic.InferredIntegration, error) {
	if inferred.Report == nil {
		return nil, inferred, fmt.Errorf("client: inference produced no report")
	}

	switch inferred.Report.Status {
	case "failed":
		return nil, inferred, fmt.Errorf("client: inference failed, use RawIntegrationSpec for manual configuration")
	case "auto_confirmed", "pending_confirm":
		// Proceed to create session; pending_confirm remains a warning-only status.
	default:
		return nil, inferred, fmt.Errorf("client: unknown inference status %q", inferred.Report.Status)
	}

	if inferred.Profile == nil {
		return nil, inferred, fmt.Errorf("client: inference produced no profile")
	}

	spec := generic.NewGenericSpec(*inferred.Profile)
	specName := "generic_inferred"
	base.Register(specName, spec)

	pc := &config.ProviderConfig{
		Name:    specName,
		Type:    specName,
		BaseURL: inferred.BaseURL,
		Headers: inferred.ExtraHeaders,
	}

	var cred *auth.Credential
	if inferred.Credential != nil {
		if c, ok := inferred.Credential.(*auth.Credential); ok {
			cred = c
		}
	}

	var convMode ConversationMode
	switch inferred.Profile.Conversation.Mode {
	case "remote_session":
		convMode = ConversationModeRemoteSession
	case "local_history":
		convMode = ConversationModeLocalHistory
	}

	sess := c.NewSessionWith(cred, pc, opts...)
	sess.conversationMode = convMode

	return sess, inferred, nil
}
