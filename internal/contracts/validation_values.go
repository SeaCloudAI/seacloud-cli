package contracts

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

func validateObjectValue(modelID, key string, input map[string]any, schema InputSchema) (map[string]any, error) {
	out := make(map[string]any)
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for name := range input {
			if _, ok := schema.Properties[name]; !ok {
				return nil, clierrors.ErrInvalidParam(modelID, nestedKey(key, name), "unknown parameter")
			}
		}
	}

	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}

	for name, prop := range schema.Properties {
		value, hasValue := input[name]
		if !hasValue {
			if prop.Default != nil {
				out[name] = prop.Default
				continue
			}
			if required[name] {
				return nil, clierrors.ErrMissingParam(modelID, nestedKey(key, name))
			}
			continue
		}
		coerced, err := coerceJSONValue(modelID, nestedKey(key, name), value, prop)
		if err != nil {
			return nil, err
		}
		out[name] = coerced
	}
	return out, nil
}

func coerceJSONValue(modelID, key string, value any, schema InputSchema) (any, error) {
	var coerced any
	switch schema.Type {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return nil, clierrors.ErrInvalidParam(modelID, key, "expected a JSON object")
		}
		return validateObjectValue(modelID, key, obj, schema)
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return nil, clierrors.ErrInvalidParam(modelID, key, "expected a JSON array")
		}
		var err error
		coerced, err = validateArrayValue(modelID, key, arr, schema)
		if err != nil {
			return nil, err
		}
	case "integer", "int":
		n, err := jsonInteger(value)
		if err != nil {
			return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%q is not a valid integer", value))
		}
		if err := checkNumberBounds(modelID, key, float64(n), schema); err != nil {
			return nil, err
		}
		if err := checkMultipleOf(modelID, key, float64(n), schema); err != nil {
			return nil, err
		}
		coerced = n
	case "number", "float":
		n, err := jsonNumber(value)
		if err != nil {
			return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%q is not a valid number", value))
		}
		if err := checkNumberBounds(modelID, key, n, schema); err != nil {
			return nil, err
		}
		if err := checkMultipleOf(modelID, key, n, schema); err != nil {
			return nil, err
		}
		coerced = n
	case "boolean", "bool":
		b, err := jsonBool(value)
		if err != nil {
			return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%q is not a valid boolean", value))
		}
		coerced = b
	default:
		text, ok := value.(string)
		if !ok {
			return nil, clierrors.ErrInvalidParam(modelID, key, "expected string")
		}
		if err := checkStringBounds(modelID, key, text, schema); err != nil {
			return nil, err
		}
		if err := checkPattern(modelID, key, text, schema); err != nil {
			return nil, err
		}
		if err := checkFormat(modelID, key, text, schema); err != nil {
			return nil, err
		}
		coerced = text
	}
	if err := checkEnum(modelID, key, coerced, schema.Enum); err != nil {
		return nil, err
	}
	return coerced, nil
}

func validateArrayValue(modelID, key string, arr []any, schema InputSchema) ([]any, error) {
	if schema.MinItems != nil && len(arr) < *schema.MinItems {
		return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("expected at least %d items", *schema.MinItems))
	}
	if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
		return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("expected at most %d items", *schema.MaxItems))
	}
	if schema.Items == nil {
		return arr, nil
	}
	out := make([]any, 0, len(arr))
	for i, item := range arr {
		coerced, err := coerceJSONValue(modelID, fmt.Sprintf("%s[%d]", key, i), item, *schema.Items)
		if err != nil {
			return nil, err
		}
		out = append(out, coerced)
	}
	return out, nil
}

func checkMultipleOf(modelID, key string, value float64, schema InputSchema) error {
	if schema.MultipleOf == nil {
		return nil
	}
	divisor := *schema.MultipleOf
	if divisor <= 0 {
		return nil
	}
	remainder := math.Mod(value, divisor)
	if math.Abs(remainder) > 1e-9 && math.Abs(remainder-divisor) > 1e-9 {
		return clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%g is not a multiple of %g", value, divisor))
	}
	return nil
}

func checkPattern(modelID, key, value string, schema InputSchema) error {
	if schema.Pattern == "" {
		return nil
	}
	ok, err := regexp.MatchString(schema.Pattern, value)
	if err != nil {
		return clierrors.ErrInvalidParam(modelID, key, "has an invalid pattern constraint")
	}
	if !ok {
		return clierrors.ErrInvalidParam(modelID, key, "does not match required pattern")
	}
	return nil
}

func checkFormat(modelID, key, value string, schema InputSchema) error {
	switch strings.ToLower(strings.TrimSpace(schema.Format)) {
	case "", "string":
		return nil
	case "uri", "url":
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return clierrors.ErrInvalidParam(modelID, key, "expected a valid URL")
		}
	}
	return nil
}

func jsonInteger(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if math.Trunc(v) != v {
			return 0, fmt.Errorf("not an integer")
		}
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		return int(n), err
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func jsonNumber(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case json.Number:
		return v.Float64()
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("not a number")
	}
}

func jsonBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("not a boolean")
	}
}

func nestedKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
