package test

import (
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestInferenceReportPendingConfirmHasSuggestions(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL: "https://example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"hello","session_id":"","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"s1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"world","session_id":"s1","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"s1"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}
	if result == nil || result.Report == nil {
		t.Fatal("report is nil")
	}
	if result.Report.Status != "pending_confirm" {
		t.Fatalf("status = %q, want pending_confirm", result.Report.Status)
	}
	if len(result.Report.Suggestions) == 0 {
		t.Fatal("pending_confirm suggestions = empty, want non-empty")
	}

	foundLowConfidenceWarning := false
	for _, w := range result.Report.Warnings {
		if strings.Contains(w, "overall confidence") && strings.Contains(w, "auto_apply_threshold") {
			foundLowConfidenceWarning = true
			break
		}
	}
	if !foundLowConfidenceWarning {
		t.Fatal("pending_confirm warnings missing low-confidence threshold hint")
	}
}

func TestInferenceReportFailedContainsFallbackSuggestion(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL: "https://example.com/v1/chat/completions",
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

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}
	if result == nil || result.Report == nil {
		t.Fatal("report is nil")
	}
	if result.Report.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Report.Status)
	}
	if !result.Report.FallbackSuggested {
		t.Fatal("fallback_suggested = false, want true")
	}
	if len(result.Report.Suggestions) == 0 {
		t.Fatal("failed suggestions = empty, want non-empty")
	}
}
