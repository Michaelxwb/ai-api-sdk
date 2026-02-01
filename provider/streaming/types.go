package streaming

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Text  string // Incremental text content
	Done  bool   // Whether the stream is complete
	Error error  // Error if any occurred
	Raw   []byte // Raw chunk data (optional, for debugging)
}

// StreamProtocol defines streaming transport protocol types.
type StreamProtocol string

const (
	ProtocolSSE    StreamProtocol = "sse"    // Server-Sent Events
	ProtocolNDJSON StreamProtocol = "ndjson" // Newline Delimited JSON
)
