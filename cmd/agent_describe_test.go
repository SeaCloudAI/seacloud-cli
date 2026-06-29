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
		"seacloud run <model_id> --param image=./input.png --output json",
		"Local file parameters",
		"audio files (.mp3, .wav, .aac, .flac) are uploaded directly",
		"seacloud run-async <model_id> --param key=value",
		"seacloud task status <task_id> --output json",
		"seacloud sandbox create base --no-connect --wait --output json --metadata app=agent",
		"seacloud sandbox webhook replay <delivery_id>",
		"seacloud sandbox team metrics-max <team_id> --metric concurrent_sandboxes",
		"seacloud sandbox observability",
		"seacloud template tags assign my-template:v1 production stable",
		"Sandbox and template safety rules",
		"https://cloud.seaart.ai/settings/credits",
	} {
		if !strings.Contains(stdout, text) {
			t.Fatalf("expected stdout to contain %q\n%s", text, stdout)
		}
	}
}
