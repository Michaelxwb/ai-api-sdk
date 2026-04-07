package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// ---------------------------------------------------------------------------
// BuildRequest tests (TASK-004)
// ---------------------------------------------------------------------------

func TestCozeSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "coze")

	t.Run("normal_streaming", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-abc",
			Messages: []base.Message{{Role: "user", Content: "hello coze"}},
			Stream:   true,
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.coze.cn/v3",
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if !strings.HasPrefix(httpReq.URL.String(), "https://api.coze.cn/v3/chat") {
			t.Fatalf("unexpected url: %s", httpReq.URL.String())
		}
		if httpReq.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected Content-Type application/json")
		}
		if httpReq.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("expected Accept text/event-stream")
		}

		payload := decodeBodyMap(t, httpReq)
		if payload["bot_id"] != "bot-abc" {
			t.Fatalf("unexpected bot_id: %v", payload["bot_id"])
		}
		if payload["user_id"] != "sdk-user" {
			t.Fatalf("unexpected user_id: %v", payload["user_id"])
		}
		if payload["stream"] != true {
			t.Fatalf("unexpected stream: %v", payload["stream"])
		}
		msgs, ok := payload["additional_messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatalf("expected additional_messages, got: %v", payload["additional_messages"])
		}
		msg0, _ := msgs[0].(map[string]any)
		if msg0["content"] != "hello coze" {
			t.Fatalf("unexpected message content: %v", msg0["content"])
		}
		if msg0["role"] != "user" {
			t.Fatalf("unexpected message role: %v", msg0["role"])
		}
		if msg0["type"] != "question" {
			t.Fatalf("unexpected message type: %v", msg0["type"])
		}
	})

	t.Run("conversation_id_in_url_query", func(t *testing.T) {
		req := base.ChatRequest{
			Model:     "bot-1",
			Messages:  []base.Message{{Role: "user", Content: "hi"}},
			SessionID: "conv-123",
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.coze.cn/v3",
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if httpReq.URL.Query().Get("conversation_id") != "conv-123" {
			t.Fatalf("expected conversation_id=conv-123 in query, got: %s", httpReq.URL.RawQuery)
		}
		// conversation_id should NOT be in body
		payload := decodeBodyMap(t, httpReq)
		if _, ok := payload["conversation_id"]; ok {
			t.Fatalf("conversation_id should not be in request body")
		}
	})

	t.Run("no_conversation_id_when_empty", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.coze.cn/v3",
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if httpReq.URL.Query().Get("conversation_id") != "" {
			t.Fatalf("expected no conversation_id query param, got: %s", httpReq.URL.RawQuery)
		}
	})

	t.Run("bot_id_from_model", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-xyz-999",
			Messages: []base.Message{{Role: "user", Content: "test"}},
		}
		opts := base.BuildOptions{}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["bot_id"] != "bot-xyz-999" {
			t.Fatalf("expected bot_id from model, got: %v", payload["bot_id"])
		}
	})

	t.Run("user_id_default", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["user_id"] != "sdk-user" {
			t.Fatalf("expected default user_id 'sdk-user', got: %v", payload["user_id"])
		}
	})

	t.Run("user_id_override_via_extra_body", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			ExtraBody: map[string]any{
				"user_id": "custom-user-42",
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["user_id"] != "custom-user-42" {
			t.Fatalf("expected user_id override, got: %v", payload["user_id"])
		}
	})

	t.Run("stream_always_true", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
			Stream:   false, // explicitly false
		}
		opts := base.BuildOptions{}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["stream"] != true {
			t.Fatalf("expected stream always true, got: %v", payload["stream"])
		}
	})

	t.Run("extra_body_passthrough", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			ExtraBody: map[string]any{
				"custom_variables": map[string]any{"key1": "val1"},
				"meta_data":        map[string]any{"source": "test"},
			},
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		if payload["custom_variables"] == nil {
			t.Fatalf("expected custom_variables passthrough")
		}
		if payload["meta_data"] == nil {
			t.Fatalf("expected meta_data passthrough")
		}
	})

	t.Run("question_from_last_user_message", func(t *testing.T) {
		req := base.ChatRequest{
			Model: "bot-1",
			Messages: []base.Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "reply"},
				{Role: "user", Content: "second"},
			},
		}
		opts := base.BuildOptions{}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		payload := decodeBodyMap(t, httpReq)
		msgs, _ := payload["additional_messages"].([]any)
		msg0, _ := msgs[0].(map[string]any)
		if msg0["content"] != "second" {
			t.Fatalf("expected last user message, got: %v", msg0["content"])
		}
	})

	t.Run("path_override", func(t *testing.T) {
		req := base.ChatRequest{
			Model:    "bot-1",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		}
		opts := base.BuildOptions{
			BaseURL: "https://api.coze.com/v3",
			Path:    "/custom-chat",
		}

		httpReq, err := spec.BuildRequest(context.Background(), opts, req)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if !strings.Contains(httpReq.URL.Path, "/custom-chat") {
			t.Fatalf("expected path override, got: %s", httpReq.URL.String())
		}
	})
}

