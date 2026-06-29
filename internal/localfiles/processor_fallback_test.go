package localfiles

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestPrepareEncodesNestedSmallImageArrayFileAsBase64(t *testing.T) {
	filePath := writeNestedFile(t, "small.png", []byte("png"))
	wantBase64 := base64.StdEncoding.EncodeToString([]byte("png"))
	raw := map[string]string{
		"images": `["` + filePath + `"]`,
	}
	uploadCalled := false

	prepared, err := Prepare(t.Context(), raw, nestedImageArraySchema(), func(_ context.Context, path string) (string, error) {
		uploadCalled = true
		t.Fatal("small nested image must not upload before fallback")
		return "", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadCalled {
		t.Fatal("small nested image uploaded before fallback")
	}
	var images []string
	if err := json.Unmarshal([]byte(prepared.Raw["images"]), &images); err != nil {
		t.Fatalf("unmarshal images: %v", err)
	}
	if got := images[0]; got != wantBase64 {
		t.Fatalf("images[0] = %q", got)
	}
}

func TestFallbackRawUploadsNestedSmallImageArrayFile(t *testing.T) {
	filePath := writeNestedFile(t, "small.png", []byte("png"))
	raw := map[string]string{
		"images": `["` + filePath + `"]`,
	}
	var uploadedPath string

	prepared, err := Prepare(t.Context(), raw, nestedImageArraySchema(), func(_ context.Context, path string) (string, error) {
		uploadedPath = path
		return "https://files.example.com/small.png", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	fallbackRaw, err := prepared.FallbackRaw(t.Context())
	if err != nil {
		t.Fatalf("FallbackRaw returned error: %v", err)
	}
	if uploadedPath != filePath {
		t.Fatalf("uploaded path = %q, want %q", uploadedPath, filePath)
	}
	var images []string
	if err := json.Unmarshal([]byte(fallbackRaw["images"]), &images); err != nil {
		t.Fatalf("unmarshal images: %v", err)
	}
	if got := images[0]; got != "https://files.example.com/small.png" {
		t.Fatalf("images[0] = %q", got)
	}
}
