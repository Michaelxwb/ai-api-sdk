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

// convertMessagesToOpenAI 将 SDK 统一的 Messages 转换为 OpenAI 原生格式。
// 支持多模态内容：当 Message.Parts 非空时，转换为 content 数组格式；
// 否则使用 Message.Content 保持向后兼容。
//
// fileIDs 参数用于 Moonshot 文件上传模式：
// - key: "msgIndex_partIndex"（如 "0_1" 表示第0条消息的第1个part）
// - value: 上传后的 file_id
// - 如果 fileIDs 中有对应的 file_id，使用 "ms://file_id" 格式（Moonshot）
// - 否则使用 "data:image/xxx;base64,..." 格式（OpenAI/FastGPT/Ollama等）
func convertMessagesToOpenAI(messages []base.Message, fileIDs map[string]string) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for msgIdx, msg := range messages {
		m := map[string]any{"role": msg.Role}

		// 多模态路径：len(Parts) > 0
		if len(msg.Parts) > 0 {
			content := make([]map[string]any, 0, len(msg.Parts))
			for partIdx, part := range msg.Parts {
				switch part.Type {
				case "text":
					content = append(content, map[string]any{
						"type": "text",
						"text": part.Text,
					})
				case "image_url":
					// 检查是否有上传的 file_id（Moonshot 模式）
					fileIDKey := fmt.Sprintf("%d_%d", msgIdx, partIdx)
					if fileID, ok := fileIDs[fileIDKey]; ok {
						// Moonshot 文件上传模式：使用 ms://file_id
						content = append(content, map[string]any{
							"type": "image_url",
							"image_url": map[string]any{
								"url": "ms://" + fileID,
							},
						})
					} else {
						// 其他供应商：使用 base64 内联
						dataURI := fmt.Sprintf("data:%s;base64,%s", part.MIMEType, part.Data)
						content = append(content, map[string]any{
							"type": "image_url",
							"image_url": map[string]any{
								"url": dataURI,
							},
						})
					}
				}
			}
			m["content"] = content
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

func (s *OpenAICompatSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.defaultBaseURL
	}

	// ========== DeepSeek 图片检测（不支持视觉模型） ==========
	// DeepSeek 当前没有视觉模型，检测到图片输入时返回明确错误。
	// 相关任务：TASK-006 C组供应商错误处理
	if s.name == "deepseek" {
		for _, msg := range req.Messages {
			if len(msg.Parts) > 0 {
				for _, part := range msg.Parts {
					if part.Type == "image_url" {
						return nil, fmt.Errorf("deepseek: vision model not available, only text models supported")
					}
				}
			}
		}
	}
	// ========== DeepSeek 图片检测结束 ==========

	// ========== Moonshot 文件上传逻辑（方案C：临时集成） ==========
	// 注意：此处将 Moonshot 的文件上传逻辑集成到 OpenAI 兼容模式中。
	// 原因：Moonshot 与 OpenAI 在纯文本对话时的请求/响应格式完全相同，
	//       只是在处理图片时方式不同（OpenAI 用 base64 内联，Moonshot 用文件上传）。
	//       集成到同一实现中可以最大化复用代码，避免重复实现相同的对话逻辑。
	// 未来优化：如果 Moonshot API 与 OpenAI 在其他方面分叉，或需要更多定制化功能，
	//           可拆分为独立的 provider/impls/moonshot/ 实现。
	// 相关任务：TASK-005-4 Moonshot 供应商文件上传
	fileIDs := make(map[string]string) // key: "msgIndex_partIndex", value: file_id

	if s.name == "moonshot" {
		// 检测是否有图片需要上传
		var imageParts []base.ContentPart
		var imageLocations []struct{ msgIdx, partIdx int } // 记录图片位置

		for msgIdx, msg := range req.Messages {
			if len(msg.Parts) > 0 {
				for partIdx, part := range msg.Parts {
					if part.Type == "image_url" {
						imageParts = append(imageParts, part)
						imageLocations = append(imageLocations, struct{ msgIdx, partIdx int }{msgIdx, partIdx})
					}
				}
			}
		}

			// 如果有图片，上传并获取 file_ids
			if len(imageParts) > 0 {
				// 从 Credential 获取 API Key
				if opts.Credential == nil || opts.Credential.APIKey == "" {
					return nil, fmt.Errorf("moonshot: API key required for file upload")
				}
				apiKey := opts.Credential.APIKey

			// 上传图片
			uploadedFileIDs, err := uploadImagesForMoonshot(ctx, baseURL, apiKey, imageParts)
			if err != nil {
				return nil, err
			}

			// 构建 fileIDs 映射
			for i, loc := range imageLocations {
				key := fmt.Sprintf("%d_%d", loc.msgIdx, loc.partIdx)
				fileIDs[key] = uploadedFileIDs[i]
			}
		}
	}
	// ========== Moonshot 文件上传逻辑结束 ==========

	payload := map[string]any{
		"model":    req.Model,
		"messages": convertMessagesToOpenAI(req.Messages, fileIDs), // 传递 fileIDs
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
	if req.ResponseFormat != nil {
		rf := map[string]any{"type": req.ResponseFormat.Type}
		if req.ResponseFormat.JSONSchema != nil {
			js := map[string]any{
				"name":   req.ResponseFormat.JSONSchema.Name,
				"schema": req.ResponseFormat.JSONSchema.Schema,
			}
			if req.ResponseFormat.JSONSchema.Description != "" {
				js["description"] = req.ResponseFormat.JSONSchema.Description
			}
			if req.ResponseFormat.JSONSchema.Strict != nil {
				js["strict"] = *req.ResponseFormat.JSONSchema.Strict
			}
			rf["json_schema"] = js
		}
		payload["response_format"] = rf
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
