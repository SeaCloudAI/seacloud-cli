package localfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

func prepareNestedJSON(ctx context.Context, key, value string, schema contracts.InputSchema, upload UploadFunc, count *int) (string, bool, error) {
	fieldSchema, ok := schema.Properties[key]
	if !ok || !isJSONContainer(fieldSchema) || !looksLikeJSON(value) {
		return value, false, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return value, true, nil
	}

	next, modified, err := prepareNestedValue(ctx, key, parsed, fieldSchema, upload, count)
	if err != nil {
		return value, true, err
	}
	if !modified {
		return value, true, nil
	}
	data, err := json.Marshal(next)
	if err != nil {
		return value, true, err
	}
	return string(data), true, nil
}

func prepareNestedValue(ctx context.Context, path string, value any, schema contracts.InputSchema, upload UploadFunc, count *int) (any, bool, error) {
	switch schema.Type {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return value, false, nil
		}
		modified := false
		for name, childSchema := range schema.Properties {
			childValue, ok := obj[name]
			if !ok {
				continue
			}
			next, childModified, err := prepareNestedValue(ctx, nestedField(path, name), childValue, childSchema, upload, count)
			if err != nil {
				return value, false, err
			}
			if childModified {
				obj[name] = next
				modified = true
			}
		}
		return obj, modified, nil
	case "array":
		arr, ok := value.([]any)
		if !ok || schema.Items == nil {
			return value, false, nil
		}
		modified := false
		for i, item := range arr {
			next, itemModified, err := prepareNestedValue(ctx, fmt.Sprintf("%s[%d]", path, i), item, *schema.Items, upload, count)
			if err != nil {
				return value, false, err
			}
			if itemModified {
				arr[i] = next
				modified = true
			}
		}
		return arr, modified, nil
	default:
		text, ok := value.(string)
		if !ok || isHTTPURL(text) {
			return value, false, nil
		}
		return prepareNestedString(ctx, path, text, schema, upload, count)
	}
}

func prepareNestedString(ctx context.Context, fieldPath, value string, schema contracts.InputSchema, upload UploadFunc, count *int) (string, bool, error) {
	path, exists, explicit, err := localPath(value)
	if err != nil {
		return value, false, nestedFileAccessError(fieldPath, path, err)
	}
	if !exists {
		if explicit {
			return value, false, &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s: %s", fieldPath, value)}
		}
		return value, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return value, false, nestedFileAccessError(fieldPath, path, err)
	}
	if !info.Mode().IsRegular() {
		return value, false, &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s: %s is not a regular file", fieldPath, path)}
	}
	if info.Size() > MaxFileBytes {
		return value, false, &clierrors.CLIError{Message: fmt.Sprintf("file_size_exceeded: %s: %s exceeds 100MB", fieldPath, path)}
	}
	if info.Size() <= Base64LimitBytes && !isURLFormat(schema) {
		return value, false, nil
	}
	*count = *count + 1
	if *count > MaxFileParams {
		return value, false, &clierrors.CLIError{Message: fmt.Sprintf("too_many_files: at most %d local file parameters are supported", MaxFileParams)}
	}
	url, err := uploadFile(ctx, upload, path)
	if err != nil {
		return value, false, err
	}
	return url, true, nil
}

func isJSONContainer(schema contracts.InputSchema) bool {
	return schema.Type == "object" || schema.Type == "array"
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func isURLFormat(schema contracts.InputSchema) bool {
	format := strings.ToLower(strings.TrimSpace(schema.Format))
	return format == "uri" || format == "url"
}

func nestedField(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func nestedFileAccessError(fieldPath, path string, err error) error {
	if os.IsNotExist(err) {
		return &clierrors.CLIError{Message: fmt.Sprintf("file_not_found: %s: %s", fieldPath, path)}
	}
	if os.IsPermission(err) {
		return &clierrors.CLIError{Message: fmt.Sprintf("file_access_denied: %s: %s", fieldPath, path)}
	}
	return &clierrors.CLIError{Message: fmt.Sprintf("file_access_denied: %s: %s: %v", fieldPath, path, err)}
}
