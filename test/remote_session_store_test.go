package test

import (
	"context"
	"sync"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// sessionMockStore is a minimal in-memory SessionStore for testing.
// Named to avoid conflict with mockStore in client_test.go.
type sessionMockStore struct {
	mu   sync.Mutex
	data map[string]*session.SessionState
}

func newSessionMockStore() *sessionMockStore {
	return &sessionMockStore{data: make(map[string]*session.SessionState)}
}

func (m *sessionMockStore) Get(_ context.Context, id string) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.data[id]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	cp := *s
	cp.Messages = make([]base.Message, len(s.Messages))
	copy(cp.Messages, s.Messages)
	return &cp, nil
}

func (m *sessionMockStore) Save(_ context.Context, state *session.SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *state
	cp.Messages = make([]base.Message, len(state.Messages))
	copy(cp.Messages, state.Messages)
	m.data[state.ID] = &cp
	return nil
}

func (m *sessionMockStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}

// --- prependStoredMessages tests ---

func TestPrependStoredMessages_NilStore(t *testing.T) {
	c := client.New()
	s := client.NewSessionForTest(c, "test-id", "", nil, "")
	current := []base.Message{{Role: "user", Content: "hello"}}

	result := s.ExposePrependStoredMessages(context.Background(), current)

	if len(result) != 1 || result[0].Content != "hello" {
		t.Fatalf("expected original messages unchanged, got %v", result)
	}
}

func TestPrependStoredMessages_EmptyID(t *testing.T) {
	c := client.New()
	store := newSessionMockStore()
	s := client.NewSessionForTest(c, "", "", store, "")
	current := []base.Message{{Role: "user", Content: "hello"}}

	result := s.ExposePrependStoredMessages(context.Background(), current)

	if len(result) != 1 || result[0].Content != "hello" {
		t.Fatalf("expected original messages unchanged, got %v", result)
	}
}

func TestPrependStoredMessages_NoExistingSession(t *testing.T) {
	c := client.New()
	store := newSessionMockStore()
	s := client.NewSessionForTest(c, "nonexistent", "", store, "")
	current := []base.Message{{Role: "user", Content: "hello"}}

	result := s.ExposePrependStoredMessages(context.Background(), current)

	if len(result) != 1 || result[0].Content != "hello" {
		t.Fatalf("expected original messages unchanged, got %v", result)
	}
}

func TestPrependStoredMessages_WithExistingMessages(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()
	_ = store.Save(ctx, &session.SessionState{
		ID: "sess-1",
		Messages: []base.Message{
			{Role: "user", Content: "round1-q"},
			{Role: "assistant", Content: "round1-a"},
		},
	})

	s := client.NewSessionForTest(c, "sess-1", "", store, "")
	current := []base.Message{{Role: "user", Content: "round2-q"}}

	result := s.ExposePrependStoredMessages(ctx, current)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Content != "round1-q" {
		t.Fatalf("result[0] = %q, want %q", result[0].Content, "round1-q")
	}
	if result[1].Content != "round1-a" {
		t.Fatalf("result[1] = %q, want %q", result[1].Content, "round1-a")
	}
	if result[2].Content != "round2-q" {
		t.Fatalf("result[2] = %q, want %q", result[2].Content, "round2-q")
	}
}

func TestPrependStoredMessages_NilCurrentMsgs(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()
	_ = store.Save(ctx, &session.SessionState{
		ID: "sess-1",
		Messages: []base.Message{
			{Role: "user", Content: "q"},
			{Role: "assistant", Content: "a"},
		},
	})

	s := client.NewSessionForTest(c, "sess-1", "", store, "")
	result := s.ExposePrependStoredMessages(ctx, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 existing messages, got %d", len(result))
	}
}

// --- saveState + prependStoredMessages accumulation tests ---

func TestRemoteSession_SaveState_AccumulatesMessages(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()
	s := client.NewSessionForTest(c, "remote-001", "generic", store, client.ConversationModeRemoteSession)

	// Round 1: save first turn
	req1 := base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "\u4f60\u597d"}},
	}
	resp1 := base.ChatResponse{Text: "\u4f60\u597d\uff01\u6211\u662fAI\u52a9\u624b\u3002"}
	accReq1 := req1
	accReq1.Messages = s.ExposePrependStoredMessages(ctx, req1.Messages)
	s.ExposeSaveState(ctx, accReq1, resp1)

	// Verify round 1 state
	state1, err := store.Get(ctx, "remote-001")
	if err != nil {
		t.Fatalf("Get after round 1: %v", err)
	}
	if len(state1.Messages) != 2 {
		t.Fatalf("round 1: expected 2 messages, got %d", len(state1.Messages))
	}
	if state1.Messages[0].Content != "\u4f60\u597d" {
		t.Fatalf("round 1 msg[0] = %q, want %q", state1.Messages[0].Content, "\u4f60\u597d")
	}
	if state1.Messages[1].Content != "\u4f60\u597d\uff01\u6211\u662fAI\u52a9\u624b\u3002" {
		t.Fatalf("round 1 msg[1] = %q, want %q", state1.Messages[1].Content, "\u4f60\u597d\uff01\u6211\u662fAI\u52a9\u624b\u3002")
	}

	// Round 2: save second turn (should accumulate)
	req2 := base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "\u4f60\u521a\u624d\u8bf4\u4e86\u4ec0\u4e48\uff1f"}},
	}
	resp2 := base.ChatResponse{Text: "\u6211\u8bf4\u4e86\u4f60\u597d\u3002"}
	accReq2 := req2
	accReq2.Messages = s.ExposePrependStoredMessages(ctx, req2.Messages)
	s.ExposeSaveState(ctx, accReq2, resp2)

	// Verify round 2 state: should have all 4 messages
	state2, err := store.Get(ctx, "remote-001")
	if err != nil {
		t.Fatalf("Get after round 2: %v", err)
	}
	if len(state2.Messages) != 4 {
		t.Fatalf("round 2: expected 4 messages, got %d", len(state2.Messages))
	}

	expected := []struct {
		role, content string
	}{
		{"user", "\u4f60\u597d"},
		{"assistant", "\u4f60\u597d\uff01\u6211\u662fAI\u52a9\u624b\u3002"},
		{"user", "\u4f60\u521a\u624d\u8bf4\u4e86\u4ec0\u4e48\uff1f"},
		{"assistant", "\u6211\u8bf4\u4e86\u4f60\u597d\u3002"},
	}
	for i, exp := range expected {
		if state2.Messages[i].Role != exp.role || state2.Messages[i].Content != exp.content {
			t.Fatalf("round 2 msg[%d] = {%q, %q}, want {%q, %q}",
				i, state2.Messages[i].Role, state2.Messages[i].Content, exp.role, exp.content)
		}
	}
}

