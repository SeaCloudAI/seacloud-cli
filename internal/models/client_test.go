package models

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestClientAddsManagedAuthHeader(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "exec-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer exec-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":20,"total_pages":0}}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	if _, err := NewClient().List(ListParams{}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestClientOmitsAuthHeaderWithoutManagedToken(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected Authorization header to be empty, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":20,"total_pages":0}}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	if _, err := NewClient().List(ListParams{}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}
