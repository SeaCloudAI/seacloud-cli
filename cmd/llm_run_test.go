package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLLMRunChatContractPrintsTextAndInjectsModel(t *testing.T) {
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
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"llm text"},"finish_reason":"stop"}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	stdout, _, err := executeRoot(t, "llm", "run", "gpt_4o_mini",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("llm run returned error: %v", err)
	}
	if !sawLLM {
		t.Fatal("LLM endpoint was not called")
	}
	if got := strings.TrimSpace(stdout); got != "llm text" {
		t.Fatalf("unexpected stdout %q", stdout)
	}
}

func TestLLMRunStreamFlagPrintsDeltas(t *testing.T) {
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
	stdout, _, err := executeRoot(t, "llm", "run", "gpt_4o_mini", "--stream",
		"--param", `messages=[{"role":"user","content":"hello"}]`)
	if err != nil {
		t.Fatalf("llm run returned error: %v", err)
	}
	if stdout != "hello\n" {
		t.Fatalf("unexpected stream stdout %q", stdout)
	}
}

func TestLLMRunDryRunInjectsModelAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeLLMResponsesContract(w)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	t.Setenv("SEACLOUD_LLM_URL", server.URL)
	_, stderr, err := executeRoot(t, "--dry-run", "llm", "run", "gpt_5_mini", "--stream", "--param", "input=hello")
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

func TestLLMRunRejectsNonLLMContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeContractResponse(t, w, r)
	}))
	defer server.Close()

	setupRunCommandTest(t, server.URL)
	_, _, err := executeRoot(t, "llm", "run", "gpt_image_1", "--param", "prompt=A red apple")
	if err == nil ||
		!strings.Contains(err.Error(), "model gpt_image_1 is not an LLM model") ||
		!strings.Contains(err.Error(), "seacloud run gpt_image_1") {
		t.Fatalf("expected non-LLM guidance error, got %v", err)
	}
}

func TestLLMRunHelpAndAgentDescribe(t *testing.T) {
	resetHelpFlags(rootCmd)
	t.Cleanup(func() { resetHelpFlags(rootCmd) })

	rootHelp, _, err := executeRoot(t, "--help")
	if err != nil {
		t.Fatalf("root help returned error: %v", err)
	}
	if !strings.Contains(rootHelp, "llm         Run LLM models through LLM-only commands") {
		t.Fatalf("root help missing llm command:\n%s", rootHelp)
	}

	llmHelp, _, err := executeRoot(t, "llm", "--help")
	if err != nil {
		t.Fatalf("llm help returned error: %v", err)
	}
	if !strings.Contains(llmHelp, "Call LLM contract models only") ||
		!strings.Contains(llmHelp, "run         Run an LLM model and print text, JSON, or SSE") {
		t.Fatalf("llm help missing expected text:\n%s", llmHelp)
	}

	llmRunHelp, _, err := executeRoot(t, "llm", "run", "--help")
	if err != nil {
		t.Fatalf("llm run help returned error: %v", err)
	}
	if !strings.Contains(llmRunHelp, "Only LLM contracts are accepted") ||
		!strings.Contains(llmRunHelp, "Output format: json (full response), sse (raw LLM stream)") {
		t.Fatalf("llm run help missing expected text:\n%s", llmRunHelp)
	}

	agentGuide, _, err := executeRoot(t, "agent", "describe")
	if err != nil {
		t.Fatalf("agent describe returned error: %v", err)
	}
	if !strings.Contains(agentGuide, "seacloud llm run <model_id> --param key=value") {
		t.Fatalf("agent describe missing llm run command:\n%s", agentGuide)
	}
}
