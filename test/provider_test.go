package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

type stubSpec struct {
	name string
}

func (s *stubSpec) Name() string { return s.name }

func (s *stubSpec) DefaultBaseURL() string { return "" }

func (s *stubSpec) SupportedAuthTypes() []auth.AuthType { return nil }

func (s *stubSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	return nil, nil
}

func (s *stubSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	return base.ChatResponse{}, nil
}

func (s *stubSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	return nil, false
}

func float32Ptr(v float32) *float32 { return &v }

func intPtr(v int) *int { return &v }

func mustGetSpec(t *testing.T, name string) base.ProviderSpec {
	t.Helper()
	spec, ok := base.Get(name)
	if !ok || spec == nil {
		t.Fatalf("provider spec %q not found", name)
	}
	return spec
}

func newHTTPResponse(body string) *http.Response {
	rec := httptest.NewRecorder()
	rec.WriteString(body)
	return &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
	}
}

func decodeBodyMap(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return payload
}

func assertParseResponse(t *testing.T, spec base.ProviderSpec, body string, wantText string, wantSession string, wantUsage *base.Usage) {
	t.Helper()
	resp, err := spec.ParseResponse(newHTTPResponse(body))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if resp.Text != wantText {
		t.Fatalf("unexpected text: got %q want %q", resp.Text, wantText)
	}
	if resp.SessionID != wantSession {
		t.Fatalf("unexpected session id: got %q want %q", resp.SessionID, wantSession)
	}
	if wantUsage == nil {
		if resp.Usage != nil {
			t.Fatalf("expected nil usage, got %+v", resp.Usage)
		}
		return
	}
	if resp.Usage == nil {
		t.Fatalf("expected usage %+v, got nil", wantUsage)
	}
	if *resp.Usage != *wantUsage {
		t.Fatalf("unexpected usage: got %+v want %+v", resp.Usage, wantUsage)
	}
}

func TestProviderRegistry(t *testing.T) {
	name := "test_registry_provider"
	spec1 := &stubSpec{name: "one"}
	spec2 := &stubSpec{name: "two"}

	base.Register(name, spec1)
	got, ok := base.Get(name)
	if !ok {
		t.Fatalf("expected registry hit for %q", name)
	}
	if got != spec1 {
		t.Fatalf("expected spec1, got %+v", got)
	}

	base.Register(name, spec2)
	got, ok = base.Get(name)
	if !ok {
		t.Fatalf("expected registry hit for %q", name)
	}
	if got != spec2 {
		t.Fatalf("expected spec2 after overwrite, got %+v", got)
	}

	base.Register("", spec1)
	if _, ok := base.Get(""); ok {
		t.Fatalf("did not expect empty name registration")
	}

	base.Register("test_registry_nil", nil)
	if _, ok := base.Get("test_registry_nil"); ok {
		t.Fatalf("did not expect nil spec registration")
	}

	if _, ok := base.Get("missing_provider_zzz"); ok {
		t.Fatalf("expected missing provider to return false")
	}
}

func TestOpenAICompatSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "openai_compat")
	req := base.ChatRequest{
		Model:       "gpt-4",
		Messages:    []base.Message{{Role: "user", Content: "hi"}},
		Temperature: float32Ptr(0.7),
		MaxTokens:   intPtr(128),
		Stream:      true,
	}
	opts := base.BuildOptions{
		BaseURL: "https://api.example.com/v1/",
		ExtraBody: map[string]any{
			"extra": "value",
		},
	}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("unexpected url: %s", got)
	}
	if ct := httpReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %s", ct)
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["model"] != "gpt-4" {
		t.Fatalf("unexpected model: %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
	if got, ok := payload["temperature"].(float64); !ok || math.Abs(got-0.7) > 1e-6 {
		t.Fatalf("unexpected temperature: %v", payload["temperature"])
	}
	if got, ok := payload["max_tokens"].(float64); !ok || got != 128 {
		t.Fatalf("unexpected max_tokens: %v", payload["max_tokens"])
	}
	if payload["extra"] != "value" {
		t.Fatalf("unexpected extra field: %v", payload["extra"])
	}

	msgs, ok := payload["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("unexpected messages: %v", payload["messages"])
	}
	msg0, ok := msgs[0].(map[string]any)
	if !ok || msg0["role"] != "user" || msg0["content"] != "hi" {
		t.Fatalf("unexpected message payload: %v", msgs[0])
	}
}

func TestOpenAISpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "openai")
	req := base.ChatRequest{
		Model:    "gpt-4",
		Messages: []base.Message{{Role: "user", Content: "hi"}},
		Stream:   false,
	}
	opts := base.BuildOptions{}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("unexpected url: %s", got)
	}
	payload := decodeBodyMap(t, httpReq)
	if payload["stream"] != false {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
}

func TestOpenAICompatSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "openai_compat")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[{"message":{"content":"hello"}}]}`, "hello", "", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[]}`, "", "", nil)
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

func TestOpenAISpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "openai")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[{"message":{"content":"hello"}}]}`, "hello", "", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{"choices":[]}`, "", "", nil)
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

func TestOpenAICompatSpec_AuthStrategyOverride(t *testing.T) {
	spec := mustGetSpec(t, "openai_compat")
	strategy, ok := spec.AuthStrategyOverride(&auth.Credential{AuthType: auth.AuthTypeAPIKey, APIKey: "key"})
	if !ok {
		t.Fatalf("expected auth override")
	}
	bearer, ok := strategy.(auth.BearerTokenStrategy)
	if !ok {
		t.Fatalf("expected BearerTokenStrategy, got %T", strategy)
	}
	if bearer.Token != "key" {
		t.Fatalf("unexpected token: %q", bearer.Token)
	}
}

func TestBailianAppSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "bailian_app")
	temp := float32(0.2)
	maxTokens := 16
	req := base.ChatRequest{
		Model:       "test",
		Messages:    []base.Message{{Role: "user", Content: "你好"}, {Role: "assistant", Content: "你好，我是助手"}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      false,
	}
	opts := base.BuildOptions{
		BaseURL: "https://dashscope.aliyuncs.com/api/v2/apps/agent/app-id/compatible-mode/v1",
		ExtraBody: map[string]any{
			"metadata": map[string]any{"trace_id": "t-1"},
		},
	}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://dashscope.aliyuncs.com/api/v2/apps/agent/app-id/compatible-mode/v1/responses" {
		t.Fatalf("unexpected url: %s", got)
	}
	if ct := httpReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %s", ct)
	}

	payload := decodeBodyMap(t, httpReq)
	if _, ok := payload["model"]; ok {
		t.Fatalf("expected model to be omitted for placeholder test model, got %v", payload["model"])
	}
	if payload["stream"] != false {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
	if got, ok := payload["temperature"].(float64); !ok || math.Abs(got-0.2) > 1e-6 {
		t.Fatalf("unexpected temperature: %v", payload["temperature"])
	}
	if got, ok := payload["max_tokens"].(float64); !ok || got != 16 {
		t.Fatalf("unexpected max_tokens: %v", payload["max_tokens"])
	}
	if _, ok := payload["metadata"].(map[string]any); !ok {
		t.Fatalf("expected metadata map, got %T", payload["metadata"])
	}
	input, ok := payload["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("unexpected input: %v", payload["input"])
	}
	item0, ok := input[0].(map[string]any)
	if !ok || item0["role"] != "user" || item0["content"] != "你好" {
		t.Fatalf("unexpected first input item: %v", input[0])
	}
}

