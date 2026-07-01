package localfiles

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDefaultUploadEndpointUsesPublishedStorageEndpoint(t *testing.T) {
	t.Setenv(EnvUploadURL, "")
	t.Setenv("SEACLOUD_MODELS_URL", "")
	old := DefaultUploadURL
	DefaultUploadURL = "https://sea-cloud-admin-web.real-cloud.seaart.ai/api/v1/storage/files"
	t.Cleanup(func() { DefaultUploadURL = old })

	got := DefaultUploadEndpoint()
	want := "https://sea-cloud-admin-web.real-cloud.seaart.ai/api/v1/storage/files"
	if got != want {
		t.Fatalf("DefaultUploadEndpoint() = %q, want %q", got, want)
	}
}

func TestDefaultUploadEndpointDerivesFromModelsURL(t *testing.T) {
	t.Setenv(EnvUploadURL, "")
	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:18080/")
	old := DefaultUploadURL
	DefaultUploadURL = "https://build.example.com/api/v1/storage/files"
	t.Cleanup(func() { DefaultUploadURL = old })

	got := DefaultUploadEndpoint()
	want := "http://127.0.0.1:18080/api/v1/storage/files"
	if got != want {
		t.Fatalf("DefaultUploadEndpoint() = %q, want %q", got, want)
	}
}

func TestDefaultUploadEndpointAllowsEnvironmentOverride(t *testing.T) {
	t.Setenv(EnvUploadURL, "http://127.0.0.1:18080/upload")
	t.Setenv("SEACLOUD_MODELS_URL", "http://127.0.0.1:18080")
	old := DefaultUploadURL
	DefaultUploadURL = "https://build.example.com/upload"
	t.Cleanup(func() { DefaultUploadURL = old })

	if got := DefaultUploadEndpoint(); got != "http://127.0.0.1:18080/upload" {
		t.Fatalf("DefaultUploadEndpoint() = %q", got)
	}
}

func TestDefaultUploadEndpointRequiresConfiguration(t *testing.T) {
	t.Setenv(EnvUploadURL, "")
	t.Setenv("SEACLOUD_MODELS_URL", "")
	old := DefaultUploadURL
	DefaultUploadURL = ""
	t.Cleanup(func() { DefaultUploadURL = old })

	if got := DefaultUploadEndpoint(); got != "" {
		t.Fatalf("DefaultUploadEndpoint() = %q, want empty", got)
	}
}

func TestHTTPUploaderUsesAPIKeyHeaderWhenBothTokensPresent(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "upload-*.png")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := file.Write([]byte("png")); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer api-key" {
			t.Fatalf("Authorization = %q, want API key", got)
		}
		_, _ = w.Write([]byte(`{"url":"https://files.example.com/ref.png"}`))
	}))
	defer server.Close()

	uploader := &HTTPUploader{Endpoint: server.URL, AuthToken: "auth-token", APIKey: "api-key", HTTPClient: server.Client()}
	url, err := uploader.Upload(t.Context(), file.Name())
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if url != "https://files.example.com/ref.png" {
		t.Fatalf("Upload URL = %q", url)
	}
}
