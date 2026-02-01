package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Manager handles credential selection and refresh.
type Manager struct {
	store    CredentialStore
	selector Selector

	mu       sync.RWMutex
	cache    map[string]*Credential
	failures map[string]time.Time
	cooldown time.Duration
}

// NewManager creates a Manager and loads credentials from the store.
func NewManager(store CredentialStore, selector Selector) (*Manager, error) {
	if selector == nil {
		selector = &RoundRobinSelector{}
	}
	m := &Manager{
		store:    store,
		selector: selector,
		cache:    make(map[string]*Credential),
		failures: make(map[string]time.Time),
		cooldown: time.Minute,
	}
	if store != nil {
		creds, err := store.Load()
		if err != nil {
			return nil, err
		}
		for _, c := range creds {
			if c == nil || c.ID == "" {
				continue
			}
			m.cache[c.ID] = c
		}
	}
	return m, nil
}

// Resolve picks a credential for the provider and returns its auth strategy.
func (m *Manager) Resolve(provider string) (*Credential, AuthStrategy, error) {
	m.mu.RLock()
	creds := m.byProviderLocked(provider)
	m.mu.RUnlock()

	if len(creds) == 0 {
		return nil, nil, fmt.Errorf("auth manager: no credentials for provider %s", provider)
	}
	cred, err := m.selector.Pick(provider, creds)
	if err != nil {
		return nil, nil, err
	}
	if cred == nil {
		return nil, nil, errors.New("auth manager: selector returned nil")
	}
	if m.isInCooldown(cred.ID) {
		return nil, nil, fmt.Errorf("auth manager: credential %s cooling down", cred.ID)
	}
	return cred, NewStrategyFromCredential(cred), nil
}

// Get returns a credential by id.
func (m *Manager) Get(id string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cred := m.cache[id]
	if cred == nil {
		return nil, fmt.Errorf("auth manager: credential %s not found", id)
	}
	return cred, nil
}

// Register adds or updates a credential in the cache.
func (m *Manager) Register(cred *Credential) {
	if cred == nil || cred.ID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[cred.ID] = cred
}

// List returns all credentials.
func (m *Manager) List() []*Credential {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Credential, 0, len(m.cache))
	for _, c := range m.cache {
		out = append(out, c)
	}
	return out
}

// MarkFailed marks a credential as temporarily failing.
func (m *Manager) MarkFailed(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[id] = time.Now().Add(m.cooldown)
}

// MarkSuccess clears failure cooldown.
func (m *Manager) MarkSuccess(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, id)
}

// RefreshOAuth refreshes OAuth credential lazily when needed.
func (m *Manager) RefreshOAuth(ctx context.Context, cred *Credential) error {
	if cred == nil || cred.AuthType != AuthTypeOAuth {
		return nil
	}
	if cred.RefreshToken == "" {
		return errors.New("auth manager: missing refresh token")
	}
	if cred.Metadata == nil {
		return errors.New("auth manager: missing oauth metadata")
	}
	endpoint, _ := cred.Metadata["token_url"].(string)
	clientID, _ := cred.Metadata["client_id"].(string)
	clientSecret, _ := cred.Metadata["client_secret"].(string)
	if endpoint == "" || clientID == "" {
		return errors.New("auth manager: oauth metadata incomplete")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", cred.RefreshToken)
	form.Set("client_id", clientID)
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("auth manager: refresh failed status %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.AccessToken == "" {
		return errors.New("auth manager: refresh missing access_token")
	}

	now := time.Now()
	cred.AccessToken = payload.AccessToken
	if payload.RefreshToken != "" {
		cred.RefreshToken = payload.RefreshToken
	}
	if payload.ExpiresIn > 0 {
		exp := now.Add(time.Duration(payload.ExpiresIn) * time.Second)
		cred.ExpiresAt = &exp
	}
	m.mu.Lock()
	m.cache[cred.ID] = cred
	m.mu.Unlock()
	if m.store != nil {
		return m.persistLocked()
	}
	return nil
}

// isInCooldown checks failure cooldown.
func (m *Manager) isInCooldown(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	until, ok := m.failures[id]
	if !ok {
		return false
	}
	return time.Now().Before(until)
}

func (m *Manager) byProviderLocked(provider string) []*Credential {
	out := make([]*Credential, 0)
	for _, c := range m.cache {
		if c == nil || c.Disabled {
			continue
		}
		if c.Provider == "" || strings.EqualFold(c.Provider, provider) {
			out = append(out, c)
		}
	}
	return out
}

func (m *Manager) persistLocked() error {
	if m.store == nil {
		return nil
	}
	creds := make([]*Credential, 0, len(m.cache))
	for _, c := range m.cache {
		creds = append(creds, c)
	}
	return m.store.Save(creds)
}
