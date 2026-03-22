package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestParseHTTPMultiRoundSpec_HappyPath2Rounds(t *testing.T) {
	spec := generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request: "POST /v1/chat/completions HTTP/1.1\n" +
					"Host: mock.example.com\n" +
					"Authorization: Bearer <MOCK_TOKEN>\n" +
					"Content-Type: application/json\n\n" +
					"{\n  \"prompt\": \"$$$\",\n  \"session_id\": \"\",\n  \"model\": \"demo\"\n}",
				Response: "HTTP/1.1 200 OK\n" +
					"Content-Type: application/json\n\n" +
					"{\"content\":\"hi\",\"session_id\":\"s1\",\"message_id\":\"m1\"}",
			},
			{
				Request: "POST /v1/chat/completions HTTP/1.1\n" +
					"Host: mock.example.com\n" +
					"Authorization: Bearer <MOCK_TOKEN>\n" +
					"Content-Type: application/json\n\n" +
					"{\"prompt\":\"$$$\",\"session_id\":\"s1\",\"model\":\"demo\"}",
				Response: "HTTP/1.1 200 OK\n" +
					"Content-Type: application/json\n\n" +
					"{\"content\":\"ok\",\"session_id\":\"s1\",\"message_id\":\"m2\"}",
			},
		},
	}

	got, err := generic.ParseHTTPMultiRoundSpec(spec)
	if err != nil {
		t.Fatalf("ParseHTTPMultiRoundSpec failed: %v", err)
	}

	if got.URL != spec.BaseURL {
		t.Fatalf("url = %q, want %q", got.URL, spec.BaseURL)
	}
	if got.Conversation.Mode != "remote_session" {
		t.Fatalf("conversation.mode = %q, want remote_session", got.Conversation.Mode)
	}
	if len(got.Rounds) != 2 {
		t.Fatalf("round count = %d, want 2", len(got.Rounds))
	}

	if got.Rounds[0].Request.Headers["Authorization"] != "Bearer <MOCK_TOKEN>" {
		t.Fatalf("request header Authorization mismatch: %q", got.Rounds[0].Request.Headers["Authorization"])
	}
	if got.Rounds[0].Request.Body != `{"model":"demo","prompt":"$$$","session_id":""}` {
		t.Fatalf("round0 request body = %q", got.Rounds[0].Request.Body)
	}
	if got.Rounds[0].Response.Headers["Content-Type"] != "application/json" {
		t.Fatalf("response Content-Type = %q", got.Rounds[0].Response.Headers["Content-Type"])
	}
	if got.Rounds[0].Response.Body != `{"content":"hi","session_id":"s1","message_id":"m1"}` {
		t.Fatalf("round0 response body = %q", got.Rounds[0].Response.Body)
	}
}

func TestParseHTTPMultiRoundSpec_EdgeRoundCount3And5(t *testing.T) {
	for _, n := range []int{3, 5} {
		t.Run(fmt.Sprintf("rounds_%d", n), func(t *testing.T) {
			rounds := make([]generic.RawHTTPRound, 0, n)
			for i := 0; i < n; i++ {
				rounds = append(rounds, generic.RawHTTPRound{
					Request:  fmt.Sprintf("POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{\"prompt\":\"$$$\",\"session_id\":\"s%d\"}", i),
					Response: fmt.Sprintf("HTTP/1.1 200 OK\nContent-Type: application/json\n\n{\"content\":\"r%d\",\"session_id\":\"s%d\"}", i, i),
				})
			}

			spec := generic.RawHTTPMultiRoundSpec{
				Model:   "local_history",
				BaseURL: "https://mock.example.com/v1/chat",
				Rounds:  rounds,
			}
			got, err := generic.ParseHTTPMultiRoundSpec(spec)
			if err != nil {
				t.Fatalf("ParseHTTPMultiRoundSpec failed: %v", err)
			}
			if got.Conversation.Mode != "local_history" {
				t.Fatalf("conversation.mode = %q, want local_history", got.Conversation.Mode)
			}
			if len(got.Rounds) != n {
				t.Fatalf("round count = %d, want %d", len(got.Rounds), n)
			}
		})
	}
}

