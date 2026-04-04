package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// ProviderConfig is a unified, application-facing configuration struct.
// It flattens auth.Credential, config.ProviderConfig, and SessionOption into one layer.
type ProviderConfig struct {
	// Provider is the SDK provider identifier: openai, claude, gemini, dify, generic, etc.
	Provider string

	// --- Credential (optional) ---

	// APIKey is the API secret. Non-empty implies AuthTypeAPIKey.
	APIKey string
	// AuthHeaders are credential-level custom request headers.
	AuthHeaders map[string]string
	// QueryParams are credential-level custom query parameters.
	QueryParams map[string]string

	// --- Provider config (optional, all have defaults) ---

	// BaseURL overrides the provider default base URL.
	BaseURL string
	// Path overrides the API endpoint path.
	Path string
	// Model is the model name injected into every ChatRequest.
	Model string
	// Headers are provider-level custom request headers.
	Headers map[string]string
	// ExtraBody holds extra fields merged into the request body.
	ExtraBody map[string]any

	// --- Session config (optional, auto-inferred) ---

	// SessionMode overrides automatic mode inference: "local_history" or "remote_session".
	SessionMode string
	// Stream controls streaming. nil uses the provider default.
	Stream *bool
	// TimeoutSec is the request timeout in seconds. Default 60.
	TimeoutSec int
	// Store is the session store. For local_history it defaults to MemoryStore; for remote_session it defaults to nil.
	Store session.SessionStore

	// --- Advanced session options (optional) ---

	// HistoryMaxMessages limits history message count (local_history mode). 0 = no limit.
	HistoryMaxMessages int
	// HistoryMaxTokens limits history token budget (local_history mode). 0 = no limit.
	HistoryMaxTokens int
	// OnError controls multi-turn error handling: "abort" (default) or "continue".
	OnError string
	// StartNewChat forces each call to be independent (no history, no session_id).
	StartNewChat bool

	// --- Generic provider: raw HTTP spec fields (optional) ---
	// When Provider == "generic" and Request != "", Quick() internally compiles
	// these into a GenericProfile via ParseHTTPSpec + ParseRawIntegration.

	// Request is the raw HTTP request template text.
	// "$$$" = user input, "$$$SESSION_ID$$$" = session ID, "$$$NAME$$$" = chain fields.
	Request string
	// Response is a representative HTTP response text for auto-detecting
	// streaming protocol, delta paths, and done conditions.
	Response string
	// ChainFields describes multi-round field chain propagation rules.
	ChainFields []generic.ChainField
}

// QuickSession is a simplified session created via Quick().
type QuickSession struct {
	session *Session
	cred    *auth.Credential
	pc      *config.ProviderConfig
	model   string
	stream  bool
}

