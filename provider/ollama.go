package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-api-sdk/auth"
)

// OllamaSpec implements Ollama /api/chat.
type OllamaSpec struct{}

func init() {
	Register("ollama", &OllamaSpec{})
}

func (s *OllamaSpec) Name() string { return "ollama" }

func (s *OllamaSpec) DefaultBaseURL() string { return "http://127.0.0.1:11434" }

func (s *OllamaSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeNone, auth.AuthTypeBearerToken}
}

func (s *OllamaSpec) BuildRequest(ctx context.Context, opts BuildOptions, req ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(baseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *OllamaSpec) ParseResponse(resp *http.Response) (ChatResponse, error) {
	if resp == nil {
		return ChatResponse{}, fmt.Errorf("ollama: response is nil")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, err
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	_ = json.Unmarshal(data, &parsed)
	return ChatResponse{Text: parsed.Message.Content, Raw: data}, nil
}

func (s *OllamaSpec) AuthStrategyOverride(_ *auth.Credential) (auth.AuthStrategy, bool) {
	return nil, false
}
