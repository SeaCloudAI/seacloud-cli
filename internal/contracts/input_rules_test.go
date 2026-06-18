package contracts

import (
	"strings"
	"testing"
)

func TestValidateInputRulesRejectsMissingOneOfFields(t *testing.T) {
	err := ValidateInputRules("wan26_r2v", map[string]any{}, []InputRule{{
		Kind:   "one_of",
		Fields: []string{"image", "video"},
	}})

	if err == nil || !strings.Contains(err.Error(), "requires one of: image, video") {
		t.Fatalf("expected one_of validation error, got %v", err)
	}
}

func TestValidateInputRulesRejectsMissingAtLeastOneFields(t *testing.T) {
	err := ValidateInputRules("wan26_r2v", map[string]any{}, []InputRule{{
		Kind:   "at_least_one",
		Fields: []string{"reference_urls", "reference_video_urls"},
	}})

	if err == nil || !strings.Contains(err.Error(), "requires one of: reference_urls, reference_video_urls") {
		t.Fatalf("expected at_least_one validation error, got %v", err)
	}
}

func TestValidateInputRulesRejectsMissingRequiredWhenConditionMatches(t *testing.T) {
	err := ValidateInputRules("tencent_hunyuan_3d", map[string]any{
		"generate_type": "Sketch",
	}, []InputRule{{
		Kind:     "requires",
		When:     map[string]any{"generate_type": "Sketch"},
		Required: []string{"image"},
	}})

	if err == nil || !strings.Contains(err.Error(), "requires parameter image") {
		t.Fatalf("expected requires validation error, got %v", err)
	}
}

func TestValidateInputRulesAllowsMissingRequiredWhenConditionDoesNotMatch(t *testing.T) {
	err := ValidateInputRules("tencent_hunyuan_3d", map[string]any{
		"generate_type": "Text",
	}, []InputRule{{
		Kind:     "requires",
		When:     map[string]any{"generate_type": "Sketch"},
		Required: []string{"image"},
	}})

	if err != nil {
		t.Fatalf("expected non-matching condition to pass, got %v", err)
	}
}

func TestValidateInputRulesRejectsMutuallyExclusiveFields(t *testing.T) {
	err := ValidateInputRules("gpt_image_1_edit", map[string]any{
		"image":      "https://example.com/image.png",
		"image_urls": []any{"https://example.com/image.png"},
	}, []InputRule{{
		Kind:   "mutually_exclusive",
		Fields: []string{"image", "image_urls"},
	}})

	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("expected mutually_exclusive validation error, got %v", err)
	}
}
