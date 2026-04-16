package coze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// CozeSpec implements Coze chat_v3 API (streaming only).
// Endpoint: POST /v3/chat?conversation_id=xxx
type CozeSpec struct{}

func init() {
	base.Register("coze", &CozeSpec{})
}

func (s *CozeSpec) Name() string { return "coze" }

func (s *CozeSpec) DefaultBaseURL() string { return "https://api.coze.cn/v3" }

func (s *CozeSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeBearerToken, auth.AuthTypeAPIKey}
}

func (s *CozeSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	if err := base.ErrResponseFormatUnsupported("coze", req.ResponseFormat); err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = s.DefaultBaseURL()
	}

	// Extract query from the last user message.
	query := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.EqualFold(req.Messages[i].Role, "user") {
			query = req.Messages[i].Content
			break
		}
	}
	if query == "" && len(req.Messages) > 0 {
		query = req.Messages[len(req.Messages)-1].Content
	}

	payload := map[string]any{
		"bot_id":  req.Model,
		"user_id": "sdk-user",
		"additional_messages": []map[string]any{
			{
				"role":         "user",
				"type":         "question",
				"content":      query,
				"content_type": "text",
			},
		},
		"stream":            true,
		"auto_save_history": true,
	}

	// ExtraBody can override user_id, inject custom_variables, meta_data, etc.
	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("coze: marshal request body: %w", err)
	}

	path := "/chat"
	if strings.TrimSpace(opts.Path) != "" {
		path = opts.Path
	}
	url := strings.TrimRight(baseURL, "/") + path

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("coze: create request: %w", err)
	}

	// conversation_id in URL query param (not body).
	if req.SessionID != "" {
		q := httpReq.URL.Query()
		q.Set("conversation_id", req.SessionID)
		httpReq.URL.RawQuery = q.Encode()
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	return httpReq, nil
}

func (s *CozeSpec) ParseResponse(_ *http.Response) (base.ChatResponse, error) {
	return base.ChatResponse{}, fmt.Errorf("coze: non-streaming not supported, use ChatStream instead")
}

func (s *CozeSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && strings.TrimSpace(cred.APIKey) != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}
