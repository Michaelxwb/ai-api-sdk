package fastgpt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// ParseStreamResponse implements ProviderStreamSpec with a custom SSE parser.
//
// 自定义解析器（非复用 SSEParser）的原因：
//   - FastGPT 流式 event 类型多（answer/fastAnswer/flowNodeStatus/flowResponses/error/...）
//   - [DONE] 之后仍有 flowResponses 事件携带 Usage 数据，generic SSEParser 会在 [DONE] 后立即退出
//   - DeltaExtractor 签名 (text, done, error) 无法返回 Usage 等元数据
func (s *FastGPTSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("fastgpt: response is nil")
	}

	ctx := streaming.StreamContext(resp)
	out := make(chan streaming.StreamChunk, 16)

	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		reader := bufio.NewReader(resp.Body)
		var currentEvent string
		dataLines := make([]string, 0, 4)
		doneReceived := false

		for {
			select {
			case <-ctx.Done():
				sendChunk(ctx, out, streaming.StreamChunk{Error: ctx.Err(), Done: true})
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// Process any remaining buffered event.
					if len(dataLines) > 0 {
						if handleFastGPTEvent(ctx, out, currentEvent, dataLines, &doneReceived) {
							return
						}
					}
					return
				}
				sendChunk(ctx, out, streaming.StreamChunk{
					Error: fmt.Errorf("fastgpt: stream read failed: %w", err), Done: true,
				})
				return
			}

			if processFastGPTLine(line, &currentEvent, &dataLines) {
				if len(dataLines) > 0 {
					if handleFastGPTEvent(ctx, out, currentEvent, dataLines, &doneReceived) {
						return
					}
				}
				dataLines = dataLines[:0]
				currentEvent = ""
			}
		}
	}()

	return out, nil
}

// processFastGPTLine parses a single SSE line. Returns true on blank line (event boundary).
func processFastGPTLine(line string, currentEvent *string, dataLines *[]string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, ":") || strings.HasPrefix(trimmed, "retry:") {
		return false
	}
	if strings.HasPrefix(trimmed, "event:") {
		*currentEvent = sseValue(trimmed, "event:")
		return false
	}
	if strings.HasPrefix(trimmed, "data:") {
		*dataLines = append(*dataLines, sseValue(trimmed, "data:"))
		return false
	}
	return false
}

// handleFastGPTEvent processes a complete SSE event. Returns true if the stream should close.
func handleFastGPTEvent(ctx context.Context, out chan<- streaming.StreamChunk, event string, dataLines []string, doneReceived *bool) bool {
	data := strings.Join(dataLines, "\n")
	normalizedEvent := strings.TrimSpace(event)
	if normalizedEvent == "" {
		normalizedEvent = "answer"
	}

	switch normalizedEvent {
	case "answer", "fastAnswer":
		return handleAnswerEvent(ctx, out, data, normalizedEvent, doneReceived)
	case "error":
		return handleErrorEvent(ctx, out, data)
	case "flowResponses":
		handleFlowResponsesEvent(ctx, out, data)
		return false
	default:
		// flowNodeStatus, toolCall, toolParams, toolResponse, updateVariables — ignored.
		return false
	}
}

func handleAnswerEvent(ctx context.Context, out chan<- streaming.StreamChunk, data string, event string, doneReceived *bool) bool {
	if data == "[DONE]" {
		*doneReceived = true
		sendChunk(ctx, out, streaming.StreamChunk{Done: true, Raw: []byte(data), Event: event})
		// Continue reading for post-[DONE] metadata (flowResponses).
		return false
	}

	var parsed struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		sendChunk(ctx, out, streaming.StreamChunk{
			Error: fmt.Errorf("fastgpt: stream parse failed: %w", err), Done: true,
		})
		return true
	}

	if len(parsed.Choices) == 0 {
		return false
	}

	choice := parsed.Choices[0]
	text := choice.Delta.Content

	// Emit text chunk if content is non-empty.
	if text != "" {
		sendChunk(ctx, out, streaming.StreamChunk{Text: text, Raw: []byte(data), Event: event})
	}

	// finish_reason present (e.g. "stop") — mark the text portion as done.
	// [DONE] marker follows separately; we don't close the channel here.
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		sendChunk(ctx, out, streaming.StreamChunk{
			Done: true, Raw: []byte(data), Event: event,
		})
		// Don't return true — wait for [DONE] and potential flowResponses.
		return false
	}

	return false
}

func handleErrorEvent(ctx context.Context, out chan<- streaming.StreamChunk, data string) bool {
	msg := extractFastGPTErrorMessage([]byte(data))
	var err error
	if msg == "" {
		err = fmt.Errorf("fastgpt: stream error event")
	} else {
		err = fmt.Errorf("fastgpt: stream error: %s", msg)
	}
	sendChunk(ctx, out, streaming.StreamChunk{Error: err, Done: true, Raw: []byte(data), Event: "error"})
	return true
}

// handleFlowResponsesEvent parses flowResponses to extract token usage.
// Emitted as a metadata-only chunk (no text, not done).
func handleFlowResponsesEvent(ctx context.Context, out chan<- streaming.StreamChunk, data string) {
	usage := parseFlowResponsesUsage([]byte(data))
	if usage != nil {
		sendChunk(ctx, out, streaming.StreamChunk{Usage: usage, Raw: []byte(data), Event: "flowResponses"})
	}
}

// parseFlowResponsesUsage aggregates token usage from flowResponses JSON array.
func parseFlowResponsesUsage(data []byte) *base.Usage {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}

	return usageFromFastGPT(nil, items)
}

func extractFastGPTErrorMessage(data []byte) string {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return ""
	}

	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return truncateMsg(trimmed)
	}

	switch v := payload.(type) {
	case string:
		return truncateMsg(v)
	case map[string]any:
		for _, key := range []string{"message", "error", "detail", "msg"} {
			if val, ok := v[key]; ok {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return truncateMsg(s)
				}
			}
		}
	}
	return truncateMsg(trimmed)
}

func truncateMsg(msg string) string {
	const limit = 4096
	msg = strings.TrimSpace(msg)
	if len(msg) <= limit {
		return msg
	}
	return msg[:limit] + "...(truncated)"
}

func sseValue(line, prefix string) string {
	value := strings.TrimPrefix(line, prefix)
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return value
}

func sendChunk(ctx context.Context, out chan<- streaming.StreamChunk, chunk streaming.StreamChunk) {
	if out == nil {
		return
	}
	select {
	case out <- chunk:
		return
	case <-ctx.Done():
		select {
		case out <- streaming.StreamChunk{Error: ctx.Err(), Done: true}:
		default:
		}
	}
}

var _ streaming.ProviderStreamSpec = (*FastGPTSpec)(nil)
