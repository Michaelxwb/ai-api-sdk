package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// CredentialStore abstracts credential persistence.
type CredentialStore interface {
	Load() ([]*Credential, error)
	Save(creds []*Credential) error
	List() ([]*Credential, error)
	Get(id string) (*Credential, error)
}

// FileStore persists credentials in a JSON file with optional AES-256-GCM encryption.
type FileStore struct {
	Path          string
	Encrypted     bool
	MasterKeyEnv  string
	MasterKeyFile string
	ScryptParams  ScryptParams
}

// ScryptParams defines scrypt KDF parameters.
type ScryptParams struct {
	N      int `json:"n"`
	R      int `json:"r"`
	P      int `json:"p"`
	KeyLen int `json:"key_len"`
}

// NewFileStore returns a FileStore with sensible defaults.
func NewFileStore(path string) *FileStore {
	return &FileStore{
		Path:         path,
		Encrypted:    true,
		ScryptParams: ScryptParams{N: 32768, R: 8, P: 1, KeyLen: 32},
	}
}

// Load loads credentials from disk.
func (s *FileStore) Load() ([]*Credential, error) {
	return s.List()
}

// List loads credentials from disk.
func (s *FileStore) List() ([]*Credential, error) {
	if strings.TrimSpace(s.Path) == "" {
		return nil, errors.New("credential store: path is empty")
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Credential{}, nil
		}
		return nil, fmt.Errorf("credential store: read failed: %w", err)
	}
	if len(data) == 0 {
		return []*Credential{}, nil
	}

	// Try to detect encrypted envelope
	var env fileEnvelope
	if err := json.Unmarshal(data, &env); err == nil && env.Ciphertext != "" {
		return s.decryptEnvelope(&env)
	}
	// Plain JSON array
	var creds []*Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("credential store: unmarshal failed: %w", err)
	}
	return creds, nil
}

// Get returns credential by id.
func (s *FileStore) Get(id string) (*Credential, error) {
	creds, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, c := range creds {
		if c != nil && c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("credential store: credential %s not found", id)
}

// Save writes credentials to disk.
func (s *FileStore) Save(creds []*Credential) error {
	if strings.TrimSpace(s.Path) == "" {
		return errors.New("credential store: path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("credential store: mkdir failed: %w", err)
	}
	if !s.Encrypted {
		plain, err := json.MarshalIndent(creds, "", "  ")
		if err != nil {
			return fmt.Errorf("credential store: marshal failed: %w", err)
		}
		if err := os.WriteFile(s.Path, plain, 0o600); err != nil {
			return fmt.Errorf("credential store: write failed: %w", err)
		}
		return nil
	}

	env, err := s.encryptEnvelope(creds)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("credential store: marshal envelope failed: %w", err)
	}
	if err := os.WriteFile(s.Path, encoded, 0o600); err != nil {
		return fmt.Errorf("credential store: write failed: %w", err)
	}
	return nil
}

type fileEnvelope struct {
	Version    int          `json:"version"`
	KDF        string       `json:"kdf"`
	KDFParams  ScryptParams `json:"kdf_params"`
	Salt       string       `json:"salt"`
	Nonce      string       `json:"nonce"`
	Ciphertext string       `json:"ciphertext"`
}

func (s *FileStore) encryptEnvelope(creds []*Credential) (*fileEnvelope, error) {
	masterKey, err := s.masterKey()
	if err != nil {
		return nil, err
	}
	plain, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("credential store: marshal failed: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("credential store: salt failed: %w", err)
	}
	key, err := scrypt.Key(masterKey, salt, s.ScryptParams.N, s.ScryptParams.R, s.ScryptParams.P, s.ScryptParams.KeyLen)
	if err != nil {
		return nil, fmt.Errorf("credential store: scrypt failed: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("credential store: cipher init failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("credential store: gcm init failed: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("credential store: nonce failed: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	return &fileEnvelope{
		Version:    1,
		KDF:        "scrypt",
		KDFParams:  s.ScryptParams,
		Salt:       base64.RawStdEncoding.EncodeToString(salt),
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (s *FileStore) decryptEnvelope(env *fileEnvelope) ([]*Credential, error) {
	masterKey, err := s.masterKey()
	if err != nil {
		return nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("credential store: decode salt failed: %w", err)
	}
	nonce, err := base64.RawStdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("credential store: decode nonce failed: %w", err)
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("credential store: decode ciphertext failed: %w", err)
	}
	key, err := scrypt.Key(masterKey, salt, env.KDFParams.N, env.KDFParams.R, env.KDFParams.P, env.KDFParams.KeyLen)
	if err != nil {
		return nil, fmt.Errorf("credential store: scrypt failed: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("credential store: cipher init failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("credential store: gcm init failed: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("credential store: decrypt failed: %w", err)
	}
	var creds []*Credential
	if err := json.Unmarshal(plain, &creds); err != nil {
		return nil, fmt.Errorf("credential store: unmarshal failed: %w", err)
	}
	return creds, nil
}

func (s *FileStore) masterKey() ([]byte, error) {
	if s.MasterKeyEnv != "" {
		if v := os.Getenv(s.MasterKeyEnv); v != "" {
			return []byte(v), nil
		}
	}
	if s.MasterKeyFile != "" {
		b, err := os.ReadFile(s.MasterKeyFile)
		if err != nil {
			return nil, fmt.Errorf("credential store: read master key file failed: %w", err)
		}
		return []byte(strings.TrimSpace(string(b))), nil
	}
	return nil, errors.New("credential store: master key not configured")
}
