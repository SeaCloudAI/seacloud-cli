package agentdescribe

func sandboxCapability() Capability {
	return Capability{
		ID:      "sandbox",
		Summary: "Manage SeaCloud sandbox workloads after seacloud auth login. Prefer explicit command maps over broad help-only discovery.",
		Commands: []CommandExample{
			{Command: "seacloud sandbox create [template]", Purpose: "Create a sandbox from a template."},
			{Command: "seacloud sandbox create base --no-connect --wait --output json --metadata app=agent", Purpose: "Create a sandbox safely for automation."},
			{Command: "seacloud sandbox list --state running,paused --metadata app=agent --limit 10 --next-token <token>", Purpose: "List sandboxes with filters and pagination."},
			{Command: "seacloud sandbox info <sandbox_id>", Purpose: "Inspect one sandbox."},
			{Command: "seacloud sandbox exec <sandbox_id> \"ls -la\"", Purpose: "Run a command in a sandbox."},
			{Command: "seacloud sandbox exec <sandbox_id> \"python --version\"", Purpose: "Smoke-test runtime availability."},
			{Command: "seacloud sandbox exec --cwd /workspace --user root --env NODE_ENV=production <sandbox_id> \"node app.js\"", Purpose: "Run with cwd, user, and env options."},
			{Command: "seacloud sandbox connect <sandbox_id> --shell bash", Purpose: "Open an interactive shell only when the user asks."},
			{Command: "seacloud sandbox kill <sandbox_id>", Purpose: "Delete one sandbox by ID."},
			{Command: "seacloud sandbox kill --all --state running,paused --metadata app=agent", Purpose: "Bulk cleanup with narrow filters."},
			{Command: "seacloud sandbox logs <sandbox_id> --limit 100 --direction backward", Purpose: "Read bounded logs."},
			{Command: "seacloud sandbox logs <sandbox_id> --limit 100 --direction backward --output json", Purpose: "Read bounded logs with cursor data."},
			{Command: "seacloud sandbox metrics <sandbox_id> --output json", Purpose: "Read sandbox metrics."},
			{Command: "seacloud sandbox pause <sandbox_id>", Purpose: "Pause a sandbox."},
			{Command: "seacloud sandbox timeout <sandbox_id> --seconds 3600", Purpose: "Set sandbox timeout."},
			{Command: "seacloud sandbox refresh <sandbox_id> --duration 300", Purpose: "Refresh sandbox lifetime."},
			{Command: "seacloud sandbox network update <sandbox_id> --allow-internet-access=false", Purpose: "Update network policy."},
			{Command: "seacloud sandbox volume create cache", Purpose: "Create persistent volume storage."},
			{Command: "seacloud sandbox volume list --output json", Purpose: "List volumes."},
			{Command: "seacloud sandbox volume get <volume_id>", Purpose: "Inspect one volume."},
			{Command: "seacloud sandbox volume delete <volume_id>", Purpose: "Delete one volume."},
			{Command: "seacloud sandbox events --type sandbox.lifecycle.created --limit 20", Purpose: "List lifecycle events."},
			{Command: "seacloud sandbox webhook create --name lifecycle --url https://example.com/webhook --secret <secret> --event sandbox.lifecycle.created", Purpose: "Create a lifecycle webhook."},
			{Command: "seacloud sandbox webhook list --limit 20", Purpose: "List webhooks."},
			{Command: "seacloud sandbox webhook get <webhook_id>", Purpose: "Inspect one webhook."},
			{Command: "seacloud sandbox webhook update <webhook_id> --enabled=false", Purpose: "Update webhook configuration."},
			{Command: "seacloud sandbox webhook delete <webhook_id>", Purpose: "Delete one webhook."},
			{Command: "seacloud sandbox webhook deliveries --status failed --limit 20", Purpose: "List webhook deliveries."},
			{Command: "seacloud sandbox webhook replay <delivery_id>", Purpose: "Replay one webhook delivery."},
			{Command: "seacloud sandbox team list", Purpose: "List teams."},
			{Command: "seacloud sandbox team metrics <team_id> --start 1710000000 --end 1710003600", Purpose: "Read team metrics."},
			{Command: "seacloud sandbox team metrics-max <team_id> --metric concurrent_sandboxes", Purpose: "Read max team metric values."},
			{Command: "seacloud sandbox observability", Purpose: "Show observability summary."},
			{Command: "seacloud sandbox --help", Purpose: "Inspect detailed flags for a specific command."},
		},
	}
}

