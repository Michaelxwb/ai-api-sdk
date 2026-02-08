package openai

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

// OpenAICompatSpec implements OpenAI-compatible chat API.
type OpenAICompatSpec struct {
	name           string
	defaultBaseURL string
	path           string
}

func init() {
	base.Register("openai_compat", NewOpenAICompatSpec("openai_compat", ""))
}

func NewOpenAICompatSpec(name, baseURL string) *OpenAICompatSpec {
	path := "/chat/completions"
	return &OpenAICompatSpec{name: name, defaultBaseURL: baseURL, path: path}
}

func (s *OpenAICompatSpec) Name() string { return s.name }

func (s *OpenAICompatSpec) DefaultBaseURL() string { return s.defaultBaseURL }

func (s *OpenAICompatSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeBearerToken, auth.AuthTypeAPIKey, auth.AuthTypeNone, auth.AuthTypeOAuth}
}

func (s *OpenAICompatSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.defaultBaseURL
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}
	payload["stream"] = req.Stream
	// merge extra body fields from config
	for k, v := range opts.ExtraBody {
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	path := s.path
	if opts.Path != "" {
		path = opts.Path
	}
	url := strings.TrimRight(baseURL, "/") + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *OpenAICompatSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("openai_compat: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("openai_compat: parse response failed: %w", err)
	}
	text := ""
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}
	return base.ChatResponse{Text: text, Raw: data}, nil
}

func (s *OpenAICompatSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && cred.APIKey != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}
