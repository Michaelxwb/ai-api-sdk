package streaming

import "net/http"

// ProviderStreamSpec is an optional interface for providers that support streaming.
type ProviderStreamSpec interface {
	// ParseStreamResponse parses SSE/NDJSON stream into chunks.
	ParseStreamResponse(resp *http.Response) (<-chan StreamChunk, error)
}
