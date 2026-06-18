package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountclient "github.com/SeaCloudAI/seacloud-cli/internal/account"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestAccountBalanceDefaultOutput(t *testing.T) {
	server := newBalanceServer(t, "128.50", "USD")
	defer server.Close()
	setupAccountCommandTest(t, server.URL, true)

	stdout, _, err := executeRoot(t, "account", "balance")
	if err != nil {
		t.Fatalf("account balance returned error: %v", err)
	}
	if !strings.Contains(stdout, "Balance: $128.50") {
		t.Fatalf("expected balance in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "Top up: https://cloud.seaart.ai/settings/credits") {
		t.Fatalf("expected top-up URL in stdout, got %q", stdout)
	}
}

func TestAccountBalanceOverdueOutput(t *testing.T) {
	server := newBalanceServer(t, "-12.30", "USD")
	defer server.Close()
	setupAccountCommandTest(t, server.URL, true)

	stdout, _, err := executeRoot(t, "account", "balance")
	if err != nil {
		t.Fatalf("account balance returned error: %v", err)
	}
	if !strings.Contains(stdout, "Balance: $-12.30") {
		t.Fatalf("expected overdue balance in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "\n  Hint: Account overdue. Top up at: https://cloud.seaart.ai/settings/credits") {
		t.Fatalf("expected overdue hint in stdout, got %q", stdout)
	}
}

func TestAccountBalanceJSONOutput(t *testing.T) {
	server := newBalanceServer(t, "128.50", "USD")
	defer server.Close()
	setupAccountCommandTest(t, server.URL, true)

	stdout, _, err := executeRoot(t, "account", "balance", "--output", "json")
	if err != nil {
		t.Fatalf("account balance returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode account balance json: %v\n%s", err, stdout)
	}
	if got["balance"] != 128.5 || got["currency"] != "USD" {
		t.Fatalf("unexpected balance json: %#v", got)
	}
	if got["top_up_url"] != "https://cloud.seaart.ai/settings/credits" {
		t.Fatalf("unexpected top-up URL: %#v", got)
	}
	if strings.Contains(stdout, "Hint") {
		t.Fatalf("json output must not contain hint, got %q", stdout)
	}
}

func TestAccountBalanceRequiresLogin(t *testing.T) {
	setupAccountCommandTest(t, "http://127.0.0.1:1", false)

	_, _, err := executeRoot(t, "account", "balance")
	if err == nil {
		t.Fatal("expected login error")
	}
	if got := err.Error(); got != "not logged in\n  Hint: Run: seacloud auth login" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func setupAccountCommandTest(t *testing.T, serviceURL string, saveAuth bool) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	t.Setenv(config.EnvFolkosExecToken, "")
	t.Setenv("SEACLOUD_BASE_URL", serviceURL)
	accountclient.BaseURL = ""
	dryRun = false
	accountBalanceOutput = ""
	if saveAuth {
		if err := config.Save(&config.Config{AuthToken: "token-123"}); err != nil {
			t.Fatalf("save config: %v", err)
		}
	}
}

func newBalanceServer(t *testing.T, balance, currency string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/balance" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected auth token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"balance":"` + balance + `","currency":"` + currency + `"}}`))
	}))
}
