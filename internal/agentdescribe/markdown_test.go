package agentdescribe

import (
	"strings"
	"testing"
)

func TestRenderMarkdownIncludesStableSectionsAndCommands(t *testing.T) {
	output := RenderMarkdown(Build("test-version"))

	expectedInOrder := []string{
		"# SeaCloud CLI Agent Guide",
		"CLI version: `test-version`",
		"## First Steps",
		"## Capabilities",
		"## Recommended Workflows",
		"## Parameter Rules",
		"## Output Rules",
		"## Proxy and Endpoint Rules",
		"## Recovery",
	}
	lastIndex := -1
	for _, text := range expectedInOrder {
		index := strings.Index(output, text)
		if index < 0 {
			t.Fatalf("expected markdown to contain %q\n%s", text, output)
		}
		if index < lastIndex {
			t.Fatalf("expected %q to appear after previous section\n%s", text, output)
		}
		lastIndex = index
	}

	for _, command := range []string{
		"seacloud --version",
		"seacloud models list",
		"seacloud --dry-run run <model_id> --param key=value",
		"seacloud run <model_id> --param image=./input.png --output json",
		"seacloud run-async <model_id> --param key=value",
		"seacloud task status <task_id> --output json",
	} {
		if !strings.Contains(output, command) {
			t.Fatalf("expected markdown to contain command %q\n%s", command, output)
		}
	}
}

func TestRenderMarkdownIncludesProxyEndpointRules(t *testing.T) {
	output := RenderMarkdown(Build("test-version"))

	for _, text := range []string{
		"### Proxy and endpoint rules",
		"Use `seacloud run <model_id>` to submit and wait for final results.",
		"Use `seacloud run-async <model_id>` to submit only and return a task ID.",
		"Model IDs with underscores such as `gpt_image_2` use queue contracts unless explicitly aliased.",
		"Queue models use `SEACLOUD_GENERATION_URL` and task polling.",
	} {
		if !strings.Contains(output, text) {
			t.Fatalf("expected markdown to contain %q\n%s", text, output)
		}
	}
}

func TestRenderMarkdownIncludesLocalFileRules(t *testing.T) {
	output := RenderMarkdown(Build("test-version"))

	for _, text := range []string{
		"### Local file parameters",
		"Local image files under or equal to 10MiB are encoded as base64 first",
		"Local video files (.mp4, .mov, .avi, .mkv) and audio files (.mp3, .wav, .aac, .flac) are uploaded directly",
	} {
		if !strings.Contains(output, text) {
			t.Fatalf("expected markdown to contain %q\n%s", text, output)
		}
	}
}

func TestRenderMarkdownIsDeterministic(t *testing.T) {
	desc := Build("test-version")
	first := RenderMarkdown(desc)
	second := RenderMarkdown(desc)
	if first != second {
		t.Fatalf("expected deterministic markdown output")
	}
}
