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

func TestListUsesSkillModelsEndpointWithFiltersAndFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models" {
			t.Fatalf("expected skill models list path, got %q", got)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Fatalf("expected page=2, got %q", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "5" {
			t.Fatalf("expected page_size=5, got %q", got)
		}
		if got := r.URL.Query().Get("type"); got != "image" {
			t.Fatalf("expected type=image, got %q", got)
		}
		if got := r.URL.Query().Get("keywords"); got != "flux" {
			t.Fatalf("expected keywords=flux, got %q", got)
		}
		if got := r.URL.Query().Get("provider"); got != "blackforestlabs" {
			t.Fatalf("expected provider=blackforestlabs, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
			"models":[{
				"id":"flux_2_pro",
				"model_id":"flux_2_pro",
				"name":"FLUX.2 [pro]",
				"type":"image",
				"provider":"blackforestlabs",
				"source_collection":"multi_models_new",
				"original_model_id":"flux_2_pro",
				"model_subtype":"pro",
				"description":"image model",
				"input_modalities":["Text"],
				"output_modalities":["Image"]
			}],
			"total":1,"page":2,"page_size":5,"total_pages":1
		}}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	result, err := NewClient().List(ListParams{
		Page:     2,
		PageSize: 5,
		Type:     "image",
		Keywords: "flux",
		Provider: "blackforestlabs",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("expected one model, got %#v", result.Models)
	}
	model := result.Models[0]
	if model.ID != "flux_2_pro" || model.ModelID != "flux_2_pro" {
		t.Fatalf("id/model_id = %q/%q, want flux_2_pro/flux_2_pro", model.ID, model.ModelID)
	}
	if model.Provider != "blackforestlabs" || model.SourceCollection != "multi_models_new" || model.OriginalModelID != "flux_2_pro" || model.ModelSubtype != "pro" {
		t.Fatalf("model metadata = %#v, want skill model metadata preserved", model)
	}
}

func TestListRoutesThroughFolkosProxyForVtrixModelsBaseURL(t *testing.T) {
	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = "https://gateway.example.com/folkos-proxy"
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
	config.DefaultFolkosProxyBaseURL = "https://gateway.example.com/folkos-proxy"
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

	wantEndpoint := "https://gateway.example.com/folkos-proxy/model/v1/generation"
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
	config.DefaultFolkosProxyBaseURL = "https://gateway.example.com/folkos-proxy"
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
