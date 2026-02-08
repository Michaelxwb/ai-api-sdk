package test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"gopkg.in/yaml.v3"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func validConfigYAML() string {
	return `
auth:
  store:
    type: file
    path: /tmp/cred.json
    encrypted: true
    encryption:
      enabled: true
      algo: aes-256-gcm
      kdf: scrypt
      kdf_params:
        n: 32768
        r: 8
        p: 1
        key_len: 32
      master_key_source: file
      master_key_file: /tmp/master.key
      master_key_env: MASTER_KEY
providers:
  - name: primary
    type: openai
    base_url: https://api.openai.com/v1
    path: /chat/completions
    auth_ref: main
    headers:
      X-Test: yes
      X-Region: us
    extra_body:
      mode: fast
      nested:
        enabled: true
credentials:
  - id: main
    provider: openai
    auth_type: api_key
    api_key: secret-key
    headers:
      X-Cred: cred
    query_params:
      qp: qv
    priority: 3
    disabled: true
    metadata:
      team: sdk
`
}

func TestLoadConfig_Success(t *testing.T) {
	path := writeTempConfig(t, validConfigYAML())

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if cfg.Auth.Store.Type != "file" {
		t.Fatalf("Auth.Store.Type = %q, want %q", cfg.Auth.Store.Type, "file")
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("Providers length = %d, want 1", len(cfg.Providers))
	}
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Credentials length = %d, want 1", len(cfg.Credentials))
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")

	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error message missing path: %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wrapped os.ErrNotExist, got: %v", err)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, "auth: not_a_map\n")

	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error message missing path: %v", err)
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Fatalf("expected yaml parse error, got: %v", err)
	}
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("expected wrapped yaml.TypeError, got: %v", err)
	}
}

func TestConfig_FieldMapping(t *testing.T) {
	path := writeTempConfig(t, validConfigYAML())

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Auth.Store.Path != "/tmp/cred.json" {
		t.Fatalf("Auth.Store.Path = %q, want %q", cfg.Auth.Store.Path, "/tmp/cred.json")
	}

	if len(cfg.Credentials) != 1 {
		t.Fatalf("Credentials length = %d, want 1", len(cfg.Credentials))
	}
	cred := cfg.Credentials[0]
	if cred.ID != "main" {
		t.Fatalf("Credential.ID = %q, want %q", cred.ID, "main")
	}
	if cred.Provider != "openai" {
		t.Fatalf("Credential.Provider = %q, want %q", cred.Provider, "openai")
	}
	if cred.AuthType != auth.AuthTypeAPIKey {
		t.Fatalf("Credential.AuthType = %q, want %q", cred.AuthType, auth.AuthTypeAPIKey)
	}
	if cred.APIKey != "secret-key" {
		t.Fatalf("Credential.APIKey = %q, want %q", cred.APIKey, "secret-key")
	}
	if cred.Headers["X-Cred"] != "cred" {
		t.Fatalf("Credential.Headers[X-Cred] = %q, want %q", cred.Headers["X-Cred"], "cred")
	}
	if cred.QueryParams["qp"] != "qv" {
		t.Fatalf("Credential.QueryParams[qp] = %q, want %q", cred.QueryParams["qp"], "qv")
	}
	if cred.Priority != 3 {
		t.Fatalf("Credential.Priority = %d, want %d", cred.Priority, 3)
	}
	if !cred.Disabled {
		t.Fatalf("Credential.Disabled = %v, want true", cred.Disabled)
	}
	if cred.Metadata["team"] != "sdk" {
		t.Fatalf("Credential.Metadata[team] = %v, want %q", cred.Metadata["team"], "sdk")
	}
}

func TestConfig_ProviderConfigFields(t *testing.T) {
	path := writeTempConfig(t, validConfigYAML())

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("Providers length = %d, want 1", len(cfg.Providers))
	}
	p := cfg.Providers[0]

	if p.Name != "primary" {
		t.Fatalf("Provider.Name = %q, want %q", p.Name, "primary")
	}
	if p.Type != "openai" {
		t.Fatalf("Provider.Type = %q, want %q", p.Type, "openai")
	}
	if p.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("Provider.BaseURL = %q, want %q", p.BaseURL, "https://api.openai.com/v1")
	}
	if p.Path != "/chat/completions" {
		t.Fatalf("Provider.Path = %q, want %q", p.Path, "/chat/completions")
	}
	if p.AuthRef != "main" {
		t.Fatalf("Provider.AuthRef = %q, want %q", p.AuthRef, "main")
	}
	if p.Headers["X-Test"] != "yes" || p.Headers["X-Region"] != "us" {
		t.Fatalf("Provider.Headers = %v, want X-Test=yes and X-Region=us", p.Headers)
	}
	if p.ExtraBody["mode"] != "fast" {
		t.Fatalf("Provider.ExtraBody[mode] = %v, want %q", p.ExtraBody["mode"], "fast")
	}
	nested, ok := p.ExtraBody["nested"].(map[string]any)
	if !ok {
		t.Fatalf("Provider.ExtraBody[nested] type = %T, want map[string]any", p.ExtraBody["nested"])
	}
	if enabled, ok := nested["enabled"].(bool); !ok || !enabled {
		t.Fatalf("Provider.ExtraBody[nested].enabled = %v, want true", nested["enabled"])
	}
}

func TestConfig_AuthStoreEncryptionParsing(t *testing.T) {
	path := writeTempConfig(t, validConfigYAML())

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	enc := cfg.Auth.Store.Encryption
	if !enc.Enabled {
		t.Fatalf("Auth.Store.Encryption.Enabled = %v, want true", enc.Enabled)
	}
	if enc.Algo != "aes-256-gcm" {
		t.Fatalf("Auth.Store.Encryption.Algo = %q, want %q", enc.Algo, "aes-256-gcm")
	}
	if enc.KDF != "scrypt" {
		t.Fatalf("Auth.Store.Encryption.KDF = %q, want %q", enc.KDF, "scrypt")
	}
	if enc.KDFParams.N != 32768 || enc.KDFParams.R != 8 || enc.KDFParams.P != 1 || enc.KDFParams.KeyLen != 32 {
		t.Fatalf("Auth.Store.Encryption.KDFParams = %+v, want N=32768 R=8 P=1 KeyLen=32", enc.KDFParams)
	}
	if enc.MasterKeySource != "file" {
		t.Fatalf("Auth.Store.Encryption.MasterKeySource = %q, want %q", enc.MasterKeySource, "file")
	}
	if enc.MasterKeyFile != "/tmp/master.key" {
		t.Fatalf("Auth.Store.Encryption.MasterKeyFile = %q, want %q", enc.MasterKeyFile, "/tmp/master.key")
	}
	if enc.MasterKeyEnv != "MASTER_KEY" {
		t.Fatalf("Auth.Store.Encryption.MasterKeyEnv = %q, want %q", enc.MasterKeyEnv, "MASTER_KEY")
	}
}
