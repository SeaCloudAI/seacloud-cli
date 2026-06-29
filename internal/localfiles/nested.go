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

func prepareNestedJSON(ctx context.Context, key, value string, schema contracts.InputSchema, upload UploadFunc, count *int, prepared *Prepared) (string, bool, error) {
	fieldSchema, ok := schema.Properties[key]
	if !ok || !isJSONContainer(fieldSchema) || !looksLikeJSON(value) {
		return value, false, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return value, true, nil
	}

	next, modified, err := prepareNestedValue(ctx, key, key, parsed, fieldSchema, upload, count, prepared)
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

func prepareNestedValue(ctx context.Context, rootKey, path string, value any, schema contracts.InputSchema, upload UploadFunc, count *int, prepared *Prepared) (any, bool, error) {
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
			next, childModified, err := prepareNestedValue(ctx, rootKey, nestedField(path, name), childValue, childSchema, upload, count, prepared)
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
			next, itemModified, err := prepareNestedValue(ctx, rootKey, fmt.Sprintf("%s[%d]", path, i), item, *schema.Items, upload, count, prepared)
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
		return prepareNestedString(ctx, rootKey, path, text, schema, upload, count, prepared)
	}
}

func prepareNestedString(ctx context.Context, rootKey, fieldPath, value string, schema contracts.InputSchema, upload UploadFunc, count *int, prepared *Prepared) (string, bool, error) {
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
	*count = *count + 1
	if *count > MaxFileParams {
		return value, false, &clierrors.CLIError{Message: fmt.Sprintf("too_many_files: at most %d local file parameters are supported", MaxFileParams)}
	}
	if shouldUploadDirect(path, schema.Format, info.Size(), true) {
		url, err := uploadFile(ctx, upload, path)
		if err != nil {
			return value, false, err
		}
		return url, true, nil
	}
	encoded, err := encodeFileBase64(path)
	if err != nil {
		return value, false, nestedFileAccessError(fieldPath, path, err)
	}
	prepared.fallback = append(prepared.fallback, fileParam{key: rootKey, path: path, encoded: encoded, nested: true})
	return encoded, true, nil
}

func isJSONContainer(schema contracts.InputSchema) bool {
	return schema.Type == "object" || schema.Type == "array"
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
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
