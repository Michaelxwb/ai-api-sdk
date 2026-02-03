package streaming

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
)

// NDJSONParser is a generic NDJSON stream parser.
type NDJSONParser struct {
	Config StreamConfig
}

// Parse parses an NDJSON stream into chunks.
func (p *NDJSONParser) Parse(
	ctx context.Context,
	resp *http.Response,
	extractor DeltaExtractor,
) (<-chan StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("ndjson: response is nil")
	}
	if extractor == nil {
		return nil, fmt.Errorf("ndjson: extractor is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				sendStreamChunk(out, ctx, StreamChunk{Error: ctx.Err()})
				return
			default:
			}

			data := scanner.Bytes()
			if len(data) == 0 {
				continue
			}

			text, done, err := extractor("", data)
			if err != nil {
				sendStreamChunk(out, ctx, StreamChunk{Error: err})
				continue
			}
			sendStreamChunk(out, ctx, StreamChunk{Text: text, Done: done, Raw: data})
			if done {
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			sendStreamChunk(out, ctx, StreamChunk{Error: err})
			return
		}
	}()

	return out, nil
}
