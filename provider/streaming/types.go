package streaming

import "github.com/Michaelxwb/ai-api-sdk/provider/base"

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Text      string      // 增量文本内容
	SessionID string      // 会话标识（若提供）
	Usage     *base.Usage // Token 使用统计（若提供）
	Done      bool        // 是否结束
	Error     error       // 流式错误
	Raw       []byte      // 原始分片数据（可选，便于调试）
}

// StreamProtocol defines streaming transport protocol types.
type StreamProtocol string

const (
	ProtocolSSE    StreamProtocol = "sse"    // Server-Sent Events
	ProtocolNDJSON StreamProtocol = "ndjson" // Newline Delimited JSON
)
