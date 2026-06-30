package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunQueueFailureIncludesProviderCodeAndMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/wan25_i2i_preview":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"revision":"local-1",
					"model_id":"wan25_i2i_preview",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/wan25_i2i_preview"},
						"status":{"method":"GET","path":"/model/v1/queue/wan25_i2i_preview/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/wan25_i2i_preview/requests/{request_id}/response"}
					},
					"input_schema":{
						"type":"object",
						"required":["prompt"],
						"additionalProperties":false,
						"properties":{"prompt":{"type":"string"}}
					}
				}
			}`))
		case "/model/v1/queue/wan25_i2i_preview":
			_, _ = w.Write([]byte(`{"request_id":"req-failed","status":"queued"}`))
		case "/model/v1/queue/wan25_i2i_preview/requests/req-failed/status":
			_, _ = w.Write([]byte(`{
				"request_id":"req-failed",
				"status":"COMPLETED",
				"error":"Image dimensions must be in [384, 5000], got 5504x3072",
				"error_type":"REQUEST_INVALID",
				"provider_error":{"code":"InvalidParameter","message":"Image dimensions must be in [384, 5000], got 5504x3072"}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "wan25_i2i_preview",
		"--param", "prompt=A test image",
		"--timeout", "1")
	if err == nil {
		t.Fatal("expected run to fail")
	}
	if got := err.Error(); !strings.Contains(got, "InvalidParameter: Image dimensions must be in [384, 5000], got 5504x3072") {
		t.Fatalf("expected provider error in run failure, got %q", got)
	}
}
