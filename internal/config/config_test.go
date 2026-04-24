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

func setDefaultFolkosProxyBaseURLForTest(t *testing.T, value string) {
	t.Helper()
	original := DefaultFolkosProxyBaseURL
	DefaultFolkosProxyBaseURL = value
	t.Cleanup(func() {
		DefaultFolkosProxyBaseURL = original
	})
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

func TestFolkosProxyBaseURLUsesFixedURLForManagedToken(t *testing.T) {
	t.Setenv(EnvFolkosExecToken, "exec-token")
	t.Setenv(EnvSeaCloudRuntime, "")
	got := FolkosProxyBaseURL()
	want := "https://folkos-client.dev.folkos.ai/folkos-proxy"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFolkosProxyBaseURLUsesFixedURLForFolkosRuntime(t *testing.T) {
	t.Setenv(EnvFolkosExecToken, "")
	t.Setenv(EnvFolkosToken, "")
	t.Setenv(EnvSeaCloudRuntime, RuntimeFolkos)
	got := FolkosProxyBaseURL()
	want := "https://folkos-client.dev.folkos.ai/folkos-proxy"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFolkosProxyBaseURLIsDisabledOutsideFolkos(t *testing.T) {
	t.Setenv(EnvFolkosExecToken, "")
	t.Setenv(EnvFolkosToken, "")
	t.Setenv(EnvSeaCloudRuntime, "")
	if got := FolkosProxyBaseURL(); got != "" {
		t.Fatalf("expected empty proxy base URL outside folkos runtime, got %q", got)
	}
}

func TestRewriteURLThroughFolkosProxyRewritesOnlyVtrixEndpointsInFolkosRuntime(t *testing.T) {
	t.Setenv(EnvSeaCloudRuntime, RuntimeFolkos)
	got := RewriteURLThroughFolkosProxy("https://cloud.vtrix.ai/model/v1/generation?debug=1")
	want := "https://folkos-client.dev.folkos.ai/folkos-proxy/model/v1/generation?debug=1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	unchanged := RewriteURLThroughFolkosProxy("https://api.openai.com/v1/responses")
	if unchanged != "https://api.openai.com/v1/responses" {
		t.Fatalf("expected non-vtrix URL to remain unchanged, got %q", unchanged)
	}
}

func TestRewriteURLThroughFolkosProxyDoesNothingOutsideFolkosRuntime(t *testing.T) {
	t.Setenv(EnvFolkosExecToken, "")
	t.Setenv(EnvFolkosToken, "")
	t.Setenv(EnvSeaCloudRuntime, "")
	raw := "https://cloud.vtrix.ai/model/v1/generation?debug=1"
	if got := RewriteURLThroughFolkosProxy(raw); got != raw {
		t.Fatalf("expected URL to stay unchanged outside folkos runtime, got %q", got)
	}
}
