package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var schemaOpts struct {
	format string
}

type cliSchema struct {
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	CLI             []string      `json:"cli"`
	Method          string        `json:"method"`
	Path            string        `json:"path"`
	Auth            []string      `json:"auth"`
	RequiredHeaders []string      `json:"requiredHeaders,omitempty"`
	PathParams      []schemaField `json:"pathParams,omitempty"`
	Query           []schemaField `json:"query,omitempty"`
	Body            []schemaField `json:"body,omitempty"`
	Response        []schemaField `json:"response"`
	Pagination      string        `json:"pagination,omitempty"`
	Destructive     bool          `json:"destructive,omitempty"`
	DryRunExample   string        `json:"dryRunExample,omitempty"`
	NextStep        string        `json:"nextStep"`
}

type schemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

var schemaCmd = &cobra.Command{
	Use:   "schema [method]",
	Short: "Inspect CLI/API schemas for Agent-safe automation",
	Long: `Inspect CLI/API schemas without making a network request.

Agents should call this command when they need to discover how to invoke a
sandbox or template operation. Schema output includes CLI examples, API method
and path, auth requirements, path/query/body fields, response shape, pagination
notes, destructive status, and a dry-run example when the operation mutates
state.

Use "seacloud schema list" to list method names. Use --format json when the
schema will be consumed by another tool.`,
	Example: `  seacloud schema list
  seacloud schema sandboxes.create
  seacloud schema get webhooks.create --format json
  seacloud schema templates.build`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOutputFormat("--format", schemaOpts.format, "json", "pretty", "table"); err != nil {
			return err
		}
		if len(args) == 0 {
			return printSchemaList()
		}
		return printSchema(args[0])
	},
}

var schemaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available schema method names",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOutputFormat("--format", schemaOpts.format, "json", "pretty", "table"); err != nil {
			return err
		}
		return printSchemaList()
	},
}

var schemaGetCmd = &cobra.Command{
	Use:   "get <method>",
	Short: "Show one schema by method name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOutputFormat("--format", schemaOpts.format, "json", "pretty", "table"); err != nil {
			return err
		}
		return printSchema(args[0])
	},
}

func printSchemaList() error {
	names := schemaNames()
	if schemaOpts.format == "json" {
		return printJSON(map[string]any{"methods": names})
	}
	for _, name := range names {
		item := schemaRegistry()[name]
		fmt.Printf("%-32s %-7s %s\n", name, item.Method, item.Path)
	}
	return nil
}

func printSchema(name string) error {
	name = strings.TrimSpace(name)
	item, ok := schemaRegistry()[name]
	if !ok {
		return aiParamError("<method>", fmt.Sprintf("unknown schema method %q", name), "Run: seacloud schema list")
	}
	if schemaOpts.format == "json" {
		return printJSON(item)
	}
	fmt.Printf("%s\n", item.Name)
	fmt.Printf("  Description: %s\n", item.Description)
	fmt.Printf("  API:         %s %s\n", item.Method, item.Path)
	fmt.Printf("  Auth:        %s\n", strings.Join(item.Auth, ", "))
	if len(item.RequiredHeaders) > 0 {
		fmt.Printf("  Headers:     %s\n", strings.Join(item.RequiredHeaders, ", "))
	}
	if item.Pagination != "" {
		fmt.Printf("  Pagination:  %s\n", item.Pagination)
	}
	if item.Destructive {
		fmt.Println("  Destructive: true")
	}
	if len(item.CLI) > 0 {
		fmt.Println("  CLI:")
		for _, example := range item.CLI {
			fmt.Printf("    %s\n", example)
		}
	}
	printSchemaFields("Path params", item.PathParams)
	printSchemaFields("Query", item.Query)
	printSchemaFields("Body", item.Body)
	printSchemaFields("Response", item.Response)
	if item.DryRunExample != "" {
		fmt.Printf("  Dry-run:     %s\n", item.DryRunExample)
	}
	fmt.Printf("  Next step:   %s\n", item.NextStep)
	return nil
}

func printSchemaFields(title string, fields []schemaField) {
	if len(fields) == 0 {
		return
	}
	fmt.Printf("  %s:\n", title)
	for _, field := range fields {
		required := ""
		if field.Required {
			required = " required"
		}
		defaultValue := ""
		if field.Default != "" {
			defaultValue = " default=" + field.Default
		}
		fmt.Printf("    %-24s %-16s%s%s %s\n", field.Name, field.Type, required, defaultValue, field.Description)
	}
}

