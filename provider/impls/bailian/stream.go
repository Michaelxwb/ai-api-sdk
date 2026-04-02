package bailian

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Bailian Responses API SSE event types:
//   response.output_text.delta  — incremental text in "delta" field
//   response.output_text.done   — full text in "text" field (ignored, we accumulate deltas)
//   response.completed          — done signal, carries usage in "usage" field

var bailianStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolSSE,
	DeltaPaths: []string{"delta"},
	EventFilter: map[string]bool{
		"response.output_text.delta": true,
		"response.completed":         true,
	},
	DonePath:  "",
	DoneValue: "response.completed",
}

// ParseStreamResponse implements streaming.ProviderStreamSpec.
func (s *BailianAppSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("bailian_app: response is nil")
	}
	parser := &streaming.SSEParser{Config: bailianStreamConfig}
	extractor := streaming.MakeJSONPathExtractor(
		bailianStreamConfig.DeltaPaths[0],
		bailianStreamConfig.DonePath,
		bailianStreamConfig.DoneValue,
		"",
	)

	raw, err := parser.Parse(streaming.StreamContext(resp), resp, extractor)
	if err != nil {
		return nil, err
	}

	// Wrap to extract usage from the response.completed event.
	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)
		for chunk := range raw {
			if chunk.Done && chunk.Event == "response.completed" && len(chunk.Raw) > 0 {
				chunk.Usage = parseCompletedUsage(chunk.Raw)
			}
			out <- chunk
		}
	}()
	return out, nil
}

// parseCompletedUsage extracts usage from a response.completed event payload.
func parseCompletedUsage(data []byte) *base.Usage {
	var envelope struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	u := envelope.Usage
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0 {
		return nil
	}
	return &base.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
}

var _ streaming.ProviderStreamSpec = (*BailianAppSpec)(nil)
