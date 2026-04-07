package coze

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

// cozeEventData is the common envelope for all Coze SSE event data payloads.
type cozeEventData struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	BotID          string `json:"bot_id"`
	Status         string `json:"status"`
	Role           string `json:"role"`
	Type           string `json:"type"`
	Content        string `json:"content"`
	ContentType    string `json:"content_type"`
	LastError      *struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	} `json:"last_error"`
	Usage *struct {
		TokenCount  int `json:"token_count"`
		OutputCount int `json:"output_count"`
		InputCount  int `json:"input_count"`
	} `json:"usage"`
}

// ParseStreamResponse implements streaming.ProviderStreamSpec.
func (s *CozeSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("coze: response is nil")
	}

	ctx := streaming.StreamContext(resp)
	out := make(chan streaming.StreamChunk, 16)

	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		reader := bufio.NewReader(resp.Body)
		currentEvent := ""
		dataLines := make([]string, 0, 4)
		conversationID := ""

		for {
			select {
			case <-ctx.Done():
				sendStreamChunk(ctx, out, streaming.StreamChunk{Error: ctx.Err(), Done: true})
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if line != "" {
						if handleCozeLine(line, &currentEvent, &dataLines) && len(dataLines) > 0 {
							if handleCozeEvent(ctx, out, currentEvent, dataLines, &conversationID) {
								return
							}
						}
					}
					if len(dataLines) > 0 {
						if handleCozeEvent(ctx, out, currentEvent, dataLines, &conversationID) {
							return
						}
					}
					return
				}
				sendStreamChunk(ctx, out, streaming.StreamChunk{Error: fmt.Errorf(
					"coze: stream read failed: %w", err), Done: true})
				return
			}

			if handleCozeLine(line, &currentEvent, &dataLines) {
				if len(dataLines) > 0 {
					if handleCozeEvent(ctx, out, currentEvent, dataLines, &conversationID) {
						return
					}
				}
				currentEvent = ""
				dataLines = dataLines[:0]
			}
		}
	}()

	return out, nil
}

// handleCozeLine processes a single SSE line. Returns true on blank line (event boundary).
func handleCozeLine(line string, currentEvent *string, dataLines *[]string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return true
	}
	// SSE comment
	if strings.HasPrefix(trimmed, ":") {
		return false
	}
	// event: header
	if strings.HasPrefix(trimmed, "event:") {
		value := strings.TrimPrefix(trimmed, "event:")
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
		*currentEvent = value
		return false
	}
	// data: payload
	if strings.HasPrefix(trimmed, "data:") {
		value := strings.TrimPrefix(trimmed, "data:")
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
		*dataLines = append(*dataLines, value)
		return false
	}
	return false
}

// handleCozeEvent dispatches a complete SSE event. Returns true when stream should terminate.
func handleCozeEvent(ctx context.Context, out chan<- streaming.StreamChunk, event string, dataLines []string, conversationID *string) bool {
	data := strings.Join(dataLines, "\n")

	// "done" event signals stream end.
	if event == "done" {
		sendStreamChunk(ctx, out, streaming.StreamChunk{
			Done:      true,
			SessionID: safeString(conversationID),
			Event:     event,
			Raw:       []byte(data),
		})
		return true
	}

	// Parse JSON payload.
	var ed cozeEventData
	if err := json.Unmarshal([]byte(data), &ed); err != nil {
		sendStreamChunk(ctx, out, streaming.StreamChunk{
			Error: fmt.Errorf("coze: stream JSON parse failed: %w", err),
			Done:  true,
			Raw:   []byte(data),
		})
		return true
	}

	// Track conversation_id across chunks.
	if ed.ConversationID != "" && conversationID != nil {
		*conversationID = ed.ConversationID
	}

	switch event {
	case "conversation.chat.created":
		// Emit a chunk with SessionID so the client persists it immediately.
		sendStreamChunk(ctx, out, streaming.StreamChunk{
			SessionID: safeString(conversationID),
			Event:     event,
			Raw:       []byte(data),
		})

	case "conversation.chat.in_progress":
		// Status update only, skip.

	case "conversation.message.delta":
		// Only emit text for answer-type messages.
		if ed.Type == "answer" && ed.Content != "" {
			sendStreamChunk(ctx, out, streaming.StreamChunk{
				Text:      ed.Content,
				SessionID: safeString(conversationID),
				Event:     event,
				Raw:       []byte(data),
			})
		}

	case "conversation.message.completed":
		// Full message already accumulated from deltas, skip.

	case "conversation.chat.completed":
		var usage *base.Usage
		if ed.Usage != nil {
			usage = &base.Usage{
				PromptTokens:     ed.Usage.InputCount,
				CompletionTokens: ed.Usage.OutputCount,
				TotalTokens:      ed.Usage.TokenCount,
			}
		}
		sendStreamChunk(ctx, out, streaming.StreamChunk{
			Done:      true,
			SessionID: safeString(conversationID),
			Usage:     usage,
			Event:     event,
			Raw:       []byte(data),
		})
		return true

	case "conversation.chat.failed":
		msg := "coze: chat failed"
		if ed.LastError != nil && ed.LastError.Msg != "" {
			msg = fmt.Sprintf("coze: %s", ed.LastError.Msg)
		}
		sendStreamChunk(ctx, out, streaming.StreamChunk{
			Error:     fmt.Errorf("%s", msg),
			Done:      true,
			SessionID: safeString(conversationID),
			Event:     event,
			Raw:       []byte(data),
		})
		return true

	default:
		// Unknown events: if they carry answer content, emit it.
		if ed.Type == "answer" && ed.Content != "" {
			sendStreamChunk(ctx, out, streaming.StreamChunk{
				Text:      ed.Content,
				SessionID: safeString(conversationID),
				Event:     event,
				Raw:       []byte(data),
			})
		}
	}

	return false
}

func sendStreamChunk(ctx context.Context, out chan<- streaming.StreamChunk, chunk streaming.StreamChunk) {
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

func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

var _ streaming.ProviderStreamSpec = (*CozeSpec)(nil)
