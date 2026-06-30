package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func TestTaskStatusUsesQueueMetadataWhenPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/model/v1/queue/gpt_image_1/requests/req-123/status":
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"completed","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-123/response":
			_, _ = w.Write([]byte(`{
				"request_id":"req-123",
				"status":"completed",
				"output":[{"content":[{"type":"image","url":"https://example.com/task.png"}]}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:      "req-123",
		ModelID:        "gpt_image_1",
		Protocol:       "queue",
		BodyMode:       "raw_json",
		StatusEndpoint: "/model/v1/queue/gpt_image_1/requests/{request_id}/status",
		ResultEndpoint: "/model/v1/queue/gpt_image_1/requests/{request_id}/response",
	}); err != nil {
		t.Fatalf("save queue metadata: %v", err)
	}

	stdout, _, err := executeRoot(t, "task", "status", "req-123", "--output", "url")
	if err != nil {
		t.Fatalf("task status returned error: %v", err)
	}
	if !strings.Contains(stdout, "https://example.com/task.png") {
		t.Fatalf("expected queue task URL, got stdout=%q", stdout)
	}
}

func TestTaskStatusPrintsQueueProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/model/v1/queue/wan25_i2i_preview/requests/req-failed/status":
			_, _ = w.Write([]byte(`{
				"request_id":"req-failed",
				"status":"COMPLETED",
				"error":"Image dimensions must be in [384, 5000], got 5504x3072",
				"error_type":"REQUEST_INVALID",
				"provider_error":{
					"code":"InvalidParameter",
					"message":"Image dimensions must be in [384, 5000], got 5504x3072",
					"task_status":"FAILED"
				},
				"logs":[{"message":"provider rejected request","timestamp":"2026-06-27T10:12:27Z"}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:      "req-failed",
		ModelID:        "wan25_i2i_preview",
		Protocol:       "queue",
		BodyMode:       "raw_json",
		StatusEndpoint: "/model/v1/queue/wan25_i2i_preview/requests/{request_id}/status",
		ResultEndpoint: "/model/v1/queue/wan25_i2i_preview/requests/{request_id}/response",
	}); err != nil {
		t.Fatalf("save queue metadata: %v", err)
	}

	stdout, _, err := executeRoot(t, "task", "status", "req-failed")
	if err != nil {
		t.Fatalf("task status returned error: %v", err)
	}
	for _, want := range []string{
		"Status: failed",
		"Error:  Image dimensions must be in [384, 5000], got 5504x3072",
		"ErrorType: REQUEST_INVALID",
		"ProviderCode: InvalidParameter",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestTaskStatusJSONKeepsQueueProviderErrorAndLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/model/v1/queue/wan25_i2i_preview/requests/req-failed/status":
			_, _ = w.Write([]byte(`{
				"request_id":"req-failed",
				"status":"COMPLETED",
				"error":"Image dimensions must be in [384, 5000], got 5504x3072",
				"error_type":"REQUEST_INVALID",
				"provider_error":{"code":"InvalidParameter","message":"Image dimensions must be in [384, 5000], got 5504x3072"},
				"logs":[{"message":"provider rejected request","timestamp":"2026-06-27T10:12:27Z"}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:      "req-failed",
		ModelID:        "wan25_i2i_preview",
		Protocol:       "queue",
		BodyMode:       "raw_json",
		StatusEndpoint: "/model/v1/queue/wan25_i2i_preview/requests/{request_id}/status",
		ResultEndpoint: "/model/v1/queue/wan25_i2i_preview/requests/{request_id}/response",
	}); err != nil {
		t.Fatalf("save queue metadata: %v", err)
	}

	stdout, _, err := executeRoot(t, "task", "status", "req-failed", "--output", "json")
	if err != nil {
		t.Fatalf("task status returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	providerError, ok := payload["provider_error"].(map[string]any)
	if !ok || providerError["code"] != "InvalidParameter" {
		t.Fatalf("unexpected provider_error: %#v", payload["provider_error"])
	}
	logs, ok := payload["logs"].([]any)
	if !ok || len(logs) != 1 {
		t.Fatalf("unexpected logs: %#v", payload["logs"])
	}
}
