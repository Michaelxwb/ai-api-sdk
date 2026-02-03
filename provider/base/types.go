package base

// Message 是简化的对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatRequest 是统一的请求结构。
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float32  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`     // 是否启用流式模式（若 Provider 支持）
	SessionID   string    `json:"session_id,omitempty"` // 可选：用于多轮对话的会话标识
}

// Usage 表示 token 使用统计。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse 是统一的响应结构。
type ChatResponse struct {
	Text      string
	SessionID string
	Usage     *Usage
	Raw       []byte
}
