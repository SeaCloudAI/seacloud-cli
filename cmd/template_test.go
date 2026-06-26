package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sandboxgo "github.com/SeaCloudAI/sandbox-go"
	sandboxapi "github.com/SeaCloudAI/seacloud-cli/internal/sandbox"
)

func TestBuildTemplateDefinitionFromImage(t *testing.T) {
	original := templateBuildOpts
	t.Cleanup(func() { templateBuildOpts = original })

	templateBuildOpts.image = "python:3.13"
	template, source, err := buildTemplateDefinition()
	if err != nil {
		t.Fatalf("buildTemplateDefinition returned error: %v", err)
	}
	if source != "python:3.13" {
		t.Fatalf("unexpected source %q", source)
	}
	jsonText, err := templateJSONForDryRun(template)
	if err != nil {
		t.Fatalf("templateJSONForDryRun returned error: %v", err)
	}
	if !strings.Contains(jsonText, `"fromImage": "python:3.13"`) {
		t.Fatalf("expected image in template json, got %s", jsonText)
	}
}

func TestBuildTemplateDefinitionFindsDockerfile(t *testing.T) {
	original := templateBuildOpts
	t.Cleanup(func() { templateBuildOpts = original })
	t.Chdir(t.TempDir())

	if err := os.WriteFile("Dockerfile", []byte("FROM ubuntu:22.04\nRUN echo ok\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	template, source, err := buildTemplateDefinition()
	if err != nil {
		t.Fatalf("buildTemplateDefinition returned error: %v", err)
	}
	if source != "Dockerfile" {
		t.Fatalf("unexpected source %q", source)
	}
	jsonText, err := templateJSONForDryRun(template)
	if err != nil {
		t.Fatalf("templateJSONForDryRun returned error: %v", err)
	}
	if !strings.Contains(jsonText, `"fromImage": "ubuntu:22.04"`) || !strings.Contains(jsonText, `"RUN"`) {
		t.Fatalf("unexpected template json: %s", jsonText)
	}
}

func TestWriteTemplateProject(t *testing.T) {
	dir := t.TempDir()
	if err := writeTemplateProject(dir, "python", "demo", false, true); err != nil {
		t.Fatalf("writeTemplateProject returned error: %v", err)
	}
	for _, name := range []string{"template.py", "build_dev.py", "build_prod.py", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	if err := writeTemplateProject(dir, "python", "demo", false, true); err == nil {
		t.Fatal("expected overwrite without force to fail")
	}
}

func TestBuildTemplateWithClientUsesAuthToken(t *testing.T) {
	t.Setenv("SEACLOUD_API_KEY", "should-not-be-used")

	var sawTemplateCreate bool
	var sawBuildCreate bool
	var authHeaders []string
	var apiKeyHeaders []string
	var createBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		apiKeyHeaders = append(apiKeyHeaders, r.Header.Get("X-API-Key"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sandbox/v1/templates":
			sawTemplateCreate = true
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"build-initial","names":["demo"],"tags":["v1"],"public":false}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/sandbox/v1/templates/tpl-1/builds/build-"):
			sawBuildCreate = true
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/sandbox/v1/templates/tpl-1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"build-initial","names":["demo"],"public":false}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := sandboxapi.NewClient(sandboxapi.Options{BaseURL: server.URL, AuthToken: "login-token"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	wait := false
	info, err := buildTemplateWithClient(
		context.Background(),
		client,
		sandboxgo.NewTemplate().FromImage("python:3.13"),
		"demo:v1",
		&sandboxgo.TemplateBuildOptions{Wait: &wait},
	)
	if err != nil {
		t.Fatalf("buildTemplateWithClient returned error: %v", err)
	}
	if !sawTemplateCreate || !sawBuildCreate {
		t.Fatalf("expected template and build create, saw template=%t build=%t", sawTemplateCreate, sawBuildCreate)
	}
	if info.TemplateID != "tpl-1" || info.Name != "demo" || info.Status != "building" {
		t.Fatalf("unexpected build info: %+v", info)
	}
	for i := range authHeaders {
		if authHeaders[i] != "Bearer login-token" || apiKeyHeaders[i] != "" {
			t.Fatalf("unexpected auth headers auth=%q apiKey=%q", authHeaders[i], apiKeyHeaders[i])
		}
	}
	if got := os.Getenv("SEACLOUD_API_KEY"); got != "should-not-be-used" {
		t.Fatalf("SEACLOUD_API_KEY was mutated: %q", got)
	}
	if createBody["name"] != "demo" {
		t.Fatalf("unexpected create body: %+v", createBody)
	}
}
