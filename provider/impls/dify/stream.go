package dify

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

// DifyStreamSpec 实现 Dify 流式响应解析。
type DifyStreamSpec struct {
	DifySpec
}

func (s *DifyStreamSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("dify: 流式响应为空")
	}

	ctx := streaming.StreamContext(resp)
	out := make(chan streaming.StreamChunk, 16)

	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		reader := bufio.NewReader(resp.Body)
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
						if handleDifyLine(line, &dataLines) && len(dataLines) > 0 {
							if handleDifyEvent(ctx, out, dataLines, &conversationID) {
								return
							}
						}
					}
					if len(dataLines) > 0 {
						if handleDifyEvent(ctx, out, dataLines, &conversationID) {
							return
						}
					}
					return
				}
				sendStreamChunk(ctx, out, streaming.StreamChunk{Error: fmt.Errorf("dify: 读取流失败: %w", err), Done: true})
				return
			}

			if handleDifyLine(line, &dataLines) {
				if len(dataLines) > 0 {
					if handleDifyEvent(ctx, out, dataLines, &conversationID) {
						return
					}
				}
				dataLines = dataLines[:0]
			}
		}
	}()

	return out, nil
}

// ParseStreamResponse 实现 ProviderStreamSpec 接口。
func (s *DifySpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	streamSpec := &DifyStreamSpec{DifySpec: *s}
	return streamSpec.ParseStreamResponse(resp)
}

func handleDifyLine(line string, dataLines *[]string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, ":") {
		return false
	}
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

func handleDifyEvent(ctx context.Context, out chan<- streaming.StreamChunk, dataLines []string, conversationID *string) bool {
	data := strings.Join(dataLines, "\n")
	if data == "[DONE]" {
		sendStreamChunk(ctx, out, streaming.StreamChunk{Done: true, SessionID: safeString(conversationID), Raw: []byte(data)})
		return true
	}

	var event struct {
		Event          string `json:"event"`
		MessageID      string `json:"message_id"`
		ConversationID string `json:"conversation_id"`
		Answer         string `json:"answer"`
		Message        string `json:"message"`
		Code           string `json:"code"`
		Metadata       struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		sendStreamChunk(ctx, out, streaming.StreamChunk{Error: fmt.Errorf("dify: 解析流事件失败: %w", err), Done: true, Raw: []byte(data)})
		return true
	}

	if event.ConversationID != "" && conversationID != nil {
		*conversationID = event.ConversationID
	}

	switch event.Event {
	case "message":
		if event.Answer != "" {
			sendStreamChunk(ctx, out, streaming.StreamChunk{Text: event.Answer, SessionID: safeString(conversationID), Raw: []byte(data)})
		}
	case "message_end":
		var usage *base.Usage
		if event.Metadata.Usage.TotalTokens > 0 || event.Metadata.Usage.PromptTokens > 0 || event.Metadata.Usage.CompletionTokens > 0 {
			usage = &base.Usage{
				PromptTokens:     event.Metadata.Usage.PromptTokens,
				CompletionTokens: event.Metadata.Usage.CompletionTokens,
				TotalTokens:      event.Metadata.Usage.TotalTokens,
			}
		}
		sendStreamChunk(ctx, out, streaming.StreamChunk{Done: true, SessionID: safeString(conversationID), Usage: usage, Raw: []byte(data)})
		return true
	case "error":
		msg := event.Message
		if msg == "" {
			msg = "dify: 流式返回错误事件"
		}
		sendStreamChunk(ctx, out, streaming.StreamChunk{Error: fmt.Errorf("%s", msg), Done: true, Raw: []byte(data)})
		return true
	default:
		if event.Answer != "" {
			sendStreamChunk(ctx, out, streaming.StreamChunk{Text: event.Answer, SessionID: safeString(conversationID), Raw: []byte(data)})
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
