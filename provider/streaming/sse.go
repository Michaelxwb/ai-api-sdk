package streaming

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// SSEParser is a generic Server-Sent Events parser.
type SSEParser struct {
	Config StreamConfig
}

// Parse parses an SSE stream into chunks.
func (p *SSEParser) Parse(
	ctx context.Context,
	resp *http.Response,
	extractor DeltaExtractor,
) (<-chan StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("sse: response is nil")
	}
	if extractor == nil {
		return nil, fmt.Errorf("sse: extractor is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		const maxLineSize = 1 << 20
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
		var currentEvent string
		dataLines := make([]string, 0, 4)

		for {
			select {
			case <-ctx.Done():
				sendStreamChunk(out, ctx, StreamChunk{Error: ctx.Err()})
				return
			default:
			}

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					if errors.Is(err, bufio.ErrTooLong) {
						sendStreamChunk(out, ctx, StreamChunk{Error: fmt.Errorf("sse: line too long")})
					} else {
						sendStreamChunk(out, ctx, StreamChunk{Error: err})
					}
					return
				}
				if len(dataLines) > 0 {
					if p.handleEvent(ctx, out, currentEvent, dataLines, extractor) {
						return
					}
				}
				return
			}

			line := scanner.Text()
			if p.processLine(line, &currentEvent, &dataLines) {
				if len(dataLines) > 0 {
					if p.handleEvent(ctx, out, currentEvent, dataLines, extractor) {
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

func (p *SSEParser) processLine(line string, currentEvent *string, dataLines *[]string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "event:") {
		*currentEvent = sseFieldValue(trimmed, "event:")
		return false
	}
	if strings.HasPrefix(trimmed, "data:") {
		*dataLines = append(*dataLines, sseFieldValue(trimmed, "data:"))
		return false
	}
	// Explicitly skip retry: lines per SSE spec
	if strings.HasPrefix(trimmed, "retry:") {
		return false
	}
	return false
}

func sseFieldValue(line, prefix string) string {
	value := strings.TrimPrefix(line, prefix)
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return value
}

func (p *SSEParser) handleEvent(
	ctx context.Context,
	out chan<- StreamChunk,
	event string,
	dataLines []string,
	extractor DeltaExtractor,
) bool {
	if len(p.Config.EventFilter) > 0 && !p.Config.EventFilter[event] {
		return false
	}

	data := strings.Join(dataLines, "\n")
	if p.Config.DoneMarker != "" && data == p.Config.DoneMarker {
		sendStreamChunk(out, ctx, StreamChunk{Done: true, Raw: []byte(data)})
		return true
	}

	text, done, err := extractor(event, []byte(data))
	if err != nil {
		sendStreamChunk(out, ctx, StreamChunk{Error: err})
		return true
	}

	sendStreamChunk(out, ctx, StreamChunk{Text: text, Done: done, Raw: []byte(data), Event: event})
	return done
}
