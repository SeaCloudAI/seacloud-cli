package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sandboxgo "github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	sandboxapi "github.com/SeaCloudAI/seacloud-cli/internal/sandbox"
	"github.com/spf13/cobra"
)

var templateOpts struct {
	output string
}

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage sandbox templates",
	Long: `Manage sandbox templates with an E2B-compatible command shape.

This command exposes template init, build, list, get, delete, build status,
build logs, build history, and tags on top of the SeaCloud sandbox template APIs.

Template commands require a SeaCloud login session from ` + "`seacloud auth login`" + `; ` + "`seacloud auth set-key <api-key>`" + ` is not enough.
Endpoint priority: --base-url, SEACLOUD_SANDBOX_URL, SEACLOUD_BASE_URL, then https://cloud.seaart.ai/api/sandbox/v1.

Each command documents defaults, required flags, output shape, and pagination
controls. Use --format json or --output json for structured output. Use
--dry-run before build, delete, init, migrate, or tag mutation commands to
preview the exact request or file writes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateTemplateOutput()
	},
}

var templateListOpts struct {
	visibility string
	limit      int
	offset     int
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List templates",
	Long: `List sandbox templates.

Use --visibility to filter public/private scope, --limit to cap the result, and
--offset for offset pagination. Use --format json for the raw API response.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		visibility, err := normalizeTemplateVisibility(templateListOpts.visibility)
		if err != nil {
			return err
		}
		params := &build.ListTemplatesParams{
			Visibility: visibility,
			Limit:      templateListOpts.limit,
			Offset:     templateListOpts.offset,
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list templates", Method: "GET", Path: "/api/v1/templates", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templates, err := client.Build.ListTemplates(cmd.Context(), params)
		if err != nil {
			return err
		}
		if templateOutputJSON() {
			return printJSON(templates)
		}
		if len(templates) == 0 {
			fmt.Println("No templates found.")
			return nil
		}
		fmt.Printf("%-28s %-28s %-12s %-10s %-20s\n", "TEMPLATE ID", "NAMES", "STATUS", "PUBLIC", "UPDATED")
		for _, tpl := range templates {
			fmt.Printf("%-28s %-28s %-12s %-10t %-20s\n",
				tpl.TemplateID,
				strings.Join(tpl.Names, ","),
				tpl.BuildStatus,
				tpl.Public,
				formatTime(tpl.UpdatedAt),
			)
		}
		return nil
	},
}

var templateGetOpts struct {
	limit     int
	nextToken string
}

var templateGetCmd = &cobra.Command{
	Use:   "get <template>",
	Short: "Get template details",
	Long:  "Get template details by ID, name, or tag. Use --limit and --next-token to page build history when the API includes it.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get template", Method: "GET", Path: "/api/v1/templates/" + args[0], Query: build.GetTemplateParams{Limit: templateGetOpts.limit, NextToken: templateGetOpts.nextToken}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		tpl, err := client.Build.GetTemplate(cmd.Context(), templateID, &build.GetTemplateParams{
			Limit:     templateGetOpts.limit,
			NextToken: templateGetOpts.nextToken,
		})
		if err != nil {
			return err
		}
		return printJSON(tpl)
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:     "delete <template>",
	Aliases: []string{"rm"},
	Short:   "Delete a template",
	Long:    "Delete a template by ID, name, or tag. This is destructive; run with --dry-run first to verify the target reference.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "delete template", Method: "DELETE", Path: "/api/v1/templates/" + args[0], IDs: args, Destructive: true})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		if err := client.Build.DeleteTemplate(cmd.Context(), templateID); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", templateID)
		return nil
	},
}

var templateExistsCmd = &cobra.Command{
	Use:   "exists <template>",
	Short: "Check whether a template exists",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "resolve template reference", Method: "GET", Path: "/api/v1/templates/resolve/" + args[0]})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		_, err = client.Build.ResolveTemplateRef(cmd.Context(), args[0])
		exists := err == nil
		if templateOutputJSON() {
			return printJSON(map[string]any{"template": args[0], "exists": exists})
		}
		fmt.Println(exists)
		return nil
	},
}