func templateCapability() Capability {
	return Capability{
		ID:      "template",
		Summary: "Manage SeaCloud sandbox templates after seacloud auth login.",
		Commands: []CommandExample{
			{Command: "seacloud template init --language typescript --name my-template", Purpose: "Initialize a local template project."},
			{Command: "seacloud template build my-template --dockerfile Dockerfile", Purpose: "Build from a Dockerfile."},
			{Command: "seacloud template build my-template --image python:3.13 --cpu-count 2 --memory-mb 2048 --tag v1", Purpose: "Build from a base image with resources and tags."},
			{Command: "seacloud template list --format json", Purpose: "List templates with structured output."},
			{Command: "seacloud template get my-template", Purpose: "Inspect one template."},
			{Command: "seacloud template delete my-template", Purpose: "Delete a template after dry-run review."},
			{Command: "seacloud template exists my-template", Purpose: "Check whether a template reference exists."},
			{Command: "seacloud template builds my-template", Purpose: "List build history."},
			{Command: "seacloud template status <template_id> <build_id>", Purpose: "Check build status."},
			{Command: "seacloud template logs <template_id> <build_id> --limit 100", Purpose: "Read bounded build logs."},
			{Command: "seacloud template migrate --language python --name my-template", Purpose: "Generate SeaCloud SDK files for an existing E2B-style template."},
			{Command: "seacloud template tags assign my-template:v1 production stable", Purpose: "Assign stable template tags."},
			{Command: "seacloud template tags list my-template", Purpose: "List template tags."},
			{Command: "seacloud template tags remove my-template staging", Purpose: "Remove template tags."},
			{Command: "seacloud template --help", Purpose: "Inspect detailed flags for a specific command."},
		},
	}
}

func sandboxAutomationWorkflow() Workflow {
	return Workflow{
		ID:      "sandbox-automation",
		Title:   "Run a sandbox automation task safely",
		Summary: "Create a sandbox without opening an interactive shell, run a command, collect diagnostics, and preview cleanup.",
		Steps: []CommandExample{
			{Command: "seacloud auth status", Purpose: "Check login state before sandbox/template operations."},
			{Command: "seacloud sandbox create base --no-connect --wait --output json --metadata app=agent", Purpose: "Create a machine-readable sandbox and keep the returned ID."},
			{Command: "seacloud sandbox exec <sandbox_id> \"python --version\"", Purpose: "Run a basic command inside the sandbox."},
			{Command: "seacloud sandbox logs <sandbox_id> --limit 100 --direction backward --output json", Purpose: "Collect bounded recent logs."},
			{Command: "seacloud sandbox metrics <sandbox_id> --output json", Purpose: "Collect metrics."},
			{Command: "seacloud --dry-run sandbox kill <sandbox_id>", Purpose: "Preview cleanup before deleting."},
			{Command: "seacloud sandbox kill <sandbox_id>", Purpose: "Delete the sandbox when cleanup is approved."},
		},
	}
}

func sandboxTemplateSafetyRule() Rule {
	return Rule{
		Title: "Sandbox and template safety rules",
		Details: []string{
			"Create sandboxes for automation with --no-connect or --output json so the CLI does not open an interactive shell.",
			"Run global --dry-run before write, delete, bulk cleanup, and webhook replay commands.",
			"Use bulk kill only with --all plus narrow --state and --metadata filters.",
			"Use --limit, --next-token, --cursor, or --offset on list, log, event, and delivery commands.",
		},
	}
}
