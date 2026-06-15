package contracts

import (
	"strings"
	"testing"
)

func TestValidateAndCoerceRejectsIntegerEnumValueOutsideEnum(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"duration": {Type: "integer", Enum: []any{"6", "10"}},
		},
	}

	_, err := ValidateAndCoerce("minimax_hailuo_23_t2v", map[string]string{
		"duration": "7",
	}, schema)

	if err == nil || !strings.Contains(err.Error(), `"7" is not allowed`) {
		t.Fatalf("expected integer enum rejection, got %v", err)
	}
}

func TestValidateAndCoerceRejectsArrayExceedingMaxItems(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"image_urls": {Type: "array", MaxItems: intPtr(3)},
		},
	}

	_, err := ValidateAndCoerce("nano_banana", map[string]string{
		"image_urls": `["1","2","3","4"]`,
	}, schema)

	if err == nil || !strings.Contains(err.Error(), "at most 3 items") {
		t.Fatalf("expected maxItems rejection, got %v", err)
	}
}

func TestValidateAndCoerceRejectsArrayItemTypeMismatch(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"image_urls": {
				Type:  "array",
				Items: &InputSchema{Type: "string"},
			},
		},
	}

	_, err := ValidateAndCoerce("nano_banana", map[string]string{
		"image_urls": `[1]`,
	}, schema)

	if err == nil || !strings.Contains(err.Error(), "expected string") {
		t.Fatalf("expected array item type rejection, got %v", err)
	}
}

func TestValidateAndCoerceValidatesObjectJSONStringChildren(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"options": {
				Type:                 "object",
				Required:             []string{"seed"},
				AdditionalProperties: boolPtr(false),
				Properties: map[string]InputSchema{
					"seed": {Type: "integer"},
				},
			},
		},
	}

	_, err := ValidateAndCoerce("gpt_image_1", map[string]string{
		"options": `{"seed":"bad"}`,
	}, schema)

	if err == nil || !strings.Contains(err.Error(), "not a valid integer") {
		t.Fatalf("expected object JSON child validation error, got %v", err)
	}
}

func intPtr(v int) *int {
	return &v
}
