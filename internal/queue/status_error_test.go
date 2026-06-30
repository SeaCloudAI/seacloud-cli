package queue

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompletedStatusWithErrorNormalizesToFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"request_id":"req-123",
			"status":"COMPLETED",
			"error":{"code":"InvalidParameter","message":"Provider rejected request"},
			"error_type":"REQUEST_INVALID",
			"provider_error":{"code":"InvalidParameter","message":"Provider rejected request","task_status":"FAILED"},
			"logs":[{"message":"provider rejected request","timestamp":"2026-06-27T10:12:27Z"}]
		}`))
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
	if status.Error.Code != "InvalidParameter" {
		t.Fatalf("error code = %q, want InvalidParameter", status.Error.Code)
	}
	if got := status.ProviderError["code"]; got != "InvalidParameter" {
		t.Fatalf("provider_error.code = %#v", got)
	}
	if len(status.Logs) != 1 || status.Logs[0].Message != "provider rejected request" {
		t.Fatalf("unexpected logs: %#v", status.Logs)
	}
}
