package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/spf13/cobra"
)

func saveLoginConfigForSandboxTest(t *testing.T, authToken, refreshToken, apiKey string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	t.Setenv(config.EnvFolkosExecToken, "")
	t.Setenv(config.EnvSeaCloudRuntime, "")
	if err := config.Save(&config.Config{AuthToken: authToken, RefreshToken: refreshToken, APIKey: apiKey}); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func TestNewSandboxClientUsesStoredLoginAuthToken(t *testing.T) {
	originalOpts := sandboxOpts
	t.Cleanup(func() { sandboxOpts = originalOpts })

	saveLoginConfigForSandboxTest(t, "login-auth-token", "refresh-token", "legacy-api-key")
	sandboxOpts.baseURL = "https://gateway.example.com"

	client, err := newSandboxClient()
	if err != nil {
		t.Fatalf("newSandboxClient returned error: %v", err)
	}
	if client.AuthToken() != "login-auth-token" {
		t.Fatalf("expected login auth token, got %q", client.AuthToken())
	}
	if client.BaseURL() != "https://gateway.example.com/api/sandbox/v1" {
		t.Fatalf("unexpected base URL %q", client.BaseURL())
	}
}

func TestSandboxListCommandUsesStoredLoginAuthorizationHeader(t *testing.T) {
	originalOpts := sandboxOpts
	originalListOpts := sandboxListOpts
	originalDryRun := dryRun
	t.Cleanup(func() {
		sandboxOpts = originalOpts
		sandboxListOpts = originalListOpts
		dryRun = originalDryRun
	})
	t.Setenv("SEACLOUD_API_KEY", "env-api-key-should-not-be-used")
	saveLoginConfigForSandboxTest(t, "login-auth-token", "refresh-token", "stored-api-key-should-not-be-used")

	var gotAuth string
	var gotAPIKey string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	dryRun = false
	sandboxOpts.baseURL = server.URL
	sandboxOpts.output = "table"
	sandboxListOpts = struct {
		state     []string
		metadata  []string
		limit     int
		nextToken string
	}{}

	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(context.Background())
	if err := sandboxListCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("sandbox list returned error: %v", err)
	}
	if gotPath != "/api/sandbox/v1/sandboxes" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotAuth != "Bearer login-auth-token" {
		t.Fatalf("unexpected authorization header %q", gotAuth)
	}
	if gotAPIKey != "" {
		t.Fatalf("expected X-API-Key to be omitted, got %q", gotAPIKey)
	}
}

func TestBuildCreateSandboxRequest(t *testing.T) {
	original := sandboxCreateOpts
	t.Cleanup(func() { sandboxCreateOpts = original })

	sandboxCreateOpts.timeout = 600
	sandboxCreateOpts.waitReady = true
	sandboxCreateOpts.autoPause = true
	sandboxCreateOpts.autoResume = true
	sandboxCreateOpts.metadata = []string{"app=agent,owner=test"}
	sandboxCreateOpts.env = []string{"A=1", "B=two"}
	sandboxCreateOpts.allowPublicTraffic = "true"
	sandboxCreateOpts.allowInternetAccess = "false"
	sandboxCreateOpts.allowOut = []string{"1.1.1.1,10.0.0.0/8"}
	sandboxCreateOpts.denyOut = []string{"8.8.8.8"}
	sandboxCreateOpts.volumeMounts = []string{"cache:/cache", "data=/data"}

	req, err := buildCreateSandboxRequest([]string{"base"})
	if err != nil {
		t.Fatalf("buildCreateSandboxRequest returned error: %v", err)
	}

	if req.TemplateID != "base" {
		t.Fatalf("expected template base, got %q", req.TemplateID)
	}
	if req.Timeout == nil || *req.Timeout != 600 {
		t.Fatalf("expected timeout 600, got %+v", req.Timeout)
	}
	if req.WaitReady == nil || !*req.WaitReady || req.AutoPause == nil || !*req.AutoPause || req.AutoResume == nil || !*req.AutoResume {
		t.Fatalf("expected lifecycle booleans to be set: %+v", req)
	}
	if !reflect.DeepEqual(req.Metadata, map[string]string{"app": "agent", "owner": "test"}) {
		t.Fatalf("unexpected metadata: %+v", req.Metadata)
	}
	if !reflect.DeepEqual(req.EnvVars, map[string]string{"A": "1", "B": "two"}) {
		t.Fatalf("unexpected env vars: %+v", req.EnvVars)
	}
	if req.AllowInternetAccess == nil || *req.AllowInternetAccess {
		t.Fatalf("expected allowInternetAccess false, got %+v", req.AllowInternetAccess)
	}
	if req.Network == nil || req.Network.AllowPublicTraffic == nil || !*req.Network.AllowPublicTraffic {
		t.Fatalf("expected public traffic network policy, got %+v", req.Network)
	}
	if !reflect.DeepEqual(req.Network.AllowOut, []string{"1.1.1.1", "10.0.0.0/8"}) {
		t.Fatalf("unexpected allowOut: %+v", req.Network.AllowOut)
	}
	if !reflect.DeepEqual(req.Network.DenyOut, []string{"8.8.8.8"}) {
		t.Fatalf("unexpected denyOut: %+v", req.Network.DenyOut)
	}
	if !reflect.DeepEqual(req.VolumeMounts, []control.VolumeMount{{Name: "cache", Path: "/cache"}, {Name: "data", Path: "/data"}}) {
		t.Fatalf("unexpected volume mounts: %+v", req.VolumeMounts)
	}
}

