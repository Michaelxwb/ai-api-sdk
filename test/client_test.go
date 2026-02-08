package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testProviderSpec struct {
	name          string
	baseURL       string
	buildFn       func(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error)
	parseFn       func(resp *http.Response) (base.ChatResponse, error)
	streamParseFn func(resp *http.Response) (<-chan streaming.StreamChunk, error)
	authOverride  func(cred *auth.Credential) (auth.AuthStrategy, bool)

	mu       sync.Mutex
	lastReq  base.ChatRequest
	lastOpts base.BuildOptions
}

func (s *testProviderSpec) Name() string { return s.name }

func (s *testProviderSpec) DefaultBaseURL() string {
	if s.baseURL != "" {
		return s.baseURL
	}
	return ""
}

func (s *testProviderSpec) SupportedAuthTypes() []auth.AuthType { return nil }

func (s *testProviderSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	s.mu.Lock()
	s.lastReq = req
	s.lastOpts = opts
	s.mu.Unlock()
	if s.buildFn != nil {
		return s.buildFn(ctx, opts, req)
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = s.baseURL
	}
	path := opts.Path
	if path == "" {
		path = "/chat"
	}
	url := joinURL(baseURL, path)
	return http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
}

func (s *testProviderSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if s.parseFn != nil {
		return s.parseFn(resp)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return base.ChatResponse{}, err
	}
	return base.ChatResponse{Text: string(data), Raw: data}, nil
}

func (s *testProviderSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if s.authOverride != nil {
		return s.authOverride(cred)
	}
	return nil, false
}

func (s *testProviderSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if s.streamParseFn != nil {
		return s.streamParseFn(resp)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	out := make(chan streaming.StreamChunk, 1)
	go func() {
		defer close(out)
		out <- streaming.StreamChunk{Text: string(data), Done: true}
	}()
	return out, nil
}

func (s *testProviderSpec) LastRequest() base.ChatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReq
}

var specCounter int64

func registerTestSpec(spec *testProviderSpec) string {
	if spec.name == "" {
		name := fmt.Sprintf("test-spec-%d", atomic.AddInt64(&specCounter, 1))
		spec.name = name
	}
	base.Register(spec.name, spec)
	return spec.name
}

func joinURL(baseURL, path string) string {
	if strings.HasSuffix(baseURL, "/") {
		baseURL = strings.TrimSuffix(baseURL, "/")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type mockStore struct {
	mu          sync.Mutex
	states      map[string]*session.SessionState
	getErr      error
	saveErr     error
	deleteErr   error
	getCount    int
	saveCount   int
	deleteCount int
	lastSaved   *session.SessionState
}

func (m *mockStore) Get(ctx context.Context, id string) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCount++
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.states == nil {
		return nil, nil
	}
	return m.states[id], nil
}

func (m *mockStore) Save(ctx context.Context, state *session.SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCount++
	m.lastSaved = state
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.states == nil {
		m.states = make(map[string]*session.SessionState)
	}
	m.states[state.ID] = state
	return nil
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCount++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.states != nil {
		delete(m.states, id)
	}
	return nil
}

func (m *mockStore) Counts() (getCount, saveCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getCount, m.saveCount
}

func (m *mockStore) LastSaved() *session.SessionState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastSaved
}

