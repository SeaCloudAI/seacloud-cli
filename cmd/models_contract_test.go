package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelsSpecReturnsNotFoundWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/p0_unknown_model_not_exist":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models/p0_unknown_model_not_exist/spec":
			t.Fatalf("models spec should not call legacy spec endpoint")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "p0_unknown_model_not_exist", "--output", "json")
	if err == nil {
		t.Fatal("expected models spec to return not found error")
	}
	if stdout != "" {
		t.Fatalf("expected no contract JSON on stdout, got %s", stdout)
	}
	text := err.Error()
	if !strings.Contains(text, `model "p0_unknown_model_not_exist" not found`) ||
		!strings.Contains(text, "seacloud models list") {
		t.Fatalf("expected not found error with models list hint, got %q", text)
	}
}

func TestModelsSpecReturnsContractJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"revision":"remote-1",
					"model_id":"gpt_image_1",
					"display_name":"GPT Image 1",
					"kind":"multimodal",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/gpt_image_1"},
						"status":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/response"},
						"cancel":{"method":"PUT","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/cancel"}
					},
					"input_schema":{
						"type":"object",
						"required":["prompt"],
						"properties":{"prompt":{"type":"string"}}
					}
				}
			}`))
		case "/api/v1/skill/models/gpt_image_1/spec":
			t.Fatalf("models spec should not call legacy spec endpoint")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "gpt_image_1", "--output", "json")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode models spec json: %v\n%s", err, stdout)
	}
	if got["model_id"] != "gpt_image_1" || got["protocol"] != "queue" || got["body_mode"] != "raw_json" {
		t.Fatalf("unexpected contract: %#v", got)
	}
}
