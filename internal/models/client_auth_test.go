package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestListRequiresAPIKey(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "")
	if err := config.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}

	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1")
	BaseURL = ""

	_, err := NewClient().List(ListParams{})
	if err == nil || !strings.Contains(err.Error(), "API key not set") || !strings.Contains(err.Error(), "seacloud auth set-key") {
		t.Fatalf("expected no API key guidance, got %v", err)
	}
}

func TestListUsesStoredAPIKeyAuthHeader(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "")
	if err := config.Save(&config.Config{APIKey: "stored-api-key"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer stored-api-key" {
			t.Fatalf("Authorization = %q, want stored API key", got)
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

func TestListSendsIncludeCurlQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("include_curl"); got != "true" {
			t.Fatalf("include_curl = %q, want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
			"models":[{"id":"flux_2_pro","model_id":"flux_2_pro","name":"Flux","type":"image","description":"","input_modalities":[],"output_modalities":[],"curl":"curl https://provider.example/flux"}],
			"total":1,"page":1,"page_size":20,"total_pages":1
		}}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	result, err := NewClient().List(ListParams{IncludeCurl: true})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got := result.Models[0].Curl; got != "curl https://provider.example/flux" {
		t.Fatalf("Curl = %q, want fallback curl", got)
	}
}
