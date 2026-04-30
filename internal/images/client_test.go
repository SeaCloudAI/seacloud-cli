package images

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestSupportsSyncModel(t *testing.T) {
	if !SupportsSyncModel("gpt-image-2") {
		t.Fatalf("expected gpt-image-2 to be supported")
	}
	if SupportsSyncModel("kirin_v2_6_i2v") {
		t.Fatalf("expected video model to be unsupported")
	}
}

func TestRequestFromValuesDefaultsResponseFormat(t *testing.T) {
	req, err := RequestFromValues("gpt-image-2", "cat", "", "")
	if err != nil {
		t.Fatalf("RequestFromValues returned error: %v", err)
	}
	if req.Size != DefaultSize {
		t.Fatalf("expected default size %q, got %q", DefaultSize, req.Size)
	}
	if req.ResponseFormat != DefaultResponseFormat {
		t.Fatalf("expected default response format %q, got %q", DefaultResponseFormat, req.ResponseFormat)
	}
}

func TestGeneratePostsToProxyRoute(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody GenerateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"abc"}],"output_format":"png","size":"1024x1024"}`))
	}))
	defer server.Close()

	t.Setenv(EnvProxyURL, server.URL)
	BaseURL = ""

	resp, err := NewClient("cli-key").Generate(GenerateRequest{
		Model:          "gpt-image-2",
		Prompt:         "blue cat",
		Size:           "1024x1024",
		ResponseFormat: "b64_json",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if gotPath != RouteGenerate {
		t.Fatalf("expected path %q, got %q", RouteGenerate, gotPath)
	}
	if gotAuth != "Bearer cli-key" {
		t.Fatalf("expected Authorization header to use CLI key, got %q", gotAuth)
	}
	if gotBody.Model != "gpt-image-2" || gotBody.Prompt != "blue cat" {
		t.Fatalf("unexpected request body: %+v", gotBody)
	}
	if len(resp.Data) != 1 || resp.Data[0].B64JSON != "abc" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestUploadResponseImagesUsesAssetsRoute(t *testing.T) {
	var gotPath string
	var gotBody Base64UploadRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cdn_url":"https://assets-dev.folkos.ai/image/20260425-deadbeef.png"}`))
	}))
	defer server.Close()

	t.Setenv(EnvProxyURL, server.URL)
	BaseURL = ""

	urls, err := NewClient("cli-key").UploadResponseImages(&GenerateResponse{
		OutputFormat: "png",
		Data: []ImageData{{
			B64JSON: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7Z0X8AAAAASUVORK5CYII=",
		}},
	})
	if err != nil {
		t.Fatalf("UploadResponseImages returned error: %v", err)
	}

	if gotPath != RouteUploadBase64 {
		t.Fatalf("expected upload path %q, got %q", RouteUploadBase64, gotPath)
	}
	if strings.TrimSpace(gotBody.Data) == "" {
		t.Fatalf("expected non-empty base64 payload")
	}
	if gotBody.MIMETypeHint != "image/png" {
		t.Fatalf("expected MIME type hint %q, got %q", "image/png", gotBody.MIMETypeHint)
	}
	if len(urls) != 1 || urls[0] != "https://assets-dev.folkos.ai/image/20260425-deadbeef.png" {
		t.Fatalf("unexpected upload urls: %+v", urls)
	}
}

func TestGeneratePrefersGatewayURLOutsideBuildProxyDefault(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"abc"}],"output_format":"png","size":"1024x1024"}`))
	}))
	defer server.Close()

	originalBuildBaseURL := BaseURL
	BaseURL = "http://folkos-gateway.dev.folkos.ai"
	t.Cleanup(func() {
		BaseURL = originalBuildBaseURL
	})

	t.Setenv(EnvProxyURL, "")
	t.Setenv(config.EnvGatewayURL, server.URL)
	t.Setenv(config.EnvFolkosExecToken, "managed-token")
	t.Setenv(config.EnvSeaCloudRuntime, config.RuntimeFolkos)

	_, err := NewClient("cli-key").Generate(GenerateRequest{
		Model:          "gpt-image-2",
		Prompt:         "blue cat",
		Size:           "1024x1024",
		ResponseFormat: "b64_json",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if gotPath != RouteGenerate {
		t.Fatalf("expected path %q, got %q", RouteGenerate, gotPath)
	}
}
