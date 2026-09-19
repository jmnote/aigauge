package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store defines the interface for persisting and retrieving provider tokens.
type Store interface {
	GetToken(provider string) (*Token, error)
	SaveToken(provider string, token *Token) error
	DeleteToken(provider string) error
	ListTokens() (map[string]*Token, error)
	Clear() error
	IsInitialized() bool
}

// fileStore persists tokens as a single JSON file, running the bytes through
// encrypt/decrypt on the way to/from disk. It is the whole Store
// implementation for both platforms - storage_windows.go and
// storage_other.go each only supply an encrypt/decrypt pair (DPAPI vs. the
// identity function) and an init() that wires one up as the default store,
// since encryption-at-rest is the only part of a token store that is
// genuinely platform-specific.
type fileStore struct {
	mu       sync.RWMutex
	filePath string
	encrypt  func([]byte) ([]byte, error)
	decrypt  func([]byte) ([]byte, error)
}

func newFileStore(filePath string, encrypt, decrypt func([]byte) ([]byte, error)) *fileStore {
	return &fileStore{filePath: filePath, encrypt: encrypt, decrypt: decrypt}
}

func (s *fileStore) loadTokens() (map[string]*Token, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Token), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]*Token), nil
	}

	decrypted, err := s.decrypt(data)
	if err != nil {
		return nil, err
	}

	tokens := make(map[string]*Token)
	if err := json.Unmarshal(decrypted, &tokens); err != nil {
		return nil, fmt.Errorf("parse tokens: %w", err)
	}
	return tokens, nil
}

func (s *fileStore) saveTokens(tokens map[string]*Token) error {
	raw, err := json.Marshal(tokens)
	if err != nil {
		return err
	}

	encrypted, err := s.encrypt(raw)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Keep the encrypted blob intact if the process crashes during a write.
	// A truncated credential file cannot be partially recovered and would
	// force every provider instance to be authenticated again.
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, encrypted, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

func (s *fileStore) GetToken(provider string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens, err := s.loadTokens()
	if err != nil {
		return nil, err
	}
	return tokens[provider], nil
}

func (s *fileStore) SaveToken(provider string, token *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens, err := s.loadTokens()
	if err != nil {
		return err
	}
	tokens[provider] = token
	return s.saveTokens(tokens)
}

func (s *fileStore) DeleteToken(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens, err := s.loadTokens()
	if err != nil {
		return err
	}
	delete(tokens, provider)
	return s.saveTokens(tokens)
}

func (s *fileStore) ListTokens() (map[string]*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadTokens()
}

func (s *fileStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveTokens(make(map[string]*Token))
}

func (s *fileStore) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.filePath)
	return err == nil
}

var (
	defaultStoreMu sync.RWMutex
	defaultStore   Store
)

// SetDefaultStore configures the global store instance.
func SetDefaultStore(s Store) {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()
	defaultStore = s
}

// GetDefaultStore returns the configured global store instance.
func GetDefaultStore() Store {
	defaultStoreMu.RLock()
	defer defaultStoreMu.RUnlock()
	return defaultStore
}

// GetToken retrieves the token for the given provider from the default store.
func GetToken(provider string) (*Token, error) {
	s := GetDefaultStore()
	if s == nil {
		return nil, nil
	}
	return s.GetToken(provider)
}

// SaveToken persists the token for the given provider in the default store.
func SaveToken(provider string, token *Token) error {
	s := GetDefaultStore()
	if s == nil {
		return nil
	}
	return s.SaveToken(provider, token)
}

// DeleteToken removes the token for the given provider from the default store.
func DeleteToken(provider string) error {
	s := GetDefaultStore()
	if s == nil {
		return nil
	}
	return s.DeleteToken(provider)
}

// ListTokens returns all stored provider tokens from the default store.
func ListTokens() (map[string]*Token, error) {
	s := GetDefaultStore()
	if s == nil {
		return make(map[string]*Token), nil
	}
	return s.ListTokens()
}

// Clear removes all provider tokens from the default store.
func Clear() error {
	s := GetDefaultStore()
	if s == nil {
		return nil
	}
	return s.Clear()
}

