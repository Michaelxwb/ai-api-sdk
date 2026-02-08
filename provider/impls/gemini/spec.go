package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// GeminiSpec implements Gemini generateContent API.
type GeminiSpec struct{}

func init() {
	base.Register("gemini", &GeminiSpec{})
}

func (s *GeminiSpec) Name() string { return "gemini" }

func (s *GeminiSpec) DefaultBaseURL() string { return "https://generativelanguage.googleapis.com" }

func (s *GeminiSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeAPIKey, auth.AuthTypeOAuth, auth.AuthTypeBearerToken}
}

func (s *GeminiSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	// Gemini API 的 system prompt 是独立的 systemInstruction 字段，contents 只支持 user/model
	var systemParts []string
	contents := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if strings.EqualFold(m.Role, "system") {
			systemParts = append(systemParts, m.Content)
			continue
		}
		role := "user"
		if strings.EqualFold(m.Role, "assistant") {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	payload := map[string]any{
		"contents": contents,
	}
	if len(systemParts) > 0 {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": strings.Join(systemParts, "\n")}},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	// Use streamGenerateContent endpoint for streaming mode
	action := "generateContent"
	queryParams := ""
	if req.Stream {
		action = "streamGenerateContent"
		queryParams = "?alt=sse"
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:%s%s", strings.TrimRight(baseURL, "/"), req.Model, action, queryParams)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *GeminiSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("gemini: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, err
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
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("gemini: parse response failed: %w", err)
	}
	text := ""
	if len(parsed.Candidates) > 0 && len(parsed.Candidates[0].Content.Parts) > 0 {
		text = parsed.Candidates[0].Content.Parts[0].Text
	}
	return base.ChatResponse{Text: text, Raw: data}, nil
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
