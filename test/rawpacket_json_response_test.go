package test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

// TestParseHTTPSpec_NonStreamingJSONResponse 覆盖 OneAPI / 任意 OpenAI 兼容非流式
// application/json 响应模板：要求 parseHTTPResponseSpec 识别为 json 协议，并正确
// 提取 $$$ 占位符路径。
func TestParseHTTPSpec_NonStreamingJSONResponse(t *testing.T) {
	request := "POST /v1/chat/completions HTTP/1.1\n" +
		"Host: mock.example.com\n" +
		"Authorization: Bearer <MOCK>\n" +
		"Content-Type: application/json\n\n" +
		"{\n  \"model\": \"Qwen3.5-27B-AWQ\",\n  \"messages\": [{\"role\":\"user\",\"content\":\"$$$\"}]\n}"
	response := "HTTP/1.1 200 OK\n" +
		"Content-Type: application/json\n\n" +
		"{\n  \"id\": \"chatcmpl-1\",\n  \"choices\": [\n    {\n      \"index\": 0,\n      \"message\": {\"role\":\"assistant\",\"content\":\"$$$\"}\n    }\n  ]\n}"

	raw, err := generic.ParseHTTPSpec(generic.RawHTTPSpec{
		Model:    "local_history",
		BaseURL:  "https://mock.example.com/v1/chat/completions",
		Request:  request,
		Response: response,
	})
	if err != nil {
		t.Fatalf("ParseHTTPSpec failed: %v", err)
	}

	if got := raw.Response.Stream.Protocol; got != "json" {
		t.Fatalf("Protocol = %q, want %q", got, "json")
	}
	wantPath := "choices.-1.message.content"
	if raw.Response.TextPath != wantPath {
		t.Fatalf("TextPath = %q, want %q", raw.Response.TextPath, wantPath)
	}
	if len(raw.Response.Stream.DeltaPaths) != 1 || raw.Response.Stream.DeltaPaths[0] != wantPath {
		t.Fatalf("DeltaPaths = %v, want [%q]", raw.Response.Stream.DeltaPaths, wantPath)
	}
}

// TestGenericSpec_ParseStreamResponse_JSONProtocol 验证 json 协议下
// ParseStreamResponse 能从一次性 application/json 响应中抽出 content 文本，
// 并发出单个 Done=true 的 chunk。
func TestGenericSpec_ParseStreamResponse_JSONProtocol(t *testing.T) {
	body := `{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"并发高效，简洁安全。"}}]}`
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{},
	}

	profile := generic.GenericProfile{
		Response: generic.ResponseProfile{
			TextPath: "choices.-1.message.content",
			Stream: generic.StreamProfile{
				Protocol:   "json",
				DeltaPaths: []string{"choices.-1.message.content"},
			},
		},
	}

	spec := generic.NewGenericSpec(profile)
	ch, err := spec.ParseStreamResponse(resp)
	if err != nil {
		t.Fatalf("ParseStreamResponse failed: %v", err)
	}

	var full string
	var sawDone bool
	var chunkCount int
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		chunkCount++
		full += chunk.Text
		if chunk.Done {
			sawDone = true
		}
	}

	if !sawDone {
		t.Fatalf("expected Done=true chunk, got none")
	}
	if chunkCount != 1 {
		t.Fatalf("chunk count = %d, want 1", chunkCount)
	}
	want := "并发高效，简洁安全。"
	if full != want {
		t.Fatalf("Text = %q, want %q", full, want)
	}
}