// IsInitialized reports whether the default store has been initialized.
func IsInitialized() bool {
	s := GetDefaultStore()
	if s == nil {
		return false
	}
	return s.IsInitialized()
}

// CanImportCredentialsFile reports whether on-disk provider credentials exist for
// the given provider type (e.g. "claude", "codex", "antigravity").
func CanImportCredentialsFile(providerType string) bool {
	return readCredentialsFileToken(providerType) != nil
}

// ImportCredentialsFile imports on-disk provider credentials for providerType
// (what CLI/file format to read) and stores the resulting token under
// tokenKey (the provider *instance* the credentials are being attached to).
// The two differ once a type can have several instances: reading is always
// type-specific, but storage must not collide with another instance of the
// same type.
func ImportCredentialsFile(providerType, tokenKey string) (*Token, error) {
	tok := readCredentialsFileToken(providerType)
	if tok == nil {
		return nil, fmt.Errorf("no local credentials found for %s", providerType)
	}
	s := GetDefaultStore()
	if s != nil {
		if err := s.SaveToken(tokenKey, tok); err != nil {
			return nil, err
		}
	}
	return tok, nil
}

// ReadCredentialsFile reads the provider's native local credential file.
// Callers that persist or display the data are responsible for protecting
// sensitive values before doing so.
func ReadCredentialsFile(provider string) ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var path string
	switch provider {
	case "claude":
		path = filepath.Join(home, ".claude", ".credentials.json")
	case "codex":
		path = filepath.Join(home, ".codex", "auth.json")
	default:
		return nil, fmt.Errorf("unsupported credentials file provider %q", provider)
	}
	return os.ReadFile(path)
}

func readCredentialsFileToken(provider string) *Token {
	switch provider {
	case "claude":
		data, err := ReadCredentialsFile(provider)
		if err != nil {
			return nil
		}
		return parseClaudeCredentials(data)

	case "codex":
		data, err := ReadCredentialsFile(provider)
		if err != nil {
			return nil
		}
		return parseCodexCredentials(data)
	}
	// No "antigravity" case: reading agy's local token store to call Google's
	// API directly would be the same third-party-tool pattern this app avoids
	// by shelling out to the agy CLI instead (see
	// internal/providers/antigravity.go).
	return nil
}

func parseClaudeCredentials(data []byte) *Token {
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken      string `json:"accessToken"`
			RefreshToken     string `json:"refreshToken"`
			ExpiresAt        int64  `json:"expiresAt"`
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
		OrganizationUUID string `json:"organizationUuid"`
	}
	if err := json.Unmarshal(data, &creds); err == nil && creds.ClaudeAiOauth.AccessToken != "" {
		tok := &Token{
			AccessToken:  creds.ClaudeAiOauth.AccessToken,
			RefreshToken: creds.ClaudeAiOauth.RefreshToken,
			Extra: Extra{
				Plan:             creds.ClaudeAiOauth.SubscriptionType,
				OrganizationUUID: creds.OrganizationUUID,
			},
		}
		if creds.ClaudeAiOauth.ExpiresAt > 0 {
			if creds.ClaudeAiOauth.ExpiresAt > 1e11 {
				tok.ExpiresAt = time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt)
			} else {
				tok.ExpiresAt = time.Unix(creds.ClaudeAiOauth.ExpiresAt, 0)
			}
		}
		return tok
	}
	return nil
}

func parseCodexCredentials(data []byte) *Token {
	var creds struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &creds); err == nil && creds.Tokens.AccessToken != "" {
		tok := &Token{
			AccessToken:  creds.Tokens.AccessToken,
			RefreshToken: creds.Tokens.RefreshToken,
		}
		if exp := parseJWTExpiresAt(creds.Tokens.AccessToken); !exp.IsZero() {
			tok.ExpiresAt = exp
		}
		return tok
	}
	return nil
}

func parseJWTExpiresAt(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payloadSegment := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		if decoded, err = base64.URLEncoding.DecodeString(payloadSegment); err != nil {
			if pad := len(payloadSegment) % 4; pad != 0 {
				payloadSegment += strings.Repeat("=", 4-pad)
				decoded, err = base64.URLEncoding.DecodeString(payloadSegment)
			}
			if err != nil {
				return time.Time{}
			}
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
