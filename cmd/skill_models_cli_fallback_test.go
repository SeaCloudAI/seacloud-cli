package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelsSpecFallbackJSONHidesReferenceCurlByDefault(t *testing.T) {
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://cloud.seaart.ai/model/v1/queue/flux_2_pro -d '{"prompt":"hello"}'`, nil)
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
	if _, ok := got["curl"]; ok {
		t.Fatalf("default fallback payload must not expose executable curl: %#v", got)
	}
	if got["reference_curl_available"] != true || got["direct_curl_execution_allowed"] != false {
		t.Fatalf("unexpected fallback policy: %#v", got)
	}
	commands, ok := got["next_cli_commands"].([]any)
	if !ok || len(commands) == 0 || !strings.Contains(commands[0].(string), "--use-skill-model-fallback") {
		t.Fatalf("missing CLI-first next commands: %#v", got)
	}
}

func TestModelsSpecFallbackJSONShowsReferenceCurlOnlyWhenRequested(t *testing.T) {
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://cloud.seaart.ai/model/v1/queue/flux_2_pro`, nil)
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "models", "spec", "flux_2_pro", "--output", "json", "--show-reference-curl")
	if err != nil {
		t.Fatalf("models spec returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode fallback json: %v\n%s", err, stdout)
	}
	if got["reference_only"] != true || !strings.Contains(got["reference_curl"].(string), "/model/v1/queue/flux_2_pro") {
		t.Fatalf("reference curl was not explicitly exposed as reference-only: %#v", got)
	}
}

func TestRunDefaultFallbackGuidanceDoesNotExposeExecutableCurl(t *testing.T) {
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://cloud.seaart.ai/model/v1/queue/flux_2_pro`, nil)
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run", "flux_2_pro", "--param", "prompt=hello")
	if err == nil {
		t.Fatal("expected contract 404 guidance")
	}
	text := err.Error()
	if strings.Contains(text, "curl https://") {
		t.Fatalf("default guidance must not expose executable curl: %q", text)
	}
	if !strings.Contains(text, "seacloud --dry-run run flux_2_pro --use-skill-model-fallback") {
		t.Fatalf("missing CLI-first guidance: %q", text)
	}
}

func TestDryRunUsesSkillModelFallbackQueueContract(t *testing.T) {
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://cloud.seaart.ai/model/v1/queue/flux_2_pro`, nil)
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, stderr, err := executeRoot(t, "--dry-run", "run", "flux_2_pro", "--use-skill-model-fallback", "--param", "prompt=hello")
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if !strings.Contains(stderr, "[dry-run] protocol=queue") ||
		!strings.Contains(stderr, "/model/v1/queue/flux_2_pro") ||
		!strings.Contains(stderr, `"prompt":"hello"`) {
		t.Fatalf("expected fallback queue dry-run, got %q", stderr)
	}
}

func TestRunUsesSkillModelFallbackQueueWithStoredAPIKey(t *testing.T) {
	var submittedAuth string
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://cloud.seaart.ai/model/v1/queue/flux_2_pro`, func(w http.ResponseWriter, r *http.Request) {
		submittedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"prompt":"hello"`) {
			t.Fatalf("unexpected submit body: %s", string(body))
		}
		_, _ = w.Write([]byte(`{"request_id":"req_123","status":"queued"}`))
	})
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run-async", "flux_2_pro", "--use-skill-model-fallback", "--param", "prompt=hello", "--output", "id")
	if err != nil {
		t.Fatalf("run-async returned error: %v", err)
	}
	if submittedAuth != "Bearer api-key" {
		t.Fatalf("Authorization = %q, want stored API key", submittedAuth)
	}
}

func TestLLMRunUsesSkillModelFallbackContract(t *testing.T) {
	server := skillModelsFallbackServer(t, "gpt_5_2", "llm", `curl https://cloud.seaart.ai/llm/chat/completions -d '{"model":"gpt_5_2"}'`, nil)
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, stderr, err := executeRoot(t, "--dry-run", "llm", "run", "gpt_5_2", "--use-skill-model-fallback", "--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("llm dry-run returned error: %v", err)
	}
	if !strings.Contains(stderr, "[dry-run] protocol=llm_chat_completions") ||
		!strings.Contains(stderr, "/llm/chat/completions") ||
		!strings.Contains(stderr, `"model":"gpt_5_2"`) {
		t.Fatalf("expected LLM fallback dry-run, got %q", stderr)
	}
}

func TestReferenceCurlExecutionOverridesAuthorization(t *testing.T) {
	var submittedAuth string
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl -H 'Authorization: Bearer stale' https://cloud.seaart.ai/model/v1/queue/flux_2_pro`, func(w http.ResponseWriter, r *http.Request) {
		submittedAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"request_id":"req_123","status":"queued"}`))
	})
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	stdout, _, err := executeRoot(t, "run-async", "flux_2_pro", "--use-reference-curl", "--param", "prompt=hello", "--output", "json")
	if err != nil {
		t.Fatalf("reference curl execution returned error: %v", err)
	}
	if submittedAuth != "Bearer api-key" {
		t.Fatalf("Authorization = %q, want stored API key", submittedAuth)
	}
	if strings.Contains(stdout, "api-key") {
		t.Fatalf("reference curl output leaked API key: %s", stdout)
	}
}

func TestUnsupportedFallbackCurlDoesNotAutoExecute(t *testing.T) {
	server := skillModelsFallbackServer(t, "flux_2_pro", "image", `curl https://provider.example/flux -F image=@input.png`, nil)
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "run-async", "flux_2_pro", "--use-skill-model-fallback", "--param", "prompt=hello")
	if err == nil {
		t.Fatal("expected unsupported fallback curl error")
	}
	if text := err.Error(); !strings.Contains(text, "fallback curl is reference-only") ||
		strings.Contains(text, "curl https://provider.example") {
		t.Fatalf("unexpected unsupported curl guidance: %q", text)
	}
}

func skillModelsFallbackServer(t *testing.T, modelID, modelType, curl string, submit func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/" + modelID:
			_, _ = w.Write([]byte(`{"status":{"code":404,"message":"missing"},"data":null}`))
		case "/api/v1/skill/models":
			_, _ = w.Write([]byte(`{"status":{"code":200,"message":"ok"},"data":{"models":[{"id":"` + modelID + `","model_id":"` + modelID + `","name":"Fallback","type":"` + modelType + `","curl":` + mustJSON(curl) + `}],"total":1,"page":1,"page_size":500,"total_pages":1}}`))
		case "/model/v1/queue/" + modelID:
			if submit == nil {
				t.Fatalf("unexpected submit for %s", modelID)
			}
			submit(w, r)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
}

func mustJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
