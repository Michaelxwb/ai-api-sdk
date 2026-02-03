package claude

import (
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Claude streaming configuration.
var claudeStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolSSE,
	DeltaPaths: []string{"delta.text"},
	EventFilter: map[string]bool{
		"content_block_delta": true,
		"message_stop":        true,
	},
}

// ParseStreamResponse implements ProviderStreamSpec.
func (s *ClaudeSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("claude: response is nil")
	}
	parser := &streaming.SSEParser{Config: claudeStreamConfig}
	extractor := func(event string, data []byte) (string, bool, error) {
		if event == "message_stop" {
			return "", true, nil
		}
		deltaType, _ := streaming.ExtractJSONPath(data, "delta.type")
		if deltaType != "text_delta" {
			return "", false, nil
		}
		text, _ := streaming.ExtractJSONPath(data, "delta.text")
		return text, false, nil
	}
	return parser.Parse(streaming.StreamContext(resp), resp, extractor)
}
