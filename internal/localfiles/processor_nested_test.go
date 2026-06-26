package localfiles

import (
	"context"
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

func TestPrepareUploadsNestedMultiViewImageURL(t *testing.T) {
	filePath := writeNestedFile(t, "front.png", []byte("png"))
	raw := map[string]string{
		"multi_view_images": `[{"View":"front","ImageUrl":"` + filePath + `"}]`,
	}

	prepared, err := Prepare(t.Context(), raw, multiViewSchema(), func(_ context.Context, path string) (string, error) {
		if path != filePath {
			t.Fatalf("uploaded path = %q, want %q", path, filePath)
		}
		return "https://files.example.com/front.png", nil
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	var media []map[string]any
	if err := json.Unmarshal([]byte(prepared.Raw["multi_view_images"]), &media); err != nil {
		t.Fatalf("unmarshal multi_view_images: %v", err)
	}
	if got := media[0]["ImageUrl"]; got != "https://files.example.com/front.png" {
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
