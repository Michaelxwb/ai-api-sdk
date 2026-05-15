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

// convertMessagesToOllamaFormat 将 SDK 统一的 Messages 转换为 Ollama /api/chat 原生格式。
// 注意：/api/chat 不是 OpenAI 兼容接口——content 必须是字符串，图片走单独的 images 数组（纯 base64）。
func convertMessagesToOllamaFormat(messages []base.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{"role": msg.Role}

		// 多模态路径：len(Parts) > 0
		if len(msg.Parts) > 0 {
			var textBuf strings.Builder
			images := make([]string, 0, len(msg.Parts))
			for _, part := range msg.Parts {
				switch part.Type {
				case "text":
					textBuf.WriteString(part.Text)
				case "image_url":
					// Ollama /api/chat 要求纯 base64 字符串，不带 data URI 前缀
					images = append(images, part.Data)
				}
			}
			m["content"] = textBuf.String()
			if len(images) > 0 {
				m["images"] = images
			}
		} else {
			// 纯文本路径（向后兼容）：使用 Content 字段
			m["content"] = msg.Content
		}

		if msg.Name != "" {
			m["name"] = msg.Name
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		result = append(result, m)
	}
	return result
}

func (s *OllamaSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": convertMessagesToOllamaFormat(req.Messages),
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
