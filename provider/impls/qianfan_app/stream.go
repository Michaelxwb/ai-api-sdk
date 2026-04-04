package qianfan_app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Qianfan App SSE format (no event: header, pure data: lines):
//   data: {"answer":"增量文本","conversation_id":"xxx","is_completion":false}
//   data: {"answer":"最后一段","conversation_id":"xxx","is_completion":true,"usage":{...}}

var qianfanAppStreamConfig = streaming.StreamConfig{
	Protocol:   streaming.ProtocolSSE,
	DeltaPaths: []string{"answer"},
	DonePath:   "is_completion",
	DoneValue:  "true",
}

// ParseStreamResponse implements streaming.ProviderStreamSpec.
func (s *QianfanAppSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("qianfan_app: response is nil")
	}
	parser := &streaming.SSEParser{Config: qianfanAppStreamConfig}
	extractor := streaming.MakeJSONPathExtractor(
		qianfanAppStreamConfig.DeltaPaths[0],
		qianfanAppStreamConfig.DonePath,
		qianfanAppStreamConfig.DoneValue,
		"",
	)

	raw, err := parser.Parse(streaming.StreamContext(resp), resp, extractor)
	if err != nil {
		return nil, err
	}

	// Wrap to extract conversation_id→SessionID and usage from each frame.
	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)
		for chunk := range raw {
			if len(chunk.Raw) > 0 {
				var frame struct {
					ConversationID string `json:"conversation_id"`
					Usage          struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal(chunk.Raw, &frame) == nil {
					if frame.ConversationID != "" {
						chunk.SessionID = frame.ConversationID
					}
					if chunk.Done {
						u := frame.Usage
						if u.PromptTokens > 0 || u.CompletionTokens > 0 || u.TotalTokens > 0 {
							chunk.Usage = &base.Usage{
								PromptTokens:     u.PromptTokens,
								CompletionTokens: u.CompletionTokens,
								TotalTokens:      u.TotalTokens,
							}
						}
					}
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}

var _ streaming.ProviderStreamSpec = (*QianfanAppSpec)(nil)