// Quick creates a simplified session from a ProviderConfig.
// It handles credential construction, provider config assembly, and conversation mode inference internally.
// Returns an error if required parameters for the provider are missing.
func (c *Client) Quick(cfg ProviderConfig) (*QuickSession, error) {
	// 0. Validate provider config using existing ProviderSpec metadata.
	spec, ok := base.Get(cfg.Provider)
	if !ok {
		return nil, fmt.Errorf("client: provider %s not registered", cfg.Provider)
	}
	if spec.DefaultBaseURL() == "" && strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("client: %s requires BaseURL", cfg.Provider)
	}
	if ResolveConversationMode(cfg.Provider) == "" && cfg.SessionMode == "" {
		return nil, fmt.Errorf("client: %s requires explicit SessionMode (\"local_history\" or \"remote_session\")", cfg.Provider)
	}

	// 1. Build Credential
	cred := auth.NewCredential(cfg.Provider, cfg.APIKey)
	if len(cfg.AuthHeaders) > 0 {
		cred.Headers = cfg.AuthHeaders
	}
	if len(cfg.QueryParams) > 0 {
		cred.QueryParams = cfg.QueryParams
	}

	// 2. Build internal ProviderConfig
	var pc *config.ProviderConfig
	var compiledConvMode string // set by generic HTTPSpec compilation path

	if cfg.Provider == "generic" && strings.TrimSpace(cfg.Request) != "" {
		// Compile raw HTTP spec into GenericProfile + credentials.
		httpSpec := generic.RawHTTPSpec{
			Model:       cfg.SessionMode,
			BaseURL:     cfg.BaseURL,
			Request:     cfg.Request,
			Response:    cfg.Response,
			ChainFields: cfg.ChainFields,
		}
		rawSpec, err := generic.ParseHTTPSpec(httpSpec)
		if err != nil {
			return nil, fmt.Errorf("client: %w", err)
		}
		compiled, err := generic.ParseRawIntegration(rawSpec)
		if err != nil {
			return nil, fmt.Errorf("client: %w", err)
		}

		spec := generic.NewGenericSpec(compiled.Profile)
		base.Register("generic_quick", spec)

		pc = &config.ProviderConfig{
			Name:    "generic_quick",
			Type:    "generic_quick",
			BaseURL: compiled.BaseURL,
			Headers: compiled.ExtraHeaders,
		}

		// Merge credential: explicit APIKey takes priority over extracted header auth.
		if cfg.APIKey == "" && compiled.Credential != nil {
			cred = compiled.Credential
		}

		compiledConvMode = compiled.Profile.Conversation.Mode
	} else {
		pc = &config.ProviderConfig{
			Name:      cfg.Provider,
			Type:      cfg.Provider,
			BaseURL:   cfg.BaseURL,
			Path:      cfg.Path,
			Headers:   cfg.Headers,
			ExtraBody: cfg.ExtraBody,
		}
	}

	// 3. Build SessionOptions
	var opts []SessionOption

	// Conversation mode (compiled generic path may override)
	sessionMode := cfg.SessionMode
	if compiledConvMode != "" {
		sessionMode = compiledConvMode
	} else if sessionMode == "" {
		sessionMode = string(ResolveConversationMode(cfg.Provider))
	}
	switch sessionMode {
	case "local_history":
		opts = append(opts, WithConversationMode(ConversationModeLocalHistory))
		store := cfg.Store
		if store == nil {
			store = session.NewMemoryStore()
		}
		opts = append(opts, WithStore(store), WithAutoID())
	case "remote_session":
		opts = append(opts, WithConversationMode(ConversationModeRemoteSession))
		if cfg.Store != nil {
			opts = append(opts, WithStore(cfg.Store))
		}
	default:
		// Unknown mode (fastgpt/generic without explicit mode) -- no ConversationMode, legacy path.
		if cfg.Store != nil {
			opts = append(opts, WithStore(cfg.Store), WithAutoID())
		}
	}

	// Timeout
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	opts = append(opts, WithTimeout(time.Duration(timeout)*time.Second))

	// HistoryWindow applies when Store is configured (both modes need truncation)
	if cfg.HistoryMaxMessages > 0 || cfg.HistoryMaxTokens > 0 {
		opts = append(opts, WithHistoryWindow(HistoryWindow{
			MaxMessages: cfg.HistoryMaxMessages,
			MaxTokens:   cfg.HistoryMaxTokens,
		}))
	}

	// OnError
	switch cfg.OnError {
	case "continue":
		opts = append(opts, WithOnError(OnErrorContinue))
	case "abort", "":
		opts = append(opts, WithOnError(OnErrorAbort))
	}

	// StartNewChat
	if cfg.StartNewChat {
		opts = append(opts, WithStartNewChat(true))
	}

	// Stream inference
	useStream := ResolveDefaultStream(cfg.Provider)
	if cfg.Stream != nil {
		useStream = *cfg.Stream
	}

	sess := c.NewSessionWith(cred, pc, opts...)

	return &QuickSession{
		session: sess,
		cred:    cred,
		pc:      pc,
		model:   cfg.Model,
		stream:  useStream,
	}, nil
}

// Send sends a chat request. It automatically chooses streaming or non-streaming based on the Stream setting.
// The returned channel always closes when the response is complete.
// For non-streaming, the channel contains a single element.
func (qs *QuickSession) Send(ctx context.Context, messages []base.Message) (<-chan streaming.StreamChunk, error) {
	req := base.ChatRequest{
		Model:    qs.model,
		Messages: messages,
	}

	if qs.stream {
		return qs.session.ChatStream(ctx, req)
	}

	// Non-streaming: wrap response in a single-element channel.
	resp, err := qs.session.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan streaming.StreamChunk, 1)
	ch <- streaming.StreamChunk{
		Text:  resp.Text,
		Done:  true,
		Usage: resp.Usage,
	}
	close(ch)
	return ch, nil
}

// SendText is a convenience wrapper that sends a single user message string.
func (qs *QuickSession) SendText(ctx context.Context, text string) (<-chan streaming.StreamChunk, error) {
	return qs.Send(ctx, []base.Message{{Role: "user", Content: text}})
}

// Test performs a connectivity test against the configured provider.
// It delegates to Client.TestWith using the saved credential and provider config.
func (qs *QuickSession) Test(ctx context.Context) (TestResult, error) {
	opt := &TestOptions{
		Model: qs.model,
	}
	// If no model is set, use a minimal placeholder so normalizeTestOptions doesn't fail.
	if opt.Model == "" {
		opt.Model = "test"
	}
	return qs.session.client.TestWith(ctx, qs.cred, qs.pc, opt)
}

// Session returns the underlying Session for advanced use cases.
func (qs *QuickSession) Session() *Session {
	return qs.session
}