var templateBuildOpts struct {
	dockerfile   string
	image        string
	fromTemplate string
	cpuCount     int32
	memoryMB     int32
	tags         []string
	env          []string
	workdir      string
	visibility   string
	noWait       bool
	pollInterval time.Duration
}

var templateBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build a template",
	Long: `Build a sandbox template.

Template source is required and is resolved in this order:
  1. --from-template template-ref + --dockerfile path applies Dockerfile steps on top of the template
  2. --dockerfile path
  3. --image image
  4. --from-template template-ref
  5. ./Dockerfile or ./e2b.Dockerfile in the current directory

Defaults:
  --poll-interval 2s
  --no-wait false, so the CLI streams build logs until completion

Use --tag for stable references, --env key=value for build-time environment,
--cpu-count and --memory-mb for template resources, and --dry-run to print the
build request without creating or updating a template.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		template, source, err := buildTemplateDefinition()
		if err != nil {
			return err
		}
		visibility, err := normalizeTemplateVisibility(templateBuildOpts.visibility)
		if err != nil {
			return err
		}
		opts := &sandboxgo.TemplateBuildOptions{
			Tags:         splitList(templateBuildOpts.tags),
			Envs:         parseKeyValues(templateBuildOpts.env),
			Workdir:      templateBuildOpts.workdir,
			Visibility:   visibility,
			PollInterval: templateBuildOpts.pollInterval,
			OnBuildLog: func(entry sandboxgo.LogEntry) {
				fmt.Fprintln(os.Stderr, entry.String())
			},
		}
		if templateBuildOpts.cpuCount > 0 {
			opts.CPUCount = &templateBuildOpts.cpuCount
		}
		if templateBuildOpts.memoryMB > 0 {
			opts.MemoryMB = &templateBuildOpts.memoryMB
		}
		if IsDryRun() {
			var request any
			if requestJSON, err := templateJSONForDryRun(template); err == nil {
				_ = json.Unmarshal([]byte(requestJSON), &request)
			}
			return printDryRunPlan(dryRunPlan{Action: "build template", Method: "POST", Path: "/api/v1/templates", Body: map[string]any{"name": name, "source": source, "options": serializableTemplateBuildOptions(opts), "template": request}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		opts.Wait = boolPtr(!templateBuildOpts.noWait)
		info, err := buildTemplateWithClient(cmd.Context(), client, template, name, opts)
		if err != nil {
			return err
		}
		return printTemplateBuildInfo(info)
	},
}

var templateBuildsCmd = &cobra.Command{
	Use:   "builds <template>",
	Short: "List template builds",
	Long:  "List template build history for a template ID, name, or tag.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list template builds", Method: "GET", Path: "/api/v1/templates/" + args[0] + "/builds"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		history, err := client.Build.ListBuilds(cmd.Context(), templateID)
		if err != nil {
			return err
		}
		return printJSON(history)
	},
}

var templateStatusOpts struct {
	logsOffset int
	limit      int
	level      string
}

var templateStatusCmd = &cobra.Command{
	Use:   "status <template> <build-id>",
	Short: "Get template build status",
	Long:  "Get template build status. Use --logs-offset, --limit, and --level to include a bounded slice of build logs.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &build.BuildStatusParams{Level: templateStatusOpts.level}
		if cmd.Flags().Changed("logs-offset") {
			params.LogsOffset = &templateStatusOpts.logsOffset
		}
		if cmd.Flags().Changed("limit") {
			params.Limit = &templateStatusOpts.limit
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get template build status", Method: "GET", Path: "/api/v1/templates/" + args[0] + "/builds/" + args[1] + "/status", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		status, err := client.Build.GetBuildStatus(cmd.Context(), templateID, args[1], params)
		if err != nil {
			return err
		}
		return printJSON(status)
	},
}

var templateLogsOpts struct {
	cursor    int64
	limit     int
	direction string
	level     string
}

var templateLogsCmd = &cobra.Command{
	Use:   "logs <template> <build-id>",
	Short: "Get template build logs",
	Long:  "Get template build logs. Use --cursor, --limit, --direction forward|backward, and --level to keep output bounded.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &build.BuildLogsParams{
			Direction: templateLogsOpts.direction,
			Level:     templateLogsOpts.level,
		}
		if cmd.Flags().Changed("cursor") {
			params.Cursor = &templateLogsOpts.cursor
		}
		if cmd.Flags().Changed("limit") {
			params.Limit = &templateLogsOpts.limit
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get template build logs", Method: "GET", Path: "/api/v1/templates/" + args[0] + "/builds/" + args[1] + "/logs", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		logs, err := client.Build.GetBuildLogs(cmd.Context(), templateID, args[1], params)
		if err != nil {
			return err
		}
		if templateOutputJSON() {
			return printJSON(logs)
		}
		for _, entry := range logs.Logs {
			fmt.Printf("[%s] %-5s %-12s %s\n", formatTime(entry.Timestamp), entry.Level, entry.Step, entry.Message)
		}
		return nil
	},
}

var templateTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage template tags",
	Long:  "Manage stable template tags. Assign and remove mutate references and support --dry-run.",
}

var templateTagsAssignCmd = &cobra.Command{
	Use:   "assign <target> <tag...>",
	Short: "Assign tags to a template build",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "assign template tags", Method: "POST", Path: "/api/v1/templates/tags", Body: build.AssignTemplateTagsRequest{Target: args[0], Tags: args[1:]}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		resp, err := client.Build.AssignTemplateTags(cmd.Context(), &build.AssignTemplateTagsRequest{Target: args[0], Tags: args[1:]})
		if err != nil {
			return err
		}
		return printJSON(resp)
	},
}

var templateTagsListCmd = &cobra.Command{
	Use:   "list <template>",
	Short: "List template tags",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list template tags", Method: "GET", Path: "/api/v1/templates/" + args[0] + "/tags"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		templateID, err := resolveTemplateID(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		tags, err := client.Build.ListTemplateTags(cmd.Context(), templateID)
		if err != nil {
			return err
		}
		return printJSON(tags)
	},
}

var templateTagsRemoveCmd = &cobra.Command{
	Use:   "remove <template> <tag...>",
	Short: "Remove tags from a template",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "remove template tags", Method: "DELETE", Path: "/api/v1/templates/tags", Body: build.DeleteTemplateTagsRequest{Name: args[0], Tags: args[1:]}, Destructive: true})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		if err := client.Build.DeleteTemplateTags(cmd.Context(), &build.DeleteTemplateTagsRequest{Name: args[0], Tags: args[1:]}); err != nil {
			return err
		}
		fmt.Printf("Removed tags from %s\n", args[0])
		return nil
	},
}

var templateInitOpts struct {
	language string
	force    bool
	name     string
}

var templateInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a local template project",
	Long: `Initialize a local template project.

