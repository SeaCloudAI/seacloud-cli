package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunDryRunRejectsContractIntegerEnumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/skill/model-contracts/minimax_hailuo_23_t2v" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"local-1",
				"model_id":"minimax_hailuo_23_t2v",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{
					"submit":{"method":"POST","path":"/model/v1/queue/minimax_hailuo_23_t2v"},
					"status":{"method":"GET","path":"/model/v1/queue/minimax_hailuo_23_t2v/requests/{request_id}/status"},
					"result":{"method":"GET","path":"/model/v1/queue/minimax_hailuo_23_t2v/requests/{request_id}/response"}
				},
				"input_schema":{
					"type":"object",
					"required":["prompt"],
					"additionalProperties":false,
					"properties":{
						"prompt":{"type":"string"},
						"duration":{"type":"integer","enum":["6","10"]},
						"resolution":{"type":"string","enum":["768P","1080P"]}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "--dry-run", "run", "minimax_hailuo_23_t2v",
		"--refresh",
		"--param", "prompt=cat test",
		"--param", "duration=7",
		"--param", "resolution=768P")

	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected duration enum validation error, got %v", err)
	}
}

func TestRunDryRunRejectsContractInputRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/skill/model-contracts/wan26_r2v" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"local-1",
				"model_id":"wan26_r2v",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{
					"submit":{"method":"POST","path":"/model/v1/queue/wan26_r2v"},
					"status":{"method":"GET","path":"/model/v1/queue/wan26_r2v/requests/{request_id}/status"},
					"result":{"method":"GET","path":"/model/v1/queue/wan26_r2v/requests/{request_id}/response"}
				},
				"input_schema":{
					"type":"object",
					"additionalProperties":false,
					"properties":{
						"prompt":{"type":"string"},
						"image":{"type":"string"},
						"video":{"type":"string"}
					}
				},
				"input_rules":[{
					"kind":"one_of",
					"fields":["image","video"]
				}]
			}
		}`))
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "--dry-run", "run", "wan26_r2v",
		"--refresh",
		"--param", "prompt=cat test")

	if err == nil || !strings.Contains(err.Error(), "requires one of: image, video") {
		t.Fatalf("expected input_rules validation error, got %v", err)
	}
}
