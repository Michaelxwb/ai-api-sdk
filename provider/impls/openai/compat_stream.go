package openai

import (
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// OpenAI streaming configuration.
var openaiStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolSSE,
	DeltaPaths: []string{"choices.0.delta.content"},
	DoneMarker: "[DONE]",
}

// ParseStreamResponse implements ProviderStreamSpec.
func (s *OpenAICompatSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("openai_compat: response is nil")
	}
	parser := &streaming.SSEParser{Config: openaiStreamConfig}
	extractor := streaming.MakeJSONPathExtractor(
		openaiStreamConfig.DeltaPaths[0],
		openaiStreamConfig.DonePath,
		openaiStreamConfig.DoneValue,
	)
	return parser.Parse(streaming.StreamContext(resp), resp, extractor)
}
