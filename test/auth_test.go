package test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
)

type memoryStore struct {
	creds      []*auth.Credential
	loadErr    error
	saveErr    error
	loadCalled int
	saveCalled int
}

func (s *memoryStore) Load() ([]*auth.Credential, error) {
	s.loadCalled++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.creds, nil
}

func (s *memoryStore) Save(creds []*auth.Credential) error {
	s.saveCalled++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.creds = creds
	return nil
}

func (s *memoryStore) List() ([]*auth.Credential, error) {
	return s.Load()
}

func (s *memoryStore) Get(id string) (*auth.Credential, error) {
	creds, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, c := range creds {
		if c != nil && c.ID == id {
			return c, nil
		}
	}
	return nil, errors.New("not found")
}

func findCredByID(creds []*auth.Credential, id string) *auth.Credential {
	for _, c := range creds {
		if c != nil && c.ID == id {
			return c
		}
	}
	return nil
}

func TestManager_NewManager_Scenario(t *testing.T) {
	t.Run("nil_params", func(t *testing.T) {
		m, err := auth.NewManager(nil, nil)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		cred := &auth.Credential{ID: "c1", Provider: "p1", AuthType: auth.AuthTypeBearerToken, AccessToken: "tok"}
		m.Register(cred)
		got, strategy, err := m.Resolve("p1")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}
		if got == nil || got.ID != "c1" {
			t.Fatalf("unexpected credential: %+v", got)
		}
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		if err := strategy.Apply(req); err != nil {
			t.Fatalf("expected strategy apply ok, got %v", err)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatalf("expected authorization header to be set")
		}
	})

	t.Run("load_from_store", func(t *testing.T) {
		store := &memoryStore{creds: []*auth.Credential{
			nil,
			{ID: "", Provider: "p1"},
			{ID: "c1", Provider: "p1", AuthType: auth.AuthTypeNone},
		}}
		m, err := auth.NewManager(store, &auth.RoundRobinSelector{})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if store.loadCalled != 1 {
			t.Fatalf("expected load to be called once, got %d", store.loadCalled)
		}
		if _, err := m.Get("c1"); err != nil {
			t.Fatalf("expected credential to be loaded, got %v", err)
		}
		if _, err := m.Get("missing"); err == nil {
			t.Fatalf("expected error for missing credential")
		}
	})
}

func TestManager_Register_Scenario(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		cred := &auth.Credential{ID: "c1", Provider: "p1"}
		m.Register(cred)
		got, err := m.Get("c1")
		if err != nil {
			t.Fatalf("expected get ok, got %v", err)
		}
		if got.ID != "c1" {
			t.Fatalf("unexpected credential: %+v", got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		m.Register(&auth.Credential{ID: "c1", Provider: "p1"})
		m.Register(&auth.Credential{ID: "c2", Provider: "p1"})
		list := m.List()
		if len(list) != 2 {
			t.Fatalf("expected 2 credentials, got %d", len(list))
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		m.Register(&auth.Credential{ID: "c1", Provider: "p1", APIKey: "old"})
		m.Register(&auth.Credential{ID: "c1", Provider: "p1", APIKey: "new"})
		got, err := m.Get("c1")
		if err != nil {
			t.Fatalf("expected get ok, got %v", err)
		}
		if got.APIKey != "new" {
			t.Fatalf("expected updated credential, got %s", got.APIKey)
		}
	})
}

func TestManager_GetCredential_Scenario(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		m.Register(&auth.Credential{ID: "c1", Provider: "p1"})
		if _, err := m.Get("c1"); err != nil {
			t.Fatalf("expected get ok, got %v", err)
		}
	})

	t.Run("not_exists", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		if _, err := m.Get("missing"); err == nil {
			t.Fatalf("expected error for missing credential")
		}
	})

	t.Run("multiple_providers", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		m.Register(&auth.Credential{ID: "c1", Provider: "p1", AuthType: auth.AuthTypeNone})
		m.Register(&auth.Credential{ID: "c2", Provider: "p2", AuthType: auth.AuthTypeNone})

		got1, _, err := m.Resolve("p1")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}
		if got1.Provider != "p1" {
			t.Fatalf("expected provider p1, got %s", got1.Provider)
		}

		got2, _, err := m.Resolve("p2")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}
		if got2.Provider != "p2" {
			t.Fatalf("expected provider p2, got %s", got2.Provider)
		}
	})
}

