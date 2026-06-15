package models

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUsesConfiguredFullListURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/custom/models" {
			t.Fatalf("expected custom list path, got %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "tmp" {
			t.Fatalf("expected preserved query source=tmp, got %q", got)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Fatalf("expected page=2, got %q", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "5" {
			t.Fatalf("expected page_size=5, got %q", got)
		}
		if got := r.URL.Query().Get("type"); got != "video" {
			t.Fatalf("expected type=video, got %q", got)
		}
		if got := r.URL.Query().Get("keywords"); got != "cat" {
			t.Fatalf("expected keywords=cat, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":2,"page_size":5,"total_pages":0}}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_LIST_URL", server.URL+"/custom/models?source=tmp")
	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:1")
	BaseURL = ""

	if _, err := NewClient().List(ListParams{
		Page:     2,
		PageSize: 5,
		Type:     "video",
		Keywords: "cat",
	}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestGetSpecUsesConfiguredFullSpecURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/custom/spec/gpt_image_1" {
			t.Fatalf("expected custom spec path, got %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "tmp" {
			t.Fatalf("expected preserved query source=tmp, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{
				"model_id":"gpt_image_1",
				"name":"GPT Image 1",
				"vendor":"openai",
				"type":"image",
				"api":{"endpoint":"https://cloud.seaart.ai/model/v1/generation","method":"POST","headers":{}},
				"parameters":[],
				"agent_prompt":""
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODELS_SPEC_URL", server.URL+"/custom/spec/{model_id}?source=tmp")
	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:1")
	BaseURL = ""

	spec, err := NewClient().GetSpec("gpt_image_1")
	if err != nil {
		t.Fatalf("GetSpec returned error: %v", err)
	}
	if spec.ModelID != "gpt_image_1" {
		t.Fatalf("expected model id gpt_image_1, got %q", spec.ModelID)
	}
}
