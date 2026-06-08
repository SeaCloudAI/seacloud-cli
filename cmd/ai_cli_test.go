package cmd

import (
	"strings"
	"testing"
)

func TestValidateOutputFormatRejectsUnknownValueWithHint(t *testing.T) {
	err := validateOutputFormat("--output", "xml", "json", "table")
	if err == nil {
		t.Fatal("expected error")
	}
	text := err.Error()
	if !strings.Contains(text, `invalid parameter --output`) || !strings.Contains(text, "Hint:") {
		t.Fatalf("expected actionable error, got %q", text)
	}
}

func TestValidateOutputFormatAllowsEmptyAndKnownValues(t *testing.T) {
	for _, value := range []string{"", "json", "table"} {
		if err := validateOutputFormat("--output", value, "json", "table"); err != nil {
			t.Fatalf("value %q should be valid: %v", value, err)
		}
	}
}