func TestManager_RefreshOAuth_Scenario(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotRequest bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRequest = true
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("expected parse form ok, got %v", err)
			}
			if r.FormValue("grant_type") != "refresh_token" {
				t.Errorf("expected grant_type refresh_token, got %s", r.FormValue("grant_type"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"new-token","expires_in":3600,"refresh_token":"new-rt"}`))
		}))
		defer server.Close()

		store := &memoryStore{}
		m, err := auth.NewManager(store, &auth.RoundRobinSelector{})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		cred := &auth.Credential{
			ID:           "c1",
			Provider:     "p1",
			AuthType:     auth.AuthTypeOAuth,
			RefreshToken: "rt",
			Metadata: map[string]any{
				"token_url":     server.URL,
				"client_id":     "cid",
				"client_secret": "secret",
			},
		}
		m.Register(cred)

		if err := m.RefreshOAuth(context.Background(), cred); err != nil {
			t.Fatalf("expected refresh ok, got %v", err)
		}
		if !gotRequest {
			t.Fatalf("expected refresh request to be sent")
		}
		if cred.AccessToken != "new-token" {
			t.Fatalf("expected access token updated, got %s", cred.AccessToken)
		}
		if cred.RefreshToken != "new-rt" {
			t.Fatalf("expected refresh token updated, got %s", cred.RefreshToken)
		}
		if cred.ExpiresAt == nil || time.Until(*cred.ExpiresAt) <= 0 {
			t.Fatalf("expected expires_at set")
		}
		if store.saveCalled == 0 {
			t.Fatalf("expected store save called")
		}
		saved := findCredByID(store.creds, "c1")
		if saved == nil || saved.AccessToken != "new-token" {
			t.Fatalf("expected saved credential updated")
		}
	})

	t.Run("failure_missing_refresh_token", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		cred := &auth.Credential{
			ID:       "c1",
			AuthType: auth.AuthTypeOAuth,
			Metadata: map[string]any{"token_url": "http://example.com", "client_id": "cid"},
		}
		if err := m.RefreshOAuth(context.Background(), cred); err == nil {
			t.Fatalf("expected error for missing refresh token")
		}
	})

	t.Run("failure_status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		cred := &auth.Credential{
			ID:           "c1",
			AuthType:     auth.AuthTypeOAuth,
			RefreshToken: "rt",
			Metadata: map[string]any{
				"token_url": server.URL,
				"client_id": "cid",
			},
		}
		if err := m.RefreshOAuth(context.Background(), cred); err == nil {
			t.Fatalf("expected error for non-2xx response")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"late"}`))
		}))
		defer server.Close()

		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		cred := &auth.Credential{
			ID:           "c1",
			AuthType:     auth.AuthTypeOAuth,
			RefreshToken: "rt",
			Metadata: map[string]any{
				"token_url": server.URL,
				"client_id": "cid",
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		if err := m.RefreshOAuth(ctx, cred); err == nil {
			t.Fatalf("expected timeout error")
		}
	})
}

func TestManager_RoundRobinSelector_Scenario(t *testing.T) {
	t.Run("round_robin", func(t *testing.T) {
		m, _ := auth.NewManager(nil, &auth.RoundRobinSelector{})
		m.Register(&auth.Credential{ID: "a", Provider: "p1", AuthType: auth.AuthTypeNone})
		m.Register(&auth.Credential{ID: "b", Provider: "p1", AuthType: auth.AuthTypeNone})

		first, _, err := m.Resolve("p1")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}
		second, _, err := m.Resolve("p1")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}
		third, _, err := m.Resolve("p1")
		if err != nil {
			t.Fatalf("expected resolve ok, got %v", err)
		}

		if first.ID != "a" || second.ID != "b" || third.ID != "a" {
			t.Fatalf("unexpected round robin order: %s, %s, %s", first.ID, second.ID, third.ID)
		}
	})
}

