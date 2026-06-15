package contracts

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetResolvesAliasAndFetchesLocalContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/model-contracts/kirin_v3_t2v" {
			t.Fatalf("expected backend contract path, got %q", got)
		}
		if got := r.Header.Get("X-Source"); got != "cli" {
			t.Fatalf("expected X-Source=cli, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"local-1",
				"model_id":"kirin_v3_t2v",
				"display_name":"Kling V3",
				"family":"kling",
				"kind":"multimodal",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{
					"submit":{"method":"POST","path":"/model/v1/queue/kirin_v3_t2v"},
					"status":{"method":"GET","path":"/model/v1/queue/kirin_v3_t2v/requests/{request_id}/status"},
					"result":{"method":"GET","path":"/model/v1/queue/kirin_v3_t2v/requests/{request_id}/response"}
				},
				"input_schema":{
					"type":"object",
					"required":["prompt"],
					"additionalProperties":false,
					"properties":{"prompt":{"type":"string"}}
				}
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	contract, err := Get("kling_v3_t2v", Options{})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if contract.ModelID != "kling_v3_t2v" {
		t.Fatalf("expected display model id, got %q", contract.ModelID)
	}
	if contract.BackendModelID != "kirin_v3_t2v" {
		t.Fatalf("expected backend model id, got %q", contract.BackendModelID)
	}
	if contract.Protocol != "queue" || contract.BodyMode != "raw_json" {
		t.Fatalf("unexpected protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
}

func TestGetStripsSeaCloudSourcePrefixBeforeFetchingContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skill/model-contracts/happyhorse_1.0_t2v" {
			t.Fatalf("expected backend contract path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"local-1",
				"model_id":"happyhorse_1.0_t2v",
				"display_name":"HappyHorse 1.0",
				"family":"happyhorse",
				"kind":"multimodal",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{
					"submit":{"method":"POST","path":"/model/v1/queue/happyhorse_1.0_t2v"},
					"status":{"method":"GET","path":"/model/v1/queue/happyhorse_1.0_t2v/requests/{request_id}/status"},
					"result":{"method":"GET","path":"/model/v1/queue/happyhorse_1.0_t2v/requests/{request_id}/response"}
				},
				"input_schema":{
					"type":"object",
					"required":["prompt"],
					"additionalProperties":false,
					"properties":{"prompt":{"type":"string"}}
				}
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	contract, err := Get("seacloud__happyhorse_1.0_t2v", Options{})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if contract.BackendModelID != "happyhorse_1.0_t2v" {
		t.Fatalf("expected backend model id, got %q", contract.BackendModelID)
	}
}

func TestGetReturnsNotFoundForContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	_, err := Get("gpt_image_1", Options{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRejectsIncompatibleSchemaVersionWithoutFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v2",
				"revision":"local-2",
				"model_id":"gpt_image_1",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{},
				"input_schema":{"type":"object","properties":{}}
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""

	_, err := Get("gpt_image_1", Options{})
	if !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected ErrIncompatibleSchema, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "model-contract.v2") {
		t.Fatalf("expected error to include remote schema version, got %v", err)
	}
}