func TestClientErrors(t *testing.T) {
	t.Run("APIErrorFormat", func(t *testing.T) {
		err := &client.APIError{StatusCode: 404, Body: "not found", Op: "chat"}
		want := "client: chat: status 404: not found"
		if err.Error() != want {
			t.Fatalf("APIError.Error() = %q, want %q", err.Error(), want)
		}
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) {
			t.Fatal("expected errors.As to match APIError")
		}
	})

	t.Run("ParseErrorWrap", func(t *testing.T) {
		root := errors.New("bad json")
		err := &client.ParseError{Provider: "openai", Err: root}
		if !strings.Contains(err.Error(), "openai") {
			t.Fatalf("ParseError.Error() missing provider: %q", err.Error())
		}
		if !errors.Is(err, root) {
			t.Fatal("ParseError should unwrap to root error")
		}
		var parseErr *client.ParseError
		if !errors.As(err, &parseErr) {
			t.Fatal("expected errors.As to match ParseError")
		}
	})

	t.Run("AuthErrorWrap", func(t *testing.T) {
		root := errors.New("invalid token")
		err := &client.AuthError{Op: "refresh", Err: root}
		if !strings.Contains(err.Error(), "auth: refresh") {
			t.Fatalf("AuthError.Error() missing op: %q", err.Error())
		}
		if !errors.Is(err, root) {
			t.Fatal("AuthError should unwrap to root error")
		}
		var authErr *client.AuthError
		if !errors.As(err, &authErr) {
			t.Fatal("expected errors.As to match AuthError")
		}
	})
}

func TestAuthTransport_RoundTrip(t *testing.T) {
	t.Run("NormalRequest", func(t *testing.T) {
		var gotAuth string
		rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			return httpResponse(http.StatusOK, "ok"), nil
		})
		tr := &client.AuthTransport{Base: rt, Strategy: auth.BearerTokenStrategy{Token: "token"}}
		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if gotAuth != "Bearer token" {
			t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer token")
		}
	})

	t.Run("NilStrategy", func(t *testing.T) {
		called := false
		rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			if got := req.Header.Get("Authorization"); got != "" {
				t.Fatalf("unexpected Authorization header: %q", got)
			}
			return httpResponse(http.StatusOK, "ok"), nil
		})
		tr := &client.AuthTransport{Base: rt, Strategy: nil}
		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip error: %v", err)
		}
		if !called {
			t.Fatal("base RoundTripper not called")
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("ApplyFailure", func(t *testing.T) {
		applyErr := errors.New("apply failed")
		rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return httpResponse(http.StatusOK, "ok"), nil
		})
		tr := &client.AuthTransport{Base: rt, Strategy: errorStrategy{err: applyErr}}
		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		_, err = tr.RoundTrip(req)
		if err == nil {
			t.Fatal("expected RoundTrip error")
		}
		if !strings.Contains(err.Error(), "client: apply auth strategy") {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(err, applyErr) {
			t.Fatal("expected wrapped apply error")
		}
	})

	t.Run("CustomHeadersAndQuery", func(t *testing.T) {
		rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("X-Cred"); got != "yes" {
				t.Fatalf("header X-Cred = %q, want %q", got, "yes")
			}
			if got := req.URL.Query().Get("qp"); got != "qv" {
				t.Fatalf("query qp = %q, want %q", got, "qv")
			}
			if got := req.URL.Query().Get("existing"); got != "1" {
				t.Fatalf("query existing = %q, want %q", got, "1")
			}
			return httpResponse(http.StatusOK, "ok"), nil
		})
		cred := &auth.Credential{
			Headers:     map[string]string{"X-Cred": "yes"},
			QueryParams: map[string]string{"qp": "qv"},
		}
		tr := &client.AuthTransport{Base: rt, Cred: cred}
		req, err := http.NewRequest(http.MethodGet, "http://example.com/path?existing=1", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})
}

type errorStrategy struct{ err error }

func (e errorStrategy) Apply(req *http.Request) error {
	return e.err
}

