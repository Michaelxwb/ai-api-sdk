package test

import (
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestExportToHTTPSpec_HappyPath(t *testing.T) {
	chain := []generic.ChainField{
		{
			Placeholder:  "$$$PARENT_REQ_ID$$$",
			ResponsePath: "req_id",
		},
	}
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request: "POST /v1/chat/completions HTTP/1.1\n" +
					"Host: mock.example.com\n" +
					"Authorization: Bearer test-token\n" +
					"Content-Type: application/json\n" +
					"Content-Length: 999\n\n" +
					`{"prompt":"$$$","session_id":"s1","req_id":"m0","parent_req_id":"m1","model":"demo"}`,
				Response: "HTTP/1.1 200 OK\n" +
					"Content-Type: text/event-stream\n\n" +
					"event: complete\n" +
					`data: {"session_id":"s1","req_id":"m2"}`,
			},
			{
				Request:  "POST /v1/chat/completions HTTP/1.1\nContent-Type: application/json\n\n{}",
				Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{}",
			},
		},
	}

	inferred := &generic.InferredIntegration{
		BaseURL: "https://mock.example.com",
		Profile: &generic.GenericProfile{
			Request: generic.RequestProfile{
				Method: "POST",
				Path:   "/v1/chat/completions",
				BodyTemplate: map[string]any{
					"prompt":        "{{input}}",
					"session_id":    "{{session_id}}",
					"parent_req_id": "$$$WRONG$$$",
					"model":         "demo",
					"messages": map[string]any{
						"0": map[string]any{"content": "{{input}}"},
					},
				},
			},
			Response: generic.ResponseProfile{
				Stream: generic.StreamProfile{
					ChainFields: chain,
				},
			},
			Conversation: generic.ConversationProfile{Mode: "remote_session"},
		},
	}

	spec, err := generic.ExportToHTTPSpec(inferred, rawSpec)
	if err != nil {
		t.Fatalf("ExportToHTTPSpec failed: %v", err)
	}

	if spec.Model != "remote_session" {
		t.Errorf("model = %q, want remote_session", spec.Model)
	}
	if spec.BaseURL != rawSpec.BaseURL {
		t.Errorf("base_url = %q, want %q", spec.BaseURL, rawSpec.BaseURL)
	}
	if spec.Response != rawSpec.Rounds[0].Response {
		t.Error("response should be from first round")
	}

	// ChainFields should be preserved (aligned from profile)
	if len(spec.ChainFields) == 0 {
		t.Fatal("chain_fields should not be empty")
	}
	if spec.ChainFields[0].Placeholder != "$$$PARENT_REQ_ID$$$" {
		t.Errorf("chain placeholder = %q, want $$$PARENT_REQ_ID$$$", spec.ChainFields[0].Placeholder)
	}

	// Request text checks
	req := spec.Request
	if !strings.Contains(req, "POST /v1/chat/completions HTTP/1.1") {
		t.Errorf("request line mismatch: %q", req)
	}
	if !strings.Contains(req, `"prompt": "$$$"`) {
		t.Errorf("input placeholder not converted: %q", req)
	}
	if !strings.Contains(req, `"session_id": "$$$SESSION_ID$$$"`) {
		t.Errorf("session placeholder not converted: %q", req)
	}
	if !strings.Contains(req, `"parent_req_id": "$$$PARENT_REQ_ID$$$"`) {
		t.Errorf("chain placeholder not aligned: %q", req)
	}
	if strings.Contains(req, "$$$WRONG$$$") {
		t.Errorf("old wrong placeholder should be replaced: %q", req)
	}
	if !strings.Contains(req, `"req_id": "{{uuid}}"`) {
		t.Errorf("req_id uuid placeholder missing: %q", req)
	}
	if !strings.Contains(req, `"messages": [`) {
		t.Errorf("indexed map should be normalized to array: %q", req)
	}
	if strings.Contains(req, "{{input}}") || strings.Contains(req, "{{session_id}}") {
		t.Errorf("template placeholders should be converted: %q", req)
	}
	if strings.Contains(strings.ToLower(req), "content-length:") {
		t.Errorf("content-length header should be stripped: %q", req)
	}
}

