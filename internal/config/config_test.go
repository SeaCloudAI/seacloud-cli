package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetTokenStoreForTest(t *testing.T) {
	t.Helper()
	storeOnce = sync.Once{}
	store = nil
}

func writeConfigFile(t *testing.T, home, content string) {
	t.Helper()
	path := filepath.Join(home, ".config", "seacloud", "config.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}

func TestLoadStoredWithoutManagedEnv(t *testing.T) {
	resetTokenStoreForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	t.Setenv(EnvFolkosExecToken, "")
	t.Setenv(EnvSeaCloudRuntime, "")
	writeConfigFile(t, home, "auth_token: stored-auth\nrefresh_token: stored-refresh\napi_key: stored-key\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Managed {
		t.Fatalf("expected unmanaged config")
	}
	if cfg.AuthToken != "stored-auth" || cfg.RefreshToken != "stored-refresh" || cfg.APIKey != "stored-key" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadManagedExecTokenOverridesStoredCredentials(t *testing.T) {
	resetTokenStoreForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	t.Setenv(EnvFolkosExecToken, "exec-token")
	t.Setenv(EnvSeaCloudRuntime, RuntimeFolkos)
	writeConfigFile(t, home, "auth_token: stored-auth\nrefresh_token: stored-refresh\napi_key: stored-key\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Managed {
		t.Fatalf("expected managed config")
	}
	if cfg.Runtime != RuntimeFolkos {
		t.Fatalf("expected runtime %q, got %q", RuntimeFolkos, cfg.Runtime)
	}
	if cfg.CredentialSource != EnvFolkosExecToken {
		t.Fatalf("expected credential source %q, got %q", EnvFolkosExecToken, cfg.CredentialSource)
	}
	if cfg.AuthToken != "exec-token" || cfg.APIKey != "exec-token" {
		t.Fatalf("expected exec token override, got %+v", cfg)
	}
	if cfg.RefreshToken != "" {
		t.Fatalf("expected refresh token to be cleared in managed mode, got %q", cfg.RefreshToken)
	}

	stored, err := LoadStored()
	if err != nil {
		t.Fatalf("LoadStored returned error: %v", err)
	}
	if stored.AuthToken != "stored-auth" || stored.RefreshToken != "stored-refresh" || stored.APIKey != "stored-key" {
		t.Fatalf("unexpected stored config: %+v", stored)
	}
}
