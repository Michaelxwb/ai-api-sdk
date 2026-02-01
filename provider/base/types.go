package base

// Message is a simplified chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatRequest is a unified request structure.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float32  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"` // Enable streaming mode when supported.
}

// ChatResponse is a unified response structure.
type ChatResponse struct {
	Text string
	Raw  []byte
}
