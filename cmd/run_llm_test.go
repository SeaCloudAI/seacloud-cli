package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunLLMChatContractPrintsTextAndInjectsModel(t *testing.T) {
	var sawLLM bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_4o_mini":
			writeLLMChatContract(w)
		case "/v1/chat/completions":
			sawLLM = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode LLM body: %v", err)
			}
			if body["model"] != "gpt_4o_mini" {
				t.Fatalf("model was not injected from contract: %#v", body)
			}
			if _, ok := body["stream"]; ok {
				t.Fatalf("non-stream request should not force stream: %#v", body)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"chat text"},"finish_reason":"stop"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_4o_mini",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !sawLLM {
		t.Fatal("LLM endpoint was not called")
	}
	if got := strings.TrimSpace(stdout); got != "chat text" {
		t.Fatalf("unexpected stdout %q", stdout)
	}
}

func TestRunLLMNonStreamOutputJSONPrintsRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_4o_mini":
			writeLLMChatContract(w)
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"id":"chatcmpl-raw","choices":[{"message":{"content":"raw text"},"finish_reason":"stop"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_4o_mini", "--output", "json",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not JSON: %q", stdout)
	}
	if out["id"] != "chatcmpl-raw" || out["text"] != nil {
		t.Fatalf("expected raw upstream JSON, got %#v", out)
	}
}

func TestRunLLMRejectsUserModelParam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMChatContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, _, err := executeRoot(t, "run", "gpt_4o_mini",
		"--param", "model=other",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err == nil || !strings.Contains(err.Error(), "model is controlled by the model contract") {
		t.Fatalf("expected controlled model error, got %v", err)
	}
}

func TestRunLLMStreamFlagPrintsDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_4o_mini":
			w.Header().Set("Content-Type", "application/json")
			writeLLMChatContract(w)
		case "/v1/chat/completions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode LLM body: %v", err)
			}
			if body["stream"] != true {
				t.Fatalf("stream flag was not injected: %#v", body)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"llo\"},\"finish_reason\":\"stop\"}]}\n\n"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_4o_mini", "--stream",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if stdout != "hello\n" {
		t.Fatalf("unexpected stream stdout %q", stdout)
	}
}

func TestRunLLMStreamOutputJSONPrintsAggregate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_5_mini":
			w.Header().Set("Content-Type", "application/json")
			writeLLMResponsesContract(w)
		case "/v1/responses":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"json\"}\n\n"))
			_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"model\":\"gpt_5_mini\",\"usage\":{\"output_tokens\":1}}}\n\n"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_5_mini", "--stream", "--output", "json", "--param", "input=hello")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not JSON: %q", stdout)
	}
	if out["text"] != "json" || out["id"] != "resp-1" {
		t.Fatalf("unexpected JSON output: %#v", out)
	}
}

func TestRunLLMOutputSSEForwardsEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skill/model-contracts/gpt_4o_mini":
			w.Header().Set("Content-Type", "application/json")
			writeLLMChatContract(w)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"raw\"},\"finish_reason\":\"stop\"}]}\n\n"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "run", "gpt_4o_mini", "--stream", "--output", "sse",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout, "data: ") || !strings.Contains(stdout, `"content":"raw"`) {
		t.Fatalf("expected raw SSE output, got %q", stdout)
	}
	if strings.Contains(stdout, "\n\n\n") {
		t.Fatalf("raw SSE output should not duplicate blank event separators: %q", stdout)
	}
}

func TestRunLLMRejectsInvalidOutputModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMChatContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, _, err := executeRoot(t, "run", "gpt_4o_mini", "--output", "url",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err == nil || !strings.Contains(err.Error(), "--output url is not supported for LLM models") {
		t.Fatalf("expected url output error, got %v", err)
	}

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, _, err = executeRoot(t, "run", "gpt_4o_mini", "--output", "sse",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err == nil || !strings.Contains(err.Error(), "--output sse requires streaming") {
		t.Fatalf("expected sse non-stream error, got %v", err)
	}
}

func TestRunLLMStreamFlagConflictsWithParamFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMChatContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, _, err := executeRoot(t, "run", "gpt_4o_mini", "--stream",
		"--param", "stream=false",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err == nil || !strings.Contains(err.Error(), "--stream conflicts with --param stream=false") {
		t.Fatalf("expected stream conflict error, got %v", err)
	}
}

func TestRunAsyncRejectsLLMContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMChatContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, _, err := executeRoot(t, "run-async", "gpt_4o_mini",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err == nil || !strings.Contains(err.Error(), "LLM contracts do not support run-async") {
		t.Fatalf("expected run-async LLM error, got %v", err)
	}
}

func TestRunLLMDryRunInjectsModelAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMResponsesContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, stderr, err := executeRoot(t, "--dry-run", "run", "gpt_5_mini", "--stream", "--param", "input=hello")
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if !strings.Contains(stderr, "protocol=llm_responses") ||
		!strings.Contains(stderr, "responses=POST /v1/responses") ||
		!strings.Contains(stderr, `"model":"gpt_5_mini"`) ||
		!strings.Contains(stderr, `"stream":true`) {
		t.Fatalf("expected LLM dry-run details, got stderr=%q", stderr)
	}
}
