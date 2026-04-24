package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	keyring "github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const keychainService = "seacloud-cli"

const (
	EnvFolkosExecToken = "FOLKOS_EXEC_TOKEN"
	EnvFolkosToken     = "FOLKOS_TOKEN"
	EnvSeaCloudRuntime = "SEACLOUD_RUNTIME"
	RuntimeFolkos      = "folkos"
)

// DefaultFolkosProxyBaseURL is the fixed Folkos proxy base used in managed runtimes.
var DefaultFolkosProxyBaseURL = "https://folkos-client.dev.folkos.ai/folkos-proxy"

// Config holds all credentials. Storage backend is transparent to callers.
type Config struct {
	AuthToken        string `yaml:"-"`
	RefreshToken     string `yaml:"-"`
	APIKey           string `yaml:"-"`
	Managed          bool   `yaml:"-"`
	Runtime          string `yaml:"-"`
	CredentialSource string `yaml:"-"`
}

// fileConfig is the on-disk YAML struct (all tokens excluded).
type fileConfig struct{}

// ── token store interface ─────────────────────────────────────────────────────

type tokenStore interface {
	loadTokens() (auth, refresh, apiKey string, err error)
	saveTokens(auth, refresh, apiKey string) error
	clearTokens() error
}

// ── keychain store ────────────────────────────────────────────────────────────

type keychainStore struct{}

func (k *keychainStore) loadTokens() (string, string, string, error) {
	auth, err := keyring.Get(keychainService, "auth_token")
	if err != nil {
		// No keychain entry — check for legacy file tokens and migrate
		if legacy := legacyFileTokens(); legacy != nil {
			if err2 := k.saveTokens(legacy[0], legacy[1], legacy[2]); err2 == nil {
				_ = clearLegacyFileTokens()
				return legacy[0], legacy[1], legacy[2], nil
			}
		}
		return "", "", "", nil // no tokens, treat as logged-out
	}
	refresh, _ := keyring.Get(keychainService, "refresh_token")
	apiKey, _ := keyring.Get(keychainService, "api_key")
	return auth, refresh, apiKey, nil
}

func (k *keychainStore) saveTokens(auth, refresh, apiKey string) error {
	if err := keyring.Set(keychainService, "auth_token", auth); err != nil {
		fmt.Fprintf(os.Stderr, "warning: keychain write failed (%v), falling back to file storage\n", err)
		return (&fileStore{}).saveTokens(auth, refresh, apiKey)
	}
	if refresh != "" {
		_ = keyring.Set(keychainService, "refresh_token", refresh)
	}
	if apiKey != "" {
		_ = keyring.Set(keychainService, "api_key", apiKey)
	}
	return nil
}

func (k *keychainStore) clearTokens() error {
	_ = keyring.Delete(keychainService, "auth_token")
	_ = keyring.Delete(keychainService, "refresh_token")
	_ = keyring.Delete(keychainService, "api_key")
	return nil
}

// ── file store (fallback) ─────────────────────────────────────────────────────

type fileStore struct{}

type fileTokens struct {
	AuthToken    string `yaml:"auth_token,omitempty"`
	RefreshToken string `yaml:"refresh_token,omitempty"`
	APIKey       string `yaml:"api_key,omitempty"`
}

func (f *fileStore) loadTokens() (string, string, string, error) {
	t := readFileTokens()
	if t == nil {
		return "", "", "", nil
	}
	return t.AuthToken, t.RefreshToken, t.APIKey, nil
}

func (f *fileStore) saveTokens(auth, refresh, apiKey string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	t := fileTokens{AuthToken: auth, RefreshToken: refresh, APIKey: apiKey}
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (f *fileStore) clearTokens() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ── factory ───────────────────────────────────────────────────────────────────

var (
	storeOnce sync.Once
	store     tokenStore
)

func newTokenStore() tokenStore {
	storeOnce.Do(func() {
		store = initTokenStore()
	})
	return store
}

func initTokenStore() tokenStore {
	if os.Getenv("SEACLOUD_NO_KEYCHAIN") == "1" {
		return &fileStore{}
	}
	if runtime.GOOS == "linux" && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return &fileStore{}
	}
	return &keychainStore{}
}

// ── public API ────────────────────────────────────────────────────────────────

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "seacloud", "config.yml"), nil
}

func Load() (*Config, error) {
	cfg, err := LoadStored()
	if err != nil {
		return nil, err
	}

	if token, source := managedTokenFromEnv(); token != "" {
		cfg.AuthToken = token
		cfg.RefreshToken = ""
		cfg.APIKey = token
		cfg.Managed = true
		cfg.Runtime = RuntimeFromEnv()
		if cfg.Runtime == "" {
			cfg.Runtime = RuntimeFolkos
		}
		cfg.CredentialSource = source
	}

	return cfg, nil
}

func LoadStored() (*Config, error) {
	auth, refresh, apiKey, err := newTokenStore().loadTokens()
	if err != nil {
		return nil, err
	}
	return &Config{AuthToken: auth, RefreshToken: refresh, APIKey: apiKey}, nil
}

func ExecTokenFromEnv() string {
	token, _ := managedTokenFromEnv()
	return token
}

func RuntimeFromEnv() string {
	return strings.TrimSpace(strings.ToLower(os.Getenv(EnvSeaCloudRuntime)))
}

func UseFolkosProxy() bool {
	if RuntimeFromEnv() == RuntimeFolkos {
		return true
	}
	token, _ := managedTokenFromEnv()
	return token != ""
}

func FolkosProxyBaseURL() string {
	if !UseFolkosProxy() {
		return ""
	}
	return normalizeAbsoluteURL(DefaultFolkosProxyBaseURL)
}

func RewriteURLThroughFolkosProxy(raw string) string {
	proxyBase := FolkosProxyBaseURL()
	if proxyBase == "" {
		return raw
	}

	target, err := url.Parse(raw)
	if err != nil || target.Host == "" {
		return raw
	}

	host := strings.ToLower(target.Hostname())
	if host != "vtrix.ai" && !strings.HasSuffix(host, ".vtrix.ai") {
		return raw
	}

	proxyURL, err := url.Parse(proxyBase)
	if err != nil || proxyURL.Host == "" {
		return raw
	}

	proxyURL.Path = strings.TrimRight(proxyURL.Path, "/") + "/" + strings.TrimLeft(target.Path, "/")
	proxyURL.RawQuery = target.RawQuery
	proxyURL.Fragment = target.Fragment
	return proxyURL.String()
}

func Save(cfg *Config) error {
	return newTokenStore().saveTokens(cfg.AuthToken, cfg.RefreshToken, cfg.APIKey)
}

func Clear() error {
	return newTokenStore().clearTokens()
}

// ── legacy migration helpers ──────────────────────────────────────────────────

func legacyFileTokens() []string {
	t := readFileTokens()
	if t == nil || t.AuthToken == "" {
		return nil
	}
	return []string{t.AuthToken, t.RefreshToken, t.APIKey}
}

func readFileTokens() *fileTokens {
	path, err := configPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var t fileTokens
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

func normalizeAbsoluteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return ""
	}

	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/")
}

func managedTokenFromEnv() (string, string) {
	if token := strings.TrimSpace(os.Getenv(EnvFolkosExecToken)); token != "" {
		return token, EnvFolkosExecToken
	}
	if token := strings.TrimSpace(os.Getenv(EnvFolkosToken)); token != "" {
		return token, EnvFolkosToken
	}
	return "", ""
}

func clearLegacyFileTokens() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(fileConfig{})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
