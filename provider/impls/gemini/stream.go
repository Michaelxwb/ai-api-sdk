package gemini

import (
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Gemini streaming configuration.
var geminiStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolSSE,
	DeltaPaths: []string{"candidates.0.content.parts.0.text"},
	DonePath:   "candidates.0.finishReason",
	DoneValue:  "STOP",
}

// ParseStreamResponse implements ProviderStreamSpec.
func (s *GeminiSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("gemini: response is nil")
	}
	parser := &streaming.SSEParser{Config: geminiStreamConfig}
	extractor := streaming.MakeJSONPathExtractor(
		geminiStreamConfig.DeltaPaths[0],
		geminiStreamConfig.DonePath,
		geminiStreamConfig.DoneValue,
		"",
	)
	return parser.Parse(streaming.StreamContext(resp), resp, extractor)
}
