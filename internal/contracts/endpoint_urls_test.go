package contracts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUsesConfiguredFullSpecURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/custom/contracts/gpt_image_1" {
			t.Fatalf("expected custom contract path, got %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "tmp" {
			t.Fatalf("expected preserved query source=tmp, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"local-1",
				"model_id":"gpt_image_1",
				"display_name":"GPT Image 1",
				"family":"openai",
				"kind":"multimodal",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{
					"submit":{"method":"POST","path":"/model/v1/queue/gpt_image_1"},
					"status":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/status"},
					"result":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/response"}
				},
				"input_schema":{"type":"object","properties":{}}
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_SPEC_URL", server.URL+"/custom/contracts/{model_id}?source=tmp")
	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:1")
	BaseURL = ""

	contract, err := NewClient().Get("gpt_image_1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if contract.ModelID != "gpt_image_1" {
		t.Fatalf("expected model id gpt_image_1, got %q", contract.ModelID)
	}
}
