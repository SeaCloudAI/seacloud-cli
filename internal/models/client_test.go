package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestNewClientUsesExtendedTimeout(t *testing.T) {
	client := NewClient()
	if client.httpClient.Timeout != 30*time.Second {
		t.Fatalf("expected timeout %s, got %s", 30*time.Second, client.httpClient.Timeout)
	}
}

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

func TestListRoutesThroughFolkosProxyForVtrixModelsBaseURL(t *testing.T) {
	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = "http://folkos-gateway.dev.folkos.ai/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv(config.EnvFolkosExecToken, "exec-token")
	t.Setenv(config.EnvSeaCloudRuntime, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/api/v1/skill/models" {
			t.Fatalf("expected proxied models list path, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer exec-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":20,"total_pages":0}}`))
	}))
	defer server.Close()

	config.DefaultFolkosProxyBaseURL = server.URL + "/folkos-proxy"
	t.Setenv("SEACLOUD_MODELS_URL", "https://cloud-model-spec.vtrix.ai")
	BaseURL = ""

	if _, err := NewClient().List(ListParams{}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestGetSpecRewritesVtrixEndpointThroughFolkosProxy(t *testing.T) {
	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = "http://folkos-gateway.dev.folkos.ai/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv(config.EnvSeaCloudRuntime, config.RuntimeFolkos)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"gpt_image_1",
				"name":"GPT Image 1",
				"vendor":"openai",
				"type":"image",
				"api":{
					"endpoint":"https://cloud.vtrix.ai/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"POST https://cloud.vtrix.ai/model/v1/generation\nGET https://cloud.vtrix.ai/model/v1/generation/task/{{id}}"
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	spec, err := NewClient().GetSpec("gpt_image_1")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}

	wantEndpoint := "http://folkos-gateway.dev.folkos.ai/folkos-proxy/model/v1/generation"
	if spec.API.Endpoint != wantEndpoint {
		t.Fatalf("expected rewritten endpoint %q, got %q", wantEndpoint, spec.API.Endpoint)
	}
	if !strings.Contains(spec.AgentPrompt, wantEndpoint) {
		t.Fatalf("expected agent prompt to contain rewritten endpoint, got %q", spec.AgentPrompt)
	}
	if strings.Contains(spec.AgentPrompt, "https://cloud.vtrix.ai/model/v1/generation") {
		t.Fatalf("expected original vtrix endpoint to be removed from agent prompt, got %q", spec.AgentPrompt)
	}
}

func TestGetSpecRoutesThroughFolkosProxyForVtrixModelsBaseURL(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "exec-token")
	t.Setenv(config.EnvSeaCloudRuntime, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/api/v1/skill/models/gpt_image_1/spec" {
			t.Fatalf("expected proxied models spec path, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer exec-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"gpt_image_1",
				"name":"GPT Image 1",
				"vendor":"openai",
				"type":"image",
				"api":{
					"endpoint":"https://cloud.vtrix.ai/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"POST https://cloud.vtrix.ai/model/v1/generation"
			}
		}`))
	}))
	defer server.Close()

	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = server.URL + "/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv("SEACLOUD_MODELS_URL", "https://cloud-model-spec.vtrix.ai")
	BaseURL = ""

	spec, err := NewClient().GetSpec("gpt_image_1")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "gpt_image_1" {
		t.Fatalf("expected model id %q, got %q", "gpt_image_1", spec.ModelID)
	}
}

func TestGetSpecPrefersGatewayURLOverBuildProxyBase(t *testing.T) {
	t.Setenv(config.EnvFolkosExecToken, "exec-token")
	t.Setenv(config.EnvSeaCloudRuntime, config.RuntimeFolkos)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/api/v1/skill/models/gpt_image_1/spec" {
			t.Fatalf("expected proxied models spec path, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer exec-token" {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"gpt_image_1",
				"name":"GPT Image 1",
				"vendor":"openai",
				"type":"image",
				"api":{
					"endpoint":"https://cloud.vtrix.ai/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"POST https://cloud.vtrix.ai/model/v1/generation"
			}
		}`))
	}))
	defer server.Close()

	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = "http://folkos-gateway.dev.folkos.ai/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv(config.EnvGatewayURL, server.URL)
	t.Setenv("SEACLOUD_MODELS_URL", "https://cloud-model-spec.vtrix.ai")
	BaseURL = ""

	spec, err := NewClient().GetSpec("gpt_image_1")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "gpt_image_1" {
		t.Fatalf("expected model id %q, got %q", "gpt_image_1", spec.ModelID)
	}
}
