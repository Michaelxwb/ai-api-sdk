package test

import (
	"fmt"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestNewSessionFromMultiRoundStatusContract(t *testing.T) {
	c := client.New()

	tests := []struct {
		name        string
		spec        generic.MultiRoundSpec
		wantStatus  string
		wantErr     bool
		wantSession bool
	}{
		{
			name:        "auto_confirmed creates session",
			spec:        buildMultiRoundSpecForContractAuto(),
			wantStatus:  "auto_confirmed",
			wantErr:     false,
			wantSession: true,
		},
		{
			name:        "pending_confirm creates session",
			spec:        buildMultiRoundSpecForContractPending(),
			wantStatus:  "pending_confirm",
			wantErr:     false,
			wantSession: true,
		},
		{
			name:        "failed returns error and no session",
			spec:        buildMultiRoundSpecForContractFailed(),
			wantStatus:  "failed",
			wantErr:     true,
			wantSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, inferred, err := c.NewSessionFromMultiRound(tt.spec)

			if tt.wantErr && err == nil {
				t.Fatal("error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if tt.wantSession && sess == nil {
				t.Fatal("session = nil, want non-nil")
			}
			if !tt.wantSession && sess != nil {
				t.Fatal("session != nil, want nil")
			}

			if inferred == nil || inferred.Report == nil {
				t.Fatal("inferred/report = nil, want non-nil")
			}
			if inferred.Report.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", inferred.Report.Status, tt.wantStatus)
			}
		})
	}
}

func TestNewSessionFromHTTPMultiRoundStatusContract(t *testing.T) {
	c := client.New()

	tests := []struct {
		name        string
		spec        generic.RawHTTPMultiRoundSpec
		wantStatus  string
		wantErr     bool
		wantSession bool
	}{
		{
			name:        "auto_confirmed creates session",
			spec:        buildHTTPMultiRoundSpecForContractAuto(),
			wantStatus:  "auto_confirmed",
			wantErr:     false,
			wantSession: true,
		},
		{
			name:        "pending_confirm creates session",
			spec:        buildHTTPMultiRoundSpecForContractPending(),
			wantStatus:  "pending_confirm",
			wantErr:     false,
			wantSession: true,
		},
		{
			name:        "failed returns error and no session",
			spec:        buildHTTPMultiRoundSpecForContractFailed(),
			wantStatus:  "failed",
			wantErr:     true,
			wantSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, inferred, err := c.NewSessionFromHTTPMultiRound(tt.spec)

			if tt.wantErr && err == nil {
				t.Fatal("error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if tt.wantSession && sess == nil {
				t.Fatal("session = nil, want non-nil")
			}
			if !tt.wantSession && sess != nil {
				t.Fatal("session != nil, want nil")
			}

			if inferred == nil || inferred.Report == nil {
				t.Fatal("inferred/report = nil, want non-nil")
			}
			if inferred.Report.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", inferred.Report.Status, tt.wantStatus)
			}
		})
	}
}

func buildMultiRoundSpecForContractAuto() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"$$$","session_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hello","session_id":"sess-1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"$$$","session_id":"sess-1"}`},
				Response: generic.RawPacket{Body: `{"text":"world","session_id":"sess-1"}`},
			},
		},
	}
}

func buildMultiRoundSpecForContractPending() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":"","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"sess-2"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"sess-2","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"sess-2"}`},
			},
		},
	}
}

func buildMultiRoundSpecForContractFailed() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{}`},
				Response: generic.RawPacket{Body: `{}`},
			},
			{
				Request:  generic.RawPacket{Body: `{}`},
				Response: generic.RawPacket{Body: `{}`},
			},
		},
	}
}

func buildHTTPMultiRoundSpecForContractAuto() generic.RawHTTPMultiRoundSpec {
	return generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request:  rawHTTPContractRequest(`{"prompt":"$$$","session_id":""}`),
				Response: rawHTTPContractResponse(`{"text":"hello","session_id":"sess-1"}`),
			},
			{
				Request:  rawHTTPContractRequest(`{"prompt":"$$$","session_id":"sess-1"}`),
				Response: rawHTTPContractResponse(`{"text":"world","session_id":"sess-1"}`),
			},
		},
	}
}

func buildHTTPMultiRoundSpecForContractPending() generic.RawHTTPMultiRoundSpec {
	return generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request:  rawHTTPContractRequest(`{"prompt":"hello","session_id":"","model":"demo"}`),
				Response: rawHTTPContractResponse(`{"text":"hi","session_id":"sess-2"}`),
			},
			{
				Request:  rawHTTPContractRequest(`{"prompt":"world","session_id":"sess-2","model":"demo"}`),
				Response: rawHTTPContractResponse(`{"text":"ok","session_id":"sess-2"}`),
			},
		},
	}
}

func buildHTTPMultiRoundSpecForContractFailed() generic.RawHTTPMultiRoundSpec {
	return generic.RawHTTPMultiRoundSpec{
		Model:   "remote_session",
		BaseURL: "https://mock.example.com/v1/chat/completions",
		Rounds: []generic.RawHTTPRound{
			{
				Request:  rawHTTPContractRequest(`{}`),
				Response: rawHTTPContractResponse(`{}`),
			},
			{
				Request:  rawHTTPContractRequest(`{}`),
				Response: rawHTTPContractResponse(`{}`),
			},
		},
	}
}

func rawHTTPContractRequest(body string) string {
	return fmt.Sprintf("POST /v1/chat/completions HTTP/1.1\nHost: mock.example.com\nContent-Type: application/json\n\n%s", body)
}

func rawHTTPContractResponse(body string) string {
	return fmt.Sprintf("HTTP/1.1 200 OK\nContent-Type: application/json\n\n%s", body)
}
