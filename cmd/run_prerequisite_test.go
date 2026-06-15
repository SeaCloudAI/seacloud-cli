package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func TestRunFillsMidjourneyActionPrerequisitesFromCachedProviderContext(t *testing.T) {
	var submitCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/midjourney_upscale":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"revision":"local-1",
					"model_id":"midjourney_upscale",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/midjourney_upscale"},
						"status":{"method":"GET","path":"/model/v1/queue/midjourney_upscale/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/midjourney_upscale/requests/{request_id}/response"}
					},
					"input_schema":{
						"type":"object",
						"required":["jobId","imageNo","type"],
						"additionalProperties":false,
						"properties":{
							"jobId":{"type":"string"},
							"imageNo":{"type":"integer"},
							"type":{"type":"integer"}
						}
					},
					"prerequisites":[
						{"field":"jobId","source_model":"midjourney_diffusion","context_kind":"midjourney_job","source_path":"outputs[].jobId"},
						{"field":"imageNo","source_model":"midjourney_diffusion","context_kind":"midjourney_job","source_path":"outputs[].index"}
					]
				}
			}`))
		case "/model/v1/queue/midjourney_upscale":
			submitCalled = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if body["jobId"] != "provider-job-123" || body["imageNo"] != float64(2) || body["type"] != float64(1) {
				t.Fatalf("unexpected action body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"request_id":"action-123","status":"queued"}`))
		case "/model/v1/queue/midjourney_upscale/requests/action-123/status":
			_, _ = w.Write([]byte(`{"request_id":"action-123","status":"completed","progress":1}`))
		case "/model/v1/queue/midjourney_upscale/requests/action-123/response":
			_, _ = w.Write([]byte(`{"request_id":"action-123","status":"completed","outputs":[{"type":"image","url":"https://example.com/upscale.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:       "upstream-123",
		ModelID:         "midjourney_diffusion",
		Protocol:        "queue",
		BodyMode:        "raw_json",
		ProviderContext: map[string]any{"jobId": "provider-job-123", "imageNo": 2},
	}); err != nil {
		t.Fatalf("save upstream provider context: %v", err)
	}

	_, _, err := executeRoot(t, "run", "midjourney_upscale",
		"--param", "jobId=midjourney_provider_job_id",
		"--param", "type=1",
		"--output", "url",
		"--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !submitCalled {
		t.Fatal("queue submit was not called")
	}
}
