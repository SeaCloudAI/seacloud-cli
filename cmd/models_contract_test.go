package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelsSpecReturnsGenericContractWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models/gpt_image_1/spec":
			t.Fatalf("models spec should not call legacy spec endpoint")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "gpt_image_1", "--output", "json")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode models spec json: %v\n%s", err, stdout)
	}
	if got["model_id"] != "gpt_image_1" || got["protocol"] != "queue" || got["body_mode"] != "raw_json" {
		t.Fatalf("unexpected generic contract: %#v", got)
	}
}
