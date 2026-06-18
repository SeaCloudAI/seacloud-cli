package account

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/auth"
	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
)

func TestGetBalanceSendsAuthHeadersAndParsesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/balance" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/user/balance")
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		assertHeader(t, r, "Accept", "application/json")
		assertHeader(t, r, "Content-Type", "application/json")
		assertHeader(t, r, "User-Agent", buildinfo.UserAgent())
		assertHeader(t, r, "Authorization", "Bearer token-123")
		assertHeader(t, r, "X-Auth-Priority", "auth_token")
		assertHeader(t, r, "X-Source", "cli")
		assertHeader(t, r, "X-App-Id", auth.AppID)
		assertHeader(t, r, "X-Version", buildinfo.Version)
		assertHeader(t, r, "X-Plat", "cli")
		assertHeader(t, r, "X-Device-Type", "cli")
		assertHeader(t, r, "X-Skip-Nextauth", "true")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"balance":"128.50","currency":"USD","total_recharged":"200.00"}}`))
	}))
	defer server.Close()

	setBaseURLForTest(t, server.URL)

	client := NewClient("token-123")
	balance, err := client.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if balance.Balance != "128.50" {
		t.Fatalf("Balance = %q, want %q", balance.Balance, "128.50")
	}
	if balance.Currency != "USD" {
		t.Fatalf("Currency = %q, want %q", balance.Currency, "USD")
	}
}

func TestGetBalanceMapsUnauthorizedToLoginHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":401,"message":"unauthorized"},"data":null}`))
	}))
	defer server.Close()

	setBaseURLForTest(t, server.URL)

	_, err := NewClient("token-123").GetBalance()
	if err == nil {
		t.Fatal("GetBalance() error = nil, want error")
	}

	want := "session expired\n  Hint: Run: seacloud auth login"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestGetBalanceMapsHTTPUnauthorizedToLoginHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Missing Token"}`))
	}))
	defer server.Close()

	setBaseURLForTest(t, server.URL)

	_, err := NewClient("token-123").GetBalance()
	if err == nil {
		t.Fatal("GetBalance() error = nil, want error")
	}

	want := "session expired\n  Hint: Run: seacloud auth login"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func setBaseURLForTest(t *testing.T, baseURL string) {
	t.Helper()
	old := BaseURL
	t.Cleanup(func() {
		BaseURL = old
	})
	t.Setenv("SEACLOUD_BASE_URL", baseURL)
	BaseURL = ""
}

func assertHeader(t *testing.T, r *http.Request, name, want string) {
	t.Helper()
	if got := r.Header.Get(name); got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}
