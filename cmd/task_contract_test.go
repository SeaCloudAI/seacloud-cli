package cmd

import (
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
