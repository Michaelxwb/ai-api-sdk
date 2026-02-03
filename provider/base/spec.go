package base

import (
	"context"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
)

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
