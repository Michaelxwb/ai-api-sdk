package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestInferSessionIDField(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"sess-123"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"sess-123"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"sess-123"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	if result.Report == nil {
		t.Fatal("report is nil")
	}

	foundSessionID := false
	for _, f := range result.Report.Fields {
		if f.Class == "session_id" && f.RequestPath == "session_id" {
			foundSessionID = true
			if f.Placeholder != "{{session_id}}" {
				t.Errorf("session_id placeholder = %q, want {{session_id}}", f.Placeholder)
			}
			if f.ResponsePath != "session_id" {
				t.Errorf("session_id response_path = %q, want session_id", f.ResponsePath)
			}
		}
	}
	if !foundSessionID {
		t.Error("session_id field not detected")
		for _, f := range result.Report.Fields {
			t.Logf("  field: path=%q class=%q confidence=%.2f reason=%q", f.RequestPath, f.Class, f.Confidence, f.Reason)
		}
	}

	foundInput := false
	for _, f := range result.Report.Fields {
		if f.Class == "input" && f.RequestPath == "prompt" {
			foundInput = true
		}
	}
	if !foundInput {
		t.Error("input field not detected for 'prompt'")
	}
}

func TestInferChainField(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":"","parent_message_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1","message_id":"m1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"s1","parent_message_id":"m1"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1","message_id":"m2"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	foundChain := false
	for _, f := range result.Report.Fields {
		if f.Class == "chain" && f.RequestPath == "parent_message_id" {
			foundChain = true
			if f.ResponsePath != "message_id" {
				t.Errorf("chain response_path = %q, want message_id", f.ResponsePath)
			}
		}
	}
	if !foundChain {
		t.Error("chain field not detected for 'parent_message_id'")
		for _, f := range result.Report.Fields {
			t.Logf("  field: path=%q class=%q confidence=%.2f reason=%q", f.RequestPath, f.Class, f.Confidence, f.Reason)
		}
	}
}

func TestInferStaticField(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","model":"gpt-4","session_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","model":"gpt-4","session_id":"s1"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	foundStatic := false
	for _, f := range result.Report.Fields {
		if f.Class == "static" && f.RequestPath == "model" {
			foundStatic = true
		}
	}
	if !foundStatic {
		t.Error("static field not detected for 'model'")
	}
}

func TestInferDynamicField(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","timestamp":"1001","session_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","timestamp":"1002","session_id":"s1"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	foundDynamic := false
	for _, f := range result.Report.Fields {
		if f.RequestPath == "timestamp" && (f.Class == "dynamic" || f.Class == "input") {
			foundDynamic = true
		}
	}
	if !foundDynamic {
		t.Error("dynamic/input field not detected for 'timestamp'")
	}
}

func TestInferFlowSpecMeta(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello"}`},
				Response: generic.RawPacket{Body: `{"text":"hi"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world"}`},
				Response: generic.RawPacket{Body: `{"text":"ok"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	if result.Report.FlowSpecMeta.Version != "v1alpha1" {
		t.Errorf("FlowSpecMeta.Version = %q, want v1alpha1", result.Report.FlowSpecMeta.Version)
	}
	if result.Report.FlowSpecMeta.Source != "MultiRoundSpec" {
		t.Errorf("FlowSpecMeta.Source = %q, want MultiRoundSpec", result.Report.FlowSpecMeta.Source)
	}
}

func TestInferConfidenceAndStatus(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"s1"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	if result.Report.Status != "auto_confirmed" && result.Report.Status != "pending_confirm" {
		t.Errorf("status = %q, want auto_confirmed or pending_confirm", result.Report.Status)
	}
}

func TestInferTwoRound(t *testing.T) {
	spec := generic.TwoRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
	}
	spec.Round1.Request = generic.RawPacket{Body: `{"prompt":"hello","session_id":""}`}
	spec.Round1.Response = generic.RawPacket{Body: `{"text":"hi","session_id":"s1"}`}
	spec.Round2.Request = generic.RawPacket{Body: `{"prompt":"world","session_id":"s1"}`}
	spec.Round2.Response = generic.RawPacket{Body: `{"text":"ok","session_id":"s1"}`}

	result, err := generic.InferIntegrationByTwoRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByTwoRound failed: %v", err)
	}

	if result.Report == nil {
		t.Fatal("report is nil")
	}
	if result.Report.FlowSpecMeta.Version != "v1alpha1" {
		t.Errorf("FlowSpecMeta.Version = %q, want v1alpha1", result.Report.FlowSpecMeta.Version)
	}
}

func TestInferTwoRoundMissingPacket(t *testing.T) {
	spec := generic.TwoRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
	}
	spec.Round1.Request = generic.RawPacket{Body: `{"prompt":"hello"}`}

	_, err := generic.InferIntegrationByTwoRound(spec)
	if err == nil {
		t.Fatal("expected error for missing packet")
	}
	if !strings.Contains(err.Error(), "4 packets") {
		t.Errorf("error = %q, want containing '4 packets'", err.Error())
	}
}

func TestInferReproducibility(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":"","parent_id":""}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1","msg_id":"m1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"s1","parent_id":"m1"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1","msg_id":"m2"}`},
			},
		},
	}

	r1, err1 := generic.InferIntegrationByMultiRound(spec)
	r2, err2 := generic.InferIntegrationByMultiRound(spec)

	if err1 != nil || err2 != nil {
		t.Fatalf("inference failed: err1=%v err2=%v", err1, err2)
	}

	j1, _ := json.Marshal(r1.Report.Fields)
	j2, _ := json.Marshal(r2.Report.Fields)
	if string(j1) != string(j2) {
		t.Errorf("inference not reproducible:\n  run1: %s\n  run2: %s", j1, j2)
	}
}

