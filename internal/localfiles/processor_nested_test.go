package localfiles

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

func TestPrepareLeavesNestedRemoteURLUnchanged(t *testing.T) {
	raw := map[string]string{
		"media": `[{"type":"video","url":"https://example.com/a.mp4"}]`,
	}
	uploadCalled := false

	prepared, err := Prepare(t.Context(), raw, nestedMediaSchema(), func(context.Context, string) (string, error) {
		uploadCalled = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadCalled {
		t.Fatal("remote nested URL must not be uploaded")
	}
	if got := prepared.Raw["media"]; got != raw["media"] {
		t.Fatalf("media = %s, want unchanged %s", got, raw["media"])
	}
}

func TestPrepareUploadsNestedMediaURL(t *testing.T) {
	filePath := writeNestedFile(t, "clip.mp4", []byte("video"))
	raw := map[string]string{
		"media": `[{"type":"video","url":"` + filePath + `"}]`,
	}
	var uploadedPath string

	prepared, err := Prepare(t.Context(), raw, nestedMediaSchema(), func(_ context.Context, path string) (string, error) {
		uploadedPath = path
		return "https://files.example.com/clip.mp4", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadedPath != filePath {
		t.Fatalf("uploaded path = %q, want %q", uploadedPath, filePath)
	}
	gotURL := nestedMediaURL(t, prepared.Raw["media"])
	if gotURL != "https://files.example.com/clip.mp4" {
		t.Fatalf("media[0].url = %q", gotURL)
	}
}

func TestPrepareUploadsNestedLargeStringArrayFileWithoutFormat(t *testing.T) {
	filePath := writeNestedLargeFile(t, "large.png", Base64LimitBytes+1)
	raw := map[string]string{
		"images": `["` + filePath + `"]`,
	}
	var uploadedPath string

	prepared, err := Prepare(t.Context(), raw, nestedImageArraySchema(), func(_ context.Context, path string) (string, error) {
		uploadedPath = path
		return "https://files.example.com/large.png", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadedPath != filePath {
		t.Fatalf("uploaded path = %q, want %q", uploadedPath, filePath)
	}
	var images []string
	if err := json.Unmarshal([]byte(prepared.Raw["images"]), &images); err != nil {
		t.Fatalf("unmarshal images: %v", err)
	}
	if got := images[0]; got != "https://files.example.com/large.png" {
		t.Fatalf("images[0] = %q", got)
	}
}

func TestPrepareLeavesNestedStringArrayTextAndRemoteURLUnchanged(t *testing.T) {
	raw := map[string]string{
		"images": `["not a file","https://example.com/a.png"]`,
	}
	uploadCalled := false

	prepared, err := Prepare(t.Context(), raw, nestedImageArraySchema(), func(context.Context, string) (string, error) {
		uploadCalled = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadCalled {
		t.Fatal("text or remote URL values must not be uploaded")
	}
	if got := prepared.Raw["images"]; got != raw["images"] {
		t.Fatalf("images = %s, want unchanged %s", got, raw["images"])
	}
}

func TestPrepareKeepsLongPromptTextWithSlashUnchanged(t *testing.T) {
	longText := strings.Repeat("describe lighting/materials/camera angle ", 30)
	raw := map[string]string{
		"prompt": longText,
	}
	uploadCalled := false

	prepared, err := Prepare(t.Context(), raw, contracts.InputSchema{
		Type: "object",
		Properties: map[string]contracts.InputSchema{
			"prompt": {Type: "string"},
		},
	}, func(context.Context, string) (string, error) {
		uploadCalled = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if uploadCalled {
		t.Fatal("prompt text must not be uploaded")
	}
	if got := prepared.Raw["prompt"]; got != longText {
		t.Fatalf("prompt = %q, want unchanged", got)
	}
}

func TestPrepareRejectsNestedLargeFileOverMaxWithoutFormat(t *testing.T) {
	filePath := writeNestedLargeFile(t, "too-large.png", MaxFileBytes+1)
	raw := map[string]string{
		"images": `["` + filePath + `"]`,
	}

	_, err := Prepare(t.Context(), raw, nestedImageArraySchema(), func(context.Context, string) (string, error) {
		t.Fatal("upload must not be called for over-limit files")
		return "", nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "file_size_exceeded: images[0]: "+filePath) {
		t.Fatalf("error = %v", err)
	}
}

func TestPrepareEncodesNestedMultiViewImageURLAsBase64(t *testing.T) {
	filePath := writeNestedFile(t, "front.png", []byte("png"))
	wantBase64 := base64.StdEncoding.EncodeToString([]byte("png"))
	raw := map[string]string{
		"multi_view_images": `[{"View":"front","ImageUrl":"` + filePath + `"}]`,
	}
	uploadCalled := false

	prepared, err := Prepare(t.Context(), raw, multiViewSchema(), func(_ context.Context, path string) (string, error) {
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
	var media []map[string]any
	if err := json.Unmarshal([]byte(prepared.Raw["multi_view_images"]), &media); err != nil {
		t.Fatalf("unmarshal multi_view_images: %v", err)
	}
	if got := media[0]["ImageUrl"]; got != wantBase64 {
		t.Fatalf("multi_view_images[0].ImageUrl = %#v", got)
	}
}

func TestPrepareReportsNestedMissingFilePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.mp4")
	raw := map[string]string{
		"media": `[{"type":"video","url":"` + missing + `"}]`,
	}

	_, err := Prepare(t.Context(), raw, nestedMediaSchema(), func(context.Context, string) (string, error) {
		t.Fatal("upload must not be called for a missing file")
		return "", nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "file_not_found: media[0].url: "+missing) {
		t.Fatalf("error = %v", err)
	}
}

func nestedMediaSchema() contracts.InputSchema {
	return contracts.InputSchema{
		Type: "object",
		Properties: map[string]contracts.InputSchema{
			"media": {
				Type: "array",
				Items: &contracts.InputSchema{
					Type: "object",
					Properties: map[string]contracts.InputSchema{
						"type": {Type: "string"},
						"url":  {Type: "string", Format: "uri"},
					},
				},
			},
		},
	}
}

func multiViewSchema() contracts.InputSchema {
	return contracts.InputSchema{
		Type: "object",
		Properties: map[string]contracts.InputSchema{
			"multi_view_images": {
				Type: "array",
				Items: &contracts.InputSchema{
					Type: "object",
					Properties: map[string]contracts.InputSchema{
						"View":     {Type: "string"},
						"ImageUrl": {Type: "string", Format: "url"},
					},
				},
			},
		},
	}
}

func nestedImageArraySchema() contracts.InputSchema {
	return contracts.InputSchema{
		Type: "object",
		Properties: map[string]contracts.InputSchema{
			"images": {
				Type:  "array",
				Items: &contracts.InputSchema{Type: "string"},
			},
		},
	}
}

func nestedMediaURL(t *testing.T, value string) string {
	t.Helper()
	var media []map[string]any
	if err := json.Unmarshal([]byte(value), &media); err != nil {
		t.Fatalf("unmarshal media: %v", err)
	}
	return media[0]["url"].(string)
}

func writeNestedFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func writeNestedLargeFile(t *testing.T, name string, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatalf("truncate temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return path
}
