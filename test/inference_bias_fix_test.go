package test

import (
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestInferenceSessionIDAllowsNonEmptyFirstRound(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL: "https://example.com/api/v2/chat",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-abc","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"hi","session_id":"sess-abc","message_id":"m1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-abc","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"ok","session_id":"sess-abc","message_id":"m2"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-abc","model":"demo"}`},
				Response: generic.RawPacket{Body: `{"text":"done","session_id":"sess-abc","message_id":"m3"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	sessionField := inferMustFindField(t, result.Report.Fields, "session_id")
	if sessionField.Class != "session_id" {
		t.Fatalf("session_id class = %q, want session_id", sessionField.Class)
	}
	if sessionField.ResponsePath != "session_id" {
		t.Fatalf("session_id response_path = %q, want session_id", sessionField.ResponsePath)
	}
	if result.Profile.Request.SessionIDField != "session_id" {
		t.Fatalf("profile.request.session_id_field = %q, want session_id", result.Profile.Request.SessionIDField)
	}
}

func TestInferenceTechnicalDynamicFieldsNotClassifiedAsInput(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL: "https://example.com/api/v2/chat",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-001","req_id":"req-001","client_tm":"1772979687322","scene_param":"first_turn","model":"Qwen3-Max"}`},
				Response: generic.RawPacket{Body: `{"text":"r1","session_id":"sess-001","message_id":"m1"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-001","req_id":"req-002","client_tm":"1772979771861","scene_param":"continue_chat","model":"Qwen3-Max"}`},
				Response: generic.RawPacket{Body: `{"text":"r2","session_id":"sess-001","message_id":"m2"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"messages":[{"content":"$$$"}],"session_id":"sess-001","req_id":"req-003","client_tm":"1772979791127","scene_param":"continue_chat","model":"Qwen3-Max"}`},
				Response: generic.RawPacket{Body: `{"text":"r3","session_id":"sess-001","message_id":"m3"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}

	for _, path := range []string{"req_id", "client_tm", "scene_param"} {
		field := inferMustFindField(t, result.Report.Fields, path)
		if field.Class == "input" {
			t.Fatalf("%s class = input, want dynamic/static", path)
		}
		if field.Class != "dynamic" && field.Class != "static" {
			t.Fatalf("%s class = %q, want dynamic/static", path, field.Class)
		}
	}
}

func TestInferenceRealSampleStyleDoesNotMisclassifySessionAsStatic(t *testing.T) {
	spec := generic.MultiRoundSpec{
		URL: "https://chat2.example.com/api/v2/chat",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request: generic.RawPacket{Body: `{
					"deep_search":"0",
					"req_id":"rid-001",
					"model":"Qwen3-Max",
					"scene":"chat",
					"session_id":"sess-real-001",
					"messages":[{"content":"$$$","mime_type":"text/plain","meta_data":{"ori_query":"` + "\u4f60\u597d" + `"}}],
					"parent_req_id":"0",
					"scene_param":"first_turn",
					"client_tm":"1772979687322",
					"biz_id":"ai_qwen"
				}`},
				Response: generic.RawPacket{Body: `{"text":"` + "\u4f60\u597d" + `","session_id":"sess-real-001","req_id":"rid-001","message_id":"m1"}`},
			},
			{
				Request: generic.RawPacket{Body: `{
					"deep_search":"0",
					"req_id":"rid-002",
					"model":"Qwen3-Max",
					"scene":"chat",
					"session_id":"sess-real-001",
					"messages":[{"content":"$$$","mime_type":"text/plain","meta_data":{"ori_query":"` + "\u4eca\u5929\u7684\u5929\u6c14\u5982\u4f55\uff1f" + `"}}],
					"parent_req_id":"rid-001",
					"scene_param":"continue_chat",
					"client_tm":"1772979771861",
					"biz_id":"ai_qwen"
				}`},
				Response: generic.RawPacket{Body: `{"text":"` + "\u7ee7\u7eed" + `","session_id":"sess-real-001","req_id":"rid-002","message_id":"m2"}`},
			},
			{
				Request: generic.RawPacket{Body: `{
					"deep_search":"0",
					"req_id":"rid-003",
					"model":"Qwen3-Max",
					"scene":"chat",
					"session_id":"sess-real-001",
					"messages":[{"content":"$$$","mime_type":"text/plain","meta_data":{"ori_query":"` + "\u6df1\u5733" + `"}}],
					"parent_req_id":"rid-002",
					"scene_param":"continue_chat",
					"client_tm":"1772979791127",
					"biz_id":"ai_qwen"
				}`},
				Response: generic.RawPacket{Body: `{"text":"` + "\u6df1\u5733" + `","session_id":"sess-real-001","req_id":"rid-003","message_id":"m3"}`},
			},
		},
	}

	result, err := generic.InferIntegrationByMultiRound(spec)
	if err != nil {
		t.Fatalf("InferIntegrationByMultiRound failed: %v", err)
	}
	if result.Report.Status != "auto_confirmed" {
		t.Fatalf("status = %q, want auto_confirmed", result.Report.Status)
	}

	sessionField := inferMustFindField(t, result.Report.Fields, "session_id")
	if sessionField.Class != "session_id" {
		t.Fatalf("session_id class = %q, want session_id", sessionField.Class)
	}
	for _, f := range result.Report.Fields {
		if f.RequestPath == "session_id" && f.Class == "static" {
			t.Fatalf("session_id misclassified as static: %+v", f)
		}
	}
}

// inferMustFindField finds an InferredField by request path, or fails the test.
func inferMustFindField(t *testing.T, fields []generic.InferredField, reqPath string) generic.InferredField {
	t.Helper()
	for _, f := range fields {
		if f.RequestPath == reqPath {
			return f
		}
	}
	t.Fatalf("field %q not found in report", reqPath)
	return generic.InferredField{}
}
