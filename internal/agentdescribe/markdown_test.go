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
		"For image generation, prefer model IDs returned by `seacloud models list --type image` unless the user explicitly asks for `images generate` or a `gpt-image-*` sync image model.",
		"`seacloud images generate` requires `SEACLOUD_FOLKOS_PROXY_URL` unless the binary was built with a proxy base URL.",
		"`seacloud run gpt-image-*` uses the same sync image proxy path.",
		"Model IDs with underscores such as `gpt_image_2` use queue contracts unless explicitly aliased.",
		"Queue models use `SEACLOUD_GENERATION_URL` and task polling.",
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