func TestFileStore_NewFileStore_Scenario(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		store := auth.NewFileStore("/tmp/creds.json")
		if store.Path != "/tmp/creds.json" {
			t.Fatalf("unexpected path: %s", store.Path)
		}
		if !store.Encrypted {
			t.Fatalf("expected encrypted by default")
		}
		if store.ScryptParams.N != 32768 || store.ScryptParams.R != 8 || store.ScryptParams.P != 1 || store.ScryptParams.KeyLen != 32 {
			t.Fatalf("unexpected scrypt params: %+v", store.ScryptParams)
		}
	})
}

func TestFileStore_SaveLoad_Scenario(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		store := auth.NewFileStore(path)
		store.Encrypted = false
		creds := []*auth.Credential{{ID: "c1", Provider: "p1", AuthType: auth.AuthTypeAPIKey, APIKey: "k1"}}

		if err := store.Save(creds); err != nil {
			t.Fatalf("expected save ok, got %v", err)
		}
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("expected load ok, got %v", err)
		}
		if !reflect.DeepEqual(loaded, creds) {
			t.Fatalf("loaded credentials mismatch: %+v", loaded)
		}
	})

	t.Run("file_not_exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.json")
		store := auth.NewFileStore(path)
		store.Encrypted = false
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("expected load ok, got %v", err)
		}
		if len(loaded) != 0 {
			t.Fatalf("expected empty credentials, got %d", len(loaded))
		}
	})

	t.Run("empty_data", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.json")
		if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
			t.Fatalf("write empty file: %v", err)
		}
		store := auth.NewFileStore(path)
		store.Encrypted = false
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("expected load ok, got %v", err)
		}
		if len(loaded) != 0 {
			t.Fatalf("expected empty credentials, got %d", len(loaded))
		}
	})
}

func TestFileStore_Encryption_Scenario(t *testing.T) {
	t.Run("round_trip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		store := auth.NewFileStore(path)
		store.MasterKeyEnv = "TEST_MASTER_KEY"
		t.Setenv("TEST_MASTER_KEY", "super-secret")

		creds := []*auth.Credential{{ID: "c1", Provider: "p1", AuthType: auth.AuthTypeBearerToken, AccessToken: "tok"}}
		if err := store.Save(creds); err != nil {
			t.Fatalf("expected save ok, got %v", err)
		}
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("expected load ok, got %v", err)
		}
		if !reflect.DeepEqual(loaded, creds) {
			t.Fatalf("loaded credentials mismatch: %+v", loaded)
		}
	})

	t.Run("missing_master_key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		store := auth.NewFileStore(path)
		store.MasterKeyEnv = "MISSING_KEY"
		if err := store.Save([]*auth.Credential{{ID: "c1"}}); err == nil {
			t.Fatalf("expected error without master key")
		}
	})
}

func TestStrategy_BearerTokenStrategy_Scenario(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		s := auth.BearerTokenStrategy{Token: "tok"}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
		}
	})

	t.Run("nil_request", func(t *testing.T) {
		s := auth.BearerTokenStrategy{Token: "tok"}
		if err := s.Apply(nil); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestStrategy_ApiKeyHeaderStrategy_Scenario(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		s := auth.ApiKeyHeaderStrategy{HeaderName: "X-API-Key", Key: "k1"}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("X-API-Key") != "k1" {
			t.Fatalf("unexpected header value: %s", req.Header.Get("X-API-Key"))
		}
	})

	t.Run("custom_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		s := auth.ApiKeyHeaderStrategy{HeaderName: "X-Custom", Key: "k1", Prefix: "Token "}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("X-Custom") != "Token k1" {
			t.Fatalf("unexpected header value: %s", req.Header.Get("X-Custom"))
		}
	})
}

