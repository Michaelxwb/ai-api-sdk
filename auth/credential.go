package auth

import (
	"time"
)

// AuthType defines the credential authentication type.
type AuthType string

const (
	AuthTypeNone        AuthType = "none"
	AuthTypeAPIKey      AuthType = "api_key"
	AuthTypeBearerToken AuthType = "bearer_token"
	AuthTypeOAuth       AuthType = "oauth"
	AuthTypeBasic       AuthType = "basic"
	AuthTypeMTLS        AuthType = "mtls"
	AuthTypeJWTSign     AuthType = "jwt_sign"
)

// Credential represents a unified authentication credential.
type Credential struct {
	ID           string            `json:"id" yaml:"id"`
	Provider     string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	AuthType     AuthType          `json:"auth_type" yaml:"auth_type"`
	APIKey       string            `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	AccessToken  string            `json:"access_token,omitempty" yaml:"access_token,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty" yaml:"refresh_token,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Headers      map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	QueryParams  map[string]string `json:"query_params,omitempty" yaml:"query_params,omitempty"`
	Priority     int               `json:"priority,omitempty" yaml:"priority,omitempty"`
	Disabled     bool              `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// NewCredential creates a Credential with automatic AuthType inference.
// Non-empty apiKey implies AuthTypeAPIKey; otherwise AuthTypeNone.
func NewCredential(provider, apiKey string) *Credential {
	c := &Credential{Provider: provider, APIKey: apiKey}
	if apiKey != "" {
		c.AuthType = AuthTypeAPIKey
	} else {
		c.AuthType = AuthTypeNone
	}
	return c
}

func (c *Credential) IsExpired(now time.Time) bool {
	if c == nil || c.ExpiresAt == nil {
		return false
	}
	return now.After(*c.ExpiresAt)
}

func (c *Credential) Clone() *Credential {
	if c == nil {
		return nil
	}
	copyC := *c
	if len(c.Headers) > 0 {
		copyC.Headers = make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			copyC.Headers[k] = v
		}
	}
	if len(c.QueryParams) > 0 {
		copyC.QueryParams = make(map[string]string, len(c.QueryParams))
		for k, v := range c.QueryParams {
			copyC.QueryParams[k] = v
		}
	}
	if len(c.Metadata) > 0 {
		copyC.Metadata = make(map[string]any, len(c.Metadata))
		for k, v := range c.Metadata {
			copyC.Metadata[k] = v
		}
	}
	return &copyC
}
