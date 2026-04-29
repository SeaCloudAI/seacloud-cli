package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListRewritesDisplayModelIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/models" {
			t.Fatalf("expected list path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"models":[
					{"id":"kirin_v2_6_i2v","name":"Kling V2.6 I2V","type":"video","description":"","input_modalities":["image"],"output_modalities":["video"]},
					{"id":"spark_dance_v2_0","name":"Seedance 2.0","type":"video","description":"","input_modalities":["text"],"output_modalities":["video"]},
					{"id":"spark_dream_4_5","name":"Seedream 4.5","type":"image","description":"","input_modalities":["text"],"output_modalities":["image"]},
					{"id":"gpt-image-2","name":"GPT Image 2","type":"image","description":"","input_modalities":["text"],"output_modalities":["image"]}
				],
				"total":4,
				"page":1,
				"page_size":20,
				"total_pages":1
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	result, err := List(ListParams{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got := result.Models[0].ID; got != "kling_v2_6_i2v" {
		t.Fatalf("expected kling alias, got %q", got)
	}
	if got := result.Models[1].ID; got != "seedance_2_0" {
		t.Fatalf("expected seedance alias, got %q", got)
	}
	if got := result.Models[2].ID; got != "seedream_4_5" {
		t.Fatalf("expected seedream alias, got %q", got)
	}
	if got := result.Models[3].ID; got != "gpt-image-2" {
		t.Fatalf("expected unrelated id to stay unchanged, got %q", got)
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