func TestStrategy_CustomHeaderStrategy_Scenario(t *testing.T) {
	t.Run("headers_and_query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com?x=1", nil)
		s := auth.CustomHeaderStrategy{
			Headers:     map[string]string{"X-Test": "v1", "": "skip"},
			QueryParams: map[string]string{"q": "1", "": "skip"},
		}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("X-Test") != "v1" {
			t.Fatalf("unexpected header value: %s", req.Header.Get("X-Test"))
		}
		if req.URL.Query().Get("q") != "1" {
			t.Fatalf("unexpected query param: %s", req.URL.RawQuery)
		}
	})

	t.Run("nil_request", func(t *testing.T) {
		s := auth.CustomHeaderStrategy{Headers: map[string]string{"X-Test": "v1"}}
		if err := s.Apply(nil); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestStrategy_NoAuth_Scenario(t *testing.T) {
	t.Run("no_change", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Header.Set("X-Existing", "v")
		s := auth.NoAuth{}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if req.Header.Get("X-Existing") != "v" {
			t.Fatalf("expected existing header preserved")
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no authorization header")
		}
	})
}

func TestStrategy_OAuthStrategies_Scenario(t *testing.T) {
	t.Run("oauth_strategy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		s := auth.OAuthStrategy{AccessToken: "tok"}
		if err := s.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
		}
	})

	t.Run("new_strategy_from_credential_oauth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		cred := &auth.Credential{ID: "c1", AuthType: auth.AuthTypeOAuth, AccessToken: "tok"}
		strategy := auth.NewStrategyFromCredential(cred)
		if err := strategy.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
		}
	})

	t.Run("jwt_sign_strategy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		cred := &auth.Credential{
			ID:       "c1",
			AuthType: auth.AuthTypeJWTSign,
			Metadata: map[string]any{
				"jwt_issuer":      "iss",
				"jwt_subject":     "sub",
				"jwt_audience":    "aud",
				"jwt_secret":      "secret",
				"jwt_exp_seconds": 1,
			},
		}
		strategy := auth.NewStrategyFromCredential(cred)
		if err := strategy.Apply(req); err != nil {
			t.Fatalf("expected apply ok, got %v", err)
		}
		authHeader := req.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Fatalf("expected bearer token, got %s", authHeader)
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("expected jwt with 3 parts, got %d", len(parts))
		}
	})
}

func TestSelector_RoundRobinSelector_Scenario(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		s := &auth.RoundRobinSelector{}
		cred := &auth.Credential{ID: "a"}
		got, err := s.Pick("p1", []*auth.Credential{cred})
		if err != nil {
			t.Fatalf("expected pick ok, got %v", err)
		}
		if got.ID != "a" {
			t.Fatalf("unexpected credential: %+v", got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		s := &auth.RoundRobinSelector{}
		creds := []*auth.Credential{
			{ID: "b"},
			{ID: "a"},
			{ID: "c", Disabled: true},
		}
		first, err := s.Pick("p1", creds)
		if err != nil {
			t.Fatalf("expected pick ok, got %v", err)
		}
		second, err := s.Pick("p1", creds)
		if err != nil {
			t.Fatalf("expected pick ok, got %v", err)
		}
		third, err := s.Pick("p1", creds)
		if err != nil {
			t.Fatalf("expected pick ok, got %v", err)
		}

		if first.ID != "a" || second.ID != "b" || third.ID != "a" {
			t.Fatalf("unexpected round robin order: %s, %s, %s", first.ID, second.ID, third.ID)
		}
	})

	t.Run("empty", func(t *testing.T) {
		s := &auth.RoundRobinSelector{}
		if _, err := s.Pick("p1", nil); err == nil {
			t.Fatalf("expected error for empty credentials")
		}
	})

	t.Run("all_disabled", func(t *testing.T) {
		s := &auth.RoundRobinSelector{}
		creds := []*auth.Credential{{ID: "a", Disabled: true}}
		if _, err := s.Pick("p1", creds); err == nil {
			t.Fatalf("expected error for no available credentials")
		}
	})
}
