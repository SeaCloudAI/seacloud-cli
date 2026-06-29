package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunQueueUploadsSmallLocalVideoDirectly(t *testing.T) {
	filePath := writeTempFile(t, "clip.mp4", []byte("video"))
	var uploadCalled bool
	var submitBodies []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_video_model":
			writeLocalFileContractForParam(t, w, "local_video_model", "video", `"video":{"type":"string","format":"video"}`)
		case "/api/v1/files":
			uploadCalled = true
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("upload missing file part: %v", err)
			}
			_, _ = w.Write([]byte(`{"url":"https://files.example.com/clip.mp4"}`))
		case "/model/v1/queue/local_video_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			submitBodies = append(submitBodies, body)
			if got := body["video"]; got != "https://files.example.com/clip.mp4" {
				t.Fatalf("video param = %#v, want uploaded URL", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-video","status":"queued"}`))
		case "/model/v1/queue/local_video_model/requests/req-video/status":
			_, _ = w.Write([]byte(`{"request_id":"req-video","status":"completed","progress":1}`))
		case "/model/v1/queue/local_video_model/requests/req-video/response":
			_, _ = w.Write([]byte(`{"request_id":"req-video","status":"completed","outputs":[{"type":"video","url":"https://example.com/out.mp4"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	_, _, err := executeRoot(t, "run", "local_video_model", "--param", "video="+filePath, "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !uploadCalled || len(submitBodies) != 1 {
		t.Fatalf("expected direct upload and one submit, upload=%v submits=%d", uploadCalled, len(submitBodies))
	}
}

func TestRunQueueUploadsSmallLocalAudioDirectly(t *testing.T) {
	filePath := writeTempFile(t, "sound.mp3", []byte("audio"))
	var uploadCalled bool
	var submitBodies []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/local_audio_model":
			writeLocalFileContractForParam(t, w, "local_audio_model", "audio", `"audio":{"type":"string","format":"audio"}`)
		case "/api/v1/files":
			uploadCalled = true
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("upload missing file part: %v", err)
			}
			_, _ = w.Write([]byte(`{"url":"https://files.example.com/sound.mp3"}`))
		case "/model/v1/queue/local_audio_model":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode queue body: %v", err)
			}
			submitBodies = append(submitBodies, body)
			if got := body["audio"]; got != "https://files.example.com/sound.mp3" {
				t.Fatalf("audio param = %#v, want uploaded URL", got)
			}
			_, _ = w.Write([]byte(`{"request_id":"req-audio","status":"queued"}`))
		case "/model/v1/queue/local_audio_model/requests/req-audio/status":
			_, _ = w.Write([]byte(`{"request_id":"req-audio","status":"completed","progress":1}`))
		case "/model/v1/queue/local_audio_model/requests/req-audio/response":
			_, _ = w.Write([]byte(`{"request_id":"req-audio","status":"completed","outputs":[{"type":"audio","url":"https://example.com/out.mp3"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_UPLOAD_URL", server.URL+"/api/v1/files")
	_, _, err := executeRoot(t, "run", "local_audio_model", "--param", "audio="+filePath, "--timeout", "1")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !uploadCalled || len(submitBodies) != 1 {
		t.Fatalf("expected direct upload and one submit, upload=%v submits=%d", uploadCalled, len(submitBodies))
	}
}

func writeLocalFileContractForParam(t *testing.T, w http.ResponseWriter, modelID, required, properties string) {
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
				"required":["` + required + `"],
				"additionalProperties":false,
				"properties":{` + properties + `}
			}
		}
	}`))
}
