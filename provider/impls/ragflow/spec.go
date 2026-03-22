package ragflow

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

// RAGFlowSpec 实现 RAGFlow Chat Completions API。
//
// RAGFlow 使用 question 字段（非 messages）发送用户问题，
// 响应嵌套在 code/data 结构中，流式终止帧为 data:true。
// 这些结构性差异使其不适合 Generic Adapter，需专用 Provider。
//
// chat_id 说明：
//   - 必须通过 ExtraBody["chat_id"] 传入
//   - 用于 URL 路径拼接：/api/v1/chats/{chat_id}/completions
//
// session_id 说明：
//   - 首轮对话不传，由 RAGFlow 自动生成
//   - 续轮通过 ChatRequest.SessionID 注入
type RAGFlowSpec struct{}

func init() {
	base.Register("ragflow", &RAGFlowSpec{})
}

func (s *RAGFlowSpec) Name() string { return "ragflow" }

func (s *RAGFlowSpec) DefaultBaseURL() string { return "" }

func (s *RAGFlowSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeAPIKey, auth.AuthTypeBearerToken}
}

func (s *RAGFlowSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	// chat_id 从 ExtraBody 提取，用于 URL 路径拼接。
	chatID := ""
	if v, ok := opts.ExtraBody["chat_id"]; ok {
		if s, ok := v.(string); ok {
			chatID = strings.TrimSpace(s)
		}
	}
	if chatID == "" {
		return nil, fmt.Errorf("ragflow: chat_id is required in ExtraBody")
	}

	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = s.DefaultBaseURL()
	}

	// 从 Messages 提取最后一条 user 消息作为 question。
	question := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.EqualFold(req.Messages[i].Role, "user") {
			question = req.Messages[i].Content
			break
		}
	}
	if question == "" && len(req.Messages) > 0 {
		question = req.Messages[len(req.Messages)-1].Content
	}

	payload := map[string]any{
		"question": question,
		"stream":   req.Stream,
	}

	if req.SessionID != "" {
		payload["session_id"] = req.SessionID
	}

	// 合并 ExtraBody 中的额外字段（chat_id 已提取，不传入请求体）。
	for k, v := range opts.ExtraBody {
		if k == "chat_id" {
			continue
		}
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ragflow: serialize request body failed: %w", err)
	}

	path := fmt.Sprintf("/api/v1/chats/%s/completions", chatID)
	if strings.TrimSpace(opts.Path) != "" {
		path = opts.Path
	}
	url := strings.TrimRight(baseURL, "/") + path

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ragflow: request creation failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	return httpReq, nil
}

func (s *RAGFlowSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("ragflow: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("ragflow: read response failed: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Answer    string `json:"answer"`
			SessionID string `json:"session_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return base.ChatResponse{}, fmt.Errorf("ragflow: response parsing failed: %w", err)
	}
	if result.Code != 0 {
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		return base.ChatResponse{}, fmt.Errorf("ragflow: server error: %s", msg)
	}

	return base.ChatResponse{
		Text:      result.Data.Answer,
		SessionID: result.Data.SessionID,
		Raw:       data,
	}, nil
}

func (s *RAGFlowSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && strings.TrimSpace(cred.APIKey) != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}

var _ base.ProviderSpec = (*RAGFlowSpec)(nil)
