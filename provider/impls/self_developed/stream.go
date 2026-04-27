package self_developed

import (
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// ParseStreamResponse 实现流式响应解析。
//
// self_developed 不支持流式输出：
//   - 后端 API 仅提供阻塞式响应
//   - 返回错误，SDK 会自动降级到非流式模式
//
// 注意：如果调用方强制开启流式（Stream: true），会返回此错误。
func (s *MultiroundSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	return nil, fmt.Errorf("self_developed: streaming is not supported")
}
