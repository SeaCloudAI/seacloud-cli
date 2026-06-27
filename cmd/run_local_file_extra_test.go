package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunQueueLeavesRemoteURLUnchanged(t *testing.T) {
	var uploadCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		case "/api/v1/files":
			uploadCalled = true
			t.Fatalf("remote URL must not be uploaded")
		case "/model/v1/queue/local_file_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if got := body["image"]; got != "https://example.com/input.png" {
				t.Fatalf("image param = %#v, want original URL", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-url","status":"queued"}`))
		case "/model/v1/queue/local_file_model/requests/req-url/status":
			_, _ = w.Write([]byte(`{"request_id":"req-url","status":"completed","progress":1}`))
		case "/model/v1/queue/local_file_model/requests/req-url/response":
			_, _ = w.Write([]byte(`{"request_id":"req-url","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run", "local_file_model", "--param", "image=https://example.com/input.png", "--output", "url", "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if uploadCalled || !strings.Contains(stdout, "https://example.com/out.png") {
		t.Fatalf("unexpected upload=%v stdout=%q", uploadCalled, stdout)
	}
}

func TestRunQueueFallsBackToUploadWhenURLFormatRejectsBase64(t *testing.T) {
	filePath := writeTempFile(t, "ref.png", []byte("tiny image"))
	var submitCalled bool
	var uploadCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"url"}`)
		case "/api/v1/files":
			uploadCalled = true
			_, _ = w.Write([]byte(`{"url":"https://files.example.com/ref.png"}`))
		case "/model/v1/queue/local_file_model":
			submitCalled = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if got := body["image"]; got != "https://files.example.com/ref.png" {
				t.Fatalf("image param = %#v, want uploaded URL after validation fallback", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-url-format","status":"queued"}`))
		case "/model/v1/queue/local_file_model/requests/req-url-format/status":
			_, _ = w.Write([]byte(`{"request_id":"req-url-format","status":"completed","progress":1}`))
		case "/model/v1/queue/local_file_model/requests/req-url-format/response":
			_, _ = w.Write([]byte(`{"request_id":"req-url-format","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	_, _, err := executeRoot(t, "run", "local_file_model", "--param", "image="+filePath, "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !uploadCalled || !submitCalled {
		t.Fatalf("expected upload fallback then submit, upload=%v submit=%v", uploadCalled, submitCalled)
	}
}

func TestRunAsyncDoesNotUploadAfterTaskIsCreated(t *testing.T) {
	filePath := writeTempFile(t, "ref.png", []byte("tiny image"))
	wantBase64 := base64.StdEncoding.EncodeToString([]byte("tiny image"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_file_model":
			writeLocalFileContract(t, w, "local_file_model", `"image":{"type":"string","format":"image"}`)
		case "/api/v1/files":
			t.Fatalf("run-async must not upload after a task is accepted")
		case "/model/v1/queue/local_file_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			if got := body["image"]; got != wantBase64 {
				t.Fatalf("image param = %#v, want base64", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-async","status":"queued"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run-async", "local_file_model", "--param", "image="+filePath, "--output", "id")
	if err != nil {
		t.Fatalf("run-async returned error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "req-async" {
		t.Fatalf("run-async id = %q, want req-async", got)
	}
}

func TestRunQueueUploadsNestedMediaURL(t *testing.T) {
	filePath := writeTempFile(t, "clip.mp4", []byte("video"))
	var uploadCalled bool
	var submitCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/happyhorse_1.0_video_edit":
			writeNestedMediaContract(t, w)
		case "/api/v1/files":
			uploadCalled = true
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("upload missing file part: %v", err)
			}
			_, _ = w.Write([]byte(`{"url":"https://files.example.com/clip.mp4"}`))
		case "/model/v1/queue/happyhorse_1.0_video_edit":
			submitCalled = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			media := body["media"].([]any)
			first := media[0].(map[string]any)
			if got := first["url"]; got != "https://files.example.com/clip.mp4" {
				t.Fatalf("media[0].url = %#v", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-nested","status":"queued"}`))
		case "/model/v1/queue/happyhorse_1.0_video_edit/requests/req-nested/status":
			_, _ = w.Write([]byte(`{"request_id":"req-nested","status":"completed","progress":1}`))
		case "/model/v1/queue/happyhorse_1.0_video_edit/requests/req-nested/response":
			_, _ = w.Write([]byte(`{"request_id":"req-nested","status":"completed","outputs":[{"type":"video","url":"https://example.com/out.mp4"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	mediaParam := fmt.Sprintf(`[{"type":"video","url":%q}]`, filePath)
	stdout, _, err := executeRoot(t, "run", "happyhorse_1.0_video_edit",
		"--param", "prompt=edit this video",
		"--param", "media="+mediaParam,
		"--output", "url",
		"--timeout", "1",
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !uploadCalled || !submitCalled || !strings.Contains(stdout, "https://example.com/out.mp4") {
		t.Fatalf("upload=%v submit=%v stdout=%q", uploadCalled, submitCalled, stdout)
	}
}

func TestRunQueueUploadsWanImagesArrayLargeLocalFileWithoutFormat(t *testing.T) {
	filePath := writeLargeRunFile(t, "large.png", 10*1024*1024+1)
	var uploadCalled bool
	var submitCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/wan25_i2i_preview":
			writeWanImagesArrayContract(t, w)
		case "/api/v1/files":
			uploadCalled = true
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("upload missing file part: %v", err)
			}
			_, _ = w.Write([]byte(`{"url":"https://files.example.com/large.png"}`))
		case "/model/v1/queue/wan25_i2i_preview":
			submitCalled = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			images := body["images"].([]any)
			if got := images[0]; got != "https://files.example.com/large.png" {
				t.Fatalf("images[0] = %#v, want uploaded URL", got)
			}
			if images[0] == filePath {
				t.Fatalf("images[0] still contains local path %q", filePath)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-wan","status":"queued"}`))
		case "/model/v1/queue/wan25_i2i_preview/requests/req-wan/status":
			_, _ = w.Write([]byte(`{"request_id":"req-wan","status":"completed","progress":1}`))
		case "/model/v1/queue/wan25_i2i_preview/requests/req-wan/response":
			_, _ = w.Write([]byte(`{"request_id":"req-wan","status":"completed","outputs":[{"type":"image","url":"https://example.com/out.png"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	imagesParam := fmt.Sprintf("[%q]", filePath)
	stdout, _, err := executeRoot(t, "run", "wan25_i2i_preview",
		"--param", "prompt=make a clean product photo",
		"--param", "images="+imagesParam,
		"--param", "n=1",
		"--output", "url",
		"--timeout", "1",
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !uploadCalled || !submitCalled || !strings.Contains(stdout, "https://example.com/out.png") {
		t.Fatalf("upload=%v submit=%v stdout=%q", uploadCalled, submitCalled, stdout)
	}
}

func TestRunQueueLeavesNestedRemoteMediaURLUnuploaded(t *testing.T) {
	var uploadCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/happyhorse_1.0_video_edit":
			writeNestedMediaContract(t, w)
		case "/api/v1/files":
			uploadCalled = true
			t.Fatalf("remote nested URL must not be uploaded")
		case "/model/v1/queue/happyhorse_1.0_video_edit":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			media := body["media"].([]any)
			first := media[0].(map[string]any)
			if got := first["url"]; got != "https://example.com/input.mp4" {
				t.Fatalf("media[0].url = %#v", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-nested-remote","status":"queued"}`))
		case "/model/v1/queue/happyhorse_1.0_video_edit/requests/req-nested-remote/status":
			_, _ = w.Write([]byte(`{"request_id":"req-nested-remote","status":"completed","progress":1}`))
		case "/model/v1/queue/happyhorse_1.0_video_edit/requests/req-nested-remote/response":
			_, _ = w.Write([]byte(`{"request_id":"req-nested-remote","status":"completed","outputs":[{"type":"video","url":"https://example.com/out.mp4"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	_, _, err := executeRoot(t, "run", "happyhorse_1.0_video_edit",
		"--param", "prompt=edit this video",
		"--param", `media=[{"type":"video","url":"https://example.com/input.mp4"}]`,
		"--timeout", "1",
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if uploadCalled {
		t.Fatal("remote nested URL was uploaded")
	}
}

func writeNestedMediaContract(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
			"model_id":"happyhorse_1.0_video_edit",
			"protocol":"queue",
			"body_mode":"raw_json",
			"endpoints":{
				"submit":{"method":"POST","path":"/model/v1/queue/happyhorse_1.0_video_edit"},
				"status":{"method":"GET","path":"/model/v1/queue/happyhorse_1.0_video_edit/requests/{request_id}/status"},
				"result":{"method":"GET","path":"/model/v1/queue/happyhorse_1.0_video_edit/requests/{request_id}/response"}
			},
			"input_schema":{
				"type":"object",
				"required":["prompt","media"],
				"additionalProperties":false,
				"properties":{
					"prompt":{"type":"string"},
					"media":{
						"type":"array",
						"items":{
							"type":"object",
							"properties":{
								"type":{"type":"string","enum":["video","reference_image"]},
								"url":{"type":"string","format":"uri"}
							}
						},
						"minItems":1,
						"maxItems":6
					}
				}
			}
		}
	}`))
}

func writeWanImagesArrayContract(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
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
				"required":["prompt","images"],
				"additionalProperties":false,
				"properties":{
					"prompt":{"type":"string"},
					"images":{
						"type":"array",
						"items":{"type":"string"},
						"minItems":1,
						"maxItems":3
					},
					"n":{"type":"integer","default":1}
				}
			}
		}
	}`))
}

func writeLargeRunFile(t *testing.T, name string, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatalf("truncate temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return path
}