func TestAuthTransport_OAuthRetry(t *testing.T) {
	t.Run("RetryOn401", func(t *testing.T) {
		var apiCount int32
		var tokenCount int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				atomic.AddInt32(&tokenCount, 1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"access_token":"new-token","expires_in":3600}`)
			case "/chat":
				count := atomic.AddInt32(&apiCount, 1)
				if count == 1 {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = io.WriteString(w, "ok")
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		mgr, err := auth.NewManager(nil, nil)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		cred := &auth.Credential{
			ID:           "cred-1",
			Provider:     "mock",
			AuthType:     auth.AuthTypeOAuth,
			AccessToken:  "old-token",
			RefreshToken: "refresh-token",
			Metadata: map[string]any{
				"token_url":     srv.URL + "/token",
				"client_id":     "client-id",
				"client_secret": "client-secret",
			},
		}
		mgr.Register(cred)

		tr := &client.AuthTransport{
			Base:     http.DefaultTransport,
			Strategy: auth.OAuthStrategy{AccessToken: cred.AccessToken},
			Manager:  mgr,
			Cred:     cred,
		}
		httpClient := &http.Client{Transport: tr}
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/chat", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("httpClient.Do: %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if atomic.LoadInt32(&apiCount) != 2 {
			t.Fatalf("apiCount = %d, want 2", atomic.LoadInt32(&apiCount))
		}
		if atomic.LoadInt32(&tokenCount) != 1 {
			t.Fatalf("tokenCount = %d, want 1", atomic.LoadInt32(&tokenCount))
		}
	})
}

func TestClient_New(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		c := client.New()
		if c == nil || c.HTTP == nil {
			t.Fatal("New returned nil client or HTTP")
		}
		if c.HTTP.Timeout != 60*time.Second {
			t.Fatalf("HTTP.Timeout = %v, want %v", c.HTTP.Timeout, 60*time.Second)
		}
	})

	t.Run("NewClient", func(t *testing.T) {
		cfg := &config.Config{}
		mgr, _ := auth.NewManager(nil, nil)
		c := client.NewClient(cfg, mgr)
		if c == nil || c.HTTP == nil {
			t.Fatal("NewClient returned nil client or HTTP")
		}
		if c.Config != cfg {
			t.Fatal("NewClient did not set Config")
		}
		if c.AuthMgr != mgr {
			t.Fatal("NewClient did not set AuthMgr")
		}
		if c.HTTP.Timeout != 60*time.Second {
			t.Fatalf("HTTP.Timeout = %v, want %v", c.HTTP.Timeout, 60*time.Second)
		}
	})
}

func TestClient_ChatWith(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"text":"hello","session_id":"sess-1"}`)
		}))
		defer srv.Close()

		spec := &testProviderSpec{
			parseFn: parseSimpleJSONResponse,
		}
		specName := registerTestSpec(spec)

		c := client.New()
		pc := &config.ProviderConfig{
			Type:    specName,
			BaseURL: srv.URL,
			Path:    "/v1/chat/completions",
		}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(cred, pc, client.WithHistoryMode(client.HistoryNone))

		resp, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatalf("Chat error: %v", err)
		}
		if resp.Text != "hello" {
			t.Fatalf("ChatResponse.Text = %q, want %q", resp.Text, "hello")
		}
	})

	t.Run("HTTPStatusErrors", func(t *testing.T) {
		cases := []struct {
			name   string
			status int
		}{
			{name: "NotFound", status: http.StatusNotFound},
			{name: "ServerError", status: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, "error-body")
				}))
				defer srv.Close()

				spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
				specName := registerTestSpec(spec)

				c := client.New()
				pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
				cred := &auth.Credential{AuthType: auth.AuthTypeNone}
				sess := c.NewSessionWith(cred, pc, client.WithHistoryMode(client.HistoryNone))

				_, err := sess.Chat(context.Background(), base.ChatRequest{
					Model:    "gpt-test",
					Messages: []base.Message{{Role: "user", Content: "hi"}},
				})
				if err == nil {
					t.Fatal("expected error for non-2xx status")
				}
				var apiErr *client.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected APIError, got %T", err)
				}
				if apiErr.StatusCode != tc.status {
					t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
				}
				if apiErr.Body != "error-body" {
					t.Fatalf("Body = %q, want %q", apiErr.Body, "error-body")
				}
				if apiErr.Op != "chat" {
					t.Fatalf("Op = %q, want %q", apiErr.Op, "chat")
				}
			})
		}
	})

	t.Run("LimitReader", func(t *testing.T) {
		bigBody := strings.Repeat("a", (1<<20)+128)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, bigBody)
		}))
		defer srv.Close()

		spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
		specName := registerTestSpec(spec)

		c := client.New()
		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(cred, pc, client.WithHistoryMode(client.HistoryNone))

		_, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		})
		if err == nil {
			t.Fatal("expected error for non-2xx status")
		}
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected APIError, got %T", err)
		}
		if !strings.HasSuffix(apiErr.Body, "...(truncated)") {
			t.Fatalf("Body suffix = %q, want %q", apiErr.Body[len(apiErr.Body)-len("...(truncated)"):], "...(truncated)")
		}
		wantLen := 4096 + len("...(truncated)")
		if len(apiErr.Body) != wantLen {
			t.Fatalf("Body length = %d, want %d", len(apiErr.Body), wantLen)
		}
	})
}

