package generation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestGetTaskRoutesThroughFolkosProxyForVtrixBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/model/v1/generation/task/task-123" {
			t.Fatalf("expected proxied task path, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer managed-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		if got := r.Header.Get("X-Folkos-Exec-Token"); got != "" {
			t.Fatalf("expected no legacy X-Folkos-Exec-Token header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-123","status":"completed","model":"gpt_image_1","output":[],"created_at":1710000000}`))
	}))
	defer server.Close()

	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = server.URL + "/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv("SEACLOUD_GENERATION_URL", "https://cloud.vtrix.ai")
	t.Setenv(config.EnvFolkosExecToken, "managed-token")
	BaseURL = ""

	task, err := NewClient("managed-token").GetTask("task-123")
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if task.ID != "task-123" {
		t.Fatalf("expected task id %q, got %q", "task-123", task.ID)
	}
}

func TestGetTaskPrefersGatewayURLOverBuildProxyBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/model/v1/generation/task/task-123" {
			t.Fatalf("expected proxied task path, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer managed-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-123","status":"completed","model":"gpt_image_1","output":[],"created_at":1710000000}`))
	}))
	defer server.Close()

	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = "https://gateway.example.com/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv(config.EnvGatewayURL, server.URL)
	t.Setenv("SEACLOUD_GENERATION_URL", "https://cloud.vtrix.ai")
	t.Setenv(config.EnvFolkosExecToken, "managed-token")
	t.Setenv(config.EnvSeaCloudRuntime, config.RuntimeFolkos)
	BaseURL = ""

	task, err := NewClient("managed-token").GetTask("task-123")
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if task.ID != "task-123" {
		t.Fatalf("expected task id %q, got %q", "task-123", task.ID)
	}
}
