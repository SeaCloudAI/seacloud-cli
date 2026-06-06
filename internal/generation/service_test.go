package generation

import (
	"reflect"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/models"
)

func TestValidateAndCoerceStringArrayParsesJSONArray(t *testing.T) {
	got, err := ValidateAndCoerce("test_model", map[string]string{
		"image": `["https://example.com/a.png","https://example.com/b.png"]`,
	}, []models.ModelParam{
		{Name: "image", Type: "string/array", Required: true},
	})
	if err != nil {
		t.Fatalf("ValidateAndCoerce returned error: %v", err)
	}

	want := map[string]any{
		"image": []any{"https://example.com/a.png", "https://example.com/b.png"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected params\n got: %#v\nwant: %#v", got, want)
	}
}

func TestValidateAndCoerceStringArrayKeepsScalarString(t *testing.T) {
	got, err := ValidateAndCoerce("test_model", map[string]string{
		"image": "https://example.com/a.png",
	}, []models.ModelParam{
		{Name: "image", Type: "string/array", Required: true},
	})
	if err != nil {
		t.Fatalf("ValidateAndCoerce returned error: %v", err)
	}

	want := map[string]any{
		"image": "https://example.com/a.png",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected params\n got: %#v\nwant: %#v", got, want)
	}
}

func TestValidateAndCoerceArrayStillRequiresJSONArray(t *testing.T) {
	_, err := ValidateAndCoerce("test_model", map[string]string{
		"images": "https://example.com/a.png",
	}, []models.ModelParam{
		{Name: "images", Type: "array", Required: true},
	})
	if err == nil {
		t.Fatal("expected error for scalar value passed to array parameter")
	}
}
