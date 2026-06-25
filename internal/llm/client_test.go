package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

func TestEndpointURLUsesAbsoluteEndpoint(t *testing.T) {
	t.Setenv("SEACLOUD_LLM_URL", "https://llm.example.com")
	got, err := endpointURL("https://upstream.example.com/v1/chat/completions")
	if err != nil {
		t.Fatalf("endpointURL returned error: %v", err)
	}
	if got != "https://upstream.example.com/v1/chat/completions" {
		t.Fatalf("unexpected endpoint URL: %q", got)
	}
}

func TestEndpointURLPrefersLLMEnvOverBaseEnvAndBuildBase(t *testing.T) {
	t.Setenv("SEACLOUD_LLM_URL", "https://llm-env.example.com")
	t.Setenv("SEACLOUD_BASE_URL", "https://base-env.example.com")
	originalBaseURL := BaseURL
	BaseURL = "https://build.example.com"
	t.Cleanup(func() { BaseURL = originalBaseURL })

	got, err := endpointURL("/v1/responses")
	if err != nil {
		t.Fatalf("endpointURL returned error: %v", err)
	}
	if got != "https://llm-env.example.com/v1/responses" {
		t.Fatalf("unexpected endpoint URL: %q", got)
	}
}

func TestCompleteChatCompletionsSendsHeadersAndExtractsText(t *testing.T) {
	var sawRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if got := r.URL.Path; got != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer api-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		if got := r.Header.Get("X-Source"); got != "cli" {
			t.Fatalf("unexpected X-Source header %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "gpt-4o-mini" {
			t.Fatalf("model was not sent from contract: %#v", body)
		}
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1",
			"model":"gpt-4o-mini",
			"choices":[{"message":{"content":"hello from chat"},"finish_reason":"stop"}],
			"usage":{"total_tokens":7}
		}`))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	result, err := NewClient("api-key").Complete(context.Background(), chatContract(), map[string]any{
		"model":    "gpt-4o-mini",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if !sawRequest {
		t.Fatal("server was not called")
	}
	if result.Text != "hello from chat" || result.FinishReason != "stop" || result.Usage["total_tokens"].(float64) != 7 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCompleteResponsesExtractsOutputText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/responses" {
			t.Fatalf("unexpected path %q", got)
		}
		_, _ = w.Write([]byte(`{
			"id":"resp-1",
			"model":"gpt-5-mini",
			"output":[{"content":[{"type":"output_text","text":"hello from responses"}]}],
			"usage":{"output_tokens":3}
		}`))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	result, err := NewClient("api-key").Complete(context.Background(), responsesContract(), map[string]any{
		"model": "gpt-5-mini",
		"input": "hello",
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if result.Text != "hello from responses" || result.Usage["output_tokens"].(float64) != 3 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCompleteNon2xxReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"status":{"code":402,"message":"Insufficient credits","error_code":40201},"data":null}`))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	_, err := NewClient("api-key").Complete(context.Background(), chatContract(), map[string]any{"model": "gpt-4o-mini"})
	var apiErr *clierrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != http.StatusPaymentRequired || apiErr.ErrorCode != 40201 {
		t.Fatalf("unexpected APIError: %#v", apiErr)
	}
}

func TestStreamChatAggregatesDeltasAndDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}],\"usage\":{\"total_tokens\":4}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	var deltas []string
	result, err := NewClient("api-key").Stream(context.Background(), chatContract(), map[string]any{
		"model":  "gpt-4o-mini",
		"stream": true,
	}, StreamOptions{OnText: func(text string) error {
		deltas = append(deltas, text)
		return nil
	}})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if strings.Join(deltas, "") != "hello" || result.Text != "hello" || result.FinishReason != "stop" {
		t.Fatalf("unexpected stream result: deltas=%#v result=%#v", deltas, result)
	}
}

func TestStreamResponsesAggregatesDeltasAndCompleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"model\":\"gpt-5-mini\",\"usage\":{\"output_tokens\":2}}}\n\n"))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	result, err := NewClient("api-key").Stream(context.Background(), responsesContract(), map[string]any{
		"model":  "gpt-5-mini",
		"stream": true,
	}, StreamOptions{})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if result.Text != "hello" || result.ID != "resp-1" || result.Usage["output_tokens"].(float64) != 2 {
		t.Fatalf("unexpected stream result: %#v", result)
	}
}

func TestStreamEOFWithoutCompletionMarkerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	_, err := NewClient("api-key").Stream(context.Background(), chatContract(), map[string]any{
		"model":  "gpt-4o-mini",
		"stream": true,
	}, StreamOptions{})
	if err == nil || !strings.Contains(err.Error(), "stream ended before completion marker") {
		t.Fatalf("expected incomplete stream error, got %v", err)
	}
}

func TestStreamErrorFrameReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: error\n"))
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"upstream failed\"}}\n\n"))
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	_, err := NewClient("api-key").Stream(context.Background(), chatContract(), map[string]any{
		"model":  "gpt-4o-mini",
		"stream": true,
	}, StreamOptions{})
	var apiErr *clierrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
}

func TestStreamUsesRequestContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()
	t.Setenv("SEACLOUD_LLM_URL", server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := NewClient("api-key").Stream(ctx, chatContract(), map[string]any{
		"model":  "gpt-4o-mini",
		"stream": true,
	}, StreamOptions{})
	if err == nil {
		t.Fatal("expected context timeout")
	}
}

func chatContract() contracts.ModelContract {
	return contracts.ModelContract{
		ModelID:  "gpt-4o-mini",
		Protocol: "llm_chat_completions",
		BodyMode: "openai_chat_json",
		Endpoints: contracts.ContractEndpoints{
			ChatCompletions: contracts.Endpoint{Method: http.MethodPost, Path: "/v1/chat/completions"},
		},
	}
}

func responsesContract() contracts.ModelContract {
	return contracts.ModelContract{
		ModelID:  "gpt-5-mini",
		Protocol: "llm_responses",
		BodyMode: "openai_responses_json",
		Endpoints: contracts.ContractEndpoints{
			Responses: contracts.Endpoint{Method: http.MethodPost, Path: "/v1/responses"},
		},
	}
}
