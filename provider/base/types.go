package base

import "fmt"

// ErrResponseFormatUnsupported 在 Provider 不支持 ResponseFormat 时由 BuildRequest 返回。
// 调用方可用 errors.Is 判断。
func ErrResponseFormatUnsupported(provider string, rf *ResponseFormat) error {
	if rf == nil || rf.Type == "" || rf.Type == "text" {
		return nil
	}
	return fmt.Errorf("%s: response_format type %q is not supported by this provider", provider, rf.Type)
}

// ContentPart 多模态内容块。
// 用于支持文本、图片、视频、音频等多种内容类型的混排。
type ContentPart struct {
	Type     string `json:"type"`                // "text" | "image_url" | "video_url" | "audio_url"
	Text     string `json:"text,omitempty"`      // Type="text" 时使用
	Data     string `json:"data,omitempty"`      // Type="image_url" 时：base64 编码数据；Type="video_url"/"audio_url" 时：文件路径
	MIMEType string `json:"mime_type,omitempty"` // image/png, image/jpeg, image/webp, image/gif, video/mp4, audio/mpeg 等
}

// Message 是简化的对话消息。
// 支持两种模式：
//   - 纯文本模式（向后兼容）：使用 Content 字段，Parts 为空或不设置
//   - 多模态模式：使用 Parts 字段，支持文本+图片/视频/音频混排
//
// 语义规则：len(Parts)==0 使用 Content，len(Parts)>0 使用 Parts（忽略 Content）
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`         // 纯文本兼容路径
	Parts      []ContentPart `json:"parts,omitempty"` // 多模态路径（可选）
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// ResponseFormat 控制模型输出格式。
type ResponseFormat struct {
	Type       string           `json:"type"`                  // "text" | "json_object" | "json_schema"
	JSONSchema *JSONSchemaParam `json:"json_schema,omitempty"` // type="json_schema" 时必填
}

// JSONSchemaParam 描述 JSON schema 约束。
type JSONSchemaParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
	Strict      *bool          `json:"strict,omitempty"`
}

// ChatRequest 是统一的请求结构。
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    *float32        `json:"temperature,omitempty"`
	MaxTokens      *int            `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream,omitempty"` // 是否启用流式模式（若 Provider 支持）
	StartNewChat   bool            `json:"startNewChat,omitempty"`

	// SessionID 的语义由 ConversationMode 决定：
	//   - remote_session: 由远端服务分配的会话标识（如 Dify conversation_id），首轮为空，续轮注入
	//   - local_history:  由应用端分配的本地会话标识，仅用于 SessionStore 查询，不发送给目标模型
	//   - 未设置 mode:    保持旧行为，直接透传给 Provider
	SessionID string `json:"session_id,omitempty"`

	// ChainValues 携带上一轮从响应提取的链路字段值，key=占位符（含$$$），val=提取值。
	// 由 Session 层在每轮请求前填充，GenericSpec.BuildRequest 消费。
	ChainValues map[string]string `json:"chain_values,omitempty"`
}

// Usage 表示 token 使用统计。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse 是统一的响应结构。
type ChatResponse struct {
	Text string
	// SessionID 的语义由 ConversationMode 决定：
	//   - remote_session: 远端服务返回的会话标识，客户端应提取并持久化以供续轮使用
	//   - local_history:  无实际意义，不使用
	SessionID string
	// ChainValues 携带从响应中提取的链路字段值（非流式模式）。
	// 流式模式下通过 StreamChunk.ChainValues 传递。
	ChainValues map[string]string
	Usage       *Usage
	Raw         []byte
}
