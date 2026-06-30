package admindetail

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGetsMultimodalDetailCurl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/multi-models/detail" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("id") != "flux_2_pro" || r.URL.Query().Get("platform") != "seacloud" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer admin-token" {
			t.Fatalf("expected admin bearer token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{"code":200,"message":"ok"},
			"data":{"model":{"id":"flux_2_pro","type":"image","curl":"curl https://provider.example/flux"}}
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODEL_CONTRACTS_URL", server.URL)
	detail, err := NewClient("admin-token").GetMultimodal("flux_2_pro")
	if err != nil {
		t.Fatalf("GetMultimodal returned error: %v", err)
	}
	if detail.Kind != KindMultimodal || detail.Curl != "curl https://provider.example/flux" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if detail.Endpoint != "/api/v1/admin/multi-models/detail?id=flux_2_pro&platform=seacloud" {
		t.Fatalf("unexpected endpoint %q", detail.Endpoint)
	}
}

func TestClientReturnsAuthRequiredWithCurlExample(t *testing.T) {
	t.Setenv("SEACLOUD_MODEL_CONTRACTS_URL", "https://admin.example")
	_, err := NewClient("").GetLLM("gpt-5.2")
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got %v", err)
	}
	text := err.Error()
	if !strings.Contains(text, "admin bearer token required") ||
		!strings.Contains(text, "/api/v1/admin/models/detail?id=gpt-5.2&platform=seacloud") ||
		!strings.Contains(text, "Authorization: Bearer <admin-token>") {
		t.Fatalf("expected auth guidance with curl, got %q", text)
	}
}

func TestClientTreatsAdmin401AsAuthRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":{"code":401,"message":"permission denied"},"data":null}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODEL_CONTRACTS_URL", server.URL)
	_, err := NewClient("bad-token").GetLLM("gpt-5.2")
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got %v", err)
	}
}

func TestClientTreatsAdminDetailNotFoundMessageAsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":{
				"code":500,
				"message":"Failed to get multi model detail: failed to get multi model: rpc error: code = NotFound desc = document missing"
			},
			"data":null
		}`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_MODEL_CONTRACTS_URL", server.URL)
	_, err := NewClient("admin-token").GetMultimodal("gpt-5.2")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
