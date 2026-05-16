package qianfan_app

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

// QianfanAppSpec implements Baidu Qianfan App conversation API.
// Endpoint: POST /v2/app/conversation/runs
type QianfanAppSpec struct {
	name           string
	defaultBaseURL string
}

func init() {
	base.Register("qianfan_app", NewQianfanAppSpec("qianfan_app"))
}

func NewQianfanAppSpec(name string) *QianfanAppSpec {
	return &QianfanAppSpec{
		name:           name,
		defaultBaseURL: "https://qianfan.baidubce.com/v2/app/conversation/runs",
	}
}

func (s *QianfanAppSpec) Name() string { return s.name }

func (s *QianfanAppSpec) DefaultBaseURL() string { return s.defaultBaseURL }

func (s *QianfanAppSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeBearerToken, auth.AuthTypeAPIKey, auth.AuthTypeNone}
}

func (s *QianfanAppSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	if err := base.ErrResponseFormatUnsupported("qianfan_app", req.ResponseFormat); err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = s.defaultBaseURL
	}
	if baseURL == "" {
		return nil, fmt.Errorf("qianfan_app: missing BaseURL")
	}

	// 提取最后一条 user 消息的文本和图片（支持多模态）
	query := ""
	var imageParts []base.ContentPart
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.EqualFold(req.Messages[i].Role, "user") {
			// 多模态路径：优先从 Parts 提取
			if len(req.Messages[i].Parts) > 0 {
				for _, part := range req.Messages[i].Parts {
					if part.Type == "text" && part.Text != "" {
						query = part.Text
					} else if part.Type == "image_url" {
						imageParts = append(imageParts, part)
					}
				}
			} else {
				// 纯文本路径：使用 Content 字段（向后兼容）
				query = req.Messages[i].Content
			}
			break
		}
	}
	if query == "" && len(req.Messages) > 0 {
		query = req.Messages[len(req.Messages)-1].Content
	}

	payload := map[string]any{
		"query":  query,
		"stream": req.Stream,
	}

	// app_id from Model field. Skip "test" placeholder used by QuickSession.Test().
	appID := strings.TrimSpace(req.Model)
	if appID != "" && !strings.EqualFold(appID, "test") {
		payload["app_id"] = appID
	}

	// 会话 ID 管理（支持图片上传）
	conversationID := req.SessionID

	// 如果有图片，需要上传并构造 file_ids
	if len(imageParts) > 0 {
		// 从 Credential 获取 API Key
		if opts.Credential == nil || opts.Credential.APIKey == "" {
			return nil, fmt.Errorf("qianfan_app: API key required for file upload")
		}
		apiKey := opts.Credential.APIKey

		// 如果没有会话 ID，先创建会话
		if conversationID == "" {
			var err error
			conversationID, err = createConversation(ctx, baseURL, apiKey, appID)
			if err != nil {
				return nil, err
			}
		}

		// 上传图片
		fileIDs, err := uploadImages(ctx, baseURL, apiKey, appID, conversationID, imageParts)
		if err != nil {
			return nil, err
		}

		// 添加 file_ids 到 payload
		if len(fileIDs) > 0 {
			payload["file_ids"] = fileIDs
		}

	}

	// Inject conversation_id for multi-turn (remote_session mode).
	if conversationID != "" {
		payload["conversation_id"] = conversationID
	}

	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("qianfan_app: marshal request body: %w", err)
	}

	url := strings.TrimRight(baseURL, "/")
	if strings.TrimSpace(opts.Path) != "" {
		path := opts.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		url = url + path
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *QianfanAppSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("qianfan_app: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("qianfan_app: read response body: %w", err)
	}

	var parsed struct {
		Answer         string `json:"answer"`
		ConversationID string `json:"conversation_id"`
		Usage          struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("qianfan_app: parse response: %w", err)
	}

	var usage *base.Usage
	if parsed.Usage.PromptTokens > 0 || parsed.Usage.CompletionTokens > 0 || parsed.Usage.TotalTokens > 0 {
		usage = &base.Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}

	return base.ChatResponse{
		Text:      strings.TrimSpace(parsed.Answer),
		SessionID: parsed.ConversationID,
		Usage:     usage,
		Raw:       data,
	}, nil
}

func (s *QianfanAppSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && cred.APIKey != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}
