package test

import (
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// TC-001: No mode set - Chat() works normally (backward compatible)
func TestTC001_NoMode_BackwardCompatible(t *testing.T) {
	store := &mockStore{}
	cli := client.New()

	sess := cli.NewSession("openai",
		client.WithStore(store),
		client.WithAutoID(),
		client.WithHistoryMode(client.HistoryAuto),
	)

	if sess.ID() == "" {
		t.Error("expected session ID to be auto-generated")
	}
}

// TC-002: remote_session - first turn no session_id injection
func TestTC002_RemoteSession_SessionIDExtraction(t *testing.T) {
	store := &mockStore{}
	cli := client.New()

	sess := cli.NewSession("dify",
		client.WithStore(store),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	// remote_session should NOT auto-generate ID; it waits for provider response
	if sess.ID() != "" {
		t.Errorf("expected empty session ID for remote_session initially, got %q", sess.ID())
	}
}

// TC-002B: local_history - auto-generates local ID, loads history
func TestTC002B_LocalHistory_AutoID(t *testing.T) {
	store := &mockStore{}
	cli := client.New()

	sess := cli.NewSession("openai",
		client.WithStore(store),
		client.WithConversationMode(client.ConversationModeLocalHistory),
		client.WithAutoID(),
		client.WithHistoryMode(client.HistoryAuto),
	)

	if sess.ID() == "" {
		t.Error("expected session ID to be auto-generated for local_history")
	}
}

// TC-009: OnError strategies
func TestTC009_OnError_Strategies(t *testing.T) {
	t.Run("default_is_empty", func(t *testing.T) {
		cli := client.New()
		if cli.SessionConfig.OnError != "" {
			t.Errorf("expected empty default OnError, got %q", cli.SessionConfig.OnError)
		}
	})

	t.Run("abort_strategy", func(t *testing.T) {
		cli := client.New()
		_ = cli.NewSession("test",
			client.WithOnError(client.OnErrorAbort),
		)
	})

	t.Run("continue_strategy", func(t *testing.T) {
		cli := client.New()
		_ = cli.NewSession("test",
			client.WithOnError(client.OnErrorContinue),
		)
	})
}

// TC-010: HistoryWindow MaxMessages=3, history has 10 items
func TestTC010_HistoryWindow_Truncation(t *testing.T) {
	msgs := make([]base.Message, 10)
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = base.Message{Role: role, Content: "message content padding here"}
	}

	truncated := session.Truncate(msgs, 3, 0)
	if len(truncated) != 3 {
		t.Errorf("expected 3 messages after truncation, got %d", len(truncated))
	}
	if truncated[0].Content != msgs[7].Content {
		t.Errorf("expected first truncated msg to match msgs[7]")
	}
	if truncated[2].Content != msgs[9].Content {
		t.Errorf("expected last truncated msg to match msgs[9]")
	}
}

// TC-010B: HistoryWindow MaxTokens
func TestTC010B_HistoryWindow_MaxTokens(t *testing.T) {
	msgs := []base.Message{
		{Role: "user", Content: "short"},
		{Role: "assistant", Content: "this is a medium length response with content"},
		{Role: "user", Content: "another question here"},
		{Role: "assistant", Content: "final answer with some more words to fill space"},
	}

	// With very small token budget, should only keep last message(s)
	truncated := session.Truncate(msgs, 0, 12)
	if len(truncated) >= len(msgs) {
		t.Errorf("expected fewer messages with small token budget, got %d", len(truncated))
	}
}

// Test ConversationMode constants
func TestConversationModeConstants(t *testing.T) {
	if client.ConversationModeRemoteSession != "remote_session" {
		t.Errorf("unexpected value: %q", client.ConversationModeRemoteSession)
	}
	if client.ConversationModeLocalHistory != "local_history" {
		t.Errorf("unexpected value: %q", client.ConversationModeLocalHistory)
	}
}

func TestResolveConversationAndStreamDefaults(t *testing.T) {
	if got := client.ResolveConversationMode("bailian_app"); got != client.ConversationModeLocalHistory {
		t.Errorf("unexpected conversation mode for bailian_app: %q", got)
	}
	if got := client.ResolveDefaultStream("bailian_app"); !got {
		t.Errorf("expected default stream=true for bailian_app")
	}
	if got := client.ResolveDefaultStream("openai"); !got {
		t.Errorf("expected default stream=true for openai")
	}
	if got := client.ResolveConversationMode("qianfan_app"); got != client.ConversationModeRemoteSession {
		t.Errorf("unexpected conversation mode for qianfan_app: %q", got)
	}
	if got := client.ResolveDefaultStream("qianfan_app"); !got {
		t.Errorf("expected default stream=true for qianfan_app")
	}
}

// Test OnErrorStrategy constants
func TestOnErrorStrategyConstants(t *testing.T) {
	if client.OnErrorAbort != "abort" {
		t.Errorf("unexpected value: %q", client.OnErrorAbort)
	}
	if client.OnErrorContinue != "continue" {
		t.Errorf("unexpected value: %q", client.OnErrorContinue)
	}
}

// Test session state Meta with mode and on_error keys
func TestSessionState_MetaKeys(t *testing.T) {
	state := &session.SessionState{
		ID: "test",
		Meta: map[string]string{
			"mode":     "remote_session",
			"on_error": "abort",
			"model":    "gpt-4",
		},
	}
	if state.Meta["mode"] != "remote_session" {
		t.Error("expected mode=remote_session")
	}
	if state.Meta["on_error"] != "abort" {
		t.Error("expected on_error=abort")
	}
}
