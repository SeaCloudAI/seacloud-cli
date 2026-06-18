package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
)

func TestRunUsesQueueContractAndPrintsURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			writeContractResponse(t, w, r)
		case "/model/v1/queue/gpt_image_1":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if _, hasModel := body["model"]; hasModel {
				t.Fatalf("queue run used legacy wrapper body: %#v", body)
			}
			if got := body["prompt"]; got != "A red apple" {
				t.Fatalf("expected raw prompt body, got %#v", body)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"queued"}`))
		case "/model/v1/queue/gpt_image_1/requests/req-123/status":
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"completed","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-123/response":
			_, _ = w.Write([]byte(`{
				"request_id":"req-123",
				"status":"completed",
				"output":[{"content":[{"type":"image","url":"https://example.com/queue.png"}]}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_image_1",
		"--param", "prompt=A red apple", "--output", "url", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout, "https://example.com/queue.png") {
		t.Fatalf("expected queue output URL, got stdout=%q", stdout)
	}
}

func TestRunUsesGenericQueueContractWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/model/v1/queue/gpt_image_1":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["prompt"] != "A red apple" {
				t.Fatalf("unexpected generic queue body: body=%#v err=%v", body, err)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-generic","status":"queued"}`))
		case "/model/v1/queue/gpt_image_1/requests/req-generic/status":
			_, _ = w.Write([]byte(`{"request_id":"req-generic","status":"completed","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-generic/response":
			_, _ = w.Write([]byte(`{"request_id":"req-generic","status":"completed","outputs":[{"type":"image","url":"https://example.com/generic.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_image_1",
		"--param", "prompt=A red apple", "--output", "url", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout, "https://example.com/generic.png") {
		t.Fatalf("expected generic queue output URL, got stdout=%q", stdout)
	}
}

func TestRunRejectsIncompatibleContractWithoutLegacyFallback(t *testing.T) {
	var specCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{"schema_version":"model-contract.v2","model_id":"gpt_image_1"}
			}`))
		case "/api/v1/skill/models/gpt_image_1/spec":
			specCalled = true
			t.Fatalf("legacy spec must not be called for incompatible contract schema")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "gpt_image_1", "--param", "prompt=A red apple")
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible schema error, got %v", err)
	}
	if specCalled {
		t.Fatalf("legacy spec fallback was called")
	}
}

func TestRunQueueDryRunReadsLocalContractWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeContractResponse(t, w, r)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv(config.EnvFolkosExecToken, "")
	_, stderr, err := executeRoot(t, "--dry-run", "run", "gpt_image_1", "--param", "prompt=A red apple")
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if !strings.Contains(stderr, "protocol=queue") || !strings.Contains(stderr, `body={"prompt":"A red apple"}`) {
		t.Fatalf("expected queue dry-run details, got stderr=%q", stderr)
	}
	if strings.Contains(stderr, `"model"`) || strings.Contains(stderr, `"input"`) {
		t.Fatalf("dry-run must not show legacy wrapper body, got stderr=%q", stderr)
	}
}

func TestRunRejectsPlaceholderPrerequisiteBeforeSubmit(t *testing.T) {
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
					"prerequisites":[{
						"field":"jobId",
						"source_model":"midjourney_diffusion",
						"context_kind":"midjourney_job",
						"source_path":"outputs[].jobId"
					}]
				}
			}`))
		case "/model/v1/queue/midjourney_upscale":
			submitCalled = true
			t.Fatalf("queue submit should not be called when prerequisite value is a placeholder")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "midjourney_upscale",
		"--param", "jobId=midjourney_provider_job_id",
		"--param", "imageNo=0",
		"--param", "type=1")
	if err == nil || !strings.Contains(err.Error(), "requires real upstream context") {
		t.Fatalf("expected prerequisite placeholder error, got %v", err)
	}
	if submitCalled {
		t.Fatal("queue submit was called")
	}
}

func TestRunQueueInsufficientBalanceUsesBalanceHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			writeContractResponse(t, w, r)
		case "/model/v1/queue/gpt_image_1":
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"status":{"code":402,"message":"Insufficient credits","error_code":40201},"data":null}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "gpt_image_1", "--param", "prompt=A red apple")
	if err == nil {
		t.Fatal("expected run error")
	}
	if got := err.Error(); !strings.Contains(got, "insufficient balance") ||
		!strings.Contains(got, "seacloud account balance") ||
		!strings.Contains(got, "https://cloud.seaart.ai/settings/credits") ||
		strings.Contains(got, "auth status") {
		t.Fatalf("unexpected insufficient balance error: %q", got)
	}
}

func writeContractResponse(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if got := r.URL.Path; got != "/api/v1/skill/model-contracts/gpt_image_1" {
		t.Fatalf("expected contract detail path, got %q", got)
	}
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
			"revision":"local-1",
			"model_id":"gpt_image_1",
			"protocol":"queue",
			"body_mode":"raw_json",
			"endpoints":{
				"submit":{"method":"POST","path":"/model/v1/queue/gpt_image_1"},
				"status":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/status"},
				"result":{"method":"GET","path":"/model/v1/queue/gpt_image_1/requests/{request_id}/response"}
			},
			"input_schema":{
				"type":"object",
				"required":["prompt"],
				"additionalProperties":false,
				"properties":{"prompt":{"type":"string"}}
			}
		}
	}`))
}

func setupRunCommandTest(t *testing.T, serviceURL string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	t.Setenv(config.EnvFolkosExecToken, "api-key")
	t.Setenv("SEACLOUD_MODELS_URL", serviceURL)
	t.Setenv("SEACLOUD_GENERATION_URL", serviceURL)
	models.BaseURL = ""
	contracts.BaseURL = ""
	queue.BaseURL = ""
	dryRun = false
	runParams = nil
	runOutput = ""
	runAsyncOutput = ""
	runTimeout = 600
	runRefresh = false
	taskStatusOutput = ""
}

func executeRoot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout, os.Stderr = stdoutW, stderrW
	stdoutC := make(chan string)
	stderrC := make(chan string)
	go readPipe(stdoutR, stdoutC)
	go readPipe(stderrR, stderrC)

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	rootCmd.SetArgs(nil)
	return <-stdoutC, <-stderrC, err
}

func readPipe(r *os.File, out chan<- string) {
	data, _ := io.ReadAll(r)
	out <- string(data)
}
