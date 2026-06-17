package agentdescribe

const SchemaVersion = "seacloud-agent-description.v1"

type Description struct {
	SchemaVersion  string
	CLIVersion     string
	Summary        string
	FirstSteps     []CommandExample
	Capabilities   []Capability
	Workflows      []Workflow
	ParameterRules []Rule
	OutputRules    []Rule
	EndpointRules  []Rule
	Recovery       []RecoveryCase
}

type CommandExample struct {
	Command string
	Purpose string
}

type Capability struct {
	ID       string
	Summary  string
	Commands []CommandExample
}

type Workflow struct {
	ID      string
	Title   string
	Summary string
	Steps   []CommandExample
}

type Rule struct {
	Title   string
	Details []string
}

type RecoveryCase struct {
	Problem string
	Actions []string
}

func Build(cliVersion string) Description {
	return Description{
		SchemaVersion: SchemaVersion,
		CLIVersion:    cliVersion,
		Summary:       "SeaCloud CLI is the agent entry point for multimodal generation, task tracking, SkillHub, sandbox, and template workflows.",
		FirstSteps: []CommandExample{
			{Command: "seacloud --version", Purpose: "Confirm the CLI is installed and visible on PATH."},
			{Command: "seacloud auth status", Purpose: "Check whether credentials are available."},
			{Command: "seacloud auth login", Purpose: "Start browser-based login when credentials are missing."},
			{Command: "seacloud auth set-key <api-key>", Purpose: "Configure an API key when the user provides one."},
		},
		Capabilities: []Capability{
			{
				ID:      "auth",
				Summary: "Authenticate and inspect local credential state.",
				Commands: []CommandExample{
					{Command: "seacloud auth status", Purpose: "Check login state."},
					{Command: "seacloud auth login", Purpose: "Start browser-based login."},
					{Command: "seacloud auth set-key <api-key>", Purpose: "Store an API key."},
				},
			},
			{
				ID:      "models",
				Summary: "List callable models and inspect model parameter contracts.",
				Commands: []CommandExample{
					{Command: "seacloud models list", Purpose: "Browse available models."},
					{Command: "seacloud models list --type video", Purpose: "Filter models by type."},
					{Command: "seacloud models list --keywords kling", Purpose: "Search models by keyword."},
					{Command: "seacloud models spec <model_id>", Purpose: "Inspect required parameters before running."},
					{Command: "seacloud models spec <model_id> --output json", Purpose: "Read the raw model contract."},
				},
			},
			{
				ID:      "run",
				Summary: "Submit a multimodal generation request and wait for a result.",
				Commands: []CommandExample{
					{Command: "seacloud --dry-run run <model_id> --param key=value", Purpose: "Preview request shape without submitting."},
					{Command: "seacloud run <model_id> --param key=value --output json", Purpose: "Submit and print the full task response."},
					{Command: "seacloud run <model_id> --param key=value --output url", Purpose: "Submit and print result URLs."},
				},
			},
			{
				ID:      "run-async",
				Summary: "Submit a multimodal generation request and return a task ID without waiting.",
				Commands: []CommandExample{
					{Command: "seacloud run-async <model_id> --param key=value", Purpose: "Submit and print structured task metadata."},
					{Command: "seacloud run-async <model_id> --param key=value --output id", Purpose: "Submit and print only the task ID."},
				},
			},
			{
				ID:      "task",
				Summary: "Inspect task state without submitting duplicate generation requests.",
				Commands: []CommandExample{
					{Command: "seacloud task status <task_id>", Purpose: "Fetch human-readable task status."},
					{Command: "seacloud task status <task_id> --output json", Purpose: "Fetch structured task status."},
					{Command: "seacloud task status <task_id> --output url", Purpose: "Print result URLs only."},
				},
			},
			{
				ID:      "skills",
				Summary: "Search, install, and configure skills from SeaCloud SkillHub.",
				Commands: []CommandExample{
					{Command: "seacloud skills find video", Purpose: "Search SkillHub by need."},
					{Command: "seacloud skills list --sort stars", Purpose: "Browse popular skills."},
					{Command: "seacloud skills add <slug> -g -y", Purpose: "Install a skill globally."},
					{Command: "seacloud skills config --show", Purpose: "Show SkillHub configuration."},
				},
			},
			{
				ID:      "sandbox",
				Summary: "Manage SeaCloud sandbox workloads. Use command help for detailed subcommands.",
				Commands: []CommandExample{
					{Command: "seacloud sandbox --help", Purpose: "Inspect sandbox commands and flags."},
				},
			},
			{
				ID:      "template",
				Summary: "Manage SeaCloud templates. Use command help for detailed subcommands.",
				Commands: []CommandExample{
					{Command: "seacloud template --help", Purpose: "Inspect template commands and flags."},
				},
			},
		},
		Workflows: []Workflow{
			{
				ID:      "run-model",
				Title:   "Run a multimodal model safely",
				Summary: "Inspect credentials, model availability, and parameters before submitting a real task.",
				Steps: []CommandExample{
					{Command: "seacloud auth status", Purpose: "Check credentials."},
					{Command: "seacloud models list", Purpose: "Find a candidate model."},
					{Command: "seacloud models spec <model_id>", Purpose: "Inspect required parameters."},
					{Command: "seacloud --dry-run run <model_id> --param key=value", Purpose: "Validate request shape."},
					{Command: "seacloud run <model_id> --param key=value --output json", Purpose: "Submit the task."},
					{Command: "seacloud run-async <model_id> --param key=value", Purpose: "Submit without waiting when the user wants an async task ID."},
					{Command: "seacloud task status <task_id> --output json", Purpose: "Check task state without resubmitting."},
				},
			},
		},
		ParameterRules: []Rule{
			{
				Title: "Use repeatable --param key=value flags",
				Details: []string{
					"Pass simple values as --param prompt=\"a cat running\".",
					"Use dot notation for nested objects, such as --param camera_control.type=simple.",
					"Pass arrays as JSON strings, such as --param content='[{\"type\":\"text\",\"text\":\"hello\"}]'.",
					"Repeat --param for multiple fields.",
				},
			},
		},
		OutputRules: []Rule{
			{
				Title: "Use structured output when automation needs stable parsing",
				Details: []string{
					"Use --output json on models, run, run-async, and task commands when supported.",
					"Use --output id on run-async when only the task ID is needed.",
					"Use --output url when only result URLs are needed.",
					"Use --format json on sandbox/template commands that expose E2B-compatible output flags.",
				},
			},
		},
		EndpointRules: []Rule{
			{
				Title: "Proxy and endpoint rules",
				Details: []string{
					"Use `seacloud run <model_id>` to submit and wait for final results.",
					"Use `seacloud run-async <model_id>` to submit only and return a task ID.",
					"Model IDs with underscores such as `gpt_image_2` use queue contracts unless explicitly aliased.",
					"Queue models use `SEACLOUD_GENERATION_URL` and task polling.",
				},
			},
		},
		Recovery: []RecoveryCase{
			{Problem: "CLI is missing", Actions: []string{"Run seacloud --version.", "If missing, install with npm install -g @seacloudai/seacloud-cli."}},
			{Problem: "Credentials are missing or expired", Actions: []string{"Run seacloud auth status.", "Use seacloud auth login or seacloud auth set-key <api-key>."}},
			{Problem: "Parameter validation fails", Actions: []string{"Run seacloud models spec <model_id>.", "Fix --param values, then retry with --dry-run before submitting."}},
			{Problem: "Balance is insufficient", Actions: []string{"Ask the user to recharge or provide valid credentials.", "Do not retry many generation tasks automatically."}},
			{Problem: "Task times out", Actions: []string{"Use seacloud task status <task_id> --output json.", "Do not submit the same generation request again unless the user asks."}},
			{Problem: "SkillHub search fails", Actions: []string{"Show the concrete CLI error.", "Try broader English keywords or seacloud skills list --sort stars."}},
		},
	}
}
