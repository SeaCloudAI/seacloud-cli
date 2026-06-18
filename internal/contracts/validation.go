package contracts

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

func ValidateAndCoerce(modelID string, raw map[string]string, schema InputSchema) (map[string]any, error) {
	if allowsOnlyFreeformParams(schema) {
		return coerceFreeformParams(raw), nil
	}
	return validateObject(modelID, "", raw, schema)
}

func allowsOnlyFreeformParams(schema InputSchema) bool {
	return schema.Type == "object" &&
		schema.AdditionalProperties != nil &&
		*schema.AdditionalProperties &&
		len(schema.Properties) == 0
}

func validateObject(modelID, prefix string, raw map[string]string, schema InputSchema) (map[string]any, error) {
	out := make(map[string]any)
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for key := range raw {
			name := strings.TrimPrefix(key, prefix)
			head := strings.SplitN(name, ".", 2)[0]
			if _, ok := schema.Properties[head]; !ok {
				return nil, clierrors.ErrInvalidParam(modelID, name, "unknown parameter")
			}
		}
	}

	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}

	for name, prop := range schema.Properties {
		key := prefix + name
		value, hasValue := raw[key]
		hasNested := hasNestedValue(raw, key+".")
		if !hasValue && !hasNested {
			if prop.Default != nil {
				out[name] = prop.Default
				continue
			}
			if required[name] {
				return nil, clierrors.ErrMissingParam(modelID, key)
			}
			continue
		}

		coerced, err := coerce(modelID, key, value, raw, prop)
		if err != nil {
			return nil, err
		}
		out[name] = coerced
	}

	return out, nil
}

func coerce(modelID, key, value string, raw map[string]string, schema InputSchema) (any, error) {
	var coerced any
	switch schema.Type {
	case "object":
		if value != "" {
			var obj map[string]any
			if err := json.Unmarshal([]byte(value), &obj); err != nil {
				return nil, clierrors.ErrInvalidParam(modelID, key, "expected a JSON object")
			}
			return validateObjectValue(modelID, key, obj, schema)
		}
		return validateObject(modelID, key+".", raw, schema)
	case "array":
		var arr []any
		if err := json.Unmarshal([]byte(value), &arr); err != nil {
			return nil, clierrors.ErrInvalidParam(modelID, key, "expected a JSON array")
		}
		var err error
		coerced, err = validateArrayValue(modelID, key, arr, schema)
		if err != nil {
			return nil, err
		}
	case "integer", "int":
		n, err := strconv.Atoi(value)
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
		n, err := strconv.ParseFloat(value, 64)
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
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%q is not a valid boolean", value))
		}
		coerced = b
	default:
		if err := checkStringBounds(modelID, key, value, schema); err != nil {
			return nil, err
		}
		if err := checkPattern(modelID, key, value, schema); err != nil {
			return nil, err
		}
		if err := checkFormat(modelID, key, value, schema); err != nil {
			return nil, err
		}
		coerced = value
	}
	if err := checkEnum(modelID, key, coerced, schema.Enum); err != nil {
		return nil, err
	}
	return coerced, nil
}

func checkEnum(modelID, key string, value any, enum []any) error {
	if len(enum) == 0 {
		return nil
	}
	for _, allowed := range enum {
		if reflect.DeepEqual(value, allowed) || fmt.Sprint(value) == fmt.Sprint(allowed) {
			return nil
		}
	}
	return clierrors.ErrInvalidParam(modelID, key,
		fmt.Sprintf("%q is not allowed. Allowed values: %s", fmt.Sprint(value), enumValues(enum)))
}

func checkNumberBounds(modelID, key string, value float64, schema InputSchema) error {
	if schema.Minimum != nil && value < *schema.Minimum {
		return clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%g is below minimum %g", value, *schema.Minimum))
	}
	if schema.Maximum != nil && value > *schema.Maximum {
		return clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("%g exceeds maximum %g", value, *schema.Maximum))
	}
	return nil
}

func checkStringBounds(modelID, key, value string, schema InputSchema) error {
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		return clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("value too short (min %d chars)", *schema.MinLength))
	}
	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		return clierrors.ErrInvalidParam(modelID, key, fmt.Sprintf("value too long (max %d chars)", *schema.MaxLength))
	}
	return nil
}

func hasNestedValue(raw map[string]string, prefix string) bool {
	for key := range raw {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func enumValues(enum []any) string {
	values := make([]string, 0, len(enum))
	for _, value := range enum {
		values = append(values, fmt.Sprint(value))
	}
	return strings.Join(values, ", ")
}