func TestBailianAppSpec_BuildRequest_NoDuplicateResponsesPath(t *testing.T) {
	spec := mustGetSpec(t, "bailian_app")
	req := base.ChatRequest{
		Model:    "qwen-plus",
		Messages: []base.Message{{Role: "user", Content: "hi"}},
	}
	opts := base.BuildOptions{
		BaseURL: "https://dashscope.aliyuncs.com/api/v2/apps/agent/app-id/compatible-mode/v1/responses",
	}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://dashscope.aliyuncs.com/api/v2/apps/agent/app-id/compatible-mode/v1/responses" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestBailianAppSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "bailian_app")
	t.Run("output_text", func(t *testing.T) {
		assertParseResponse(t, spec, `{"output_text":"hello","usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}`,
			"hello", "", &base.Usage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8})
	})
	t.Run("output_content_fallback", func(t *testing.T) {
		assertParseResponse(t, spec, `{"output":[{"content":[{"type":"output_text","text":"hel"},{"type":"output_text","text":"lo"}]}]}`,
			"hello", "", nil)
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

func TestBailianAppSpec_ParseStreamResponse(t *testing.T) {
	spec := mustGetSpec(t, "bailian_app")
	streamSpec, ok := spec.(streaming.ProviderStreamSpec)
	if !ok {
		t.Fatalf("bailian_app does not implement ProviderStreamSpec")
	}

	t.Run("delta_events_with_usage", func(t *testing.T) {
		payload := "" +
			"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello \"}\n\n" +
			"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"usage\":{\"input_tokens\":5,\"output_tokens\":10,\"total_tokens\":15}}\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 3 {
			t.Fatalf("unexpected chunk count: %d, chunks: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "hello " || chunks[0].Error != nil {
			t.Fatalf("unexpected first chunk: %+v", chunks[0])
		}
		if chunks[1].Text != "world" || chunks[1].Error != nil {
			t.Fatalf("unexpected second chunk: %+v", chunks[1])
		}
		if !chunks[2].Done {
			t.Fatalf("expected done on last chunk")
		}
		if chunks[2].Usage == nil {
			t.Fatalf("expected usage on completed chunk")
		}
		if chunks[2].Usage.PromptTokens != 5 || chunks[2].Usage.CompletionTokens != 10 || chunks[2].Usage.TotalTokens != 15 {
			t.Fatalf("unexpected usage: %+v", chunks[2].Usage)
		}
	})

	t.Run("filters_irrelevant_events", func(t *testing.T) {
		payload := "" +
			"event: response.created\n" +
			"data: {\"type\":\"response.created\"}\n\n" +
			"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
			"event: response.output_item.done\n" +
			"data: {\"type\":\"response.output_item.done\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\"}\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks (1 delta + 1 done), got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "ok" {
			t.Fatalf("unexpected text: %q", chunks[0].Text)
		}
		if !chunks[1].Done {
			t.Fatalf("expected done")
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := streamSpec.ParseStreamResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func TestBailianAppSpec_AuthStrategyOverride(t *testing.T) {
	spec := mustGetSpec(t, "bailian_app")
	strategy, ok := spec.AuthStrategyOverride(&auth.Credential{AuthType: auth.AuthTypeAPIKey, APIKey: "key"})
	if !ok {
		t.Fatalf("expected auth override")
	}
	bearer, ok := strategy.(auth.BearerTokenStrategy)
	if !ok {
		t.Fatalf("expected BearerTokenStrategy, got %T", strategy)
	}
	if bearer.Token != "key" {
		t.Fatalf("unexpected token: %q", bearer.Token)
	}
}

func TestClaudeSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "claude")
	req := base.ChatRequest{
		Model: "claude-3",
		Messages: []base.Message{
			{Role: "system", Content: "sys1"},
			{Role: "user", Content: "hi"},
			{Role: "system", Content: "sys2"},
		},
		Stream: false,
	}
	opts := base.BuildOptions{BaseURL: "https://api.example.com"}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://api.example.com/v1/messages" {
		t.Fatalf("unexpected url: %s", got)
	}
	if httpReq.Header.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("missing anthropic-version header")
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["model"] != "claude-3" {
		t.Fatalf("unexpected model: %v", payload["model"])
	}
	if payload["stream"] != false {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
	if got, ok := payload["max_tokens"].(float64); !ok || got != 1024 {
		t.Fatalf("unexpected max_tokens: %v", payload["max_tokens"])
	}
	if payload["system"] != "sys1\nsys2" {
		t.Fatalf("unexpected system: %v", payload["system"])
	}
	msgs, ok := payload["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("unexpected messages: %v", payload["messages"])
	}
	msg0, ok := msgs[0].(map[string]any)
	if !ok || msg0["role"] != "user" || msg0["content"] != "hi" {
		t.Fatalf("unexpected message payload: %v", msgs[0])
	}
}

func TestClaudeSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "claude")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"content":[{"text":"hello"}]}`, "hello", "", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{"content":[]}`, "", "", nil)
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

func TestGeminiSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "gemini")
	req := base.ChatRequest{
		Model: "gemini-1.5",
		Messages: []base.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "question"},
			{Role: "assistant", Content: "answer"},
		},
		Stream: true,
	}
	opts := base.BuildOptions{BaseURL: "https://api.example.com"}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://api.example.com/v1beta/models/gemini-1.5:streamGenerateContent?alt=sse" {
		t.Fatalf("unexpected url: %s", got)
	}

	payload := decodeBodyMap(t, httpReq)
	contents, ok := payload["contents"].([]any)
	if !ok || len(contents) != 2 {
		t.Fatalf("unexpected contents: %v", payload["contents"])
	}
	first, ok := contents[0].(map[string]any)
	if !ok || first["role"] != "user" {
		t.Fatalf("unexpected first role: %v", contents[0])
	}
	second, ok := contents[1].(map[string]any)
	if !ok || second["role"] != "model" {
		t.Fatalf("unexpected second role: %v", contents[1])
	}

	system, ok := payload["systemInstruction"].(map[string]any)
	if !ok {
		t.Fatalf("missing systemInstruction")
	}
	parts, ok := system["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("unexpected systemInstruction parts: %v", system["parts"])
	}
	part0, ok := parts[0].(map[string]any)
	if !ok || part0["text"] != "sys" {
		t.Fatalf("unexpected systemInstruction text: %v", parts[0])
	}
}

func TestGeminiSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "gemini")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`, "hello", "", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{"candidates":[]}`, "", "", nil)
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

func TestGeminiSpec_AuthStrategyOverride(t *testing.T) {
	spec := mustGetSpec(t, "gemini")
	strategy, ok := spec.AuthStrategyOverride(&auth.Credential{AuthType: auth.AuthTypeAPIKey, APIKey: "key"})
	if !ok {
		t.Fatalf("expected auth override")
	}
	apiKey, ok := strategy.(auth.ApiKeyHeaderStrategy)
	if !ok {
		t.Fatalf("expected ApiKeyHeaderStrategy, got %T", strategy)
	}
	if apiKey.HeaderName != "x-goog-api-key" || apiKey.Key != "key" {
		t.Fatalf("unexpected api key strategy: %+v", apiKey)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_ = apiKey.Apply(req)
	if req.Header.Get("x-goog-api-key") != "key" {
		t.Fatalf("expected x-goog-api-key header to be set")
	}
}

func TestOllamaSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "ollama")
	req := base.ChatRequest{
		Model:    "llama3",
		Messages: []base.Message{{Role: "user", Content: "hi"}},
		Stream:   true,
	}
	opts := base.BuildOptions{BaseURL: "http://localhost:11434/"}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "http://localhost:11434/api/chat" {
		t.Fatalf("unexpected url: %s", got)
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["model"] != "llama3" {
		t.Fatalf("unexpected model: %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
}

func TestOllamaSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "ollama")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"message":{"content":"hello"}}`, "hello", "", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{}`, "", "", nil)
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

func TestDifySpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "dify")
	req := base.ChatRequest{
		Messages: []base.Message{
			{Role: "assistant", Content: "ignored"},
			{Role: "user", Content: "question"},
			{Role: "assistant", Content: "tail"},
		},
		Stream:    true,
		SessionID: "conv-1",
	}
	opts := base.BuildOptions{BaseURL: "https://api.example.com/v1"}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://api.example.com/v1/chat-messages" {
		t.Fatalf("unexpected url: %s", got)
	}
	if httpReq.Header.Get("Accept") != "text/event-stream" {
		t.Fatalf("expected Accept header for stream")
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["query"] != "question" {
		t.Fatalf("unexpected query: %v", payload["query"])
	}
	if payload["response_mode"] != "streaming" {
		t.Fatalf("unexpected response_mode: %v", payload["response_mode"])
	}
	if payload["user"] != "sdk-user" {
		t.Fatalf("unexpected user: %v", payload["user"])
	}
	if payload["conversation_id"] != "conv-1" {
		t.Fatalf("unexpected conversation_id: %v", payload["conversation_id"])
	}
	if _, ok := payload["inputs"].(map[string]any); !ok {
		t.Fatalf("expected inputs map, got %T", payload["inputs"])
	}
}

func TestDifySpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "dify")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"answer":"hello","conversation_id":"conv","metadata":{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}}`,
			"hello", "conv", &base.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{}`, "", "", nil)
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

func TestPluginSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "plugin")
	req := base.ChatRequest{
		Messages:     []base.Message{{Role: "user", Content: "hi"}},
		Stream:       true,
		SessionID:    "sess-1",
		StartNewChat: true,
	}
	opts := base.BuildOptions{BaseURL: "http://plugin.example"}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "http://plugin.example/chat" {
		t.Fatalf("unexpected url: %s", got)
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["stream"] != true {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
	if payload["session_id"] != "sess-1" {
		t.Fatalf("unexpected session_id: %v", payload["session_id"])
	}
	if payload["startNewChat"] != true {
		t.Fatalf("unexpected startNewChat: %v", payload["startNewChat"])
	}
}

func TestPluginSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "plugin")
	t.Run("normal", func(t *testing.T) {
		assertParseResponse(t, spec, `{"text":"hello","sessionId":"sess"}`, "hello", "sess", nil)
	})
	t.Run("empty", func(t *testing.T) {
		assertParseResponse(t, spec, `{}`, "", "", nil)
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

func TestPluginSpec_AuthStrategyOverride(t *testing.T) {
	spec := mustGetSpec(t, "plugin")
	strategy, ok := spec.AuthStrategyOverride(&auth.Credential{AuthType: auth.AuthTypeAPIKey, APIKey: "key"})
	if !ok {
		t.Fatalf("expected auth override")
	}
	if _, ok := strategy.(auth.NoAuth); !ok {
		t.Fatalf("expected NoAuth, got %T", strategy)
	}
}

func TestQianfanAppSpec_BuildRequest(t *testing.T) {
	spec := mustGetSpec(t, "qianfan_app")
	req := base.ChatRequest{
		Model:     "app-abc123",
		Messages:  []base.Message{{Role: "system", Content: "ignored"}, {Role: "user", Content: "你好"}},
		Stream:    true,
		SessionID: "conv-xyz",
	}
	opts := base.BuildOptions{
		ExtraBody: map[string]any{
			"end_user_id": "u-1",
			"file_ids":    []string{"f1"},
		},
	}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	if got := httpReq.URL.String(); got != "https://qianfan.baidubce.com/v2/app/conversation/runs" {
		t.Fatalf("unexpected url: %s", got)
	}
	if ct := httpReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %s", ct)
	}

	payload := decodeBodyMap(t, httpReq)
	if payload["app_id"] != "app-abc123" {
		t.Fatalf("unexpected app_id: %v", payload["app_id"])
	}
	if payload["query"] != "你好" {
		t.Fatalf("unexpected query: %v", payload["query"])
	}
	if payload["stream"] != true {
		t.Fatalf("unexpected stream: %v", payload["stream"])
	}
	if payload["conversation_id"] != "conv-xyz" {
		t.Fatalf("unexpected conversation_id: %v", payload["conversation_id"])
	}
	if payload["end_user_id"] != "u-1" {
		t.Fatalf("unexpected end_user_id: %v", payload["end_user_id"])
	}
}

func TestQianfanAppSpec_BuildRequest_SkipsTestModel(t *testing.T) {
	spec := mustGetSpec(t, "qianfan_app")
	req := base.ChatRequest{
		Model:    "test",
		Messages: []base.Message{{Role: "user", Content: "hi"}},
	}
	opts := base.BuildOptions{}

	httpReq, err := spec.BuildRequest(context.Background(), opts, req)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}
	payload := decodeBodyMap(t, httpReq)
	if _, ok := payload["app_id"]; ok {
		t.Fatalf("expected app_id to be omitted for test model, got %v", payload["app_id"])
	}
	if payload["conversation_id"] != nil {
		t.Fatalf("expected no conversation_id when SessionID is empty, got %v", payload["conversation_id"])
	}
}