Generated files use the SeaCloud SDK packages that match the three SDKs:
TypeScript imports @seacloudai/sandbox and Python imports sandbox from the
seacloud-sandbox package. Use --language python|typescript, --name to set build
script names, and --force to overwrite existing generated files.

Run with --dry-run first to preview the files that would be written.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return writeTemplateProject(".", templateInitOpts.language, templateInitOpts.name, templateInitOpts.force, false)
	},
}

var templateMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Generate SDK template files for an existing E2B-style template folder",
	Long: `Generate SeaCloud SDK template files for an existing E2B-style folder.

Migration is non-destructive: it does not rename or delete existing E2B files.
If e2b.Dockerfile exists, generated template files point to it; otherwise they
use Dockerfile. Use --dry-run to preview file writes and --force to overwrite
previous generated files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return writeTemplateProject(".", templateInitOpts.language, templateInitOpts.name, templateInitOpts.force, true)
	},
}

func buildTemplateDefinition() (*sandboxgo.Template, string, error) {
	template := sandboxgo.NewTemplate()
	switch {
	case strings.TrimSpace(templateBuildOpts.fromTemplate) != "" && strings.TrimSpace(templateBuildOpts.dockerfile) != "":
		ref := strings.TrimSpace(templateBuildOpts.fromTemplate)
		path := strings.TrimSpace(templateBuildOpts.dockerfile)
		tpl, err := templateFromBaseAndDockerfile(ref, path)
		return tpl, ref + "+" + path, err
	case strings.TrimSpace(templateBuildOpts.dockerfile) != "":
		path := strings.TrimSpace(templateBuildOpts.dockerfile)
		tpl, err := template.FromDockerfile(path)
		return tpl, path, err
	case strings.TrimSpace(templateBuildOpts.image) != "":
		image := strings.TrimSpace(templateBuildOpts.image)
		return template.FromImage(image), image, nil
	case strings.TrimSpace(templateBuildOpts.fromTemplate) != "":
		ref := strings.TrimSpace(templateBuildOpts.fromTemplate)
		return template.FromTemplate(ref), ref, nil
	default:
		for _, candidate := range []string{"Dockerfile", "e2b.Dockerfile"} {
			if _, err := os.Stat(candidate); err == nil {
				tpl, err := template.FromDockerfile(candidate)
				return tpl, candidate, err
			}
		}
		return nil, "", cliMissingParam("--dockerfile/--image/--from-template", "Pass a source flag or add ./Dockerfile, for example: seacloud --dry-run template build demo --image python:3.13")
	}
}

func templateFromBaseAndDockerfile(baseRef, dockerfilePath string) (*sandboxgo.Template, error) {
	dockerfileTemplate, err := sandboxgo.NewTemplate().FromDockerfile(dockerfilePath)
	if err != nil {
		return nil, err
	}
	baseTemplate := sandboxgo.NewTemplate().FromTemplate(baseRef)
	return applyDockerfileRequestToTemplate(baseTemplate, dockerfileTemplate.Request())
}

func applyDockerfileRequestToTemplate(template *sandboxgo.Template, req *build.BuildRequest) (*sandboxgo.Template, error) {
	if template == nil {
		return nil, fmt.Errorf("sandbox template is required")
	}
	if req == nil {
		return template, nil
	}
	for _, step := range req.Steps {
		switch step.Type {
		case "RUN":
			if len(step.Args) > 0 {
				template.RunCmd(step.Args[0], nil)
			}
		case "ENV":
			if len(step.Args)%2 != 0 {
				return nil, fmt.Errorf("sandbox: invalid Dockerfile ENV step")
			}
			envs := make(map[string]string, len(step.Args)/2)
			for i := 0; i+1 < len(step.Args); i += 2 {
				envs[step.Args[i]] = step.Args[i+1]
			}
			template.SetEnvs(envs)
		case "WORKDIR":
			if len(step.Args) > 0 {
				template.SetWorkdir(step.Args[0])
			}
		case "USER":
			if len(step.Args) > 0 {
				template.SetUser(step.Args[0])
			}
		case "COPY":
			if len(step.Args) >= 2 {
				template.Copy(step.Args[0], step.Args[1], nil)
			}
		default:
			return nil, fmt.Errorf("sandbox: unsupported Dockerfile step: %s", step.Type)
		}
	}
	if strings.TrimSpace(req.StartCmd) != "" {
		template.SetStartCmd(req.StartCmd, sandboxgo.ReadyCmd{})
	}
	return template, nil
}

func templateJSONForDryRun(template *sandboxgo.Template) (string, error) {
	return sandboxgo.TemplateToJSON(template, false)
}

func serializableTemplateBuildOptions(opts *sandboxgo.TemplateBuildOptions) map[string]any {
	if opts == nil {
		return nil
	}
	return map[string]any{
		"tags":           opts.Tags,
		"baseTemplateID": opts.BaseTemplateID,
		"visibility":     opts.Visibility,
		"envs":           opts.Envs,
		"workdir":        opts.Workdir,
		"cpuCount":       opts.CPUCount,
		"memoryMB":       opts.MemoryMB,
		"wait":           opts.Wait,
		"pollInterval":   opts.PollInterval.String(),
	}
}

func printTemplateBuildInfo(info *sandboxgo.TemplateBuildInfo) error {
	if templateOutputJSON() {
		return printJSON(info)
	}
	fmt.Printf("Template %s\n", info.TemplateID)
	fmt.Printf("  Build:  %s\n", info.BuildID)
	if info.Name != "" {
		fmt.Printf("  Name:   %s\n", info.Name)
	}
	if info.Status != "" {
		fmt.Printf("  Status: %s\n", info.Status)
	}
	return nil
}

func resolveTemplateID(ctx context.Context, client *sandboxapi.Client, ref string) (string, error) {
	if strings.HasPrefix(ref, "tpl-") {
		return ref, nil
	}
	resolved, err := client.Build.ResolveTemplateRef(ctx, ref)
	if err != nil {
		return "", err
	}
	return resolved.TemplateID, nil
}

func templateOutputJSON() bool {
	return templateOpts.output == "json" || sandboxOpts.output == "json"
}

func normalizeTemplateVisibility(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "personal", "team", "official":
		return value, nil
	case "private":
		return "personal", nil
	default:
		return "", cliParamError("--visibility", fmt.Sprintf("unsupported value %q; allowed values are: personal, team, official", value), "Use --visibility personal for private user-owned templates.")
	}
}

func buildTemplateWithClient(ctx context.Context, client *sandboxapi.Client, template *sandboxgo.Template, name string, opts *sandboxgo.TemplateBuildOptions) (*sandboxgo.TemplateBuildInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("sandbox client is required")
	}
	if template == nil {
		return nil, fmt.Errorf("sandbox template is required")
	}
	if opts == nil {
		opts = &sandboxgo.TemplateBuildOptions{}
	}
	templateName, parsedTags, err := parseTemplateNameForCLI(name)
	if err != nil {
		return nil, err
	}
	tags := dedupeTemplateTags(append(parsedTags, opts.Tags...))
	created, err := client.Build.CreateTemplate(ctx, &build.TemplateCreateRequest{
		Name:       templateName,
		Tags:       tags,
		CPUCount:   opts.CPUCount,
		MemoryMB:   opts.MemoryMB,
		Extensions: templateBuildExtensions(opts),
	})
	if err != nil {
		return nil, err
	}

	buildID := "build-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 16)
	if opts.OnBuildLog != nil {
		opts.OnBuildLog(sandboxgo.LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: "Starting build " + buildID})
	}
	if _, err := client.Build.CreateBuild(ctx, created.TemplateID, buildID, template.Request()); err != nil {
		return nil, err
	}
	wait := true
	if opts.Wait != nil {
		wait = *opts.Wait
	}
	if !wait {
		templateResp, err := client.Build.GetTemplate(ctx, created.TemplateID, nil)
		if err != nil {
			return nil, err
		}
		return &sandboxgo.TemplateBuildInfo{
			TemplateID: created.TemplateID,
			BuildID:    buildID,
			Name:       templateName,
			Tags:       tags,
			Alias:      templateName,
			Status:     "building",
			Template:   templateResp,
		}, nil
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	logsOffset := 0
	var status *build.BuildStatusResponse
	for {
		status, err = client.Build.GetBuildStatus(ctx, created.TemplateID, buildID, &build.BuildStatusParams{
			LogsOffset: &logsOffset,
			Limit:      intPtr(100),
		})
		if err != nil {
			return nil, err
		}
		logsOffset += len(status.LogEntries)
		if opts.OnBuildLog != nil {
			for _, entry := range status.LogEntries {
				opts.OnBuildLog(sandboxgo.LogEntry{
					Timestamp: entry.Timestamp,
					Level:     normalizeBuildLogLevel(entry.Level),
					Message:   strings.TrimPrefix(entry.Step+": "+entry.Message, ": "),
				})
			}
		}
		if isTerminalBuildStatus(status.Status) {
			break
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if opts.OnBuildLog != nil {
		opts.OnBuildLog(sandboxgo.LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: "Build " + buildID + " finished with status " + status.Status})
	}
	templateResp, err := client.Build.GetTemplate(ctx, created.TemplateID, nil)
	if err != nil {
		return nil, err
	}
	buildResp, err := client.Build.GetBuild(ctx, created.TemplateID, buildID)
	if err != nil {
		return nil, err
	}
	return &sandboxgo.TemplateBuildInfo{
		TemplateID: created.TemplateID,
		BuildID:    buildID,
		Name:       templateName,
		Tags:       tags,
		Alias:      templateName,
		Status:     status.Status,
		Template:   templateResp,
		Build:      buildResp,
	}, nil
}

func parseTemplateNameForCLI(name string) (string, []string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", nil, cliMissingParam("name", "Use: seacloud template build <name>")
	}
	lastColon := strings.LastIndex(trimmed, ":")
	if lastColon < 0 {
		return trimmed, nil, nil
	}
	baseName := strings.TrimSpace(trimmed[:lastColon])
	tag := strings.TrimSpace(trimmed[lastColon+1:])
	if baseName == "" || tag == "" {
		return "", nil, cliParamError("name", "must be name or name:tag", "Use: seacloud template build demo:v1 --image python:3.13")
	}
	return baseName, []string{tag}, nil
}

func dedupeTemplateTags(values []string) []string {
	seen := map[string]struct{}{}
	tags := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		tags = append(tags, trimmed)
	}
	return tags
}

func templateBuildExtensions(opts *sandboxgo.TemplateBuildOptions) *build.PublicTemplateExtensions {
	if opts == nil {
		return nil
	}
	extensions := &build.PublicTemplateExtensions{}
	hasExtensions := false
	if strings.TrimSpace(opts.BaseTemplateID) != "" {
		extensions.BaseTemplateID = strings.TrimSpace(opts.BaseTemplateID)
		hasExtensions = true
	}
	if strings.TrimSpace(opts.Visibility) != "" {
		extensions.Visibility = strings.TrimSpace(opts.Visibility)
		hasExtensions = true
	}
	if len(opts.Envs) > 0 {
		extensions.Envs = make(map[string]string, len(opts.Envs))
		for k, v := range opts.Envs {
			extensions.Envs[k] = v
		}
		hasExtensions = true
	}
	if len(opts.VolumeMounts) > 0 {
		extensions.VolumeMounts = append([]build.TemplateVolumeMount(nil), opts.VolumeMounts...)
		hasExtensions = true
	}
	if strings.TrimSpace(opts.Workdir) != "" {
		extensions.Workdir = strings.TrimSpace(opts.Workdir)
		hasExtensions = true
	}
	if !hasExtensions {
		return nil
	}
	return extensions
}

func isTerminalBuildStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "failed", "error", "cancelled":
		return true
	default:
		return false
	}
}

func normalizeBuildLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "warn", "error":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "info"
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func writeTemplateProject(dir, language, name string, force, migrate bool) error {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		language = "typescript"
	}
	if name = strings.TrimSpace(name); name == "" {
		name = filepath.Base(mustAbs(dir))
	}
	dockerfile := "Dockerfile"
	if migrate {
		if _, err := os.Stat(filepath.Join(dir, "e2b.Dockerfile")); err == nil {
			dockerfile = "e2b.Dockerfile"
		}
	}
	switch language {
	case "ts", "typescript", "js", "javascript":
		files := map[string]string{
			"template.ts":   tsTemplateFile(dockerfile),
			"build.dev.ts":  tsBuildFile(name + "-dev"),
			"build.prod.ts": tsBuildFile(name),
			"README.md":     templateReadme(name, "npm install @seacloudai/sandbox"),
		}
		if IsDryRun() {
			return printJSON(map[string]any{"action": "write-template-project", "dir": dir, "language": language, "files": fileNames(files)})
		}
		return writeTemplateFiles(dir, force, files)
	case "py", "python":
		files := map[string]string{
			"template.py":   pyTemplateFile(dockerfile),
			"build_dev.py":  pyBuildFile(name + "-dev"),
			"build_prod.py": pyBuildFile(name),
			"README.md":     templateReadme(name, "pip install seacloud-sandbox"),
		}
		if IsDryRun() {
			return printJSON(map[string]any{"action": "write-template-project", "dir": dir, "language": language, "files": fileNames(files)})
		}
		return writeTemplateFiles(dir, force, files)
	default:
		return cliParamError("--language", "must be typescript or python", "Use: seacloud template init --language typescript or seacloud template init --language python")
	}
}

func fileNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
}

func writeTemplateFiles(dir string, force bool, files map[string]string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if !force {
			if _, err := os.Stat(path); err == nil {
				return &clierrors.CLIError{
					Message: fmt.Sprintf("refusing to overwrite existing file %s", path),
					Hint:    "Review the file, then rerun with --force if overwriting is intended. Use --dry-run to preview generated files.",
				}
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", path)
	}
	return nil
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func tsTemplateFile(dockerfile string) string {
	return fmt.Sprintf(`import { Template } from '@seacloudai/sandbox'

