package ragflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// ParseStreamResponse 实现 ProviderStreamSpec 接口。
//
// RAGFlow 流式响应格式：
//   - 数据帧: data:{"code":0,"data":{"answer":"增量文本","session_id":"xxx"}}
//   - 终止帧: data:{"code":0,"data":true}
//   - 错误帧: data:{"code":102,"message":"unauthorized"}
//
// 关键差异：终止帧的 data 字段从 object 变为 bool true，
// 需要通过 json.RawMessage 延迟解析来检测类型。
func (s *RAGFlowSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("ragflow: response is nil")
	}

	ctx := streaming.StreamContext(resp)
	out := make(chan streaming.StreamChunk, 16)

	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		reader := bufio.NewReader(resp.Body)
		sessionID := ""

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
					// 处理最后一行（可能没有换行符）。
					if trimmed := strings.TrimRight(line, "\r\n"); trimmed != "" {
						if value, ok := extractDataValue(trimmed); ok {
							if handleRAGFlowData(ctx, out, value, &sessionID) {
								return
							}
						}
					}
					return
				}
				sendChunk(ctx, out, streaming.StreamChunk{
					Error: fmt.Errorf("ragflow: stream read failed: %w", err), Done: true,
				})
				return
			}

			trimmed := strings.TrimRight(line, "\r\n")
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, ":") {
				continue
			}

			if value, ok := extractDataValue(trimmed); ok {
				if handleRAGFlowData(ctx, out, value, &sessionID) {
					return
				}
			}
		}
	}()

	return out, nil
}

// extractDataValue 从 SSE 行中提取 data: 后的值。
func extractDataValue(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	value := strings.TrimPrefix(line, "data:")
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return value, true
}

// ragflowEnvelope 是 RAGFlow SSE 帧的外层结构。
// data 字段使用 json.RawMessage 延迟解析，因为它可能是 object 或 bool。
type ragflowEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ragflowDataPayload 是 data 字段为 object 时的内部结构。
type ragflowDataPayload struct {
	Answer    string `json:"answer"`
	SessionID string `json:"session_id"`
}

// handleRAGFlowData 处理单个 SSE data 值。返回 true 表示流应终止。
func handleRAGFlowData(ctx context.Context, out chan<- streaming.StreamChunk, data string, sessionID *string) bool {
	raw := []byte(data)

	var envelope ragflowEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		sendChunk(ctx, out, streaming.StreamChunk{
			Error: fmt.Errorf("ragflow: stream parsing failed: %w", err),
			Done:  true,
			Raw:   raw,
		})
		return true
	}

	// 错误帧：code != 0。
	if envelope.Code != 0 {
		msg := envelope.Message
		if msg == "" {
			msg = "unknown error"
		}
		sendChunk(ctx, out, streaming.StreamChunk{
			Error: fmt.Errorf("ragflow: server error: %s", msg),
			Done:  true,
			Raw:   raw,
		})
		return true
	}

	// 检测终止帧：data 字段为 bool true。
	// json.RawMessage 去掉空白后如果是 "true"，表示流结束。
	trimmedData := strings.TrimSpace(string(envelope.Data))
	if trimmedData == "true" {
		sid := ""
		if sessionID != nil {
			sid = *sessionID
		}
		sendChunk(ctx, out, streaming.StreamChunk{Done: true, SessionID: sid, Raw: raw})
		return true
	}

	// 数据帧：data 字段为 object，提取 answer 和 session_id。
	var payload ragflowDataPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		sendChunk(ctx, out, streaming.StreamChunk{
			Error: fmt.Errorf("ragflow: stream data parsing failed: %w", err),
			Done:  true,
			Raw:   raw,
		})
		return true
	}

	if payload.SessionID != "" && sessionID != nil {
		*sessionID = payload.SessionID
	}

	sid := ""
	if sessionID != nil {
		sid = *sessionID
	}

	if payload.Answer != "" {
		sendChunk(ctx, out, streaming.StreamChunk{
			Text:      payload.Answer,
			SessionID: sid,
			Raw:       raw,
		})
	}

	return false
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

var _ streaming.ProviderStreamSpec = (*RAGFlowSpec)(nil)
