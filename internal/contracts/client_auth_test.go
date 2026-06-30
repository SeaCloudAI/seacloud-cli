package contracts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestGetUsesStoredAPIKeyAuthHeader(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "")
	if err := config.Save(&config.Config{APIKey: "stored-api-key"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer stored-api-key" {
			t.Fatalf("Authorization = %q, want stored API key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	_, _ = NewClient().Get("missing")
}

func TestGetRequiresAPIKey(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "")
	if err := config.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}

	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1")
	BaseURL = ""

	_, err := NewClient().Get("gpt_image_1")
	if err == nil || !strings.Contains(err.Error(), "API key not set") || !strings.Contains(err.Error(), "seacloud auth set-key") {
		t.Fatalf("expected no API key guidance, got %v", err)
	}
}
