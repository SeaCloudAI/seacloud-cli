package agentdescribe

import "testing"

func TestBuildIncludesSandboxAutomationWorkflow(t *testing.T) {
	desc := Build("test-version")

	workflow := findWorkflow(desc, "sandbox-automation")
	if workflow == nil {
		t.Fatalf("expected sandbox automation workflow in %#v", desc.Workflows)
	}
	for _, command := range []string{
		"seacloud sandbox create base --no-connect --wait --output json --metadata app=agent",
		"seacloud sandbox exec <sandbox_id> \"python --version\"",
		"seacloud sandbox logs <sandbox_id> --limit 100 --direction backward --output json",
		"seacloud sandbox metrics <sandbox_id> --output json",
		"seacloud --dry-run sandbox kill <sandbox_id>",
	} {
		if !workflowHasCommand(*workflow, command) {
			t.Fatalf("expected sandbox workflow command %q in %#v", command, workflow.Steps)
		}
	}
}

func TestBuildIncludesSandboxCommandMap(t *testing.T) {
	desc := Build("test-version")
	sandbox := findCapability(desc, "sandbox")
	if sandbox == nil {
		t.Fatalf("expected sandbox capability in %#v", desc.Capabilities)
	}

	for _, command := range []string{
		"seacloud sandbox create [template]",
		"seacloud sandbox list --state running,paused --metadata app=agent --limit 10 --next-token <token>",
		"seacloud sandbox info <sandbox_id>",
		"seacloud sandbox exec <sandbox_id> \"ls -la\"",
		"seacloud sandbox connect <sandbox_id> --shell bash",
		"seacloud sandbox kill --all --state running,paused --metadata app=agent",
		"seacloud sandbox logs <sandbox_id> --limit 100 --direction backward",
		"seacloud sandbox metrics <sandbox_id> --output json",
		"seacloud sandbox pause <sandbox_id>",
		"seacloud sandbox timeout <sandbox_id> --seconds 3600",
		"seacloud sandbox refresh <sandbox_id> --duration 300",
		"seacloud sandbox network update <sandbox_id> --allow-internet-access=false",
		"seacloud sandbox volume create cache",
		"seacloud sandbox events --type sandbox.lifecycle.created --limit 20",
		"seacloud sandbox webhook replay <delivery_id>",
		"seacloud sandbox team metrics-max <team_id> --metric concurrent_sandboxes",
		"seacloud sandbox observability",
	} {
		if !capabilityHasCommand(*sandbox, command) {
			t.Fatalf("expected sandbox command %q in %#v", command, sandbox.Commands)
		}
	}
}

func TestBuildIncludesTemplateCommandMap(t *testing.T) {
	desc := Build("test-version")
	template := findCapability(desc, "template")
	if template == nil {
		t.Fatalf("expected template capability in %#v", desc.Capabilities)
	}

	for _, command := range []string{
		"seacloud template init --language typescript --name my-template",
		"seacloud template build my-template --dockerfile Dockerfile",
		"seacloud template list --format json",
		"seacloud template get my-template",
		"seacloud template delete my-template",
		"seacloud template exists my-template",
		"seacloud template builds my-template",
		"seacloud template status <template_id> <build_id>",
		"seacloud template logs <template_id> <build_id> --limit 100",
		"seacloud template migrate --language python --name my-template",
		"seacloud template tags assign my-template:v1 production stable",
		"seacloud template tags list my-template",
		"seacloud template tags remove my-template staging",
	} {
		if !capabilityHasCommand(*template, command) {
			t.Fatalf("expected template command %q in %#v", command, template.Commands)
		}
	}
}

func TestBuildIncludesSandboxTemplateSafetyRules(t *testing.T) {
	desc := Build("test-version")

	rule := findRule(desc.OutputRules, "Sandbox and template safety rules")
	if rule == nil {
		t.Fatalf("expected sandbox/template safety rule in %#v", desc.OutputRules)
	}
	for _, detail := range []string{
		"Create sandboxes for automation with --no-connect or --output json so the CLI does not open an interactive shell.",
		"Run global --dry-run before write, delete, bulk cleanup, and webhook replay commands.",
		"Use bulk kill only with --all plus narrow --state and --metadata filters.",
		"Use --limit, --next-token, --cursor, or --offset on list, log, event, and delivery commands.",
	} {
		if !ruleHasDetail(*rule, detail) {
			t.Fatalf("expected safety detail %q in %#v", detail, rule.Details)
		}
	}
}

func findWorkflow(desc Description, id string) *Workflow {
	for i := range desc.Workflows {
		if desc.Workflows[i].ID == id {
			return &desc.Workflows[i]
		}
	}
	return nil
}

func findRule(rules []Rule, title string) *Rule {
	for i := range rules {
		if rules[i].Title == title {
			return &rules[i]
		}
	}
	return nil
}