func schemaNames() []string {
	names := make([]string, 0, len(schemaRegistry()))
	for name := range schemaRegistry() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func schemaRegistry() map[string]cliSchema {
	commonAuth := []string{"SEACLOUD_API_KEY", "E2B_API_KEY", "E2B_ACCESS_TOKEN", "stored seacloud auth config"}
	headers := []string{"Authorization: Bearer <api-key>", "X-Namespace-ID optional", "X-User-ID optional", "X-Project-ID optional"}
	return map[string]cliSchema{
		"sandboxes.create": {
			Name:        "sandboxes.create",
			Description: "Create a sandbox from an optional template and optional runtime policy.",
			CLI:         []string{"seacloud sandbox create [template] --wait --output json", "seacloud --dry-run sandbox create base --metadata app=agent"},
			Method:      "POST", Path: "/api/v1/sandboxes", Auth: commonAuth, RequiredHeaders: headers,
			Body: []schemaField{
				{"templateID", "string", false, "base server default", "Template ID/name/tag. Optional."},
				{"timeout", "int64", false, "", "Sandbox timeout in seconds."},
				{"waitReady", "bool", false, "false", "Wait until ready before returning."},
				{"autoPause", "bool", false, "false", "Pause instead of killing on timeout."},
				{"autoResume", "bool", false, "false", "Allow router-triggered resume."},
				{"metadata", "map[string]string", false, "", "User metadata filters and labels."},
				{"envVars", "map[string]string", false, "", "Environment variables."},
				{"volumeMounts", "[]VolumeMount", false, "", "Persistent volume mounts as name/path pairs."},
				{"network", "SandboxNetworkPolicy", false, "", "Public traffic, internet access, allowOut, denyOut."},
			},
			Response:      []schemaField{{"sandboxID", "string", true, "", "Created sandbox ID."}, {"state", "string", false, "", "Current sandbox state."}, {"envdURL", "string", false, "", "Runtime connection URL."}},
			DryRunExample: "seacloud --dry-run sandbox create base --output json",
			NextStep:      "Use sandbox.connect or sandbox.exec after the sandbox is ready.",
		},
		"sandboxes.list": {
			Name:        "sandboxes.list",
			Description: "List sandboxes with optional filters.",
			CLI:         []string{"seacloud sandbox list --state running --limit 20 --output json"},
			Method:      "GET", Path: "/api/v1/sandboxes", Auth: commonAuth, RequiredHeaders: headers,
			Query:      []schemaField{{"state", "[]string", false, "", "State filters."}, {"metadata", "map[string]string", false, "", "Metadata key/value filters."}, {"limit", "int", false, "server default", "Maximum items."}, {"nextToken", "string", false, "", "Page token."}},
			Response:   []schemaField{{"items", "[]Sandbox", true, "", "Sandboxes."}, {"hasNext", "bool", false, "", "Whether another page exists."}, {"nextToken", "string", false, "", "Token for the next page."}},
			Pagination: "Use --limit and --next-token.",
			NextStep:   "Use sandbox.info, sandbox.exec, sandbox.connect, or sandbox.kill with a returned sandboxID.",
		},
		"sandboxes.exec": {
			Name:        "sandboxes.exec",
			Description: "Execute a command inside a running sandbox through the runtime connection.",
			CLI:         []string{"seacloud sandbox exec sb_123 'python main.py'", "echo input | seacloud sandbox exec sb_123 cat"},
			Method:      "POST", Path: "/api/v1/sandboxes/{sandboxID}/connect", Auth: commonAuth, RequiredHeaders: headers,
			PathParams: []schemaField{{"sandboxID", "string", true, "", "Sandbox ID."}},
			Body:       []schemaField{{"command", "string", true, "", "Shell command run via sh -lc."}, {"cwd", "string", false, "", "Working directory."}, {"env", "map[string]string", false, "", "Process environment."}, {"timeoutMS", "int64", false, "0", "Command timeout."}},
			Response:   []schemaField{{"stdout", "stream", false, "", "Process stdout."}, {"stderr", "stream", false, "", "Process stderr."}, {"exitCode", "int", true, "", "Final process exit code."}},
			NextStep:   "Use --background for long-running commands.",
		},
		"sandboxes.delete": destructiveSchema("sandboxes.delete", "DELETE", "/api/v1/sandboxes/{sandboxID}", "Kill/delete a sandbox.", "seacloud --dry-run sandbox kill sb_123"),
		"sandboxes.metrics": {
			Name:        "sandboxes.metrics",
			Description: "Get latest or paginated sandbox metrics.",
			CLI:         []string{"seacloud sandbox metrics sb_123 --output json", "seacloud sandbox metrics --limit 50 --output json"},
			Method:      "GET", Path: "/api/v1/sandboxes/metrics", Auth: commonAuth, RequiredHeaders: headers,
			Query:      []schemaField{{"sandboxIDs", "[]string", false, "", "Optional sandbox IDs."}, {"limit", "int", false, "", "Metric snapshot limit."}},
			Response:   []schemaField{{"items", "[]SandboxMetrics", false, "", "Metric snapshots."}},
			Pagination: "Use --limit for batch metrics.",
			NextStep:   "Use team metrics for aggregate capacity data.",
		},
		"sandboxes.logs": {
			Name:        "sandboxes.logs",
			Description: "Read sandbox logs with cursor pagination.",
			CLI:         []string{"seacloud sandbox logs sb_123 --limit 100 --cursor 0 --output json"},
			Method:      "GET", Path: "/api/v1/sandboxes/{sandboxID}/logs", Auth: commonAuth, RequiredHeaders: headers,
			PathParams: []schemaField{{"sandboxID", "string", true, "", "Sandbox ID."}},
			Query:      []schemaField{{"cursor", "int64", false, "", "Log cursor."}, {"limit", "int", false, "", "Maximum log entries."}, {"direction", "string", false, "", "forward or backward."}, {"level", "string", false, "", "Log level filter."}, {"search", "string", false, "", "Text search."}},
			Response:   []schemaField{{"logs", "[]LogEntry", true, "", "Log entries."}, {"cursor", "int64", false, "", "Next cursor when returned."}},
			Pagination: "Use --limit and --cursor.",
			NextStep:   "Use --search or --level if logs are too large.",
		},
		"sandboxes.network.update": {
			Name:        "sandboxes.network.update",
			Description: "Update sandbox network policy.",
			CLI:         []string{"seacloud --dry-run sandbox network update sb_123 --allow-internet-access=false --allow-out 10.0.0.0/8"},
			Method:      "PUT", Path: "/api/v1/sandboxes/{sandboxID}/network", Auth: commonAuth, RequiredHeaders: headers,
			PathParams:    []schemaField{{"sandboxID", "string", true, "", "Sandbox ID."}},
			Body:          []schemaField{{"allowPublicTraffic", "bool", false, "", "Public inbound access."}, {"allowInternetAccess", "bool", false, "", "Public internet egress."}, {"allowOut", "[]string", false, "", "Egress allowlist CIDRs."}, {"denyOut", "[]string", false, "", "Egress denylist CIDRs."}},
			Response:      []schemaField{{"ok", "bool", false, "", "Success status."}},
			DryRunExample: "seacloud --dry-run sandbox network update sb_123 --allow-internet-access=false",
			NextStep:      "Use sandbox.info or logs to verify behavior after applying.",
		},
		"volumes.create": {
			Name:        "volumes.create",
			Description: "Create a persistent volume.",
			CLI:         []string{"seacloud --dry-run sandbox volume create cache"},
			Method:      "POST", Path: "/api/v1/volumes", Auth: commonAuth, RequiredHeaders: headers,
			Body:          []schemaField{{"name", "string", true, "", "Volume name."}},
			Response:      []schemaField{{"volumeID", "string", true, "", "Volume ID."}, {"name", "string", true, "", "Volume name."}},
			DryRunExample: "seacloud --dry-run sandbox volume create cache",
			NextStep:      "Mount during create with: seacloud sandbox create --volume-mount cache:/cache",
		},
		"volumes.list":   readSchema("volumes.list", "GET", "/api/v1/volumes", "List persistent volumes.", "seacloud sandbox volume list --output json"),
		"volumes.get":    readSchema("volumes.get", "GET", "/api/v1/volumes/{volumeID}", "Get one persistent volume.", "seacloud sandbox volume get vol_123 --output json"),
		"volumes.delete": destructiveSchema("volumes.delete", "DELETE", "/api/v1/volumes/{volumeID}", "Delete a persistent volume.", "seacloud --dry-run sandbox volume delete vol_123"),
		"events.list": {
			Name:        "events.list",
			Description: "List sandbox lifecycle events.",
			CLI:         []string{"seacloud sandbox events --limit 20 --output json", "seacloud sandbox events sb_123 --type sandbox.lifecycle.created --output json"},
			Method:      "GET", Path: "/api/v1/events", Auth: commonAuth, RequiredHeaders: headers,
			Query:      []schemaField{{"limit", "int", false, "10", "Maximum events."}, {"offset", "int", false, "0", "Offset."}, {"type", "[]string", false, "", "Event type filters."}, {"orderAsc", "bool", false, "false", "Ascending order."}},
			Response:   []schemaField{{"events", "[]SandboxLifecycleEvent", true, "", "Lifecycle events."}},
			Pagination: "Use --limit and --offset.",
			NextStep:   "Use webhooks.deliveries to inspect callback delivery for these events.",
		},
		"webhooks.create": {
			Name:        "webhooks.create",
			Description: "Create a signed lifecycle webhook.",
			CLI:         []string{"seacloud --dry-run sandbox webhook create --name lifecycle --url https://example.com/hook --secret $WEBHOOK_SECRET --event sandbox.lifecycle.created"},
			Method:      "POST", Path: "/api/v1/events/webhooks", Auth: commonAuth, RequiredHeaders: headers,
			Body:          []schemaField{{"name", "string", true, "", "Webhook name."}, {"url", "string", true, "", "Callback URL."}, {"signatureSecret", "string", true, "", "Signing secret."}, {"events", "[]string", false, "", "Lifecycle event types."}, {"enabled", "bool", false, "true", "Enable delivery."}, {"retryPolicy", "WebhookRetryPolicy", false, "", "maxAttempts, delaySeconds, deadLetterEnabled."}, {"deadLetterURL", "string", false, "", "Dead-letter callback URL."}},
			Response:      []schemaField{{"webhookID", "string", true, "", "Webhook ID."}, {"enabled", "bool", true, "", "Enabled state."}, {"retryPolicy", "WebhookRetryPolicy", false, "", "Retry configuration."}},
			DryRunExample: "seacloud --dry-run sandbox webhook create --name lifecycle --url https://example.com/hook --secret $WEBHOOK_SECRET",
			NextStep:      "Use webhooks.deliveries to verify callbacks after events are emitted.",
		},
		"webhooks.update":     readWriteSchema("webhooks.update", "PATCH", "/api/v1/events/webhooks/{webhookID}", "Update webhook fields, retry policy, or dead-letter URL.", "seacloud --dry-run sandbox webhook update wh_123 --enabled=false"),
		"webhooks.list":       readSchema("webhooks.list", "GET", "/api/v1/events/webhooks", "List lifecycle webhooks.", "seacloud sandbox webhook list --output json"),
		"webhooks.get":        readSchema("webhooks.get", "GET", "/api/v1/events/webhooks/{webhookID}", "Get one lifecycle webhook.", "seacloud sandbox webhook get wh_123 --output json"),
		"webhooks.delete":     destructiveSchema("webhooks.delete", "DELETE", "/api/v1/events/webhooks/{webhookID}", "Delete a lifecycle webhook.", "seacloud --dry-run sandbox webhook delete wh_123"),
		"webhooks.deliveries": readSchema("webhooks.deliveries", "GET", "/api/v1/events/webhook-deliveries", "List webhook deliveries with filters.", "seacloud sandbox webhook deliveries --webhook-id wh_123 --limit 20 --output json"),
		"webhooks.replay":     readWriteSchema("webhooks.replay", "POST", "/api/v1/events/webhook-deliveries/{deliveryID}/replay", "Replay a webhook delivery.", "seacloud --dry-run sandbox webhook replay del_123"),
		"teams.list":          readSchema("teams.list", "GET", "/api/v1/teams", "List teams.", "seacloud sandbox team list --output json"),
		"teams.metrics":       readSchema("teams.metrics", "GET", "/api/v1/teams/{teamID}/metrics", "Get team metrics bounded by Unix seconds.", "seacloud sandbox team metrics team_123 --start 1710000000 --end 1710003600 --output json"),
		"teams.metrics.max":   readSchema("teams.metrics.max", "GET", "/api/v1/teams/{teamID}/metrics/max", "Get a max team metric.", "seacloud sandbox team metrics-max team_123 --metric concurrent_sandboxes --output json"),
		"templates.build": {
			Name:        "templates.build",
			Description: "Build a sandbox template from Dockerfile, image, or another template.",
			CLI:         []string{"seacloud --dry-run template build demo --image python:3.13 --tag v1", "seacloud template build demo --dockerfile Dockerfile --format json"},
			Method:      "POST", Path: "/api/v1/templates", Auth: commonAuth, RequiredHeaders: headers,
			Body:          []schemaField{{"name", "string", true, "", "Template name."}, {"source", "Dockerfile|image|template", true, "", "Build source."}, {"tags", "[]string", false, "", "Template tags."}, {"envs", "map[string]string", false, "", "Build env vars."}, {"cpuCount", "int32", false, "", "Template CPU."}, {"memoryMB", "int32", false, "", "Template memory."}, {"visibility", "string", false, "", "Visibility."}},
			Response:      []schemaField{{"templateID", "string", true, "", "Template ID."}, {"buildID", "string", true, "", "Build ID."}, {"status", "string", false, "", "Build status."}},
			DryRunExample: "seacloud --dry-run template build demo --image python:3.13",
			NextStep:      "Use templates.status or templates.logs with the buildID.",
		},
		"templates.list":        readSchema("templates.list", "GET", "/api/v1/templates", "List templates with visibility, limit, and offset filters.", "seacloud template list --limit 20 --format json"),
		"templates.get":         readSchema("templates.get", "GET", "/api/v1/templates/{templateID}", "Get template details.", "seacloud template get base --format json"),
		"templates.delete":      destructiveSchema("templates.delete", "DELETE", "/api/v1/templates/{templateID}", "Delete a template.", "seacloud --dry-run template delete tpl_123"),
		"templates.status":      readSchema("templates.status", "GET", "/api/v1/templates/{templateID}/builds/{buildID}/status", "Get template build status.", "seacloud template status tpl_123 build_123 --limit 50 --format json"),
		"templates.logs":        readSchema("templates.logs", "GET", "/api/v1/templates/{templateID}/builds/{buildID}/logs", "Get template build logs.", "seacloud template logs tpl_123 build_123 --limit 100 --format json"),
		"templates.tags.assign": readWriteSchema("templates.tags.assign", "POST", "/api/v1/templates/tags", "Assign tags to a template build.", "seacloud --dry-run template tags assign build_123 latest"),
		"templates.tags.remove": destructiveSchema("templates.tags.remove", "DELETE", "/api/v1/templates/tags", "Remove tags from a template.", "seacloud --dry-run template tags remove base old-tag"),
	}
}

func readSchema(name, method, path, description, cli string) cliSchema {
	return cliSchema{
		Name:        name,
		Description: description,
		CLI:         []string{cli},
		Method:      method,
		Path:        path,
		Auth:        []string{"SEACLOUD_API_KEY", "E2B_API_KEY", "E2B_ACCESS_TOKEN", "stored seacloud auth config"},
		RequiredHeaders: []string{
			"Authorization: Bearer <api-key>",
			"X-Namespace-ID optional",
			"X-User-ID optional",
			"X-Project-ID optional",
		},
		Response: []schemaField{{"response", "object|array", true, "", "See --format json output for exact fields returned by the API."}},
		NextStep: "Use --dry-run to preview request shape or --format json/--output json for structured execution output.",
	}
}

func readWriteSchema(name, method, path, description, dryRunExample string) cliSchema {
	item := readSchema(name, method, path, description, dryRunExample)
	item.DryRunExample = dryRunExample
	return item
}

func destructiveSchema(name, method, path, description, dryRunExample string) cliSchema {
	item := readWriteSchema(name, method, path, description, dryRunExample)
	item.Destructive = true
	item.NextStep = "Run the dry-run example first; rerun without --dry-run only after confirming the target."
	return item
}

func init() {
	schemaCmd.PersistentFlags().StringVar(&schemaOpts.format, "format", "", "Output format: table, pretty, or json")
	schemaCmd.AddCommand(schemaListCmd, schemaGetCmd)
	rootCmd.AddCommand(schemaCmd)
}
