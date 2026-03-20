package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

func TestRAGFlowSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "ragflow")

	t.Run("normal_with_chat_id", func(t *testing.T) {
		req := base.ChatRequest{
			Messages:  []base.Message{{Role: "user", Content: "what is RAGFlow?"}},
			Stream:    true,
			SessionID: "sess-1",
		}
		opts := base.BuildOptions{
			BaseURL: "https://ragflow.example.com",
			ExtraBody: map[string]any{
				"chat_id": "chat-abc",
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if got := httpReq.URL.String(); got != "https://ragflow.example.com/api/v1/chats/chat-abc/completions" {
			t.Fatalf("unexpected url: %s", got)
		}
		if httpReq.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("expected Accept header for stream")
		}

		payload := decodeBodyMap(t, httpReq)
		if payload["question"] != "what is RAGFlow?" {
			t.Fatalf("unexpected question: %v", payload["question"])
		}
		if payload["stream"] != true {
			t.Fatalf("unexpected stream: %v", payload["stream"])
		}
		if payload["session_id"] != "sess-1" {
			t.Fatalf("unexpected session_id: %v", payload["session_id"])
		}
		// chat_id should not appear in request body.
		if _, ok := payload["chat_id"]; ok {
			t.Fatalf("chat_id should not be in request body")
		}
	})

	t.Run("missing_chat_id", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://ragflow.example.com",
		}

		_, err := spec.BuildRequest(context.Background(), opts, req)
		if err == nil {
			t.Fatalf("expected error for missing chat_id")
		}
		if !strings.Contains(err.Error(), "chat_id is required") {
			t.Fatalf("expected 'chat_id is required' in error, got: %v", err)
		}
	})

	t.Run("empty_chat_id", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL:   "https://ragflow.example.com",
			ExtraBody: map[string]any{"chat_id": "  "},
		}

		_, err := spec.BuildRequest(context.Background(), opts, req)
		if err == nil {
			t.Fatalf("expected error for empty chat_id")
		}
	})

	t.Run("omit_session_id_when_empty", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL:   "https://ragflow.example.com",
			ExtraBody: map[string]any{"chat_id": "chat-1"},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if _, ok := payload["session_id"]; ok {
			t.Fatalf("expected session_id to be omitted when empty")
		}
	})

	t.Run("question_from_last_user_message", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "reply"},
				{Role: "user", Content: "second"},
			},
		}
		opts := base.BuildOptions{
			BaseURL:   "https://ragflow.example.com",
			ExtraBody: map[string]any{"chat_id": "chat-1"},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["question"] != "second" {
			t.Fatalf("expected question from last user message, got: %v", payload["question"])
		}
	})

	t.Run("empty_messages", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{},
		}
		opts := base.BuildOptions{
			BaseURL:   "https://ragflow.example.com",
			ExtraBody: map[string]any{"chat_id": "chat-1"},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["question"] != "" {
			t.Fatalf("expected empty question, got: %v", payload["question"])
		}
	})

	t.Run("extra_body_passthrough", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://ragflow.example.com",
			ExtraBody: map[string]any{
				"chat_id": "chat-1",
				"top_k":   float64(5),
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["top_k"] != float64(5) {
			t.Fatalf("expected top_k passthrough, got: %v", payload["top_k"])
		}
	})

	t.Run("non_stream_no_accept_header", func(t *testing.T) {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "hi"}},
			Stream:   false,
		}
		opts := base.BuildOptions{
			BaseURL:   "https://ragflow.example.com",
			ExtraBody: map[string]any{"chat_id": "chat-1"},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if httpReq.Header.Get("Accept") == "text/event-stream" {
			t.Fatalf("non-stream request should not have Accept: text/event-stream")
		}
	})
}

func TestRAGFlowSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "ragflow")

	t.Run("normal", func(t *testing.T) {
		body := `{"code":0,"data":{"answer":"hello world","session_id":"sess-abc","reference":{}}}`
		assertParseResponse(t, spec, body, "hello world", "sess-abc", nil)
	})

	t.Run("empty_answer", func(t *testing.T) {
		body := `{"code":0,"data":{"answer":"","session_id":"sess-1"}}`
		assertParseResponse(t, spec, body, "", "sess-1", nil)
	})

	t.Run("server_error", func(t *testing.T) {
		body := `{"code":102,"message":"unauthorized"}`
		_, err := spec.ParseResponse(newHTTPResponse(body))
		if err == nil {
			t.Fatalf("expected error for code != 0")
		}
		if !strings.Contains(err.Error(), "ragflow:") {
			t.Fatalf("expected ragflow: prefix, got: %v", err)
		}
		if !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected 'unauthorized' in error, got: %v", err)
		}
	})

	t.Run("server_error_no_message", func(t *testing.T) {
		body := `{"code":500}`
		_, err := spec.ParseResponse(newHTTPResponse(body))
		if err == nil {
			t.Fatalf("expected error for code != 0")
		}
		if !strings.Contains(err.Error(), "unknown error") {
			t.Fatalf("expected 'unknown error' in error, got: %v", err)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		if _, err := spec.ParseResponse(newHTTPResponse("{")); err == nil {
			t.Fatalf("expected error for invalid json")
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		if _, err := spec.ParseResponse(nil); err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func TestRAGFlowSpec_ParseStreamResponse(t *testing.T) {
	spec := mustGetSpec(t, "ragflow")
	streamSpec, ok := spec.(streaming.ProviderStreamSpec)
	if !ok {
		t.Fatalf("ragflow does not implement ProviderStreamSpec")
	}

	t.Run("normal_data_and_done", func(t *testing.T) {
		payload := "" +
			"data:{\"code\":0,\"data\":{\"answer\":\"hello \",\"session_id\":\"sess-1\"}}\n\n" +
			"data:{\"code\":0,\"data\":{\"answer\":\"world\",\"session_id\":\"sess-1\"}}\n\n" +
			"data:{\"code\":0,\"data\":true}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "hello " || chunks[0].SessionID != "sess-1" {
			t.Fatalf("unexpected first chunk: %+v", chunks[0])
		}
		if chunks[1].Text != "world" || chunks[1].SessionID != "sess-1" {
			t.Fatalf("unexpected second chunk: %+v", chunks[1])
		}
		if !chunks[2].Done || chunks[2].SessionID != "sess-1" {
			t.Fatalf("unexpected done chunk: %+v", chunks[2])
		}
	})

	t.Run("error_frame", func(t *testing.T) {
		payload := "data:{\"code\":102,\"message\":\"unauthorized\"}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 || chunks[0].Error == nil {
			t.Fatalf("expected error chunk, got: %+v", chunks)
		}
		if !strings.Contains(chunks[0].Error.Error(), "ragflow:") {
			t.Fatalf("expected ragflow: prefix, got: %v", chunks[0].Error)
		}
		if !strings.Contains(chunks[0].Error.Error(), "unauthorized") {
			t.Fatalf("expected 'unauthorized' in error, got: %v", chunks[0].Error)
		}
	})

	t.Run("invalid_json_frame", func(t *testing.T) {
		payload := "data:{invalid json\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 || chunks[0].Error == nil {
			t.Fatalf("expected error chunk for invalid json, got: %+v", chunks)
		}
		if !chunks[0].Done {
			t.Fatalf("expected done=true on error chunk")
		}
	})

	t.Run("only_done_frame", func(t *testing.T) {
		payload := "data:{\"code\":0,\"data\":true}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d: %+v", len(chunks), chunks)
		}
		if !chunks[0].Done {
			t.Fatalf("expected done chunk, got: %+v", chunks[0])
		}
	})

	t.Run("connection_close_without_done", func(t *testing.T) {
		payload := "data:{\"code\":0,\"data\":{\"answer\":\"partial\",\"session_id\":\"s1\"}}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "partial" {
			t.Fatalf("expected text 'partial', got: %q", chunks[0].Text)
		}
	})

	t.Run("sse_comment_ignored", func(t *testing.T) {
		payload := "" +
			": this is a comment\n" +
			"data:{\"code\":0,\"data\":{\"answer\":\"text\",\"session_id\":\"s1\"}}\n\n" +
			"data:{\"code\":0,\"data\":true}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "text" {
			t.Fatalf("expected text 'text', got: %q", chunks[0].Text)
		}
		if !chunks[1].Done {
			t.Fatalf("expected done chunk, got: %+v", chunks[1])
		}
	})

	t.Run("data_with_space_after_colon", func(t *testing.T) {
		payload := "data: {\"code\":0,\"data\":{\"answer\":\"spaced\",\"session_id\":\"s1\"}}\n\n" +
			"data: {\"code\":0,\"data\":true}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "spaced" {
			t.Fatalf("expected text 'spaced', got: %q", chunks[0].Text)
		}
	})

	t.Run("session_id_carried_to_done", func(t *testing.T) {
		payload := "" +
			"data:{\"code\":0,\"data\":{\"answer\":\"a\",\"session_id\":\"sid-xyz\"}}\n\n" +
			"data:{\"code\":0,\"data\":true}\n\n"

		resp := newRAGFlowStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].SessionID != "sid-xyz" {
			t.Fatalf("expected session_id 'sid-xyz' in data chunk, got: %q", chunks[0].SessionID)
		}
		if chunks[1].SessionID != "sid-xyz" {
			t.Fatalf("expected session_id 'sid-xyz' in done chunk, got: %q", chunks[1].SessionID)
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := streamSpec.ParseStreamResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func newRAGFlowStreamResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewBufferString(payload)),
	}
}
