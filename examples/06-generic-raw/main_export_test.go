package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestBuildRequestJSONPayload_FromInferred(t *testing.T) {
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

	chain := []generic.ChainField{
		{
			Placeholder:    "$$$PARENT_REQ_ID$$$",
			ResponsePath:   "req_id",
			ExtractOnEvent: "",
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
					"parent_req_id": "$$$WRONG_PARENT_REQ_ID$$$",
					"model":         "demo",
					"messages": map[string]any{
						"0": map[string]any{
							"content": "{{input}}",
						},
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

	payload, err := buildRequestJSONPayload(rawSpec, inferred)
	if err != nil {
		t.Fatalf("buildRequestJSONPayload failed: %v", err)
	}

	specs, ok := payload["remote_session"]
	if !ok || len(specs) != 1 {
		t.Fatalf("remote_session entry missing or invalid length: %v", payload)
	}

	got := specs[0]
	if got.Model != "remote_session" {
		t.Fatalf("model = %q, want remote_session", got.Model)
	}
	if got.BaseURL != rawSpec.BaseURL {
		t.Fatalf("base_url = %q, want %q", got.BaseURL, rawSpec.BaseURL)
	}
	if got.Response != rawSpec.Rounds[0].Response {
		t.Fatalf("response not from first round")
	}
	if !reflect.DeepEqual(got.ChainFields, chain) {
		t.Fatalf("chain_fields mismatch: got=%+v want=%+v", got.ChainFields, chain)
	}

	req := got.Request
	if !strings.Contains(req, "POST /v1/chat/completions HTTP/1.1") {
		t.Fatalf("request line mismatch: %q", req)
	}
	if !strings.Contains(req, `"prompt": "$$$"`) {
		t.Fatalf("input placeholder not converted: %q", req)
	}
	if !strings.Contains(req, `"session_id": "$$$SESSION_ID$$$"`) {
		t.Fatalf("session placeholder not converted: %q", req)
	}
	if !strings.Contains(req, `"parent_req_id": "$$$PARENT_REQ_ID$$$"`) {
		t.Fatalf("chain placeholder missing: %q", req)
	}
	if strings.Contains(req, "$$$WRONG_PARENT_REQ_ID$$$") {
		t.Fatalf("parent_req_id placeholder should align with chain_fields: %q", req)
	}
	if !strings.Contains(req, `"req_id": "{{uuid}}"`) {
		t.Fatalf("req_id uuid placeholder missing: %q", req)
	}
	if !strings.Contains(req, `"messages": [`) {
		t.Fatalf("indexed map should be normalized to array: %q", req)
	}
	if strings.Contains(req, "{{input}}") || strings.Contains(req, "{{session_id}}") {
		t.Fatalf("template placeholders should be converted to raw placeholders: %q", req)
	}
	if strings.Contains(strings.ToLower(req), "content-length:") {
		t.Fatalf("request headers should not include content-length: %q", req)
	}
}

func TestBuildRequestJSONPayload_UsesInferredModeAsPayloadKey(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request: "POST /v1/chat/completions HTTP/1.1\n" +
					"Host: mock.example.com\n" +
					"Content-Type: application/json\n\n" +
					`{"messages":[{"role":"user","content":"$$$"}],"model":"demo"}`,
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
					"messages": []any{
						map[string]any{"role": "user", "content": "{{input}}"},
					},
					"model": "demo",
				},
			},
			Conversation: generic.ConversationProfile{Mode: "local_history"},
		},
	}

	payload, err := buildRequestJSONPayload(rawSpec, inferred)
	if err != nil {
		t.Fatalf("buildRequestJSONPayload failed: %v", err)
	}

	if _, ok := payload["remote_session"]; ok {
		t.Fatalf("payload should not contain remote_session key in local_history mode: %v", payload)
	}
	specs, ok := payload["local_history"]
	if !ok || len(specs) != 1 {
		t.Fatalf("local_history entry missing or invalid length: %v", payload)
	}
	if specs[0].Model != "local_history" {
		t.Fatalf("model = %q, want local_history", specs[0].Model)
	}
}

func TestBuildRequestJSONPayload_RequiresInferredProfile(t *testing.T) {
	rawSpec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request:  "POST /v1/chat/completions HTTP/1.1\nContent-Type: application/json\n\n{}",
				Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{}",
			},
		},
	}

	_, err := buildRequestJSONPayload(rawSpec, &generic.InferredIntegration{})
	if err == nil {
		t.Fatal("expected error for empty inferred profile, got nil")
	}
}

func TestLoadFirstSpecByModeFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "new_request_json.json")
	data := `{
  "remote_session": [
    {
      "model": "remote_session",
      "base_url": "https://mock.example.com/v1/chat/completions",
      "request": "POST /v1/chat/completions HTTP/1.1\n\n{}",
      "response": "HTTP/1.1 200 OK\n\n{}"
    }
  ],
  "local_history": [
    {
      "model": "local_history",
      "base_url": "https://mock.example.com/v1/messages",
      "request": "POST /v1/messages HTTP/1.1\n\n{}",
      "response": "HTTP/1.1 200 OK\n\n{}"
    }
  ]
}
`
	if err := os.WriteFile(jsonPath, []byte(data), 0644); err != nil {
		t.Fatalf("write temp export file failed: %v", err)
	}

	spec, err := loadFirstSpecByModeFromFile(jsonPath, "local_history")
	if err != nil {
		t.Fatalf("loadFirstSpecByModeFromFile failed: %v", err)
	}

	if spec.Model != "local_history" {
		t.Fatalf("model = %q, want local_history", spec.Model)
	}
	if spec.BaseURL != "https://mock.example.com/v1/messages" {
		t.Fatalf("base_url = %q", spec.BaseURL)
	}
	if !strings.Contains(spec.Request, "POST /v1/messages HTTP/1.1") {
		t.Fatalf("request line missing in request: %q", spec.Request)
	}
}
