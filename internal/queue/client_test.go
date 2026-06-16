package queue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

func TestSubmitSendsRawJSONBodyAndReturnsRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/model/v1/queue/gpt_image_1" {
			t.Fatalf("expected submit path, got %q", got)
		}
		if got := r.Method; got != http.MethodPost {
			t.Fatalf("expected POST, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer api-key" {
			t.Fatalf("expected API key auth header, got %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, hasModel := body["model"]; hasModel {
			t.Fatalf("queue submit must not send generation wrapper body: %#v", body)
		}
		if got := body["prompt"]; got != "A red apple" {
			t.Fatalf("expected raw prompt param, got %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req-123","status":"QUEUED"}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_GENERATION_URL", server.URL)
	BaseURL = ""

	resp, err := NewClient("api-key").Submit(queueContract(), map[string]any{
		"prompt": "A red apple",
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if resp.ID != "req-123" || resp.Status != "queued" {
		t.Fatalf("unexpected submit response: %#v", resp)
	}
}

func TestStatusAndResultReplaceRequestID(t *testing.T) {
	var seenStatus, seenResult bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/model/v1/queue/gpt_image_1/requests/req-123/status":
			seenStatus = true
			_, _ = w.Write([]byte(`{"request_id":"req-123","status":"COMPLETED","progress":1}`))
		case "/model/v1/queue/gpt_image_1/requests/req-123/response":
			seenResult = true
			_, _ = w.Write([]byte(`{
				"request_id":"req-123",
				"outputs":[{"type":"image","url":"https://example.com/out.png"}]
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_GENERATION_URL", server.URL)
	BaseURL = ""

	client := NewClient("api-key")
	status, err := client.GetStatus(queueContract(), "req-123")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	result, err := client.GetResult(queueContract(), "req-123")
	if err != nil {
		t.Fatalf("GetResult returned error: %v", err)
	}

	if !seenStatus || !seenResult {
		t.Fatalf("expected both status and result endpoints to be called")
	}
	if status.Status != "completed" || status.Progress != 1 {
		t.Fatalf("unexpected status response: %#v", status)
	}
	if urls := result.URLs(); len(urls) != 1 || urls[0] != "https://example.com/out.png" {
		t.Fatalf("unexpected result URLs: %#v", urls)
	}
	if result.Status != "completed" {
		t.Fatalf("expected output-only result to be completed, got %#v", result)
	}
}

func TestGetStatusRetriesTransientRequestErrors(t *testing.T) {
	t.Setenv("SEACLOUD_GENERATION_URL", "https://cloud.seaart.ai")
	BaseURL = ""

	attempts := 0
	client := NewClient("api-key")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, context.DeadlineExceeded
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-123","status":"completed","progress":1}`)),
				Request:    req,
			}, nil
		}),
	}

	status, err := client.GetStatus(queueContract(), "req-123")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected retry after transient error, got %d attempts", attempts)
	}
	if status.Status != "completed" || status.Progress != 1 {
		t.Fatalf("unexpected status response: %#v", status)
	}
}

func TestSubmitDoesNotRetryTransientRequestErrors(t *testing.T) {
	t.Setenv("SEACLOUD_GENERATION_URL", "https://cloud.seaart.ai")
	BaseURL = ""

	attempts := 0
	client := NewClient("api-key")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, context.DeadlineExceeded
		}),
	}

	_, err := client.Submit(queueContract(), map[string]any{"prompt": "A red apple"})
	if err == nil {
		t.Fatal("expected submit error")
	}
	if attempts != 1 {
		t.Fatalf("submit must not retry because it can create duplicate tasks, got %d attempts", attempts)
	}
}

func TestSubmitPreservesInsufficientBalanceErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"status":{"code":402,"message":"Insufficient credits","error_code":40201},"data":null}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_GENERATION_URL", server.URL)
	BaseURL = ""
	_, err := NewClient("api-key").Submit(queueContract(), map[string]any{"prompt": "A red apple"})
	if err == nil {
		t.Fatal("expected submit error")
	}
	var apiErr *clierrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.ErrorCode != 40201 || apiErr.Message != "Insufficient credits" {
		t.Fatalf("unexpected APIError: %#v", apiErr)
	}
	if !clierrors.IsInsufficientBalance(err) {
		t.Fatalf("expected insufficient balance classification for %v", err)
	}
}

func TestCompletedStatusWithErrorNormalizesToFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req-123","status":"COMPLETED","error":"Provider rejected request","error_type":"REQUEST_INVALID"}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_GENERATION_URL", server.URL)
	BaseURL = ""

	status, err := NewClient("api-key").GetStatus(queueContract(), "req-123")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if status.Status != "failed" {
		t.Fatalf("expected failed status, got %#v", status)
	}
	if status.Error == nil || status.Error.Message != "Provider rejected request" {
		t.Fatalf("unexpected error payload: %#v", status.Error)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSubmitRoutesRelativeVtrixEndpointThroughFolkosProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folkos-proxy/model/v1/queue/gpt_image_1" {
			t.Fatalf("expected proxied queue path, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req-123","status":"queued"}`))
	}))
	defer server.Close()

	originalProxyBaseURL := config.DefaultFolkosProxyBaseURL
	config.DefaultFolkosProxyBaseURL = server.URL + "/folkos-proxy"
	t.Cleanup(func() {
		config.DefaultFolkosProxyBaseURL = originalProxyBaseURL
	})
	t.Setenv(config.EnvFolkosExecToken, "managed-token")
	t.Setenv("SEACLOUD_GENERATION_URL", "https://cloud.vtrix.ai")
	BaseURL = ""

	if _, err := NewClient("managed-token").Submit(queueContract(), map[string]any{"prompt": "A red apple"}); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
}

func queueContract() contracts.ModelContract {
	return contracts.ModelContract{
		ModelID:  "gpt_image_1",
		Protocol: "queue",
		BodyMode: "raw_json",
		Endpoints: contracts.ContractEndpoints{
			Submit: contracts.Endpoint{Method: http.MethodPost, Path: "/model/v1/queue/gpt_image_1"},
			Status: contracts.Endpoint{Method: http.MethodGet, Path: "/model/v1/queue/gpt_image_1/requests/{request_id}/status"},
			Result: contracts.Endpoint{Method: http.MethodGet, Path: "/model/v1/queue/gpt_image_1/requests/{request_id}/response"},
		},
	}
}