func TestExportToHTTPSpec_NilInferred(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Rounds: []generic.RawHTTPRound{{Request: "POST / HTTP/1.1\n\n{}", Response: "{}"}},
	}
	_, err := generic.ExportToHTTPSpec(nil, rawSpec)
	if err == nil {
		t.Fatal("expected error for nil inferred")
	}
	if !strings.Contains(err.Error(), "generic:") {
		t.Errorf("error should have generic: prefix: %v", err)
	}
}

func TestExportToHTTPSpec_NilProfile(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Rounds: []generic.RawHTTPRound{{Request: "POST / HTTP/1.1\n\n{}", Response: "{}"}},
	}
	_, err := generic.ExportToHTTPSpec(&generic.InferredIntegration{}, rawSpec)
	if err == nil {
		t.Fatal("expected error for nil profile")
	}
}

func TestExportToHTTPSpec_EmptyRounds(t *testing.T) {
	inferred := &generic.InferredIntegration{
		Profile: &generic.GenericProfile{},
	}
	_, err := generic.ExportToHTTPSpec(inferred, generic.RawHTTPMultiRoundSpec{})
	if err == nil {
		t.Fatal("expected error for empty rounds")
	}
	if !strings.Contains(err.Error(), "rounds") {
		t.Errorf("error should mention rounds: %v", err)
	}
}

func TestExportToHTTPSpec_InferredModeOverridesRawModel(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://example.com/api",
		Rounds: []generic.RawHTTPRound{
			{
				Request: "POST /api HTTP/1.1\n" +
					"Content-Type: application/json\n\n" +
					`{"messages":[{"role":"user","content":"$$$"}]}`,
				Response: "HTTP/1.1 200 OK\n\n{}",
			},
		},
	}
	inferred := &generic.InferredIntegration{
		BaseURL: "https://example.com",
		Profile: &generic.GenericProfile{
			Request: generic.RequestProfile{
				Method: "POST",
				Path:   "/api",
				BodyTemplate: map[string]any{
					"messages": []any{
						map[string]any{"role": "user", "content": "{{input}}"},
					},
				},
			},
			Conversation: generic.ConversationProfile{Mode: "local_history"},
		},
	}

	spec, err := generic.ExportToHTTPSpec(inferred, rawSpec)
	if err != nil {
		t.Fatalf("ExportToHTTPSpec failed: %v", err)
	}
	if spec.Model != "local_history" {
		t.Errorf("model = %q, want local_history (inferred profile should override)", spec.Model)
	}
}

func TestExportToHTTPSpec_RoundTrip(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://api.example.com/v1/chat",
		Rounds: []generic.RawHTTPRound{
			{
				Request: "POST /v1/chat HTTP/1.1\n" +
					"Authorization: Bearer tok\n" +
					"Content-Type: application/json\n\n" +
					`{"prompt":"$$$","session_id":"s1"}`,
				Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
					`{"content":"hello","session_id":"s1"}`,
			},
			{
				Request: "POST /v1/chat HTTP/1.1\n" +
					"Content-Type: application/json\n\n" +
					`{"prompt":"$$$","session_id":"s1"}`,
				Response: "HTTP/1.1 200 OK\n\n{}",
			},
		},
	}
	inferred := &generic.InferredIntegration{
		BaseURL: "https://api.example.com",
		Profile: &generic.GenericProfile{
			Request: generic.RequestProfile{
				Method:       "POST",
				Path:         "/v1/chat",
				BodyTemplate: map[string]any{"prompt": "{{input}}", "session_id": "{{session_id}}"},
			},
			Conversation: generic.ConversationProfile{Mode: "remote_session"},
		},
	}

	// Export
	spec, err := generic.ExportToHTTPSpec(inferred, rawSpec)
	if err != nil {
		t.Fatalf("ExportToHTTPSpec failed: %v", err)
	}

	// Round-trip: parse back
	rawIntSpec, err := generic.ParseHTTPSpec(*spec)
	if err != nil {
		t.Fatalf("ParseHTTPSpec round-trip failed: %v", err)
	}
	compiled, err := generic.ParseRawIntegration(rawIntSpec)
	if err != nil {
		t.Fatalf("ParseRawIntegration round-trip failed: %v", err)
	}
	if compiled.Profile.Conversation.Mode != "remote_session" {
		t.Errorf("round-trip mode = %q, want remote_session", compiled.Profile.Conversation.Mode)
	}
}
