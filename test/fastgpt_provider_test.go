package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

func TestFastGPTSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "fastgpt")

	t.Run("default_fields", func(t *testing.T) {
		req := base.ChatRequest{
			Messages:  []base.Message{{Role: "user", Content: "hi"}},
			Stream:    true,
			SessionID: "sess-1",
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.example.com",
			ExtraBody: map[string]any{
				"detail":    true,
				"variables": map[string]any{"lang": "zh"},
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if got := httpReq.URL.String(); got != "https://api.example.com/api/v1/chat/completions" {
			t.Fatalf("unexpected url: %s", got)
		}

		payload := decodeBodyMap(t, httpReq)
		if payload["stream"] != true {
			t.Fatalf("unexpected stream: %v", payload["stream"])
		}
		if payload["chatId"] != "sess-1" {
			t.Fatalf("unexpected chatId: %v", payload["chatId"])
		}
		if _, ok := payload["detail"].(bool); !ok {
			t.Fatalf("missing detail flag: %v", payload["detail"])
		}
		if _, ok := payload["variables"].(map[string]any); !ok {
			t.Fatalf("missing variables: %v", payload["variables"])
		}
		if id, ok := payload["responseChatItemId"].(string); !ok || strings.TrimSpace(id) == "" {
			t.Fatalf("expected responseChatItemId to be generated, got: %v", payload["responseChatItemId"])
		}
	})

	t.Run("preserve_responseChatItemId", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.example.com",
			ExtraBody: map[string]any{
				"responseChatItemId": "fixed-id",
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["responseChatItemId"] != "fixed-id" {
			t.Fatalf("unexpected responseChatItemId: %v", payload["responseChatItemId"])
		}
	})

	t.Run("omit_chatId_when_session_empty", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{BaseURL: "https://api.example.com"}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if _, ok := payload["chatId"]; ok {
			t.Fatalf("expected chatId to be omitted, got: %v", payload["chatId"])
		}
	})

	t.Run("mode_filtered_from_payload", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.example.com",
			ExtraBody: map[string]any{
				"mode":   "local_history",
				"detail": true,
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if _, ok := payload["mode"]; ok {
			t.Fatalf("expected mode to be filtered, got: %v", payload["mode"])
		}
		if payload["detail"] != true {
			t.Fatalf("expected detail to remain, got: %v", payload["detail"])
		}
	})

	t.Run("model_not_in_payload", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "Qwen3-32B",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{BaseURL: "https://api.example.com"}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if _, ok := payload["model"]; ok {
			t.Fatalf("expected model to be absent (FastGPT ignores it), got: %v", payload["model"])
		}
	})
}

func TestFastGPTSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "fastgpt")

	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[{"message":{"content":"hello"}}]}`, "hello", "", nil)
	})
	t.Run("detail_usage", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[{"message":{"content":"hello"}}],"responseData":[{"tokens":4},{"tokens":6}]}`,
			"hello", "", &base.Usage{TotalTokens: 10})
	})
	t.Run("usage_top_level", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
			"hello", "", &base.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	})
	t.Run("error_in_body", func(t *testing.T) {
		resp, err := spec.ParseResponse(newHTTPResponse(`{"error":"unauthorized","choices":[]}`))
		if err == nil {
			t.Fatalf("expected error for error in body, got: %+v", resp)
		}
		if !strings.Contains(err.Error(), "fastgpt:") {
			t.Fatalf("expected fastgpt: prefix in error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected 'unauthorized' in error, got: %v", err)
		}
	})
	t.Run("error_message_field", func(t *testing.T) {
		_, err := spec.ParseResponse(newHTTPResponse(`{"message":"invalid_request"}`))
		if err == nil {
			t.Fatalf("expected error for error in body")
		}
		if !strings.Contains(err.Error(), "invalid_request") {
			t.Fatalf("expected 'invalid_request' in error, got: %v", err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := spec.ParseResponse(newHTTPResponse("{")); err == nil {
			t.Fatalf("expected error for invalid json")
		}
	})
	t.Run("nil", func(t *testing.T) {
		if _, err := spec.ParseResponse(nil); err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func TestFastGPTSpec_ParseStreamResponse(t *testing.T) {
	spec := mustGetSpec(t, "fastgpt")
	streamSpec, ok := spec.(streaming.ProviderStreamSpec)
	if !ok {
		t.Fatalf("fastgpt does not implement ProviderStreamSpec")
	}

	t.Run("answer_fastAnswer_done_and_retry", func(t *testing.T) {
		payload := "" +
			"retry: 1000\n\n" +
			"event: flowNodeStatus\n" +
			"data: {\"status\":\"running\"}\n\n" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n" +
			"event: fastAnswer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n\n" +
			"data: [DONE]\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 4 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "hello " || chunks[0].Error != nil {
			t.Fatalf("unexpected first chunk: %+v", chunks[0])
		}
		if chunks[1].Text != "world" || chunks[1].Error != nil {
			t.Fatalf("unexpected second chunk: %+v", chunks[1])
		}
		if chunks[2].Text != "!" || chunks[2].Error != nil {
			t.Fatalf("unexpected third chunk: %+v", chunks[2])
		}
		if !chunks[3].Done || string(chunks[3].Raw) != "[DONE]" {
			t.Fatalf("unexpected done chunk: %+v", chunks[3])
		}
	})

	t.Run("error_event", func(t *testing.T) {
		payload := "" +
			"event: error\n" +
			"data: {\"message\":\"bad\"}\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 || chunks[0].Error == nil {
			t.Fatalf("expected error chunk, got: %+v", chunks)
		}
		if !strings.HasPrefix(chunks[0].Error.Error(), "fastgpt:") {
			t.Fatalf("unexpected error prefix: %v", chunks[0].Error)
		}
	})

	t.Run("detail_true_with_flowResponses", func(t *testing.T) {
		payload := "" +
			"event: flowNodeStatus\n" +
			"data: {\"status\":\"running\",\"name\":\"知识库搜索\"}\n\n" +
			"event: flowNodeStatus\n" +
			"data: {\"status\":\"running\",\"name\":\"AI 对话\"}\n\n" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"event: answer\n" +
			"data: [DONE]\n\n" +
			"event: flowResponses\n" +
			"data: [{\"moduleName\":\"知识库搜索\",\"tokens\":6},{\"moduleName\":\"AI Chat\",\"tokens\":303}]\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)

		// Expected: text("hello"), done(finish_reason), done([DONE]), usage(flowResponses)
		if len(chunks) != 4 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}

		// Chunk 0: text content
		if chunks[0].Text != "hello" || chunks[0].Error != nil {
			t.Fatalf("unexpected text chunk: %+v", chunks[0])
		}

		// Chunk 1: finish_reason=stop
		if !chunks[1].Done {
			t.Fatalf("expected done chunk for finish_reason, got: %+v", chunks[1])
		}

		// Chunk 2: [DONE] marker
		if !chunks[2].Done || string(chunks[2].Raw) != "[DONE]" {
			t.Fatalf("unexpected done chunk: %+v", chunks[2])
		}

		// Chunk 3: flowResponses with Usage
		if chunks[3].Usage == nil {
			t.Fatalf("expected usage in flowResponses chunk, got: %+v", chunks[3])
		}
		if chunks[3].Usage.TotalTokens != 309 {
			t.Fatalf("expected total tokens 309, got: %d", chunks[3].Usage.TotalTokens)
		}
		if chunks[3].Event != "flowResponses" {
			t.Fatalf("expected event=flowResponses, got: %q", chunks[3].Event)
		}
	})

	t.Run("finish_reason_stop", func(t *testing.T) {
		payload := "" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"event: answer\n" +
			"data: [DONE]\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 3 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "hi" {
			t.Fatalf("expected text 'hi', got: %q", chunks[0].Text)
		}
		if !chunks[1].Done {
			t.Fatalf("expected done for finish_reason=stop, got: %+v", chunks[1])
		}
		if !chunks[2].Done || string(chunks[2].Raw) != "[DONE]" {
			t.Fatalf("unexpected final done chunk: %+v", chunks[2])
		}
	})

	t.Run("connection_close_termination", func(t *testing.T) {
		// Stream ends without [DONE] — connection-level termination.
		payload := "" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "partial" {
			t.Fatalf("expected text 'partial', got: %q", chunks[0].Text)
		}
	})

	t.Run("empty_event_as_answer", func(t *testing.T) {
		// No event: field → treated as answer.
		payload := "" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"implicit\"}}]}\n\n" +
			"data: [DONE]\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "implicit" {
			t.Fatalf("expected text 'implicit', got: %q", chunks[0].Text)
		}
		if !chunks[1].Done {
			t.Fatalf("expected done chunk, got: %+v", chunks[1])
		}
	})

	t.Run("ignore_non_answer_events", func(t *testing.T) {
		payload := "" +
			"event: toolCall\n" +
			"data: {\"toolName\":\"search\"}\n\n" +
			"event: toolParams\n" +
			"data: {\"query\":\"test\"}\n\n" +
			"event: toolResponse\n" +
			"data: {\"result\":\"ok\"}\n\n" +
			"event: updateVariables\n" +
			"data: {\"key\":\"val\"}\n\n" +
			"event: flowNodeStatus\n" +
			"data: {\"status\":\"running\"}\n\n" +
			"event: answer\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"result\"}}]}\n\n" +
			"data: [DONE]\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks (text + done), got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "result" {
			t.Fatalf("expected text 'result', got: %q", chunks[0].Text)
		}
		if !chunks[1].Done {
			t.Fatalf("expected done chunk, got: %+v", chunks[1])
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := streamSpec.ParseStreamResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func newStreamResponse(payload string) *http.Response {
	rec := httptest.NewRecorder()
	return &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(bytes.NewBufferString(payload)),
	}
}