func TestParseHTTPMultiRoundSpec_ErrorCases(t *testing.T) {
	validRound := generic.RawHTTPRound{
		Request:  "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{\"prompt\":\"$$$\"}",
		Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{\"content\":\"ok\"}",
	}

	tests := []struct {
		name    string
		spec    generic.RawHTTPMultiRoundSpec
		wantErr string
	}{
		{
			name:    "empty base_url",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", Rounds: []generic.RawHTTPRound{validRound, validRound}},
			wantErr: "base_url is required",
		},
		{
			name:    "empty rounds",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com"},
			wantErr: "rounds is required",
		},
		{
			name:    "rounds less than 2",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com", Rounds: []generic.RawHTTPRound{validRound}},
			wantErr: "rounds must contain 2-5 items",
		},
		{
			name:    "rounds greater than 5",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com", Rounds: []generic.RawHTTPRound{validRound, validRound, validRound, validRound, validRound, validRound}},
			wantErr: "rounds must contain 2-5 items",
		},
		{
			name:    "empty request",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com", Rounds: []generic.RawHTTPRound{{Request: "", Response: validRound.Response}, validRound}},
			wantErr: "rounds[0].request is required",
		},
		{
			name:    "empty response",
			spec:    generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com", Rounds: []generic.RawHTTPRound{{Request: validRound.Request, Response: ""}, validRound}},
			wantErr: "rounds[0].response is required",
		},
		{
			name: "request body non json",
			spec: generic.RawHTTPMultiRoundSpec{Model: "remote_session", BaseURL: "https://mock.example.com", Rounds: []generic.RawHTTPRound{{
				Request:  "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{\"prompt\":",
				Response: validRound.Response,
			}, validRound}},
			wantErr: "request body is not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generic.ParseHTTPMultiRoundSpec(tt.spec)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.HasPrefix(err.Error(), "generic:") {
				t.Fatalf("error prefix = %q, want generic:", err.Error())
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseHTTPMultiRoundSpec_ResponseBodyExtraction(t *testing.T) {
	baseRoundRequest := "POST /v1/chat HTTP/1.1\nContent-Type: application/json\n\n{\"prompt\":\"$$$\"}"
	secondRound := generic.RawHTTPRound{
		Request:  baseRoundRequest,
		Response: "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{\"content\":\"ok\"}",
	}

	tests := []struct {
		name         string
		firstResp    string
		wantBodyPath string
		wantBodyVal  string
		wantErr      string
	}{
		{
			name: "sse prefers event complete frame",
			firstResp: "HTTP/1.1 200 OK\n" +
				"Content-Type: text/event-stream\n\n" +
				"event: message\n" +
				"data: {\"event\":\"message\",\"delta\":\"hel\"}\n\n" +
				"event: complete\n" +
				"data: {\"event\":\"complete\",\"status\":\"complete\",\"session_id\":\"s1\",\"message_id\":\"m1\"}\n\n" +
				"event: message\n" +
				"data: {\"event\":\"message\",\"delta\":\"ignored\"}\n",
			wantBodyPath: "message_id",
			wantBodyVal:  "m1",
		},
		{
			name: "sse without complete uses last parseable object",
			firstResp: "HTTP/1.1 200 OK\n" +
				"Content-Type: text/event-stream\n\n" +
				"event: message\n" +
				"data: {\"event\":\"message\",\"delta\":\"first\"}\n\n" +
				"event: message\n" +
				"data: not-json\n\n" +
				"event: message\n" +
				"data: {\"event\":\"message\",\"delta\":\"last\"}\n",
			wantBodyPath: "delta",
			wantBodyVal:  "last",
		},
		{
			name: "non sse json remains usable",
			firstResp: "HTTP/1.1 200 OK\n" +
				"Content-Type: application/json\n\n" +
				"{\n  \"content\": \"hello\",\n  \"session_id\": \"s1\"\n}",
			wantBodyPath: "session_id",
			wantBodyVal:  "s1",
		},
		{
			name: "sse without parseable json returns error",
			firstResp: "HTTP/1.1 200 OK\n" +
				"Content-Type: text/event-stream\n\n" +
				"event: message\n" +
				"data: not-json\n\n" +
				"data: [DONE]\n",
			wantErr: "no parseable JSON object data frame",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := generic.RawHTTPMultiRoundSpec{
				Model:   "remote_session",
				BaseURL: "https://mock.example.com/v1/chat",
				Rounds: []generic.RawHTTPRound{
					{
						Request:  baseRoundRequest,
						Response: tt.firstResp,
					},
					secondRound,
				},
			}

			got, err := generic.ParseHTTPMultiRoundSpec(spec)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.HasPrefix(err.Error(), "generic:") {
					t.Fatalf("error prefix = %q, want generic:", err.Error())
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHTTPMultiRoundSpec failed: %v", err)
			}

			respBody := got.Rounds[0].Response.Body
			var obj map[string]any
			if unmarshalErr := json.Unmarshal([]byte(respBody), &obj); unmarshalErr != nil {
				t.Fatalf("round0 response body not json object: %v; body=%q", unmarshalErr, respBody)
			}

			gotVal, ok := obj[tt.wantBodyPath]
			if !ok {
				t.Fatalf("response body missing key %q: %q", tt.wantBodyPath, respBody)
			}
			if fmt.Sprintf("%v", gotVal) != tt.wantBodyVal {
				t.Fatalf("response body key %q = %v, want %q; body=%q", tt.wantBodyPath, gotVal, tt.wantBodyVal, respBody)
			}
		})
	}
}
