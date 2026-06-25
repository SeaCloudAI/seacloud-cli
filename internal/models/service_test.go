package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListPreservesSkillModelIDsAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models" {
			t.Fatalf("expected list path, got %q", got)
		}
		if got := r.URL.Query().Get("keywords"); got != "gpt" {
			t.Fatalf("expected keywords query, got %q", got)
		}
		if got := r.URL.Query().Get("provider"); got != "blackforestlabs" {
			t.Fatalf("expected provider query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"models":[
					{"id":"kirin_v2_6_i2v","model_id":"kirin_v2_6_i2v","name":"Kling V2.6 I2V","type":"video","provider":"kuaishou","source_collection":"multi_models_new","original_model_id":"kirin_v2_6_i2v","model_subtype":"i2v","description":"Kling model","input_modalities":["Text","Image"],"output_modalities":["Video"]},
					{"id":"flux_2_pro","model_id":"flux_2_pro","name":"FLUX.2 [pro]","type":"image","provider":"blackforestlabs","source_collection":"multi_models_new","original_model_id":"flux_2_pro","model_subtype":"pro","description":"Flux model","input_modalities":["Text"],"output_modalities":["Image"]}
				],
				"total":2,
				"page":1,
				"page_size":20,
				"total_pages":1
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	result, err := List(ListParams{Keywords: "gpt", Provider: "blackforestlabs"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got := result.Models[0].ID; got != "kirin_v2_6_i2v" {
		t.Fatalf("expected skill model id to stay unchanged, got %q", got)
	}
	if got := result.Models[0].ModelID; got != result.Models[0].ID {
		t.Fatalf("expected model_id == id, got %q/%q", result.Models[0].ModelID, result.Models[0].ID)
	}
	if got := result.Models[1].Provider; got != "blackforestlabs" {
		t.Fatalf("expected provider to be preserved, got %q", got)
	}
	if got := result.Models[1].SourceCollection; got != "multi_models_new" {
		t.Fatalf("expected source_collection to be preserved, got %q", got)
	}
	if got := result.Models[1].OriginalModelID; got != "flux_2_pro" {
		t.Fatalf("expected original_model_id to be preserved, got %q", got)
	}
	if got := result.Models[1].ModelSubtype; got != "pro" {
		t.Fatalf("expected model_subtype to be preserved, got %q", got)
	}
}

func TestGetSpecResolvesAliasAndRewritesDisplayModelID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models/kirin_v3_t2v/spec" {
			t.Fatalf("expected backend model id path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"kirin_v3_t2v",
				"name":"Kling V3",
				"vendor":"kling",
				"type":"video",
				"api":{
					"endpoint":"https://example.com/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"submit kirin_v3_t2v with model kirin_v3_t2v"
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	spec, err := GetSpec("kling_v3_t2v")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "kling_v3_t2v" {
		t.Fatalf("expected display model id %q, got %q", "kling_v3_t2v", spec.ModelID)
	}
	if !strings.Contains(spec.AgentPrompt, "kling_v3_t2v") {
		t.Fatalf("expected agent prompt to contain display model id, got %q", spec.AgentPrompt)
	}
	if strings.Contains(spec.AgentPrompt, "kirin_v3_t2v") {
		t.Fatalf("expected agent prompt to hide backend model id, got %q", spec.AgentPrompt)
	}
}

func TestGetSpecKeepsLegacyModelIDWhenRequestedDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models/spark_dance_v2_0/spec" {
			t.Fatalf("expected direct legacy model id path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"spark_dance_v2_0",
				"name":"Seedance 2.0",
				"vendor":"volces",
				"type":"video",
				"api":{
					"endpoint":"https://example.com/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"submit spark_dance_v2_0"
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	spec, err := GetSpec("spark_dance_v2_0")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "spark_dance_v2_0" {
		t.Fatalf("expected legacy model id to stay unchanged, got %q", spec.ModelID)
	}
	if spec.AgentPrompt != "submit spark_dance_v2_0" {
		t.Fatalf("expected legacy agent prompt to stay unchanged, got %q", spec.AgentPrompt)
	}
}

func TestGetSpecResolvesSeedreamAliasAndRewritesDisplayModelID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models/spark_dream_4_5/spec" {
			t.Fatalf("expected backend model id path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"spark_dream_4_5",
				"name":"Seedream 4.5",
				"vendor":"volces",
				"type":"image",
				"api":{
					"endpoint":"https://example.com/model/v1/generation",
					"method":"POST",
					"headers":{"Authorization":"Bearer {{API_KEY}}"}
				},
				"parameters":[],
				"agent_prompt":"submit spark_dream_4_5 with model spark_dream_4_5"
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	spec, err := GetSpec("seedream_4_5")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "seedream_4_5" {
		t.Fatalf("expected display model id %q, got %q", "seedream_4_5", spec.ModelID)
	}
	if !strings.Contains(spec.AgentPrompt, "seedream_4_5") {
		t.Fatalf("expected agent prompt to contain display model id, got %q", spec.AgentPrompt)
	}
	if strings.Contains(spec.AgentPrompt, "spark_dream_4_5") {
		t.Fatalf("expected agent prompt to hide backend model id, got %q", spec.AgentPrompt)
	}
}
