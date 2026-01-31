package provider

import (
	"context"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
)

// Message is a simplified chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatRequest is a unified request structure.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float32  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse is a unified response structure.
type ChatResponse struct {
	Text string
	Raw  []byte
}

// BuildOptions carries per-call overrides from ProviderConfig.
type BuildOptions struct {
	BaseURL   string            // API base URL
	Path      string            // override default endpoint path (e.g. "/chat/completions")
	ExtraBody map[string]any    // extra fields merged into request body
	Headers   map[string]string // extra headers (injected by client, not by spec)
}

// ProviderSpec defines provider behavior.
type ProviderSpec interface {
	Name() string
	DefaultBaseURL() string
	SupportedAuthTypes() []auth.AuthType
	BuildRequest(ctx context.Context, opts BuildOptions, req ChatRequest) (*http.Request, error)
	ParseResponse(resp *http.Response) (ChatResponse, error)
	AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool)
}
