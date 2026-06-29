package localfiles

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

const (
	Base64LimitBytes = int64(10 * 1024 * 1024)
	MaxFileBytes     = int64(100 * 1024 * 1024)
	MaxFileParams    = 5
)

type UploadFunc func(context.Context, string) (string, error)

type Prepared struct {
	Raw          map[string]string
	upload       UploadFunc
	fallback     []fileParam
	fallbackRaw  map[string]string
	fallbackUsed bool
}

type fileParam struct {
	key     string
	path    string
	encoded string
	nested  bool
}

func Prepare(ctx context.Context, raw map[string]string, schema contracts.InputSchema, upload UploadFunc) (*Prepared, error) {
	out := copyRaw(raw)
	prepared := &Prepared{Raw: out, upload: upload}
	count := 0
	for key, value := range raw {
		nestedValue, handled, err := prepareNestedJSON(ctx, key, value, schema, upload, &count, prepared)
		if err != nil {
			return nil, err
		}
		if handled {
			out[key] = nestedValue
			continue
		}
		path, exists, explicit, err := localPath(value)
		if err != nil {
			return nil, err
		}
		if !exists {
			if explicit {
				return nil, &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s", value)}
			}
			continue
		}
		count++
		if count > MaxFileParams {
			return nil, &clierrors.CLIError{Message: fmt.Sprintf("too_many_files: at most %d local file parameters are supported", MaxFileParams)}
		}
		if err := validateFormat(key, path, schema); err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fileAccessError(path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s is not a regular file", path)}
		}
		if info.Size() > MaxFileBytes {
			return nil, &clierrors.CLIError{Message: fmt.Sprintf("file_size_exceeded: %s exceeds 100MB", path)}
		}
		if shouldUploadDirect(path, formatForKey(key, schema), info.Size(), false) {
			url, err := uploadFile(ctx, upload, path)
			if err != nil {
				return nil, err
			}
			out[key] = url
			continue
		}
		encoded, err := encodeFileBase64(path)
		if err != nil {
			return nil, fileAccessError(path, err)
		}
		out[key] = encoded
		prepared.fallback = append(prepared.fallback, fileParam{key: key, path: path})
	}
	return prepared, nil
}

func (p *Prepared) ShouldFallback(err error) bool {
	if p == nil || p.fallbackUsed || len(p.fallback) == 0 || err == nil {
		return false
	}
	var apiErr *clierrors.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus == 400 || apiErr.HTTPStatus == 422 || apiErr.StatusCode == 400 || apiErr.StatusCode == 422
	}
	text := strings.ToLower(err.Error())
	for _, item := range p.fallback {
		if strings.Contains(text, strings.ToLower(item.key)) {
			return true
		}
	}
	return strings.Contains(text, "base64") || strings.Contains(text, "format") ||
		strings.Contains(text, "invalid image") || strings.Contains(text, "invalid file")
}

func (p *Prepared) FallbackRaw(ctx context.Context) (map[string]string, error) {
	if p == nil {
		return nil, nil
	}
	if p.fallbackRaw != nil {
		p.fallbackUsed = true
		return copyRaw(p.fallbackRaw), nil
	}
	out := copyRaw(p.Raw)
	for _, item := range p.fallback {
		url, err := uploadFile(ctx, p.upload, item.path)
		if err != nil {
			return nil, err
		}
		if item.nested {
			next, err := replaceNestedFallbackValue(out[item.key], item.encoded, url)
			if err != nil {
				return nil, err
			}
			out[item.key] = next
			continue
		}
		out[item.key] = url
	}
	p.fallbackUsed = true
	p.fallbackRaw = copyRaw(out)
	return out, nil
}

func encodeFileBase64(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(content), nil
}

func replaceNestedFallbackValue(raw, encoded, url string) (string, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: invalid nested fallback JSON: %v", err)}
	}
	if !replaceFirstStringValue(&parsed, encoded, url) {
		return "", &clierrors.CLIError{Message: "upload_failed: nested fallback value was not found"}
	}
	data, err := json.Marshal(parsed)
	if err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: invalid nested fallback JSON: %v", err)}
	}
	return string(data), nil
}

func replaceFirstStringValue(value *any, oldValue, newValue string) bool {
	switch typed := (*value).(type) {
	case string:
		if typed != oldValue {
			return false
		}
		*value = newValue
		return true
	case []any:
		for i := range typed {
			if replaceFirstStringValue(&typed[i], oldValue, newValue) {
				return true
			}
		}
	case map[string]any:
		for key, child := range typed {
			if replaceFirstStringValue(&child, oldValue, newValue) {
				typed[key] = child
				return true
			}
		}
	}
	return false
}

func copyRaw(raw map[string]string) map[string]string {
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	return out
}

func uploadFile(ctx context.Context, upload UploadFunc, path string) (string, error) {
	if upload == nil {
		return "", &clierrors.CLIError{Message: "upload_failed: upload URL is not configured"}
	}
	return upload(ctx, path)
}

func fileAccessError(path string, err error) error {
	if os.IsNotExist(err) {
		return &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s", path)}
	}
	if os.IsPermission(err) {
		return &clierrors.CLIError{Message: fmt.Sprintf("file_access_denied: %s", path)}
	}
	return &clierrors.CLIError{Message: fmt.Sprintf("file_access_denied: %s: %v", path, err)}
}

func localPath(value string) (string, bool, bool, error) {
	if isHTTPURL(value) {
		return "", false, false, nil
	}
	explicit := isExplicitPath(value)
	path := expandHome(value)
	info, err := os.Stat(path)
	if err == nil {
		return path, !info.IsDir(), true, nil
	}
	if os.IsNotExist(err) {
		return path, false, explicit, nil
	}
	if !explicit {
		return path, false, false, nil
	}
	return path, false, explicit, fileAccessError(path, err)
}

func isHTTPURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func expandHome(value string) string {
	if value == "~" || strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	return value
}

func isExplicitPath(value string) bool {
	if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~/") {
		return true
	}
	if len(value) >= 3 && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if runtime.GOOS == "windows" && filepath.IsAbs(value) {
		return true
	}
	return false
}