func TestRemoteSession_SaveState_ThreeRounds(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()
	s := client.NewSessionForTest(c, "remote-002", "generic", store, client.ConversationModeRemoteSession)

	rounds := []struct {
		userMsg      string
		assistantMsg string
	}{
		{"\u7b2c\u4e00\u4e2a\u95ee\u9898", "\u7b2c\u4e00\u4e2a\u56de\u7b54"},
		{"\u7b2c\u4e8c\u4e2a\u95ee\u9898", "\u7b2c\u4e8c\u4e2a\u56de\u7b54"},
		{"\u7b2c\u4e09\u4e2a\u95ee\u9898", "\u7b2c\u4e09\u4e2a\u56de\u7b54"},
	}

	for _, r := range rounds {
		req := base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: r.userMsg}},
		}
		resp := base.ChatResponse{Text: r.assistantMsg}
		accReq := req
		accReq.Messages = s.ExposePrependStoredMessages(ctx, req.Messages)
		s.ExposeSaveState(ctx, accReq, resp)
	}

	state, err := store.Get(ctx, "remote-002")
	if err != nil {
		t.Fatalf("Get after 3 rounds: %v", err)
	}
	if len(state.Messages) != 6 {
		t.Fatalf("expected 6 messages after 3 rounds, got %d", len(state.Messages))
	}

	// Verify order
	for i, r := range rounds {
		userIdx := i * 2
		assistantIdx := i*2 + 1
		if state.Messages[userIdx].Content != r.userMsg {
			t.Fatalf("msg[%d] = %q, want %q", userIdx, state.Messages[userIdx].Content, r.userMsg)
		}
		if state.Messages[assistantIdx].Content != r.assistantMsg {
			t.Fatalf("msg[%d] = %q, want %q", assistantIdx, state.Messages[assistantIdx].Content, r.assistantMsg)
		}
	}
}

// --- placeholder save test ---

func TestRemoteSession_PlaceholderSave_PreservesExistingMessages(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()

	providerName := "generic"
	convMode := client.ConversationModeRemoteSession

	// Pre-populate store with round 1 data
	_ = store.Save(ctx, &session.SessionState{
		ID:       "remote-003",
		Provider: providerName,
		Messages: []base.Message{
			{Role: "user", Content: "round1-q"},
			{Role: "assistant", Content: "round1-a"},
		},
	})

	s := client.NewSessionForTest(c, "remote-003", providerName, store, convMode)

	// Simulate placeholder save (what chatStreamRemoteSession does on first SessionID chunk)
	existingMsgs := s.ExposePrependStoredMessages(ctx, nil)
	placeholder := &session.SessionState{
		ID:       s.ID(),
		Provider: providerName,
		Messages: existingMsgs,
	}
	if convMode != "" {
		placeholder.Meta = map[string]string{"mode": string(convMode)}
	}
	_ = store.Save(ctx, placeholder)

	// Verify: placeholder save did NOT wipe existing messages
	state, err := store.Get(ctx, "remote-003")
	if err != nil {
		t.Fatalf("Get after placeholder save: %v", err)
	}
	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages preserved, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "round1-q" {
		t.Fatalf("msg[0] = %q, want %q", state.Messages[0].Content, "round1-q")
	}
	if state.Messages[1].Content != "round1-a" {
		t.Fatalf("msg[1] = %q, want %q", state.Messages[1].Content, "round1-a")
	}
	// Verify meta was set
	if state.Meta["mode"] != "remote_session" {
		t.Fatalf("meta[mode] = %q, want %q", state.Meta["mode"], "remote_session")
	}
}

// --- saveState meta test ---

func TestRemoteSession_SaveState_MetaContainsMode(t *testing.T) {
	c := client.New()
	ctx := context.Background()
	store := newSessionMockStore()
	s := client.NewSessionForTest(c, "remote-004", "generic", store, client.ConversationModeRemoteSession)

	req := base.ChatRequest{
		Model:    "test-model",
		Messages: []base.Message{{Role: "user", Content: "hello"}},
	}
	resp := base.ChatResponse{Text: "hi"}
	s.ExposeSaveState(ctx, req, resp)

	state, err := store.Get(ctx, "remote-004")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.Meta["mode"] != "remote_session" {
		t.Fatalf("meta[mode] = %q, want %q", state.Meta["mode"], "remote_session")
	}
	if state.Meta["model"] != "test-model" {
		t.Fatalf("meta[model] = %q, want %q", state.Meta["model"], "test-model")
	}
}
