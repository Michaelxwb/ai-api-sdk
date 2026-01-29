package client

import (
	"context"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
)

// AuthTransport injects auth into requests and retries on 401 for OAuth.
type AuthTransport struct {
	Base     http.RoundTripper
	Strategy auth.AuthStrategy
	Manager  *auth.Manager
	Cred     *auth.Credential
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.Strategy != nil {
		if err := t.Strategy.Apply(req); err != nil {
			return nil, err
		}
	}
	if t.Cred != nil {
		_ = auth.CustomHeaderStrategy{Headers: t.Cred.Headers, QueryParams: t.Cred.QueryParams}.Apply(req)
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusUnauthorized && t.Manager != nil && t.Cred != nil && t.Cred.AuthType == auth.AuthTypeOAuth {
		_ = resp.Body.Close()
		if err := t.Manager.RefreshOAuth(context.Background(), t.Cred); err != nil {
			return nil, err
		}
		// Retry once with updated token
		if t.Strategy != nil {
			_ = t.Strategy.Apply(req)
		}
		return base.RoundTrip(req)
	}
	return resp, nil
}