func TestInferInvalidJSON(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/chat",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `not json`},
				Response: generic.RawPacket{Body: `{"text":"hi"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world"}`},
				Response: generic.RawPacket{Body: `{"text":"ok"}`},
			},
		},
	}

	_, err := generic.InferIntegrationByMultiRound(spec)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error = %q, want containing 'not valid JSON'", err.Error())
	}
}

func TestInferProfileHasCorrectPath(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL:          "https://example.com/api/v1/chat/completion",
		Conversation: generic.ConversationProfile{Mode: "remote_session"},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello"}`},
				Response: generic.RawPacket{Body: `{"text":"hi"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world"}`},
				Response: generic.RawPacket{Body: `{"text":"ok"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result.Profile.Request.Path != "/api/v1/chat/completion" {
		t.Errorf("path = %q, want /api/v1/chat/completion", result.Profile.Request.Path)
	}
	if result.BaseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want https://example.com", result.BaseURL)
	}
}

// TestInferValidateMultiRoundSpec_ViaPublicAPI tests validation through the exported function.
func TestInferValidateMultiRoundSpec_ViaPublicAPI(t *testing.T) {
	tests := []struct {
		name    string
		spec    generic.MultiRoundSpec
		wantErr string
	}{
		{
			name:    "missing mode",
			spec:    generic.MultiRoundSpec{Rounds: make([]generic.RoundPair, 2)},
			wantErr: "conversation mode is required",
		},
		{
			name: "invalid mode",
			spec: generic.MultiRoundSpec{
				Conversation: generic.ConversationProfile{Mode: "invalid"},
				Rounds:       make([]generic.RoundPair, 2),
			},
			wantErr: "invalid conversation mode",
		},
		{
			name: "too few rounds",
			spec: generic.MultiRoundSpec{
				Conversation: generic.ConversationProfile{Mode: "remote_session"},
				Rounds:       make([]generic.RoundPair, 1),
			},
			wantErr: "requires 2-5 rounds",
		},
		{
			name: "too many rounds",
			spec: generic.MultiRoundSpec{
				Conversation: generic.ConversationProfile{Mode: "remote_session"},
				Rounds:       make([]generic.RoundPair, 6),
			},
			wantErr: "requires 2-5 rounds",
		},
		{
			name: "missing request body",
			spec: generic.MultiRoundSpec{
				Conversation: generic.ConversationProfile{Mode: "remote_session"},
				Rounds: []generic.RoundPair{
					{Request: generic.RawPacket{Body: ""}, Response: generic.RawPacket{Body: "{}"}},
					{Request: generic.RawPacket{Body: "{}"}, Response: generic.RawPacket{Body: "{}"}},
				},
			},
			wantErr: "round[0].request.body is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generic.InferIntegrationByMultiRound(tt.spec)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
