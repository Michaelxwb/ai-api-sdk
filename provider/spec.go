package provider

import (
	"context"
	"net/http"

	"ai-sec-eval-sdk/auth"
)

// Message is a simplified chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// ProviderSpec defines provider behavior.
type ProviderSpec interface {
	Name() string
	DefaultBaseURL() string
	SupportedAuthTypes() []auth.AuthType
	BuildRequest(ctx context.Context, baseURL string, req ChatRequest) (*http.Request, error)
	ParseResponse(resp *http.Response) (ChatResponse, error)
	AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool)
}