func TestSession_NewSessionOptions(t *testing.T) {
	t.Run("AutoID", func(t *testing.T) {
		c := client.New()
		sess := c.NewSession("mock", client.WithAutoID())
		if sess.ID() == "" {
			t.Fatal("expected auto-generated session ID")
		}
	})

	t.Run("IDOverridesAutoID", func(t *testing.T) {
		c := client.New()
		sess := c.NewSession("mock", client.WithID("fixed"), client.WithAutoID())
		if sess.ID() != "fixed" {
			t.Fatalf("Session.ID = %q, want %q", sess.ID(), "fixed")
		}
	})

	t.Run("StartNewChatSkipsStore", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"text":"ok"}`)
		}))
		defer srv.Close()

		spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
		specName := registerTestSpec(spec)

		c := client.New()
		store := &mockStore{}
		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(cred, pc, client.WithStore(store), client.WithStartNewChat(true))

		_, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatalf("Chat error: %v", err)
		}
		_, saveCount := store.Counts()
		if saveCount != 0 {
			t.Fatalf("Save count = %d, want 0", saveCount)
		}
	})
}

func TestSession_Chat_HistoryModes(t *testing.T) {
	t.Run("HistoryAutoLoadsAndSaves", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"text":"assistant"}`)
		}))
		defer srv.Close()

		spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
		specName := registerTestSpec(spec)

		store := &mockStore{states: map[string]*session.SessionState{
			"sess-1": {
				ID:       "sess-1",
				Provider: "mock",
				Messages: []base.Message{{Role: "user", Content: "history"}},
			},
		}}

		c := client.New()
		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(
			cred,
			pc,
			client.WithStore(store),
			client.WithID("sess-1"),
			client.WithHistoryMode(client.HistoryAuto),
			client.WithMeta(map[string]string{"team": "sdk"}),
		)

		_, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "new"}},
		})
		if err != nil {
			t.Fatalf("Chat error: %v", err)
		}

		lastReq := spec.LastRequest()
		if len(lastReq.Messages) != 2 {
			t.Fatalf("request message count = %d, want 2", len(lastReq.Messages))
		}
		if lastReq.Messages[0].Content != "history" {
			t.Fatalf("first message = %q, want %q", lastReq.Messages[0].Content, "history")
		}

		_, saveCount := store.Counts()
		if saveCount != 1 {
			t.Fatalf("Save count = %d, want 1", saveCount)
		}
		saved := store.LastSaved()
		if saved == nil {
			t.Fatal("expected saved session state")
		}
		if len(saved.Messages) != 3 {
			t.Fatalf("saved messages count = %d, want 3", len(saved.Messages))
		}
		if saved.Messages[2].Role != "assistant" || saved.Messages[2].Content != "assistant" {
			t.Fatalf("assistant message = %+v, want role assistant with content", saved.Messages[2])
		}
		if saved.Meta == nil || saved.Meta["team"] != "sdk" || saved.Meta["model"] != "gpt-test" {
			t.Fatalf("saved meta = %+v, want team and model", saved.Meta)
		}
	})

	t.Run("HistoryNoneSkipsLoad", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"text":"assistant"}`)
		}))
		defer srv.Close()

		spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
		specName := registerTestSpec(spec)

		store := &mockStore{states: map[string]*session.SessionState{
			"sess-2": {
				ID:       "sess-2",
				Provider: "mock",
				Messages: []base.Message{{Role: "user", Content: "history"}},
			},
		}}

		c := client.New()
		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(
			cred,
			pc,
			client.WithStore(store),
			client.WithID("sess-2"),
			client.WithHistoryMode(client.HistoryNone),
		)

		_, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "new"}},
		})
		if err != nil {
			t.Fatalf("Chat error: %v", err)
		}

		lastReq := spec.LastRequest()
		if len(lastReq.Messages) != 1 {
			t.Fatalf("request message count = %d, want 1", len(lastReq.Messages))
		}
		getCount, _ := store.Counts()
		if getCount != 0 {
			t.Fatalf("Get count = %d, want 0", getCount)
		}
	})

	t.Run("OnStoreErrorCallback", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"text":"assistant"}`)
		}))
		defer srv.Close()

		spec := &testProviderSpec{parseFn: parseSimpleJSONResponse}
		specName := registerTestSpec(spec)

		store := &mockStore{saveErr: errors.New("save failed")}
		var callbackCount int32

		c := client.New()
		c.SessionConfig.OnStoreError = func(ctx context.Context, err error) {
			atomic.AddInt32(&callbackCount, 1)
		}

		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(
			cred,
			pc,
			client.WithStore(store),
			client.WithID("sess-3"),
			client.WithHistoryMode(client.HistoryNone),
		)

		_, err := sess.Chat(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "new"}},
		})
		if err != nil {
			t.Fatalf("Chat error: %v", err)
		}
		if atomic.LoadInt32(&callbackCount) != 1 {
			t.Fatalf("OnStoreError callback count = %d, want 1", atomic.LoadInt32(&callbackCount))
		}
	})
}