export const template = await new Template().fromDockerfile('%s')
`, dockerfile)
}

func tsBuildFile(name string) string {
	return fmt.Sprintf(`import { Template, defaultBuildLogger } from '@seacloudai/sandbox'
import { template } from './template'

await Template.build(template, '%s', {
  onBuildLogs: defaultBuildLogger(),
})
`, name)
}

func pyTemplateFile(dockerfile string) string {
	return fmt.Sprintf(`from sandbox import Template

template = Template().from_dockerfile("%s")
`, dockerfile)
}

func pyBuildFile(name string) string {
	return fmt.Sprintf(`from sandbox import Template, default_build_logger
from template import template

Template.build(
    template,
    "%s",
    on_build_logs=default_build_logger(),
)
`, name)
}

func templateReadme(name, install string) string {
	return fmt.Sprintf(`# %s

## Install

%s

## Build

Authenticate before real template operations:

`+"```bash"+`
seacloud auth login
`+"```"+`

Development:

`+"```bash"+`
seacloud --dry-run template build %s-dev
seacloud template build %s-dev
`+"```"+`

Production:

`+"```bash"+`
seacloud template build %s
`+"```"+`
`, name, install, name, name, name)
}

func init() {
	templateCmd.PersistentFlags().StringVar(&templateOpts.output, "format", "", "Output format: json or table (default table)")
	templateCmd.PersistentFlags().StringVar(&sandboxOpts.baseURL, "base-url", "", "Sandbox API base URL (default: https://cloud.seaart.ai/api/sandbox/v1)")
	templateCmd.PersistentFlags().StringVar(&sandboxOpts.namespaceID, "namespace", "", "Sandbox namespace ID")
	templateCmd.PersistentFlags().StringVar(&sandboxOpts.userID, "user-id", "", "User ID header for sandbox APIs")
	templateCmd.PersistentFlags().StringVar(&sandboxOpts.projectID, "project-id", "", "Project/team ID header for sandbox APIs")
	templateCmd.PersistentFlags().StringVar(&sandboxOpts.output, "output", "", "Output format: json or table (default table)")

	templateListCmd.Flags().StringVar(&templateListOpts.visibility, "visibility", "", "Visibility filter")
	templateListCmd.Flags().IntVar(&templateListOpts.limit, "limit", 0, "Maximum number of templates")
	templateListCmd.Flags().IntVar(&templateListOpts.offset, "offset", 0, "Template list offset")

	templateGetCmd.Flags().IntVar(&templateGetOpts.limit, "limit", 0, "Build history limit")
	templateGetCmd.Flags().StringVar(&templateGetOpts.nextToken, "next-token", "", "Build history next token")

	templateBuildCmd.Flags().StringVar(&templateBuildOpts.dockerfile, "dockerfile", "", "Dockerfile path")
	templateBuildCmd.Flags().StringVar(&templateBuildOpts.image, "image", "", "Base image")
	templateBuildCmd.Flags().StringVar(&templateBuildOpts.fromTemplate, "from-template", "", "Base template reference")
	templateBuildCmd.Flags().Int32Var(&templateBuildOpts.cpuCount, "cpu-count", 0, "Template CPU count")
	templateBuildCmd.Flags().Int32Var(&templateBuildOpts.memoryMB, "memory-mb", 0, "Template memory in MB")
	templateBuildCmd.Flags().StringArrayVar(&templateBuildOpts.tags, "tag", nil, "Template tag, repeatable or comma-separated")
	templateBuildCmd.Flags().StringArrayVar(&templateBuildOpts.env, "env", nil, "Build env key=value, repeatable or comma-separated")
	templateBuildCmd.Flags().StringVar(&templateBuildOpts.workdir, "workdir", "", "Default template workdir")
	templateBuildCmd.Flags().StringVar(&templateBuildOpts.visibility, "visibility", "", "Template visibility")
	templateBuildCmd.Flags().BoolVar(&templateBuildOpts.noWait, "no-wait", false, "Start the build and return immediately")
	templateBuildCmd.Flags().DurationVar(&templateBuildOpts.pollInterval, "poll-interval", 2*time.Second, "Build polling interval")

	templateStatusCmd.Flags().IntVar(&templateStatusOpts.logsOffset, "logs-offset", 0, "Log offset")
	templateStatusCmd.Flags().IntVar(&templateStatusOpts.limit, "limit", 0, "Log entry limit")
	templateStatusCmd.Flags().StringVar(&templateStatusOpts.level, "level", "", "Log level")

	templateLogsCmd.Flags().Int64Var(&templateLogsOpts.cursor, "cursor", 0, "Log cursor")
	templateLogsCmd.Flags().IntVar(&templateLogsOpts.limit, "limit", 0, "Log line limit")
	templateLogsCmd.Flags().StringVar(&templateLogsOpts.direction, "direction", "", "Log direction: forward or backward")
	templateLogsCmd.Flags().StringVar(&templateLogsOpts.level, "level", "", "Log level")

	templateInitCmd.Flags().StringVar(&templateInitOpts.language, "language", "typescript", "Template language: typescript or python")
	templateInitCmd.Flags().StringVar(&templateInitOpts.name, "name", "", "Template name used in generated build scripts")
	templateInitCmd.Flags().BoolVar(&templateInitOpts.force, "force", false, "Overwrite generated files")
	templateMigrateCmd.Flags().StringVar(&templateInitOpts.language, "language", "typescript", "Template language: typescript or python")
	templateMigrateCmd.Flags().StringVar(&templateInitOpts.name, "name", "", "Template name used in generated build scripts")
	templateMigrateCmd.Flags().BoolVar(&templateInitOpts.force, "force", false, "Overwrite generated files")

	templateTagsCmd.AddCommand(templateTagsListCmd, templateTagsAssignCmd, templateTagsRemoveCmd)
	templateCmd.AddCommand(
		templateInitCmd,
		templateMigrateCmd,
		templateBuildCmd,
		templateListCmd,
		templateGetCmd,
		templateDeleteCmd,
		templateExistsCmd,
		templateBuildsCmd,
		templateStatusCmd,
		templateLogsCmd,
		templateTagsCmd,
	)
	rootCmd.AddCommand(templateCmd)
}
