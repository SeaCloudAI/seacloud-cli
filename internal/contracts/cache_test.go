package contracts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFallsBackToCachedContractOnTemporaryFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"schema_version":"model-contract.v1",
				"revision":"remote-1",
				"model_id":"gpt_image_1",
				"protocol":"queue",
				"body_mode":"raw_json",
				"endpoints":{},
				"input_schema":{"type":"object","properties":{}}
			}
		}`))
	}))

	t.Setenv("SEACLOUD_MODELS_URL", server.URL)
	BaseURL = ""
	first, err := Get("gpt_image_1", Options{})
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	if first.Revision != "remote-1" {
		t.Fatalf("expected remote revision, got %q", first.Revision)
	}
	server.Close()

	cached, err := Get("gpt_image_1", Options{})
	if err != nil {
		t.Fatalf("cached Get returned error: %v", err)
	}
	if cached.Revision != "remote-1" {
		t.Fatalf("expected cached revision, got %q", cached.Revision)
	}
}

func TestGetRefreshDoesNotUseCachedContractOnTemporaryFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveCached("gpt_image_1", &ModelContract{
		SchemaVersion: SupportedSchemaVersion,
		Revision:      "cached-1",
		ModelID:       "gpt_image_1",
		Protocol:      "queue",
		BodyMode:      "raw_json",
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:1")
	BaseURL = ""
	if got, err := Get("gpt_image_1", Options{Refresh: true}); err == nil || got != nil {
		t.Fatalf("expected refresh to return remote error without cache fallback, got contract=%#v err=%v", got, err)
	}
}
