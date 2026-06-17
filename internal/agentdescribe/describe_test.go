package agentdescribe

import (
	"strings"
	"testing"
)

func TestBuildIncludesCoreCapabilities(t *testing.T) {
	desc := Build("test-version")

	if desc.SchemaVersion == "" {
		t.Fatalf("schema version is required")
	}
	if desc.CLIVersion != "test-version" {
		t.Fatalf("expected CLI version to be preserved, got %q", desc.CLIVersion)
	}

	for _, id := range []string{"auth", "models", "run", "run-async", "task", "skills", "sandbox", "template"} {
		if !hasCapability(desc, id) {
			t.Fatalf("expected capability %q in %#v", id, desc.Capabilities)
		}
	}
	if hasCapability(desc, "images") {
		t.Fatalf("images capability should not be advertised: %#v", desc.Capabilities)
	}
}

func TestBuildIncludesSafeRunWorkflow(t *testing.T) {
	desc := Build("test-version")

	var workflow *Workflow
	for i := range desc.Workflows {
		if desc.Workflows[i].ID == "run-model" {
			workflow = &desc.Workflows[i]
			break
		}
	}
	if workflow == nil {
		t.Fatalf("run-model workflow is required")
	}

	for _, command := range []string{
		"seacloud auth status",
		"seacloud models list",
		"seacloud models spec <model_id>",
		"seacloud --dry-run run <model_id> --param key=value",
		"seacloud run <model_id> --param key=value --output json",
		"seacloud run-async <model_id> --param key=value",
		"seacloud task status <task_id> --output json",
	} {
		if !workflowHasCommand(*workflow, command) {
			t.Fatalf("expected workflow command %q in %#v", command, workflow.Steps)
		}
	}
}

func TestBuildIncludesProxyEndpointRules(t *testing.T) {
	desc := Build("test-version")

	var found bool
	for _, rule := range desc.EndpointRules {
		if rule.Title != "Proxy and endpoint rules" {
			continue
		}
		found = true
		for _, detail := range []string{
			"Use `seacloud run <model_id>` to submit and wait for final results.",
			"Use `seacloud run-async <model_id>` to submit only and return a task ID.",
			"Model IDs with underscores such as `gpt_image_2` use queue contracts unless explicitly aliased.",
			"Queue models use `SEACLOUD_GENERATION_URL` and task polling.",
		} {
			if !ruleHasDetail(rule, detail) {
				t.Fatalf("expected endpoint rule detail %q in %#v", detail, rule.Details)
			}
		}
	}
	if !found {
		t.Fatalf("expected Proxy and endpoint rules in %#v", desc.EndpointRules)
	}
}

func TestBuildDoesNotAdvertiseRemovedImageCommands(t *testing.T) {
	desc := Build("test-version")
	markdown := RenderMarkdown(desc)
	for _, removed := range []string{
		"seacloud images generate",
		"gpt-image-* sync image model",
		"`seacloud run gpt-image-*` uses the same sync image proxy path.",
	} {
		if strings.Contains(markdown, removed) {
			t.Fatalf("agent guide should not contain removed command text %q\n%s", removed, markdown)
		}
	}
}

func hasCapability(desc Description, id string) bool {
	for _, capability := range desc.Capabilities {
		if capability.ID == id {
			return true
		}
	}
	return false
}

func workflowHasCommand(workflow Workflow, command string) bool {
	for _, step := range workflow.Steps {
		if step.Command == command {
			return true
		}
	}
	return false
}

func ruleHasDetail(rule Rule, detail string) bool {
	for _, got := range rule.Details {
		if got == detail {
			return true
		}
	}
	return false
}
