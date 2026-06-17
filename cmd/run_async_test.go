package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunAsyncQueueContractPrintsSubmissionJSON(t *testing.T) {
	var statusCalled bool
	var resultCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			writeContractResponse(t, w, r)
		case "/model/v1/queue/gpt_image_1":
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"queued"}`))
		case "/model/v1/queue/gpt_image_1/requests/req-123/status":
			statusCalled = true
			t.Fatalf("run-async must not poll status")
		case "/model/v1/queue/gpt_image_1/requests/req-123/response":
			resultCalled = true
			t.Fatalf("run-async must not fetch result")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run-async", "gpt_image_1", "--param", "prompt=A red apple")
	if err != nil {
		t.Fatalf("run-async returned error: %v", err)
	}
	if statusCalled || resultCalled {
		t.Fatalf("run-async unexpectedly polled status=%v result=%v", statusCalled, resultCalled)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("run-async output is not JSON: %q", stdout)
	}
	if out["task_id"] != "req-123" || out["model_id"] != "gpt_image_1" || out["status"] != "submitted" || out["protocol"] != "queue" {
		t.Fatalf("unexpected run-async output: %#v", out)
	}
	if out["next"] != "seacloud task status req-123 --output json" {
		t.Fatalf("unexpected next command: %#v", out["next"])
	}
}

func TestRunAsyncQueueOutputID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			writeContractResponse(t, w, r)
		case "/model/v1/queue/gpt_image_1":
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"queued"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run-async", "gpt_image_1",
		"--param", "prompt=A red apple", "--output", "id")
	if err != nil {
		t.Fatalf("run-async returned error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "req-123" {
		t.Fatalf("expected id output, got %q", stdout)
	}
}

func TestRunAsyncRejectsURLOutput(t *testing.T) {
	setupRunCommandTest(t, "http://127.0.0.1")
	_, _, err := executeRoot(t, "run-async", "gpt_image_1",
		"--param", "prompt=A red apple", "--output", "url")
	if err == nil || !strings.Contains(err.Error(), "--output url is not supported for run-async") {
		t.Fatalf("expected run-async url output error, got %v", err)
	}
}

func TestRunAsyncLegacyGenerationSubmitsWithoutPolling(t *testing.T) {
	var pollCalled bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/legacy_image":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"model_id":"legacy_image",
					"protocol":"generation",
					"body_mode":"generation_wrapper"
				}
			}`))
		case "/api/v1/skill/models/legacy_image/spec":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"id":"legacy_image",
					"api":{"endpoint":"` + server.URL + `/model/v1/generation"},
					"parameters":[{"name":"prompt","type":"string","required":true}]
				}
			}`))
		case "/model/v1/generation":
			_, _ = w.Write([]byte(`{"id":"task-legacy","status":"in_progress","model":"legacy_image"}`))
		case "/model/v1/generation/task/task-legacy":
			pollCalled = true
			t.Fatalf("run-async must not poll legacy generation task")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run-async", "legacy_image", "--param", "prompt=A red apple")
	if err != nil {
		t.Fatalf("run-async returned error: %v", err)
	}
	if pollCalled {
		t.Fatalf("run-async unexpectedly polled legacy task")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("run-async output is not JSON: %q", stdout)
	}
	if out["task_id"] != "task-legacy" || out["model_id"] != "legacy_image" || out["protocol"] != "generation" {
		t.Fatalf("unexpected legacy run-async output: %#v", out)
	}
}
