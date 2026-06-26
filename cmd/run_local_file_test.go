package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunQueueEncodesSmallLocalFileAsBase64(t *testing.T) {
	filePath := writeTempFile(t, "ref.png", []byte("tiny image"))
	wantBase64 := base64.StdEncoding.EncodeToString([]byte("tiny image"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		case "/model/v1/queue/local_file_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if got := body["image"]; got != wantBase64 {
				t.Fatalf("image param = %#v, want base64 %q", got, wantBase64)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-local","status":"queued"}`))
		case "/model/v1/queue/local_file_model/requests/req-local/status":
			_, _ = w.Write([]byte(`{"request_id":"req-local","status":"completed","progress":1}`))
		case "/model/v1/queue/local_file_model/requests/req-local/response":
			_, _ = w.Write([]byte(`{"request_id":"req-local","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run", "local_file_model", "--param", "image="+filePath, "--output", "url", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout, "https://example.com/out.png") {
		t.Fatalf("expected output URL, got stdout=%q", stdout)
	}
}

func TestRunQueueFallsBackToUploadedURLWhenBase64IsRejected(t *testing.T) {
	filePath := writeTempFile(t, "ref.png", []byte("tiny image"))
	var submitBodies []map[string]any
	var uploadCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		case "/api/v1/files":
			uploadCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("upload method = %s, want POST", r.Method)
			}
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("upload missing file part: %v", err)
			}
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"url":"https://files.example.com/ref.png"}}`))
		case "/model/v1/queue/local_file_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			submitBodies = append(submitBodies, body)
			if len(submitBodies) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":{"code":400,"message":"invalid image format"},"data":null}`))
				return
			}
			if got := body["image"]; got != "https://files.example.com/ref.png" {
				t.Fatalf("fallback image param = %#v, want uploaded URL", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-fallback","status":"queued"}`))
		case "/model/v1/queue/local_file_model/requests/req-fallback/status":
			_, _ = w.Write([]byte(`{"request_id":"req-fallback","status":"completed","progress":1}`))
		case "/model/v1/queue/local_file_model/requests/req-fallback/response":
			_, _ = w.Write([]byte(`{"request_id":"req-fallback","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	stdout, _, err := executeRoot(t, "run", "local_file_model", "--param", "image="+filePath, "--output", "url", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if len(submitBodies) != 2 || !uploadCalled {
		t.Fatalf("expected base64 submit then upload fallback, submits=%d upload=%v", len(submitBodies), uploadCalled)
	}
	if !strings.Contains(stdout, "https://example.com/out.png") {
		t.Fatalf("expected output URL, got stdout=%q", stdout)
	}
}

func TestRunQueueUploadsLargeLocalFileDirectly(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "large.png")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create large file: %v", err)
	}
	if err := file.Truncate(10*1024*1024 + 1); err != nil {
		t.Fatalf("truncate large file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close large file: %v", err)
	}
	var submitBodies []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		case "/api/v1/files":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"url":"https://files.example.com/large.png"}}`))
		case "/model/v1/queue/local_file_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			submitBodies = append(submitBodies, body)
			if got := body["image"]; got != "https://files.example.com/large.png" {
				t.Fatalf("large image param = %#v, want uploaded URL", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-large","status":"queued"}`))
		case "/model/v1/queue/local_file_model/requests/req-large/status":
			_, _ = w.Write([]byte(`{"request_id":"req-large","status":"completed","progress":1}`))
		case "/model/v1/queue/local_file_model/requests/req-large/response":
			_, _ = w.Write([]byte(`{"request_id":"req-large","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	_, _, err = executeRoot(t, "run", "local_file_model", "--param", "image="+filePath, "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if len(submitBodies) != 1 {
		t.Fatalf("large file should submit once with URL, got %d submits", len(submitBodies))
	}
}

func TestRunRejectsExplicitMissingLocalFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "local_file_model", "--param", "image=./missing.png")
	if err == nil || !strings.Contains(err.Error(), "file_not_found") {
		t.Fatalf("expected file_not_found error, got %v", err)
	}
}

func TestRunRejectsTooManyLocalFiles(t *testing.T) {
	dir := t.TempDir()
	var args []string
	for i := 0; i < 6; i++ {
		path := filepath.Join(dir, string(rune('a'+i))+".png")
		if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
			t.Fatalf("write file %d: %v", i, err)
		}
		args = append(args, "--param", "image"+string(rune('0'+i))+"="+path)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			_, _ = w.Write([]byte(`{
				"status":{"code":200,"message":"ok"},
				"data":{
					"schema_version":"model-contract.v1",
					"model_id":"local_file_model",
					"protocol":"queue",
					"body_mode":"raw_json",
					"endpoints":{
						"submit":{"method":"POST","path":"/model/v1/queue/local_file_model"},
						"status":{"method":"GET","path":"/model/v1/queue/local_file_model/requests/{request_id}/status"},
						"result":{"method":"GET","path":"/model/v1/queue/local_file_model/requests/{request_id}/response"}
					},
					"input_schema":{"type":"object","additionalProperties":true,"properties":{}}
				}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	fullArgs := append([]string{"run", "local_file_model"}, args...)
	_, _, err := executeRoot(t, fullArgs...)
	if err == nil || !strings.Contains(err.Error(), "too_many_files") {
		t.Fatalf("expected too_many_files error, got %v", err)
	}
}

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func writeLocalFileContract(t *testing.T, w http.ResponseWriter, modelID, properties string) {
	t.Helper()
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
			"model_id":"` + modelID + `",
			"protocol":"queue",
			"body_mode":"raw_json",
			"endpoints":{
				"submit":{"method":"POST","path":"/model/v1/queue/` + modelID + `"},
				"status":{"method":"GET","path":"/model/v1/queue/` + modelID + `/requests/{request_id}/status"},
				"result":{"method":"GET","path":"/model/v1/queue/` + modelID + `/requests/{request_id}/response"}
			},
			"input_schema":{
				"type":"object",
				"required":["image"],
				"additionalProperties":false,
				"properties":{` + properties + `}
			}
		}
	}`))
}
