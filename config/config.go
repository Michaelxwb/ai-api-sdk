package config

import (
	"github.com/Michaelxwb/ai-api-sdk/auth"
)

// Config is the top-level configuration.
type Config struct {
	Auth        AuthConfig         `yaml:"auth"`
	Providers   []ProviderConfig   `yaml:"providers"`
	Credentials []*auth.Credential `yaml:"credentials"`
}

// AuthConfig defines auth store settings.
type AuthConfig struct {
	Store StoreConfig `yaml:"store"`
}

// StoreConfig defines credential store settings.
type StoreConfig struct {
	Type       string           `yaml:"type"`
	Path       string           `yaml:"path"`
	Encrypted  bool             `yaml:"encrypted"`
	Encryption EncryptionConfig `yaml:"encryption"`
}

// EncryptionConfig defines encryption settings.
type EncryptionConfig struct {
	Enabled         bool      `yaml:"enabled"`
	Algo            string    `yaml:"algo"`
	KDF             string    `yaml:"kdf"`
	KDFParams       KDFParams `yaml:"kdf_params"`
	MasterKeySource string    `yaml:"master_key_source"`
	MasterKeyFile   string    `yaml:"master_key_file"`
	MasterKeyEnv    string    `yaml:"master_key_env"`
}

// KDFParams configures scrypt.
type KDFParams struct {
	N      int `yaml:"n"`
	R      int `yaml:"r"`
	P      int `yaml:"p"`
	KeyLen int `yaml:"key_len"`
}

// ProviderConfig configures a provider instance.
type ProviderConfig struct {
	Name      string            `yaml:"name"`
	Type      string            `yaml:"type"`
	BaseURL   string            `yaml:"base_url"`
	Path      string            `yaml:"path,omitempty"` // override endpoint path (e.g. "/chat/completions")
	AuthRef   string            `yaml:"auth_ref"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	ExtraBody map[string]any    `yaml:"extra_body,omitempty"` // extra fields merged into request body

	// GenericProfile holds Generic Adapter configuration for non-standard API providers.
	// Used when Type is "generic" to define request/response mapping templates.
	GenericProfile map[string]any `yaml:"generic_profile,omitempty"`
}

// FindProvider returns provider config by name.
func (c *Config) FindProvider(name string) *ProviderConfig {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}
