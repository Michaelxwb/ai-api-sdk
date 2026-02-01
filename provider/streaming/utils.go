package streaming

import (
	"context"
	"net/http"
)

func StreamContext(resp *http.Response) context.Context {
	if resp != nil && resp.Request != nil && resp.Request.Context() != nil {
		return resp.Request.Context()
	}
	return context.Background()
}

func sendStreamChunk(out chan<- StreamChunk, ctx context.Context, chunk StreamChunk) {
	if out == nil {
		return
	}
	select {
	case out <- chunk:
		return
	case <-ctx.Done():
		select {
		case out <- StreamChunk{Error: ctx.Err()}:
		default:
		}
	}
}