// ---------------------------------------------------------------------------
// ParseResponse tests (TASK-005)
// ---------------------------------------------------------------------------

func TestCozeSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "coze")

	t.Run("not_supported", func(t *testing.T) {
		resp := newHTTPResponse(`{}`)
		_, err := spec.ParseResponse(resp)
		if err == nil {
			t.Fatalf("expected error for non-streaming")
		}
		if !strings.Contains(err.Error(), "non-streaming not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := spec.ParseResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

// ---------------------------------------------------------------------------
// ParseStreamResponse tests (TASK-005)
// ---------------------------------------------------------------------------

func TestCozeSpec_ParseStreamResponse(t *testing.T) {
	spec := mustGetSpec(t, "coze")
	streamSpec, ok := spec.(streaming.ProviderStreamSpec)
	if !ok {
		t.Fatalf("coze does not implement ProviderStreamSpec")
	}

	t.Run("normal_delta_and_done", func(t *testing.T) {
		payload := "" +
			"event:conversation.chat.created\n" +
			"data:{\"id\":\"chat-1\",\"conversation_id\":\"conv-abc\",\"bot_id\":\"bot-1\",\"status\":\"created\"}\n\n" +
			"event:conversation.chat.in_progress\n" +
			"data:{\"id\":\"chat-1\",\"conversation_id\":\"conv-abc\",\"status\":\"in_progress\"}\n\n" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"msg-1\",\"conversation_id\":\"conv-abc\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"Hello \"}\n\n" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"msg-1\",\"conversation_id\":\"conv-abc\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"world\"}\n\n" +
			"event:conversation.chat.completed\n" +
			"data:{\"id\":\"chat-1\",\"conversation_id\":\"conv-abc\",\"status\":\"completed\",\"usage\":{\"token_count\":100,\"output_count\":60,\"input_count\":40}}\n\n" +
			"event:done\n" +
			"data:{}\n\n"

		resp := newCozeStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)

		// Expect: created(SessionID) + delta("Hello ") + delta("world") + completed(done+usage)
		// chat.completed terminates the stream before done event is read
		var texts []string
		var gotDone bool
		var gotUsage *base.Usage
		var lastSessionID string
		for _, c := range chunks {
			if c.Error != nil {
				t.Fatalf("unexpected error: %v", c.Error)
			}
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
			if c.SessionID != "" {
				lastSessionID = c.SessionID
			}
			if c.Done {
				gotDone = true
			}
			if c.Usage != nil {
				gotUsage = c.Usage
			}
		}

		fullText := strings.Join(texts, "")
		if fullText != "Hello world" {
			t.Fatalf("expected 'Hello world', got: %q", fullText)
		}
		if lastSessionID != "conv-abc" {
			t.Fatalf("expected session_id 'conv-abc', got: %q", lastSessionID)
		}
		if !gotDone {
			t.Fatalf("expected done chunk")
		}
		if gotUsage == nil {
			t.Fatalf("expected usage")
		}
		if gotUsage.PromptTokens != 40 {
			t.Fatalf("expected PromptTokens=40, got: %d", gotUsage.PromptTokens)
		}
		if gotUsage.CompletionTokens != 60 {
			t.Fatalf("expected CompletionTokens=60, got: %d", gotUsage.CompletionTokens)
		}
		if gotUsage.TotalTokens != 100 {
			t.Fatalf("expected TotalTokens=100, got: %d", gotUsage.TotalTokens)
		}
	})

	t.Run("conversation_id_carried", func(t *testing.T) {
		payload := "" +
			"event:conversation.chat.created\n" +
			"data:{\"id\":\"c1\",\"conversation_id\":\"conv-xyz\"}\n\n" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"m1\",\"conversation_id\":\"conv-xyz\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"hi\"}\n\n" +
			"event:conversation.chat.completed\n" +
			"data:{\"id\":\"c1\",\"conversation_id\":\"conv-xyz\",\"status\":\"completed\"}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		for chunk := range ch {
			if chunk.Error != nil {
				t.Fatalf("unexpected error: %v", chunk.Error)
			}
			if chunk.SessionID != "conv-xyz" {
				t.Fatalf("expected SessionID 'conv-xyz' on every chunk, got: %q (event=%s)", chunk.SessionID, chunk.Event)
			}
		}
	})

	t.Run("only_answer_emits_text", func(t *testing.T) {
		payload := "" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"m1\",\"conversation_id\":\"c1\",\"role\":\"assistant\",\"type\":\"verbose_log\",\"content\":\"debug info\"}\n\n" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"m2\",\"conversation_id\":\"c1\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"real answer\"}\n\n" +
			"event:conversation.chat.completed\n" +
			"data:{\"id\":\"ch1\",\"conversation_id\":\"c1\",\"status\":\"completed\"}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var texts []string
		for chunk := range ch {
			if chunk.Error != nil {
				t.Fatalf("unexpected error: %v", chunk.Error)
			}
			if chunk.Text != "" {
				texts = append(texts, chunk.Text)
			}
		}
		if len(texts) != 1 || texts[0] != "real answer" {
			t.Fatalf("expected only 'real answer', got: %v", texts)
		}
	})

	t.Run("error_event", func(t *testing.T) {
		payload := "" +
			"event:conversation.chat.failed\n" +
			"data:{\"id\":\"c1\",\"conversation_id\":\"c1\",\"status\":\"failed\",\"last_error\":{\"code\":500,\"msg\":\"internal error\"}}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 || chunks[0].Error == nil {
			t.Fatalf("expected error chunk, got: %+v", chunks)
		}
		if !strings.Contains(chunks[0].Error.Error(), "internal error") {
			t.Fatalf("expected 'internal error' in error, got: %v", chunks[0].Error)
		}
		if !strings.Contains(chunks[0].Error.Error(), "coze:") {
			t.Fatalf("expected 'coze:' prefix, got: %v", chunks[0].Error)
		}
		if !chunks[0].Done {
			t.Fatalf("expected done=true on error chunk")
		}
	})

	t.Run("usage_mapping", func(t *testing.T) {
		payload := "" +
			"event:conversation.chat.completed\n" +
			"data:{\"id\":\"c1\",\"conversation_id\":\"c1\",\"status\":\"completed\",\"usage\":{\"token_count\":150,\"output_count\":80,\"input_count\":70}}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(chunks))
		}
		u := chunks[0].Usage
		if u == nil {
			t.Fatalf("expected usage")
		}
		if u.PromptTokens != 70 {
			t.Fatalf("expected PromptTokens=70 (input_count), got: %d", u.PromptTokens)
		}
		if u.CompletionTokens != 80 {
			t.Fatalf("expected CompletionTokens=80 (output_count), got: %d", u.CompletionTokens)
		}
		if u.TotalTokens != 150 {
			t.Fatalf("expected TotalTokens=150 (token_count), got: %d", u.TotalTokens)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		payload := "" +
			"event:conversation.message.delta\n" +
			"data:{invalid json\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 1 || chunks[0].Error == nil {
			t.Fatalf("expected error chunk for invalid json, got: %+v", chunks)
		}
		if !chunks[0].Done {
			t.Fatalf("expected done=true on error chunk")
		}
	})

	t.Run("sse_comment_ignored", func(t *testing.T) {
		payload := "" +
			": this is a comment\n" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"m1\",\"conversation_id\":\"c1\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"text\"}\n\n" +
			"event:conversation.chat.completed\n" +
			"data:{\"id\":\"c1\",\"conversation_id\":\"c1\",\"status\":\"completed\"}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		var texts []string
		for chunk := range ch {
			if chunk.Error != nil {
				t.Fatalf("unexpected error: %v", chunk.Error)
			}
			if chunk.Text != "" {
				texts = append(texts, chunk.Text)
			}
		}
		if len(texts) != 1 || texts[0] != "text" {
			t.Fatalf("expected 'text', got: %v", texts)
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := streamSpec.ParseStreamResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})

	t.Run("done_event_terminates", func(t *testing.T) {
		// Stream that ends with done event instead of chat.completed
		payload := "" +
			"event:conversation.message.delta\n" +
			"data:{\"id\":\"m1\",\"conversation_id\":\"c1\",\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"hi\"}\n\n" +
			"event:done\n" +
			"data:{}\n\n"

		ch, err := streamSpec.ParseStreamResponse(newCozeStreamResponse(payload))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		chunks := collectChunks(ch)
		var gotDone bool
		for _, c := range chunks {
			if c.Error != nil {
				t.Fatalf("unexpected error: %v", c.Error)
			}
			if c.Done {
				gotDone = true
			}
		}
		if !gotDone {
			t.Fatalf("expected done chunk from done event")
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decodeBodyMap is already defined in provider_test.go but we need it to work
// with requests whose body has already been consumed. Re-read from the request.
func decodeCozeBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// Reset body for potential re-reads
	req.Body = io.NopCloser(bytes.NewReader(data))
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return m
}

func newCozeStreamResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewBufferString(payload)),
	}
}
