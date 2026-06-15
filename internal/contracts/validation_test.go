package contracts

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateAndCoerceAppliesDefaultsAndTypes(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		Required:             []string{"prompt"},
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"prompt": {Type: "string"},
			"size": {
				Type:    "string",
				Enum:    []any{"1024x1024", "auto"},
				Default: "auto",
			},
			"n":           {Type: "integer", Minimum: floatPtr(1), Maximum: floatPtr(4)},
			"transparent": {Type: "boolean"},
			"images":      {Type: "array"},
		},
	}

	got, err := ValidateAndCoerce("gpt_image_1", map[string]string{
		"prompt":      "A red apple",
		"n":           "2",
		"transparent": "true",
		"images":      `["https://example.com/a.png"]`,
	}, schema)
	if err != nil {
		t.Fatalf("ValidateAndCoerce returned error: %v", err)
	}

	want := map[string]any{
		"prompt":      "A red apple",
		"size":        "auto",
		"n":           2,
		"transparent": true,
		"images":      []any{"https://example.com/a.png"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected params:\nwant %#v\ngot  %#v", want, got)
	}
}

func TestValidateAndCoerceRejectsMissingRequired(t *testing.T) {
	schema := InputSchema{
		Type:     "object",
		Required: []string{"prompt"},
		Properties: map[string]InputSchema{
			"prompt": {Type: "string"},
		},
	}

	_, err := ValidateAndCoerce("gpt_image_1", map[string]string{}, schema)
	if err == nil || !strings.Contains(err.Error(), "missing required parameter") {
		t.Fatalf("expected missing required error, got %v", err)
	}
}

func TestValidateAndCoerceRejectsUnknownTopLevelParam(t *testing.T) {
	schema := InputSchema{
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties: map[string]InputSchema{
			"prompt": {Type: "string"},
		},
	}

	_, err := ValidateAndCoerce("gpt_image_1", map[string]string{
		"prompt": "A red apple",
		"extra":  "nope",
	}, schema)
	if err == nil || !strings.Contains(err.Error(), "unknown parameter") {
		t.Fatalf("expected unknown parameter error, got %v", err)
	}
}

func TestValidateAndCoerceExpandsNestedObjectParams(t *testing.T) {
	schema := InputSchema{
		Type: "object",
		Properties: map[string]InputSchema{
			"options": {
				Type:                 "object",
				Required:             []string{"seed"},
				AdditionalProperties: boolPtr(false),
				Properties: map[string]InputSchema{
					"seed": {Type: "integer"},
					"mode": {Type: "string", Default: "fast"},
				},
			},
		},
	}

	got, err := ValidateAndCoerce("gpt_image_1", map[string]string{
		"options.seed": "123",
	}, schema)
	if err != nil {
		t.Fatalf("ValidateAndCoerce returned error: %v", err)
	}

	want := map[string]any{
		"options": map[string]any{
			"seed": 123,
			"mode": "fast",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected nested params:\nwant %#v\ngot  %#v", want, got)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}