func TestShouldConnectAfterCreateSkipsAutomationModes(t *testing.T) {
	originalCreate := sandboxCreateOpts
	originalOpts := sandboxOpts
	t.Cleanup(func() {
		sandboxCreateOpts = originalCreate
		sandboxOpts = originalOpts
	})

	sandboxCreateOpts.noConnect = true
	if shouldConnectAfterCreate(nil) {
		t.Fatal("expected --no-connect to skip create connection")
	}

	sandboxCreateOpts.noConnect = false
	sandboxOpts.output = "json"
	if shouldConnectAfterCreate(nil) {
		t.Fatal("expected json output to skip create connection")
	}
}

func TestBuildNetworkUpdateBody(t *testing.T) {
	original := sandboxNetworkOpts
	t.Cleanup(func() { sandboxNetworkOpts = original })

	sandboxNetworkOpts.allowPublicTraffic = "false"
	sandboxNetworkOpts.allowInternetAccess = "true"
	sandboxNetworkOpts.allowOut = []string{"1.1.1.1,2.2.2.2"}
	sandboxNetworkOpts.denyOut = []string{"10.0.0.0/8"}

	body, err := buildNetworkUpdateBody()
	if err != nil {
		t.Fatalf("buildNetworkUpdateBody returned error: %v", err)
	}
	if body["allowPublicTraffic"] != false || body["allowInternetAccess"] != true {
		t.Fatalf("unexpected booleans: %+v", body)
	}
	if !reflect.DeepEqual(body["allowOut"], []string{"1.1.1.1", "2.2.2.2"}) {
		t.Fatalf("unexpected allowOut: %+v", body["allowOut"])
	}
	if !reflect.DeepEqual(body["denyOut"], []string{"10.0.0.0/8"}) {
		t.Fatalf("unexpected denyOut: %+v", body["denyOut"])
	}
}

func TestBuildWebhookUpdateRetryPolicy(t *testing.T) {
	original := webhookUpdateOpts
	t.Cleanup(func() { webhookUpdateOpts = original })

	webhookUpdateOpts.maxAttempts = 5
	webhookUpdateOpts.delaySeconds = []int{1, 5, 30}
	webhookUpdateOpts.deadLetterEnabled = "true"

	cmd := &cobra.Command{Use: "update"}
	cmd.Flags().Int("max-attempts", 0, "")
	cmd.Flags().IntSlice("delay-seconds", nil, "")
	cmd.Flags().String("dead-letter-enabled", "", "")
	if err := cmd.Flags().Set("max-attempts", "5"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("delay-seconds", "1,5,30"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("dead-letter-enabled", "true"); err != nil {
		t.Fatal(err)
	}

	policy, err := buildWebhookUpdateRetryPolicy(cmd)
	if err != nil {
		t.Fatalf("buildWebhookUpdateRetryPolicy returned error: %v", err)
	}
	if policy == nil || policy.MaxAttempts != 5 || !policy.DeadLetterEnabled {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	if !reflect.DeepEqual(policy.DelaySeconds, []int{1, 5, 30}) {
		t.Fatalf("unexpected delay seconds: %+v", policy.DelaySeconds)
	}
}
