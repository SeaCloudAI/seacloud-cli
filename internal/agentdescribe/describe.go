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
			{Command: "seacloud account balance --output json", Purpose: "Check available credits before running paid model tasks."},
			{Command: "seacloud auth login", Purpose: "Start browser-based login for sandbox/template commands or a full login session."},
			{Command: "seacloud auth set-key <api-key>", Purpose: "Configure an API key for model execution when the user provides one."},
		},
		Capabilities: []Capability{
			{
				ID:      "auth",
				Summary: "Authenticate and inspect local credential state.",
				Commands: []CommandExample{
					{Command: "seacloud auth status", Purpose: "Check login state."},
					{Command: "seacloud auth login", Purpose: "Start browser-based login."},
					{Command: "seacloud auth set-key <api-key>", Purpose: "Store an API key for model execution."},
				},
			},
			{
				ID:      "account",
				Summary: "Check account balance and billing readiness before running paid model tasks.",
				Commands: []CommandExample{
					{Command: "seacloud account balance", Purpose: "Show the current account balance."},
					{Command: "seacloud account balance --output json", Purpose: "Print structured balance data for agent diagnostics."},
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
					{Command: "seacloud run <model_id> --param image=./input.png --output json", Purpose: "Pass a local file path for model fields that accept image, video, or audio files."},
					{Command: "seacloud run <model_id> --param key=value --output url", Purpose: "Submit and print result URLs."},
				},
			},
			{
				ID:      "llm",
				Summary: "Call LLM contract models through an LLM-only command path.",
				Commands: []CommandExample{
					{Command: "seacloud llm run <model_id> --param key=value", Purpose: "Run an LLM model and print text."},
					{Command: "seacloud llm run <model_id> --stream --param key=value", Purpose: "Stream LLM text as it is generated."},
					{Command: "seacloud llm run <model_id> --param key=value --output json", Purpose: "Print the raw or aggregated LLM response as JSON."},
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
			sandboxCapability(),
			templateCapability(),
		},
		Workflows: []Workflow{
			{
				ID:      "run-model",
				Title:   "Run a multimodal model safely",
				Summary: "Inspect credentials, model availability, and parameters before submitting a real task.",
				Steps: []CommandExample{
					{Command: "seacloud auth status", Purpose: "Check credentials."},
					{Command: "seacloud account balance --output json", Purpose: "Check available credits before submitting paid work."},
					{Command: "seacloud models list", Purpose: "Find a candidate model."},
					{Command: "seacloud models spec <model_id>", Purpose: "Inspect required parameters."},
					{Command: "seacloud --dry-run run <model_id> --param key=value", Purpose: "Validate request shape."},
					{Command: "seacloud run <model_id> --param key=value --output json", Purpose: "Submit the task."},
					{Command: "seacloud llm run <model_id> --param key=value", Purpose: "Use this LLM-only path when the chosen model is an LLM contract."},
					{Command: "seacloud run-async <model_id> --param key=value", Purpose: "Submit without waiting when the user wants an async task ID."},
					{Command: "seacloud task status <task_id> --output json", Purpose: "Check task state without resubmitting."},
				},
			},
			sandboxAutomationWorkflow(),
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
			{
				Title: "Local file parameters",
				Details: []string{
					"Local image files under or equal to 10MiB are encoded as base64 first, then uploaded as a URL only if validation or submission rejects the base64 value.",
					"Local image files over 10MiB and up to 100MB are uploaded directly and the parameter is replaced with the returned URL.",
					"Local video files (.mp4, .mov, .avi, .mkv) and audio files (.mp3, .wav, .aac, .flac) are uploaded directly and the parameter is replaced with the returned URL.",
					"Remote HTTP(S) URLs stay unchanged; explicit local paths that do not exist fail before submission.",
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
			sandboxTemplateSafetyRule(),
		},
		EndpointRules: []Rule{
			{
				Title: "Proxy and endpoint rules",
				Details: []string{
					"Use `seacloud run <model_id>` to submit and wait for final results.",
					"Use `seacloud llm run <model_id>` when the selected model must be an LLM contract.",
					"Use `seacloud run-async <model_id>` to submit only and return a task ID.",
					"Model IDs with underscores such as `gpt_image_2` use queue contracts unless explicitly aliased.",
					"Queue models use `SEACLOUD_GENERATION_URL` and task polling.",
					"When model-contracts returns 404, do not execute fallback curl directly from the shell.",
					"Use seacloud --dry-run run <model_id> --use-skill-model-fallback before any real multimodal fallback call.",
					"Use seacloud --dry-run llm run <model_id> --use-skill-model-fallback for LLM fallback checks.",
					"Use --use-reference-curl only after the CLI-managed fallback fails; the CLI must load the stored API key or managed runtime token and redact credentials.",
					"If no usable skill model fallback is found, search the official provider documentation for required parameters, enum values, media dimensions, formats, and request body shape before any paid call.",
				},
			},
			sandboxTemplateEndpointRule(),
		},
		Recovery: []RecoveryCase{
			{Problem: "CLI is missing", Actions: []string{"Run seacloud --version.", "If missing, install with npm install -g @seacloudai/seacloud-cli."}},
			{Problem: "Model execution credentials are missing", Actions: []string{"Run seacloud auth status.", "Use seacloud auth set-key <api-key> when the user provides a model API key, or use seacloud auth login for a full login session."}},
			{Problem: "Sandbox or template login is missing", Actions: []string{"Run seacloud auth status.", "Use seacloud auth login; seacloud auth set-key <api-key> is not enough for sandbox/template commands."}},
			{Problem: "Parameter validation fails", Actions: []string{"Run seacloud models spec <model_id>.", "Fix --param values, then retry with --dry-run before submitting."}},
			{Problem: "Balance is insufficient", Actions: []string{"Run seacloud account balance.", "Top up at: https://cloud.seaart.ai/settings/credits.", "Do not retry login or resubmit generation tasks until credits are available."}},
			{Problem: "Task times out", Actions: []string{"Use seacloud task status <task_id> --output json.", "Do not submit the same generation request again unless the user asks."}},
			{Problem: "SkillHub search fails", Actions: []string{"Show the concrete CLI error.", "Try broader English keywords or seacloud skills list --sort stars."}},
		},
	}
}
