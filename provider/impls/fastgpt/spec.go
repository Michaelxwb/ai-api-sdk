package fastgpt

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
	"github.com/google/uuid"
)

// FastGPTSpec implements FastGPT chat/completions API (方案 C: 专用 Provider).
//
// 会话模式说明：
//   - local_history:  Session 层设置 req.SessionID=""，Provider 不传 chatId，历史由 Session 层拼接。
//   - remote_session: Session 层注入 req.SessionID=s.id，Provider 将其映射为 chatId。
//   - 模式判断由 Session 层 (client/session.go) 完成，与 Dify/OpenAI/Generic 等 Provider 一致。
//
// model/temperature 说明：
//   - FastGPT 不使用 model/temperature 字段（无效），因此 BuildRequest 不映射这些字段到请求体。
//   - 如需透传，可通过 ExtraBody 传递。
type FastGPTSpec struct{}

func init() {
	base.Register("fastgpt", &FastGPTSpec{})
}

func (s *FastGPTSpec) Name() string { return "fastgpt" }

func (s *FastGPTSpec) DefaultBaseURL() string { return "" }

func (s *FastGPTSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeAPIKey, auth.AuthTypeBearerToken, auth.AuthTypeOAuth, auth.AuthTypeNone}
}

func (s *FastGPTSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = s.DefaultBaseURL()
	}
	path := opts.Path
	if strings.TrimSpace(path) == "" {
		path = "/api/v1/chat/completions"
	}

	payload := map[string]any{
		"messages": req.Messages,
		"stream":   req.Stream,
	}

	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	// Remove SDK-internal keys not recognized by FastGPT API.
	delete(payload, "mode")

	// chatId: 由 Session 层根据 ConversationMode 决定是否填充 req.SessionID。
	// local_history 模式下 SessionID 为空，不传 chatId；remote_session 模式下注入 chatId。
	if strings.TrimSpace(req.SessionID) != "" {
		payload["chatId"] = req.SessionID
	}

	if shouldGenerateResponseChatItemID(payload) {
		payload["responseChatItemId"] = uuid.New().String()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("fastgpt: serialize request body failed: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fastgpt: request creation failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

// ParseResponse 解析 FastGPT 非流式响应。
// id/created/model 等字段可通过 ChatResponse.Raw（完整响应体）获取，此处不做显式提取（设计标注为"可选"）。
func (s *FastGPTSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("fastgpt: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("fastgpt: read response failed: %w", err)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage        *fastGPTUsage    `json:"usage"`
		ResponseData []map[string]any `json:"responseData"`
		// Error fields for non-standard error responses.
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("fastgpt: parse response failed: %w", err)
	}

	// When choices is empty, check for error fields in the response body.
	if len(parsed.Choices) == 0 {
		if errMsg := firstNonEmpty(parsed.Error, parsed.Message); errMsg != "" {
			return base.ChatResponse{}, fmt.Errorf("fastgpt: %s", errMsg)
		}
	}

	text := ""
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}

	respPayload := base.ChatResponse{Text: text, Raw: data}
	if usage := usageFromFastGPT(parsed.Usage, parsed.ResponseData); usage != nil {
		respPayload.Usage = usage
	}
	return respPayload, nil
}

func (s *FastGPTSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && strings.TrimSpace(cred.APIKey) != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	if cred.AuthType == auth.AuthTypeBearerToken || cred.AuthType == auth.AuthTypeOAuth {
		return auth.BearerTokenStrategy{Token: cred.AccessToken}, true
	}
	return nil, false
}

type fastGPTUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func usageFromFastGPT(usage *fastGPTUsage, responseData []map[string]any) *base.Usage {
	if usage != nil && (usage.TotalTokens > 0 || usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		return &base.Usage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	}

	var promptTokens int
	var completionTokens int
	var totalTokens int

	for _, item := range responseData {
		if item == nil {
			continue
		}
		if tokens, ok := numberToInt(item["tokens"]); ok {
			totalTokens += tokens
		}
		if usageField, ok := item["usage"].(map[string]any); ok {
			if v, ok := numberToInt(usageField["prompt_tokens"]); ok {
				promptTokens += v
			}
			if v, ok := numberToInt(usageField["completion_tokens"]); ok {
				completionTokens += v
			}
			if v, ok := numberToInt(usageField["total_tokens"]); ok {
				totalTokens += v
			}
		}
	}

	if totalTokens == 0 && (promptTokens > 0 || completionTokens > 0) {
		totalTokens = promptTokens + completionTokens
	}

	if totalTokens > 0 || promptTokens > 0 || completionTokens > 0 {
		return &base.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}
	}
	return nil
}

func numberToInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case json.Number:
		i64, err := val.Int64()
		if err != nil {
			return 0, false
		}
		return int(i64), true
	case string:
		if strings.TrimSpace(val) == "" {
			return 0, false
		}
		i64, err := json.Number(val).Int64()
		if err != nil {
			return 0, false
		}
		return int(i64), true
	default:
		return 0, false
	}
}

func shouldGenerateResponseChatItemID(payload map[string]any) bool {
	value, ok := payload["responseChatItemId"]
	if !ok || value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

var _ base.ProviderSpec = (*FastGPTSpec)(nil)
