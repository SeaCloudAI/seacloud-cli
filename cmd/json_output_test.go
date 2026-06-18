package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

const signedQuery = "q-sign-algorithm=sha1&q-ak=abc&q-signature=def"

func TestRunQueueJSONDoesNotEscapeURLQuerySeparators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			writeContractResponse(t, w, r)
		case "/model/v1/queue/gpt_image_1":
			_, _ = w.Write([]byte(`{"request_id":"req-json","status":"queued"}`))
		case "/model/v1/queue/gpt_image_1/requests/req-json/status":
			_, _ = w.Write([]byte(`{"request_id":"req-json","status":"completed","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-json/response":
			_, _ = w.Write([]byte(`{
				"request_id":"req-json",
				"status":"completed",
				"outputs":[{"type":"image","url":"https://example.com/out.png?` + signedQuery + `"}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_image_1",
		"--param", "prompt=A red apple", "--output", "json", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	assertJSONDoesNotEscapeURLQuerySeparators(t, stdout)
}

func TestTaskStatusQueueJSONDoesNotEscapeURLQuerySeparators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/model/v1/queue/gpt_image_1/requests/req-json/status":
			_, _ = w.Write([]byte(`{"request_id":"req-json","status":"completed","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-json/response":
			_, _ = w.Write([]byte(`{
				"request_id":"req-json",
				"status":"completed",
				"outputs":[{"type":"image","url":"https://example.com/task.png?` + signedQuery + `"}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:      "req-json",
		ModelID:        "gpt_image_1",
		Protocol:       "queue",
		BodyMode:       "raw_json",
		StatusEndpoint: "/model/v1/queue/gpt_image_1/requests/{request_id}/status",
		ResultEndpoint: "/model/v1/queue/gpt_image_1/requests/{request_id}/response",
	}); err != nil {
		t.Fatalf("save queue metadata: %v", err)
	}

	stdout, _, err := executeRoot(t, "task", "status", "req-json", "--output", "json")
	if err != nil {
		t.Fatalf("task status returned error: %v", err)
	}
	assertJSONDoesNotEscapeURLQuerySeparators(t, stdout)
}

func assertJSONDoesNotEscapeURLQuerySeparators(t *testing.T, stdout string) {
	t.Helper()
	if strings.Contains(stdout, `\u0026`) {
		t.Fatalf("json output must not HTML-escape URL separators: %s", stdout)
	}
	if !strings.Contains(stdout, signedQuery) {
		t.Fatalf("json output missing raw URL query separators: %s", stdout)
	}
}
