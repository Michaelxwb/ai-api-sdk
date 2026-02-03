package dify

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

// DifySpec 实现 Dify Chat Messages API。
type DifySpec struct{}

func init() {
	base.Register("dify", &DifySpec{})
}

func (s *DifySpec) Name() string { return "dify" }

func (s *DifySpec) DefaultBaseURL() string { return "https://api.dify.ai/v1" }

func (s *DifySpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeBearerToken, auth.AuthTypeAPIKey}
}

func (s *DifySpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = s.DefaultBaseURL()
	}

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
		"inputs":        map[string]any{},
		"query":         query,
		"response_mode": "blocking",
		"user":          "sdk-user",
	}

	if req.Stream {
		payload["response_mode"] = "streaming"
	}
	// Dify 的 conversation_id 由服务端管理：
	// 仅当客户端明确拿到会话 ID（从响应中获取）时才传递。
	// 首次对话不传入，让 Dify 生成新的会话 ID。
	if req.SessionID != "" {
		payload["conversation_id"] = req.SessionID
	}

	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("dify: serialization request body failed: %w", err)
	}

	path := "/chat-messages"
	if strings.TrimSpace(opts.Path) != "" {
		path = opts.Path
	}
	url := strings.TrimRight(baseURL, "/") + path

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dify: request creation failed: %w", err)
	}
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *DifySpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("dify: response is nil")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("dify: read response failed: %w", err)
	}

	var result struct {
		Event          string `json:"event"`
		MessageID      string `json:"message_id"`
		ConversationID string `json:"conversation_id"`
		Answer         string `json:"answer"`
		Metadata       struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return base.ChatResponse{}, fmt.Errorf("dify: response parsing failed: %w", err)
	}

	chatResp := base.ChatResponse{
		Text:      result.Answer,
		SessionID: result.ConversationID,
		Raw:       data,
	}

	if result.Metadata.Usage.TotalTokens > 0 || result.Metadata.Usage.PromptTokens > 0 || result.Metadata.Usage.CompletionTokens > 0 {
		chatResp.Usage = &base.Usage{
			PromptTokens:     result.Metadata.Usage.PromptTokens,
			CompletionTokens: result.Metadata.Usage.CompletionTokens,
			TotalTokens:      result.Metadata.Usage.TotalTokens,
		}
	}

	return chatResp, nil
}

func (s *DifySpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && strings.TrimSpace(cred.APIKey) != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}
