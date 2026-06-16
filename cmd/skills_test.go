package cmd

import (
	"strings"
	"testing"
)

func TestSkillsFindAndListExposeJSONOutputFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "find", args: []string{"skills", "find", "--help"}},
		{name: "list", args: []string{"skills", "list", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := executeRoot(t, tt.args...)
			if err != nil {
				t.Fatalf("help command returned error: %v", err)
			}
			if !strings.Contains(stdout, "--output string") ||
				!strings.Contains(stdout, "Output format: json") {
				t.Fatalf("help output missing --output json flag:\n%s", stdout)
			}
		})
	}
}
