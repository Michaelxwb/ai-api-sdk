package ollama

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

// OllamaSpec implements Ollama /api/chat.
type OllamaSpec struct{}

func init() {
	base.Register("ollama", &OllamaSpec{})
}

func (s *OllamaSpec) Name() string { return "ollama" }

func (s *OllamaSpec) DefaultBaseURL() string { return "http://127.0.0.1:11434" }

func (s *OllamaSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeNone, auth.AuthTypeBearerToken}
}

func (s *OllamaSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	if req.ResponseFormat != nil {
		switch req.ResponseFormat.Type {
		case "json_object":
			payload["format"] = "json"
		case "json_schema":
			if req.ResponseFormat.JSONSchema != nil {
				payload["format"] = req.ResponseFormat.JSONSchema.Schema
			} else {
				payload["format"] = "json"
			}
		}
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

func (s *OllamaSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("ollama: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, err
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("ollama: parse response failed: %w", err)
	}
	return base.ChatResponse{Text: parsed.Message.Content, Raw: data}, nil
}

func (s *OllamaSpec) AuthStrategyOverride(_ *auth.Credential) (auth.AuthStrategy, bool) {
	return nil, false
}
