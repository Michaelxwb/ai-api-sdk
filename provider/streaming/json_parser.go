package streaming

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// JSONParser 处理一次性 application/json（非流式）响应。
// 读取完整 body 后调用 extractor 抽取文本，发射单个 Done=true 的 StreamChunk。
type JSONParser struct {
	Config StreamConfig
}

// Parse 读取整个响应 body 并通过 extractor 提取文本，发射单个 chunk。
func (p *JSONParser) Parse(
	ctx context.Context,
	resp *http.Response,
	extractor DeltaExtractor,
) (<-chan StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("json: response is nil")
	}
	if extractor == nil {
		return nil, fmt.Errorf("json: extractor is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan StreamChunk, 1)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		select {
		case <-ctx.Done():
			sendStreamChunk(out, ctx, StreamChunk{Error: ctx.Err()})
			return
		default:
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			sendStreamChunk(out, ctx, StreamChunk{Error: fmt.Errorf("json: read body failed: %w", err)})
			return
		}
		if len(data) == 0 {
			sendStreamChunk(out, ctx, StreamChunk{Done: true})
			return
		}

		text, _, err := extractor("", data)
		if err != nil {
			sendStreamChunk(out, ctx, StreamChunk{Error: err, Raw: data})
			return
		}
		sendStreamChunk(out, ctx, StreamChunk{Text: text, Done: true, Raw: data})
	}()

	return out, nil
}