func TestQianfanAppSpec_ParseResponse(t *testing.T) {
	spec := mustGetSpec(t, "qianfan_app")
	t.Run("answer_with_conversation_id_and_usage", func(t *testing.T) {
		assertParseResponse(t, spec,
			`{"answer":"你好世界","conversation_id":"conv-001","usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`,
			"你好世界", "conv-001", &base.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15})
	})
	t.Run("answer_without_usage", func(t *testing.T) {
		assertParseResponse(t, spec, `{"answer":"ok","conversation_id":"c2"}`, "ok", "c2", nil)
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

func TestQianfanAppSpec_ParseStreamResponse(t *testing.T) {
	spec := mustGetSpec(t, "qianfan_app")
	streamSpec, ok := spec.(streaming.ProviderStreamSpec)
	if !ok {
		t.Fatalf("qianfan_app does not implement ProviderStreamSpec")
	}

	t.Run("incremental_text_with_conversation_id_and_usage", func(t *testing.T) {
		payload := "" +
			"data: {\"answer\":\"hello \",\"conversation_id\":\"conv-1\",\"is_completion\":false}\n\n" +
			"data: {\"answer\":\"world\",\"conversation_id\":\"conv-1\",\"is_completion\":false}\n\n" +
			"data: {\"answer\":\"\",\"conversation_id\":\"conv-1\",\"is_completion\":true,\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":7,\"total_tokens\":10}}\n\n"

		resp := newStreamResponse(payload)
		ch, err := streamSpec.ParseStreamResponse(resp)
		if err != nil {
			t.Fatalf("ParseStreamResponse error: %v", err)
		}

		chunks := collectChunks(ch)
		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d: %+v", len(chunks), chunks)
		}
		if chunks[0].Text != "hello " || chunks[0].SessionID != "conv-1" {
			t.Fatalf("unexpected first chunk: text=%q session=%q", chunks[0].Text, chunks[0].SessionID)
		}
		if chunks[1].Text != "world" || chunks[1].SessionID != "conv-1" {
			t.Fatalf("unexpected second chunk: text=%q session=%q", chunks[1].Text, chunks[1].SessionID)
		}
		if !chunks[2].Done {
			t.Fatalf("expected done on last chunk")
		}
		if chunks[2].SessionID != "conv-1" {
			t.Fatalf("expected SessionID on done chunk, got %q", chunks[2].SessionID)
		}
		if chunks[2].Usage == nil {
			t.Fatalf("expected usage on done chunk")
		}
		if chunks[2].Usage.PromptTokens != 3 || chunks[2].Usage.CompletionTokens != 7 || chunks[2].Usage.TotalTokens != 10 {
			t.Fatalf("unexpected usage: %+v", chunks[2].Usage)
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		_, err := streamSpec.ParseStreamResponse(nil)
		if err == nil {
			t.Fatalf("expected error for nil response")
		}
	})
}

func TestQianfanAppSpec_AuthStrategyOverride(t *testing.T) {
	spec := mustGetSpec(t, "qianfan_app")
	t.Run("apikey_to_bearer", func(t *testing.T) {
		strategy, ok := spec.AuthStrategyOverride(&auth.Credential{AuthType: auth.AuthTypeAPIKey, APIKey: "my-key"})
		if !ok {
			t.Fatalf("expected auth override")
		}
		bearer, ok := strategy.(auth.BearerTokenStrategy)
		if !ok {
			t.Fatalf("expected BearerTokenStrategy, got %T", strategy)
		}
		if bearer.Token != "my-key" {
			t.Fatalf("unexpected token: %q", bearer.Token)
		}
	})
	t.Run("nil_credential", func(t *testing.T) {
		strategy, ok := spec.AuthStrategyOverride(nil)
		if !ok {
			t.Fatalf("expected auth override for nil cred")
		}
		if _, ok := strategy.(auth.NoAuth); !ok {
			t.Fatalf("expected NoAuth for nil cred, got %T", strategy)
		}
	})
}
