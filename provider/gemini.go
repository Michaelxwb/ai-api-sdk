package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-sec-eval-sdk/auth"
)

// GeminiSpec implements Gemini generateContent API.
type GeminiSpec struct{}

func init() {
	Register("gemini", &GeminiSpec{})
}

func (s *GeminiSpec) Name() string { return "gemini" }

func (s *GeminiSpec) DefaultBaseURL() string { return "https://generativelanguage.googleapis.com" }

func (s *GeminiSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeAPIKey, auth.AuthTypeOAuth, auth.AuthTypeBearerToken}
}

func (s *GeminiSpec) BuildRequest(ctx context.Context, baseURL string, req ChatRequest) (*http.Request, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	contents := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if strings.EqualFold(m.Role, "assistant") {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role": role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	payload := map[string]any{
		"contents": contents,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", strings.TrimRight(baseURL, "/"), req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *GeminiSpec) ParseResponse(resp *http.Response) (ChatResponse, error) {
	if resp == nil {
		return ChatResponse{}, fmt.Errorf("gemini: response is nil")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, err
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	_ = json.Unmarshal(data, &parsed)
	text := ""
	if len(parsed.Candidates) > 0 && len(parsed.Candidates[0].Content.Parts) > 0 {
		text = parsed.Candidates[0].Content.Parts[0].Text
	}
	return ChatResponse{Text: text, Raw: data}, nil
}

func (s *GeminiSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && cred.APIKey != "" {
		return auth.ApiKeyHeaderStrategy{HeaderName: "x-goog-api-key", Key: cred.APIKey}, true
	}
	if cred.AuthType == auth.AuthTypeOAuth || cred.AuthType == auth.AuthTypeBearerToken {
		return auth.BearerTokenStrategy{Token: cred.AccessToken}, true
	}
	return nil, false
}
