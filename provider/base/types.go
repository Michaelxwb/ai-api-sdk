package base

// Message 是简化的对话消息。
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ChatRequest 是统一的请求结构。
type ChatRequest struct {
	Model        string    `json:"model"`
	Messages     []Message `json:"messages"`
	Temperature  *float32  `json:"temperature,omitempty"`
	MaxTokens    *int      `json:"max_tokens,omitempty"`
	Stream       bool      `json:"stream,omitempty"` // 是否启用流式模式（若 Provider 支持）
	StartNewChat bool      `json:"startNewChat,omitempty"`

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
