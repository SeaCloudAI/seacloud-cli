package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLLMRunUsesOnlyLLMSkillModelsFallbackWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_5_2":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			if r.URL.Query().Get("type") != "llm" || r.URL.Query().Get("include_curl") != "true" {
				t.Fatalf("unexpected LLM fallback query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
				"models":[{"id":"gpt_5_2","model_id":"gpt_5_2","name":"GPT","type":"llm","curl":"curl https://cloud.seaart.ai/llm/chat/completions -d '{\"model\":\"gpt_5_2\"}'"}],
				"total":1,"page":1,"page_size":500,"total_pages":1
			}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "llm", "run", "gpt_5_2", "--param", "input=hello")
	if err == nil {
		t.Fatal("expected llm run to stop with fallback curl guidance")
	}
	text := err.Error()
	if strings.Contains(text, "/llm/chat/completions") ||
		!strings.Contains(text, "seacloud --dry-run llm run gpt_5_2 --use-skill-model-fallback") ||
		!strings.Contains(text, "Search the official provider documentation") {
		t.Fatalf("expected CLI-first LLM fallback guidance, got %q", text)
	}
}

func TestLLMRunIgnoresNonLLMSkillModelsFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_5_2":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
				"models":[{"id":"gpt_5_2","model_id":"gpt_5_2","name":"Wrong","type":"image","curl":"curl https://provider.example/image"}],
				"total":1,"page":1,"page_size":500,"total_pages":1
			}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "llm", "run", "gpt_5_2", "--param", "input=hello")
	if err == nil {
		t.Fatal("expected official docs guidance")
	}
	if text := err.Error(); !strings.Contains(text, "No contract or skill model curl found") ||
		!strings.Contains(text, "Search the official provider documentation") {
		t.Fatalf("expected official docs guidance, got %q", text)
	}
}
