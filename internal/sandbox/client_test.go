package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveBaseURLAddsAPIV1ForGatewayRoot(t *testing.T) {
	t.Setenv(EnvSandboxURL, "")
	t.Setenv("SEACLOUD_BASE_URL", "")
	if got := resolveBaseURL(""); got != "https://cloud.seaart.ai/api/v1" {
		t.Fatalf("expected default cloud api root, got %q", got)
	}
	if got := resolveBaseURL("https://gateway.example.com/"); got != "https://gateway.example.com/api/v1" {
		t.Fatalf("expected api root, got %q", got)
	}
	if got := resolveBaseURL("https://gateway.example.com/api/v1/sandbox"); got != "https://gateway.example.com/api/v1/sandbox" {
		t.Fatalf("expected custom api root to stay unchanged, got %q", got)
	}
	if got := resolveBaseURL("https://gateway.example.com/api/sandbox/v1"); got != "https://gateway.example.com/api/sandbox/v1" {
		t.Fatalf("expected web proxy api root to stay unchanged, got %q", got)
	}
	if got := resolveBaseURL("https://gateway.example.com/api/v12"); got != "https://gateway.example.com/api/v12/api/v1" {
		t.Fatalf("expected non-v1 path to be extended, got %q", got)
	}
}

func TestUpdateNetworkUsesAPIPathHeadersAndEscapedSandboxID(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotAPIKey string
	var gotNamespace string
	var gotUserID string
	var gotProjectID string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-API-Key")
		gotNamespace = r.Header.Get("X-Namespace-ID")
		gotUserID = r.Header.Get("X-User-ID")
		gotProjectID = r.Header.Get("X-Project-ID")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(Options{
		BaseURL:     server.URL,
		AuthToken:   "unit-token",
		NamespaceID: "ns-1",
		UserID:      "user-1",
		ProjectID:   "project-1",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if err := client.UpdateNetwork(context.Background(), "sb/1", map[string]any{"allowInternetAccess": false}); err != nil {
		t.Fatalf("UpdateNetwork returned error: %v", err)
	}

	if gotPath != "/api/v1/sandboxes/sb%2F1/network" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotAuth != "Bearer unit-token" || gotAPIKey != "" {
		t.Fatalf("unexpected auth headers auth=%q apiKey=%q", gotAuth, gotAPIKey)
	}
	if gotNamespace != "ns-1" || gotUserID != "user-1" || gotProjectID != "project-1" {
		t.Fatalf("unexpected scope headers namespace=%q user=%q project=%q", gotNamespace, gotUserID, gotProjectID)
	}
	if gotBody["allowInternetAccess"] != false {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}