func TestSession_ChatStream(t *testing.T) {
	t.Run("StreamSavesHistory", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "streamed")
		}))
		defer srv.Close()

		spec := &testProviderSpec{
			streamParseFn: func(resp *http.Response) (<-chan streaming.StreamChunk, error) {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				_ = resp.Body.Close()
				out := make(chan streaming.StreamChunk, 2)
				go func() {
					defer close(out)
					out <- streaming.StreamChunk{Text: string(data), Done: true}
				}()
				return out, nil
			},
		}
		specName := registerTestSpec(spec)

		store := &mockStore{}
		c := client.New()
		pc := &config.ProviderConfig{Type: specName, BaseURL: srv.URL, Path: "/chat"}
		cred := &auth.Credential{AuthType: auth.AuthTypeNone}
		sess := c.NewSessionWith(
			cred,
			pc,
			client.WithStore(store),
			client.WithID("sess-stream"),
			client.WithHistoryMode(client.HistoryNone),
		)

		stream, err := sess.ChatStream(context.Background(), base.ChatRequest{
			Model:    "gpt-test",
			Messages: []base.Message{{Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatalf("ChatStream error: %v", err)
		}

		var gotText string
		for chunk := range stream {
			if chunk.Error != nil {
				t.Fatalf("stream chunk error: %v", chunk.Error)
			}
			gotText += chunk.Text
		}
		if gotText != "streamed" {
			t.Fatalf("streamed text = %q, want %q", gotText, "streamed")
		}

		_, saveCount := store.Counts()
		if saveCount != 1 {
			t.Fatalf("Save count = %d, want 1", saveCount)
		}
		saved := store.LastSaved()
		if saved == nil {
			t.Fatal("expected saved session state")
		}
		if len(saved.Messages) != 2 {
			t.Fatalf("saved messages count = %d, want 2", len(saved.Messages))
		}
		if saved.Messages[1].Content != "streamed" {
			t.Fatalf("assistant message content = %q, want %q", saved.Messages[1].Content, "streamed")
		}
	})
}

func parseSimpleJSONResponse(resp *http.Response) (base.ChatResponse, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return base.ChatResponse{}, err
	}
	var payload struct {
		Text      string `json:"text"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return base.ChatResponse{}, err
	}
	return base.ChatResponse{Text: payload.Text, SessionID: payload.SessionID, Raw: data}, nil
}
