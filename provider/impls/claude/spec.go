package claude

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

// ClaudeSpec implements Anthropic Claude messages API.
type ClaudeSpec struct{}

func init() {
	base.Register("claude", &ClaudeSpec{})
}

func (s *ClaudeSpec) Name() string { return "claude" }

func (s *ClaudeSpec) DefaultBaseURL() string { return "https://api.anthropic.com" }

func (s *ClaudeSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeAPIKey, auth.AuthTypeOAuth, auth.AuthTypeBearerToken}
}

func (s *ClaudeSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	maxTokens := 1024
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	// Claude API 的 system prompt 是独立顶级字段，不能放在 messages 里
	var systemParts []string
	var messages []base.Message
	for _, m := range req.Messages {
		if strings.EqualFold(m.Role, "system") {
			systemParts = append(systemParts, m.Content)
		} else {
			messages = append(messages, m)
		}
	}

	payload := map[string]any{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     req.Stream,
	}
	if len(systemParts) > 0 {
		payload["system"] = strings.Join(systemParts, "\n")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(baseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return httpReq, nil
}

func (s *ClaudeSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("claude: response is nil")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return base.ChatResponse{}, err
	}
	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(data, &parsed)
	text := ""
	if len(parsed.Content) > 0 {
		text = parsed.Content[0].Text
	}
	return base.ChatResponse{Text: text, Raw: data}, nil
}

func (s *ClaudeSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && cred.APIKey != "" {
		return auth.ApiKeyHeaderStrategy{HeaderName: "x-api-key", Key: cred.APIKey}, true
	}
	if cred.AuthType == auth.AuthTypeOAuth || cred.AuthType == auth.AuthTypeBearerToken {
		return auth.BearerTokenStrategy{Token: cred.AccessToken}, true
	}
	return nil, false
}
