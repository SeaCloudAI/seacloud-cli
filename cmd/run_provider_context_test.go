package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func TestRunCachesProviderContextFromQueueStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/midjourney_diffusion":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"model_id":"midjourney_diffusion",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/midjourney_diffusion"},
						"status":{"method":"GET","path":"/model/v1/queue/midjourney_diffusion/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/midjourney_diffusion/requests/{request_id}/response"}
					},
					"input_schema":{
						"type":"object",
						"required":["text"],
						"additionalProperties":false,
						"properties":{"text":{"type":"string"}}
					}
				}
			}`))
		case "/model/v1/queue/midjourney_diffusion":
			_, _ = w.Write([]byte(`{"request_id":"upstream-123","status":"queued"}`))
		case "/model/v1/queue/midjourney_diffusion/requests/upstream-123/status":
			_, _ = w.Write([]byte(`{
				"request_id":"upstream-123",
				"status":"completed",
				"metadata":{"jobId":"provider-job-123","imageNo":2}
			}`))
		case "/model/v1/queue/midjourney_diffusion/requests/upstream-123/response":
			_, _ = w.Write([]byte(`{"request_id":"upstream-123","status":"completed","outputs":[]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "midjourney_diffusion",
		"--param", "text=A red apple",
		"--output", "json",
		"--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	meta, err := taskcache.Load("upstream-123")
	if err != nil {
		t.Fatalf("load task metadata: %v", err)
	}
	if meta.ProviderContext["jobId"] != "provider-job-123" || meta.ProviderContext["imageNo"] != float64(2) {
		t.Fatalf("provider context = %#v", meta.ProviderContext)
	}
}

func TestRunCachesProviderContextFromTopLevelQueueOutputs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/midjourney_diffusion":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"model_id":"midjourney_diffusion",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/midjourney_diffusion"},
						"status":{"method":"GET","path":"/model/v1/queue/midjourney_diffusion/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/midjourney_diffusion/requests/{request_id}/response"}
					},
					"input_schema":{
						"type":"object",
						"required":["prompt"],
						"additionalProperties":false,
						"properties":{"prompt":{"type":"string"}}
					}
				}
			}`))
		case "/model/v1/queue/midjourney_diffusion":
			_, _ = w.Write([]byte(`{"request_id":"upstream-456","status":"queued"}`))
		case "/model/v1/queue/midjourney_diffusion/requests/upstream-456/status":
			_, _ = w.Write([]byte(`{"request_id":"upstream-456","status":"completed"}`))
		case "/model/v1/queue/midjourney_diffusion/requests/upstream-456/response":
			_, _ = w.Write([]byte(`{
				"request_id":"upstream-456",
				"status":"completed",
				"outputs":[
					{"type":"image","url":"https://example.com/1.png","jobId":"provider-job-456"},
					{"type":"image","url":"https://example.com/2.png","jobId":"provider-job-456"}
				],
				"metadata":{"prompt":"A blue ceramic cup"}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "midjourney_diffusion",
		"--param", "prompt=A blue ceramic cup",
		"--output", "json",
		"--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	meta, err := taskcache.Load("upstream-456")
	if err != nil {
		t.Fatalf("load task metadata: %v", err)
	}
	if meta.ProviderContext["jobId"] != "provider-job-456" || meta.ProviderContext["imageNo"] != float64(0) {
		t.Fatalf("provider context = %#v", meta.ProviderContext)
	}
}
