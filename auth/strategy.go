package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// AuthStrategy injects authentication into an HTTP request.
type AuthStrategy interface {
	Apply(req *http.Request) error
}

// NoAuth does nothing.
type NoAuth struct{}

func (NoAuth) Apply(req *http.Request) error { return nil }

// BearerTokenStrategy sets Authorization: Bearer <token>.
type BearerTokenStrategy struct {
	Token string
}

func (s BearerTokenStrategy) Apply(req *http.Request) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(s.Token) == "" {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	return nil
}

// ApiKeyHeaderStrategy sets a specific header with the API key.
type ApiKeyHeaderStrategy struct {
	HeaderName string
	Key        string
	Prefix     string
}

func (s ApiKeyHeaderStrategy) Apply(req *http.Request) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(s.HeaderName) == "" || strings.TrimSpace(s.Key) == "" {
		return nil
	}
	value := s.Key
	if s.Prefix != "" {
		value = s.Prefix + s.Key
	}
	req.Header.Set(s.HeaderName, value)
	return nil
}

// CustomHeaderStrategy injects arbitrary headers and query params.
type CustomHeaderStrategy struct {
	Headers     map[string]string
	QueryParams map[string]string
}

func (s CustomHeaderStrategy) Apply(req *http.Request) error {
	if req == nil {
		return nil
	}
	for k, v := range s.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if len(s.QueryParams) > 0 && req.URL != nil {
		q := req.URL.Query()
		for k, v := range s.QueryParams {
			if strings.TrimSpace(k) == "" {
				continue
			}
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	return nil
}

// OAuthStrategy is a bearer token strategy but marked as OAuth by AuthType.
type OAuthStrategy struct {
	AccessToken string
}

func (s OAuthStrategy) Apply(req *http.Request) error {
	return BearerTokenStrategy{Token: s.AccessToken}.Apply(req)
}

// JWTSignStrategy builds a short-lived JWT and injects it as Authorization: Bearer <jwt>.
// Expected metadata keys: jwt_issuer, jwt_subject, jwt_audience, jwt_secret, jwt_exp_seconds.
type JWTSignStrategy struct {
	Issuer      string
	Subject     string
	Audience    string
	Secret      string
	ExpSeconds  int64
}

func (s JWTSignStrategy) Apply(req *http.Request) error {
	if req == nil {
		return nil
	}
	if s.Secret == "" {
		return errors.New("jwt_sign: missing secret")
	}
	if s.ExpSeconds <= 0 {
		s.ExpSeconds = 300
	}
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	payload := map[string]any{
		"iss": s.Issuer,
		"sub": s.Subject,
		"aud": s.Audience,
		"exp": time.Now().Add(time.Duration(s.ExpSeconds) * time.Second).Unix(),
	}
	jwt, err := signJWT(header, payload, []byte(s.Secret))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	return nil
}

func signJWT(header map[string]string, payload map[string]any, secret []byte) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	segmentHeader := enc.EncodeToString(headerJSON)
	segmentPayload := enc.EncodeToString(payloadJSON)
	unsigned := segmentHeader + "." + segmentPayload
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(unsigned))
	sig := enc.EncodeToString(h.Sum(nil))
	return unsigned + "." + sig, nil
}

// NewStrategyFromCredential creates a strategy based on the credential's auth type.
func NewStrategyFromCredential(cred *Credential) AuthStrategy {
	if cred == nil {
		return NoAuth{}
	}
	switch cred.AuthType {
	case AuthTypeBearerToken:
		token := cred.AccessToken
		if token == "" {
			token = cred.APIKey
		}
		return BearerTokenStrategy{Token: token}
	case AuthTypeAPIKey:
		header := "Authorization"
		prefix := ""
		if cred.Metadata != nil {
			if h, ok := cred.Metadata["header_name"].(string); ok && strings.TrimSpace(h) != "" {
				header = h
			}
			if p, ok := cred.Metadata["header_prefix"].(string); ok {
				prefix = p
			}
		}
		return ApiKeyHeaderStrategy{HeaderName: header, Key: cred.APIKey, Prefix: prefix}
	case AuthTypeOAuth:
		return OAuthStrategy{AccessToken: cred.AccessToken}
	case AuthTypeJWTSign:
		return JWTSignStrategy{
			Issuer:     stringFromMeta(cred.Metadata, "jwt_issuer"),
			Subject:    stringFromMeta(cred.Metadata, "jwt_subject"),
			Audience:   stringFromMeta(cred.Metadata, "jwt_audience"),
			Secret:     stringFromMeta(cred.Metadata, "jwt_secret"),
			ExpSeconds: int64FromMeta(cred.Metadata, "jwt_exp_seconds", 300),
		}
	default:
		if len(cred.Headers) > 0 || len(cred.QueryParams) > 0 {
			return CustomHeaderStrategy{Headers: cred.Headers, QueryParams: cred.QueryParams}
		}
		return NoAuth{}
	}
}

func stringFromMeta(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key].(string); ok {
		return v
	}
	return ""
}

func int64FromMeta(meta map[string]any, key string, def int64) int64 {
	if meta == nil {
		return def
	}
	switch v := meta[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return def
	}
}
