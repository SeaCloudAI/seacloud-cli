package localfiles

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

var mediaExtensions = map[string]map[string]bool{
	"image": {
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true,
	},
	"video": {
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
	},
	"audio": {
		".mp3": true, ".wav": true, ".aac": true, ".flac": true,
	},
}

func validateFormat(key, path string, schema contracts.InputSchema) error {
	format := strings.ToLower(strings.TrimSpace(formatForKey(key, schema)))
	if format == "" || format == "uri" || format == "url" || format == "string" {
		return nil
	}
	allowed, ok := mediaExtensions[format]
	if !ok {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if allowed[ext] {
		return nil
	}
	return &clierrors.CLIError{
		Message: fmt.Sprintf("unsupported_file_type: parameter %q expects %s file, got %s", key, format, ext),
	}
}

func shouldUploadDirect(path, format string, size int64, nested bool) bool {
	if size > Base64LimitBytes {
		return true
	}
	kind := localFileKind(path, format)
	if kind == "video" || kind == "audio" {
		return true
	}
	return nested && kind != "image"
}

func localFileKind(path, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if _, ok := mediaExtensions[format]; ok {
		return format
	}
	ext := strings.ToLower(filepath.Ext(path))
	for kind, allowed := range mediaExtensions {
		if allowed[ext] {
			return kind
		}
	}
	return ""
}

func formatForKey(key string, schema contracts.InputSchema) string {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return ""
	}
	prop, ok := schema.Properties[parts[0]]
	if !ok {
		return ""
	}
	for _, part := range parts[1:] {
		if prop.Properties == nil {
			return prop.Format
		}
		next, ok := prop.Properties[part]
		if !ok {
			return prop.Format
		}
		prop = next
	}
	return prop.Format
}
