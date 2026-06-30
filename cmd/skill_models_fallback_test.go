package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelsSpecReturnsSkillModelsFallbackReferenceWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/flux_2_pro":
			assertBearerAPIKey(t, r)
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			assertBearerAPIKey(t, r)
			if r.URL.Query().Get("keywords") != "flux_2_pro" || r.URL.Query().Get("include_curl") != "true" {
				t.Fatalf("unexpected fallback query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
				"models":[{"id":"flux_2_pro","model_id":"flux_2_pro","name":"Flux","type":"image","curl":"curl https://provider.example/flux -d '{\"prompt\":\"hello\"}'"}],
				"total":1,"page":1,"page_size":500,"total_pages":1
			}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called for skill models fallback")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "flux_2_pro", "--output", "json")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode fallback json: %v\n%s", err, stdout)
	}
	if got["contract_found"] != false ||
		got["fallback_source"] != "skill_models" ||
		got["fallback_kind"] != "multimodal" ||
		got["model_id"] != "flux_2_pro" ||
		got["official_parameter_docs_required"] != true {
		t.Fatalf("unexpected fallback payload: %#v", got)
	}
	if got["reference_curl_available"] != true || got["direct_curl_execution_allowed"] != false {
		t.Fatalf("unexpected reference curl policy: %#v", got)
	}
	if _, ok := got["curl"]; ok {
		t.Fatalf("default payload must not expose executable curl: %#v", got)
	}
	commands, ok := got["next_cli_commands"].([]any)
	if !ok || len(commands) == 0 || !strings.Contains(commands[0].(string), "--use-skill-model-fallback") {
		t.Fatalf("missing CLI-first commands: %#v", got)
	}
}

func TestModelsSpecRewritesSeaCloudFallbackCurlToBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/wan25_t2i_preview":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
				"models":[{"id":"wan25_t2i_preview","model_id":"wan25_t2i_preview","name":"Wan","type":"image","curl":"curl --location 'https://cloud.seaart.ai/model/v1/queue/wan25_t2i_preview'"}],
				"total":1,"page":1,"page_size":500,"total_pages":1
			}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_BASE_URL", "https://real-cloud.seaart.dev/")
	stdout, _, err := executeRoot(t, "models", "spec", "wan25_t2i_preview", "--output", "json", "--show-reference-curl")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode fallback json: %v\n%s", err, stdout)
	}
	curl := got["reference_curl"].(string)
	if !strings.Contains(curl, "https://real-cloud.seaart.dev/model/v1/queue/wan25_t2i_preview") {
		t.Fatalf("fallback curl was not rewritten to SEACLOUD_BASE_URL: %q", curl)
	}
	if strings.Contains(curl, "https://cloud.seaart.ai") {
		t.Fatalf("fallback curl still contains default cloud host: %q", curl)
	}
}

func TestModelsSpecReturnsOfficialDocsPayloadWhenFallbackCurlMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":500,"total_pages":0}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called when skill model curl is missing")
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
		t.Fatalf("decode fallback json: %v\n%s", err, stdout)
	}
	if got["fallback_source"] != "none" ||
		got["reference_curl_available"] != false ||
		got["official_parameter_docs_required"] != true ||
		!strings.Contains(got["message"].(string), "Search the official provider documentation") {
		t.Fatalf("unexpected missing fallback payload: %#v", got)
	}
}

func TestRunStopsWithSkillModelsFallbackCurlWhenContract404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/flux_2_pro":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{
				"models":[{"id":"flux_2_pro","model_id":"flux_2_pro","name":"Flux","type":"image","curl":"curl https://provider.example/flux -d '{\"prompt\":\"hello\"}'"}],
				"total":1,"page":1,"page_size":500,"total_pages":1
			}}`))
		case "/model/v1/queue/flux_2_pro":
			t.Fatalf("generic queue submit must not be called when fallback curl exists")
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "flux_2_pro", "--param", "prompt=hello")
	if err == nil {
		t.Fatal("expected run to stop with fallback curl guidance")
	}
	text := err.Error()
	if !strings.Contains(text, "model contract not found for \"flux_2_pro\"") ||
		strings.Contains(text, "curl https://provider.example/flux") ||
		!strings.Contains(text, "--use-skill-model-fallback") ||
		!strings.Contains(text, "Search the official provider documentation") {
		t.Fatalf("expected CLI-first fallback guidance, got %q", text)
	}
}

func TestRunStopsWithOfficialDocsHintWhenFallbackCurlMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_image_1":
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[],"total":0,"page":1,"page_size":500,"total_pages":0}}`))
		case "/api/v1/admin/multi-models/detail", "/api/v1/admin/models/detail":
			t.Fatalf("admin detail must not be called when fallback curl is missing")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "gpt_image_1", "--param", "prompt=hello")
	if err == nil {
		t.Fatal("expected official docs guidance")
	}
	if text := err.Error(); !strings.Contains(text, "No contract or skill model curl found") ||
		!strings.Contains(text, "Search the official provider documentation") {
		t.Fatalf("expected official docs guidance, got %q", text)
	}
}

func assertBearerAPIKey(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer api-key" {
		t.Fatalf("Authorization = %q, want API key", got)
	}
}
