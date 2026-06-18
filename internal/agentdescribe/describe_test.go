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

	for _, id := range []string{"auth", "account", "models", "run", "run-async", "task", "skills", "sandbox", "template"} {
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
		"seacloud account balance --output json",
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

func TestBuildIncludesAccountBalanceDiscovery(t *testing.T) {
	desc := Build("test-version")

	if !firstStepsHasPurpose(desc, "seacloud account balance --output json", "Check available credits") {
		t.Fatalf("expected balance check in first steps: %#v", desc.FirstSteps)
	}

	account := findCapability(desc, "account")
	if account == nil {
		t.Fatalf("expected account capability in %#v", desc.Capabilities)
	}
	for _, command := range []string{
		"seacloud account balance",
		"seacloud account balance --output json",
	} {
		if !capabilityHasCommand(*account, command) {
			t.Fatalf("expected account command %q in %#v", command, account.Commands)
		}
	}

	recovery := findRecovery(desc, "Balance is insufficient")
	if recovery == nil {
		t.Fatalf("expected balance recovery case in %#v", desc.Recovery)
	}
	for _, want := range []string{
		"seacloud account balance",
		"https://cloud.seaart.ai/settings/credits",
	} {
		if !recoveryHasActionContaining(*recovery, want) {
			t.Fatalf("expected balance recovery to mention %q in %#v", want, recovery.Actions)
		}
	}
	if recoveryHasActionContaining(*recovery, "provide valid credentials") {
		t.Fatalf("balance recovery should not point agents back to credentials: %#v", recovery.Actions)
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

func TestBuildSeparatesModelAPIKeyFromSandboxLogin(t *testing.T) {
	desc := Build("test-version")

	if !firstStepsHasPurpose(desc, "seacloud auth login", "sandbox/template") {
		t.Fatalf("expected auth login first step to mention sandbox/template: %#v", desc.FirstSteps)
	}
	if !firstStepsHasPurpose(desc, "seacloud auth set-key <api-key>", "model execution") {
		t.Fatalf("expected auth set-key first step to be scoped to model execution: %#v", desc.FirstSteps)
	}

	var sandboxRecovery *RecoveryCase
	for i := range desc.Recovery {
		if desc.Recovery[i].Problem == "Sandbox or template login is missing" {
			sandboxRecovery = &desc.Recovery[i]
			break
		}
	}
	if sandboxRecovery == nil {
		t.Fatalf("expected sandbox/template recovery case in %#v", desc.Recovery)
	}
	for _, action := range sandboxRecovery.Actions {
		if strings.Contains(action, "auth set-key") && !strings.Contains(action, "not enough") {
			t.Fatalf("sandbox/template recovery must not treat auth set-key as sufficient: %#v", sandboxRecovery.Actions)
		}
	}
}

func hasCapability(desc Description, id string) bool {
	return findCapability(desc, id) != nil
}

func findCapability(desc Description, id string) *Capability {
	for _, capability := range desc.Capabilities {
		if capability.ID == id {
			return &capability
		}
	}
	return nil
}

func capabilityHasCommand(capability Capability, command string) bool {
	for _, got := range capability.Commands {
		if got.Command == command {
			return true
		}
	}
	return false
}

func findRecovery(desc Description, problem string) *RecoveryCase {
	for _, recovery := range desc.Recovery {
		if recovery.Problem == problem {
			return &recovery
		}
	}
	return nil
}

func recoveryHasActionContaining(recovery RecoveryCase, text string) bool {
	for _, action := range recovery.Actions {
		if strings.Contains(action, text) {
			return true
		}
	}
	return false
}

func firstStepsHasPurpose(desc Description, command, text string) bool {
	for _, step := range desc.FirstSteps {
		if step.Command == command && strings.Contains(step.Purpose, text) {
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
