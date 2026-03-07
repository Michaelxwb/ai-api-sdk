package streaming

// StreamConfig defines streaming parser behavior.
type StreamConfig struct {
	// Transport protocol.
	Protocol StreamProtocol

	// JSON field paths for delta extraction (supports multiple paths).
	DeltaPaths []string

	// Done detection configuration.
	DonePath   string // Done field path (e.g. "finish_reason")
	DoneValue  string // Done value (e.g. "stop")
	DoneMarker string // SSE done marker (e.g. "[DONE]")

	// SSE event filter (Claude requires this).
	EventFilter map[string]bool // Allowed event types

	// ErrorPath is the JSON path to extract error message from a data frame (e.g. "error.message").
	// When set and text extraction yields nothing, the extractor checks this path and returns an error.
	ErrorPath string
}

// DeltaExtractor extracts incremental content from a JSON event payload.
type DeltaExtractor func(event string, data []byte) (text string, done bool, err error)
