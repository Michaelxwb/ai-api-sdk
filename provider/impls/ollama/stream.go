package ollama

import (
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Ollama streaming configuration.
var ollamaStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolNDJSON,
	DeltaPaths: []string{"message.content"},
	DonePath:   "done",
	DoneValue:  "true",
}

// ParseStreamResponse implements ProviderStreamSpec.
func (s *OllamaSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("ollama: response is nil")
	}
	parser := &streaming.NDJSONParser{Config: ollamaStreamConfig}
	extractor := streaming.MakeJSONPathExtractor(
		ollamaStreamConfig.DeltaPaths[0],
		ollamaStreamConfig.DonePath,
		ollamaStreamConfig.DoneValue,
		"",
	)
	return parser.Parse(streaming.StreamContext(resp), resp, extractor)
}
