package cmd

import (
	"strings"
	"testing"
)

func TestAgentDescribePrintsGuide(t *testing.T) {
	stdout, stderr, err := executeRoot(t, "agent", "describe")
	if err != nil {
		t.Fatalf("agent describe returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("agent describe should not write stderr, got %q", stderr)
	}

	for _, text := range []string{
		"# SeaCloud CLI Agent Guide",
		"### account",
		"seacloud account balance --output json",
		"seacloud models list",
		"seacloud --dry-run run <model_id> --param key=value",
		"seacloud run-async <model_id> --param key=value",
		"seacloud task status <task_id> --output json",
		"https://cloud.seaart.ai/settings/credits",
	} {
		if !strings.Contains(stdout, text) {
			t.Fatalf("expected stdout to contain %q\n%s", text, stdout)
		}
	}
}
