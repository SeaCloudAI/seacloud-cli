package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelsSpecReturnsOfficialDocsPayloadWhenContractAndFallbackCurlMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/p0_unknown_model_not_exist":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":500,"total_pages":0}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called when skill models fallback is missing")
		case "/api/v1/skill/models/p0_unknown_model_not_exist/spec":
			t.Fatalf("models spec should not call legacy spec endpoint")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "p0_unknown_model_not_exist", "--output", "json")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode fallback json: %v\n%s", err, stdout)
	}
	if got["fallback_source"] != "none" ||
		got["reference_curl_available"] != false ||
		!strings.Contains(got["message"].(string), "Search the official provider documentation") {
		t.Fatalf("unexpected missing fallback payload: %#v", got)
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
						"properties":{
							"prompt":{
								"type":"string",
								"description":"Text prompt used to generate the image",
								"examples":["A red apple"]
							}
						}
					},
					"examples":[{"name":"basic","input":{"prompt":"A red apple"}}]
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
	examples, ok := got["examples"].([]any)
	if !ok || len(examples) != 1 {
		t.Fatalf("expected contract examples in JSON output, got %#v", got["examples"])
	}
	inputSchema, ok := got["input_schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected input_schema object, got %#v", got["input_schema"])
	}
	properties, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected input_schema.properties object, got %#v", inputSchema["properties"])
	}
	prompt, ok := properties["prompt"].(map[string]any)
	if !ok {
		t.Fatalf("expected prompt property object, got %#v", properties["prompt"])
	}
	if prompt["description"] != "Text prompt used to generate the image" {
		t.Fatalf("expected prompt description in JSON output, got %#v", prompt["description"])
	}
	propertyExamples, ok := prompt["examples"].([]any)
	if !ok || len(propertyExamples) != 1 || propertyExamples[0] != "A red apple" {
		t.Fatalf("expected prompt examples in JSON output, got %#v", prompt["examples"])
	}
}
