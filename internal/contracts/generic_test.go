package contracts

import (
	"reflect"
	"testing"
)

func TestGenericContractUsesQueueEndpointsAndAllowsFreeformParams(t *testing.T) {
	contract := Generic("gpt_image_1")

	if contract.ModelID != "gpt_image_1" || contract.BackendModelID != "gpt_image_1" {
		t.Fatalf("unexpected model ids: model=%q backend=%q", contract.ModelID, contract.BackendModelID)
	}
	if contract.Protocol != "queue" || contract.BodyMode != "raw_json" {
		t.Fatalf("unexpected protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
	if contract.Endpoints.Submit.Path != "/model/v1/queue/gpt_image_1" {
		t.Fatalf("unexpected submit path: %q", contract.Endpoints.Submit.Path)
	}
	if contract.InputSchema.AdditionalProperties == nil || !*contract.InputSchema.AdditionalProperties {
		t.Fatalf("generic contract must allow freeform params")
	}
}

func TestValidateAndCoerceFreeformParamsPreservesStringsAndParsesJSON(t *testing.T) {
	allowAdditional := true
	got, err := ValidateAndCoerce("gpt_image_1", map[string]string{
		"prompt":          "A red apple",
		"size":            "1024x1024",
		"images":          `["https://example.com/a.png"]`,
		"options.quality": "high",
	}, InputSchema{
		Type:                 "object",
		AdditionalProperties: &allowAdditional,
		Properties:           map[string]InputSchema{},
	})
	if err != nil {
		t.Fatalf("ValidateAndCoerce returned error: %v", err)
	}

	want := map[string]any{
		"prompt": "A red apple",
		"size":   "1024x1024",
		"images": []any{"https://example.com/a.png"},
		"options": map[string]any{
			"quality": "high",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected params:\nwant %#v\ngot  %#v", want, got)
	}
}
