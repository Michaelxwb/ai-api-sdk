package test

import (
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func TestQuickGenericHTTPSpec_HappyPath(t *testing.T) {
	cli := client.New()

	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     "https://api.example.com/v1/chat",
		SessionMode: "remote_session",
		APIKey:      "test-token",
		Request: "POST /v1/chat HTTP/1.1\n" +
			"Content-Type: application/json\n\n" +
			`{"prompt":"$$$","session_id":"$$$SESSION_ID$$$"}`,
		Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
			`{"content":"hello","session_id":"s1"}`,
	})
	if err != nil {
		t.Fatalf("Quick with generic HTTPSpec failed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}

func TestQuickGenericHTTPSpec_WithChainFields(t *testing.T) {
	cli := client.New()

	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     "https://api.example.com/v1/chat",
		SessionMode: "remote_session",
		APIKey:      "test-token",
		Request: "POST /v1/chat HTTP/1.1\n" +
			"Content-Type: application/json\n\n" +
			`{"prompt":"$$$","session_id":"$$$SESSION_ID$$$","parent_msg":"$$$PARENT_MSG$$$"}`,
		Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" +
			`{"content":"hello","session_id":"s1","msg_id":"m1"}`,
		ChainFields: []generic.ChainField{
			{
				Placeholder:  "$$$PARENT_MSG$$$",
				ResponsePath: "msg_id",
			},
		},
	})
	if err != nil {
		t.Fatalf("Quick with ChainFields failed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}

func TestQuickGenericHTTPSpec_MissingSessionMode(t *testing.T) {
	cli := client.New()

	_, err := cli.Quick(client.ProviderConfig{
		Provider: "generic",
		BaseURL:  "https://api.example.com/v1/chat",
		APIKey:   "test-token",
		Request:  "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{}",
	})
	if err == nil {
		t.Fatal("expected error for generic without SessionMode")
	}
	if !strings.Contains(err.Error(), "requires explicit SessionMode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQuickGenericHTTPSpec_MissingBaseURL(t *testing.T) {
	cli := client.New()

	_, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		SessionMode: "remote_session",
		APIKey:      "test-token",
		Request:     "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{}",
	})
	if err == nil {
		t.Fatal("expected error for generic without BaseURL")
	}
	if !strings.Contains(err.Error(), "requires BaseURL") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQuickGenericHTTPSpec_NoRequestField(t *testing.T) {
	cli := client.New()

	// Without Request field, generic provider falls through to normal path.
	// It should still create a session (just with empty profile).
	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     "https://api.example.com",
		SessionMode: "remote_session",
		APIKey:      "test-token",
	})
	if err != nil {
		t.Fatalf("Quick generic without Request should succeed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}

func TestQuickGenericHTTPSpec_EmptyResponse(t *testing.T) {
	cli := client.New()

	// Empty Response is valid — results in non-streaming session.
	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     "https://api.example.com/v1/chat",
		SessionMode: "remote_session",
		APIKey:      "test-token",
		Request: "POST /v1/chat HTTP/1.1\n" +
			"Content-Type: application/json\n\n" +
			`{"prompt":"$$$"}`,
	})
	if err != nil {
		t.Fatalf("Quick with empty Response failed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}

func TestQuickGenericHTTPSpec_CredentialFromRequest(t *testing.T) {
	cli := client.New()

	// When APIKey is empty, credential should be extracted from Request headers.
	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     "https://api.example.com/v1/chat",
		SessionMode: "remote_session",
		Request: "POST /v1/chat HTTP/1.1\n" +
			"Authorization: Bearer extracted-token\n" +
			"Content-Type: application/json\n\n" +
			`{"prompt":"$$$"}`,
	})
	if err != nil {
		t.Fatalf("Quick with credential from Request failed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}

func TestRawReasoning_HappyPath(t *testing.T) {
	cli := client.New()

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

	spec, err := cli.RawReasoning(rawSpec)
	if err != nil {
		t.Fatalf("RawReasoning failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil RawHTTPSpec")
	}
	if spec.BaseURL == "" {
		t.Error("expected non-empty BaseURL")
	}
	if spec.Request == "" {
		t.Error("expected non-empty Request")
	}
	if spec.Model == "" {
		t.Error("expected non-empty Model")
	}
}

func TestRawReasoning_ThenQuick(t *testing.T) {
	cli := client.New()

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

	spec, err := cli.RawReasoning(rawSpec)
	if err != nil {
		t.Fatalf("RawReasoning failed: %v", err)
	}

	// Round-trip: feed exported spec into Quick
	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     spec.BaseURL,
		SessionMode: spec.Model,
		Request:     spec.Request,
		Response:    spec.Response,
		ChainFields: spec.ChainFields,
	})
	if err != nil {
		t.Fatalf("Quick from RawReasoning spec failed: %v", err)
	}
	if qs == nil {
		t.Fatal("expected non-nil QuickSession")
	}
}
