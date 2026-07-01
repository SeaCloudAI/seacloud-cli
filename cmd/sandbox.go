package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	runtimecmd "github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	sandboxapi "github.com/SeaCloudAI/seacloud-cli/internal/sandbox"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var sandboxOpts struct {
	baseURL     string
	namespaceID string
	userID      string
	projectID   string
	output      string
}

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Manage and interact with sandboxes",
	Long: `Manage and interact with SeaCloud sandboxes.

The command shape follows E2B's sandbox CLI for create/list/exec/connect/kill/metrics,
and also exposes Atlas v1 APIs that are available in the SDKs: volumes, lifecycle
events, webhooks, teams, logs, pause, refresh, timeout, and network policy updates.

Sandbox commands require a SeaCloud login session from ` + "`seacloud auth login`" + `; ` + "`seacloud auth set-key <api-key>`" + ` is not enough.
Endpoint priority: --base-url, SEACLOUD_SANDBOX_URL, SEACLOUD_BASE_URL, then https://cloud.seaart.ai/api/sandbox/v1.

Output defaults to a compact table for humans. Use --output json for structured
output. List and log commands expose --limit plus cursor or token flags to keep
responses small.

All write/delete commands support the global --dry-run flag. Dry-run prints the
HTTP method, path, request body, filters, destructive status, and next step without
requiring credentials or mutating remote state.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateSandboxOutput()
	},
}

var sandboxCreateOpts struct {
	timeout             int64
	waitReady           bool
	autoPause           bool
	autoResume          bool
	metadata            []string
	env                 []string
	allowPublicTraffic  string
	allowInternetAccess string
	allowOut            []string
	denyOut             []string
	volumeMounts        []string
	connect             bool
	noConnect           bool
	killOnExit          bool
	shell               string
	connectTimeout      int64
}

var sandboxCreateCmd = &cobra.Command{
	Use:   "create [template]",
	Short: "Create a sandbox",
	Long: `Create a sandbox from a template.

The template argument is optional for SDK parity. When omitted, the server uses
the default base template. In an interactive terminal, create behaves like E2B:
it creates the sandbox, opens a shell, and kills the sandbox on shell exit unless
--no-connect or --output json is used.

Use --dry-run before automation writes. It prints the exact request body,
including metadata, env vars, network policy, and volume mounts.`,
	Example: "  seacloud sandbox create base\n  seacloud sandbox create --wait --output json\n  seacloud sandbox create base --volume-mount cache:/cache --allow-internet-access=false\n  seacloud --dry-run sandbox create --metadata app=agent --output json",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := buildCreateSandboxRequest(args)
		if err != nil {
			return err
		}
		connectAfterCreate := shouldConnectAfterCreate(cmd)
		if connectAfterCreate {
			req.WaitReady = boolPtr(true)
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{
				Action: "create sandbox",
				Method: "POST",
				Path:   "/api/v1/sandboxes",
				Body:   req,
				PreviewNotes: []string{
					"templateID is optional; when empty, the server default base template is used.",
					"interactive create may connect after creation; --output json or --no-connect keeps it automation-only.",
				},
			})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		created, err := client.Control.CreateSandbox(ctx, req)
		if err != nil {
			return err
		}
		if connectAfterCreate {
			killOnExit := sandboxCreateOpts.killOnExit
			if !cmd.Flags().Changed("kill-on-exit") {
				killOnExit = true
			}
			if killOnExit {
				defer func() {
					cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					_ = client.Control.DeleteSandbox(cleanupCtx, created.SandboxID)
				}()
			}
			if err := connectSandboxShell(ctx, client, created.SandboxID, sandboxCreateOpts.shell, sandboxCreateOpts.connectTimeout); err != nil {
				return err
			}
			return nil
		}
		return printSandbox(created, sandboxOpts.output)
	},
}

var sandboxListOpts struct {
	state     []string
	metadata  []string
	limit     int
	nextToken string
}

var sandboxListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List sandboxes",
	Long: `List sandboxes with optional state and metadata filters.

Use --limit to cap response size and --next-token to fetch the next page.
Use --output json when another tool needs the full page object, including items,
hasNext, and nextToken.`,
	Example: "  seacloud sandbox list\n  seacloud sandbox list --state running,paused --metadata app=agent --limit 10 --output json",
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.ListSandboxesParams{
			Metadata:  parseKeyValues(sandboxListOpts.metadata),
			State:     splitList(sandboxListOpts.state),
			Limit:     sandboxListOpts.limit,
			NextToken: sandboxListOpts.nextToken,
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list sandboxes", Method: "GET", Path: "/api/v1/sandboxes", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		page, err := client.Control.ListSandboxesPage(cmd.Context(), params)
		if err != nil {
			return err
		}
		if sandboxOpts.output == "json" {
			return printJSON(page)
		}
		if len(page.Items) == 0 {
			fmt.Println("No sandboxes found.")
			return nil
		}
		fmt.Printf("%-28s %-12s %-18s %-20s %-20s\n", "SANDBOX ID", "STATE", "TEMPLATE", "STARTED", "END")
		for _, item := range page.Items {
			fmt.Printf("%-28s %-12s %-18s %-20s %-20s\n",
				item.SandboxID,
				firstNonEmpty(item.State, item.Status),
				item.TemplateID,
				formatTime(item.StartedAt),
				formatTime(item.EndAt),
			)
		}
		if page.HasNext {
			fmt.Fprintf(os.Stderr, "Next token: %s\n", page.NextToken)
		}
		return nil
	},
}

var sandboxInfoCmd = &cobra.Command{
	Use:   "info <sandbox-id>",
	Short: "Show sandbox details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printJSON(map[string]any{"method": "GET", "path": "/api/v1/sandboxes/" + args[0]})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		detail, err := client.Control.GetSandbox(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(detail)
	},
}

var sandboxKillOpts struct {
	all      bool
	state    []string
	metadata []string
}

var sandboxKillCmd = &cobra.Command{
	Use:     "kill [sandbox-id...]",
	Aliases: []string{"delete", "rm"},
	Short:   "Kill one or more sandboxes",
	Long: `Kill one or more sandboxes.

Pass explicit sandbox IDs for targeted deletion. Use --all with --state and
--metadata filters for bulk cleanup. Bulk cleanup is destructive; run with
--dry-run first to inspect the filters that will be used before any sandbox is
deleted.`,
	Example: "  seacloud sandbox kill sb_123\n  seacloud --dry-run sandbox kill --all --state running,paused --metadata app=agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			if sandboxKillOpts.all {
				return printDryRunPlan(dryRunPlan{
					Action:      "kill all matching sandboxes",
					Method:      "DELETE",
					Path:        "/api/v1/sandboxes/{sandboxID}",
					Destructive: true,
					Query: control.ListSandboxesParams{
						Metadata: parseKeyValues(sandboxKillOpts.metadata),
						State:    splitList(sandboxKillOpts.state),
						Limit:    100,
					},
					PreviewNotes: []string{"the CLI will first list up to 100 matching sandboxes, then delete each returned sandbox ID"},
				})
			}
			if len(args) == 0 {
				return cliMissingParam("<sandbox-id...>", "Pass one or more IDs, or run: seacloud --dry-run sandbox kill --all --state running")
			}
			return printDryRunPlan(dryRunPlan{Action: "kill sandboxes", Method: "DELETE", Path: "/api/v1/sandboxes/{sandboxID}", IDs: args, Destructive: true})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		ids := args
		if sandboxKillOpts.all {
			page, err := client.Control.ListSandboxesPage(cmd.Context(), &control.ListSandboxesParams{
				Metadata: parseKeyValues(sandboxKillOpts.metadata),
				State:    splitList(sandboxKillOpts.state),
				Limit:    100,
			})
			if err != nil {
				return err
			}
			ids = make([]string, 0, len(page.Items))
			for _, item := range page.Items {
				ids = append(ids, item.SandboxID)
			}
		}
		if len(ids) == 0 {
			return cliMissingParam("<sandbox-id...>", "Pass one or more IDs, or use --all with filters such as --state running.")
		}
		for _, id := range ids {
			if err := client.Control.DeleteSandbox(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Printf("Killed %s\n", id)
		}
		return nil
	},
}

var sandboxExecOpts struct {
	background bool
	cwd        string
	user       string
	env        []string
	timeoutMS  int64
}

var sandboxExecCmd = &cobra.Command{
	Use:   "exec <sandbox-id> <command...>",
	Short: "Execute a command in a running sandbox",
	Long: `Execute a command in a running sandbox.

The command is executed through the sandbox runtime service. stdin is forwarded
when piped. Use --background to start the command and print process identifiers
without streaming output. Use --timeout-ms to bound long-running commands.`,
	Example: "  seacloud sandbox exec sb_123 ls -la\n  echo foo | seacloud sandbox exec sb_123 cat\n  seacloud sandbox exec --background sb_123 'sleep 60 && echo done'",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sandboxID := args[0]
		command := strings.Join(args[1:], " ")
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "execute sandbox command", Method: "POST", Path: "/api/v1/sandboxes/" + sandboxID + "/connect", Body: map[string]any{"command": command, "cwd": sandboxExecOpts.cwd, "user": sandboxExecOpts.user, "env": parseKeyValues(sandboxExecOpts.env), "timeoutMS": sandboxExecOpts.timeoutMS, "background": sandboxExecOpts.background}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		_, runtime, err := client.ConnectRuntime(cmd.Context(), sandboxID, 0)
		if err != nil {
			return err
		}
		return runSandboxCommand(cmd.Context(), runtime.Service, command, sandboxExecOpts)
	},
}

var sandboxConnectOpts struct {
	timeout int64
	shell   string
}

var sandboxConnectCmd = &cobra.Command{
	Use:   "connect <sandbox-id>",
	Short: "Connect an interactive terminal to a sandbox",
	Long: `Connect an interactive terminal to a sandbox.

The default shell is sh. Pass --shell bash, zsh, or another command when the
template provides it. Use --timeout to bound connection setup time.`,
	Example: "  seacloud sandbox connect sb_123\n  seacloud sandbox connect sb_123 --shell bash",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "connect sandbox shell", Method: "POST", Path: "/api/v1/sandboxes/" + args[0] + "/connect", Body: map[string]any{"shell": sandboxConnectOpts.shell, "timeout": sandboxConnectOpts.timeout}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		return connectSandboxShell(cmd.Context(), client, args[0], sandboxConnectOpts.shell, sandboxConnectOpts.timeout)
	},
}

var sandboxMetricsCmd = &cobra.Command{
	Use:   "metrics [sandbox-id...]",
	Short: "Show sandbox metrics",
	Long: `Show sandbox metrics.

With one sandbox ID and no --limit, this returns the latest metric snapshot.
With multiple IDs or --limit, it lists metric snapshots. Use --limit to control
output size and --output json for structured metrics.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			if len(args) != 1 || cmd.Flags().Changed("limit") {
				return printDryRunPlan(dryRunPlan{Action: "list sandbox metrics", Method: "GET", Path: "/api/v1/sandboxes/metrics", Query: control.SandboxMetricsParams{SandboxIDs: args, Limit: sandboxMetricsLimit}})
			}
			return printDryRunPlan(dryRunPlan{Action: "get sandbox metrics", Method: "GET", Path: "/api/v1/sandboxes/" + args[0] + "/metrics"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		if len(args) != 1 || cmd.Flags().Changed("limit") {
			metrics, err := client.Control.ListSandboxMetrics(cmd.Context(), &control.SandboxMetricsParams{
				SandboxIDs: args,
				Limit:      sandboxMetricsLimit,
			})
			if err != nil {
				return err
			}
			if sandboxOpts.output == "json" {
				return printJSON(metrics)
			}
			for _, item := range metrics.Items {
				fmt.Printf("%-28s %-20s load1=%s memory=%s%%\n",
					item.SandboxID,
					formatTime(item.CollectedAt),
					formatFloatPtr(item.Load1),
					formatFloatPtr(item.MemoryUsagePercent),
				)
			}
			return nil
		}
		metrics, err := client.Control.GetSandboxMetrics(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if sandboxOpts.output == "json" {
			return printJSON(metrics)
		}
		fmt.Printf("Metrics for sandbox %s\n", args[0])
		fmt.Printf("[%s] load1=%s memory=%s%% net_rx=%sB/s net_tx=%sB/s\n",
			formatTime(metrics.CollectedAt),
			formatFloatPtr(metrics.Load1),
			formatFloatPtr(metrics.MemoryUsagePercent),
			formatFloatPtr(metrics.NetworkRecvBytesPerSecond),
			formatFloatPtr(metrics.NetworkSentBytesPerSecond),
		)
		return nil
	},
}

var sandboxMetricsLimit int

var sandboxLogsOpts struct {
	limit     int
	cursor    int64
	direction string
	level     string
	search    string
}

var sandboxLogsCmd = &cobra.Command{
	Use:   "logs <sandbox-id>",
	Short: "Show sandbox logs",
	Long: `Show sandbox logs with cursor pagination.

Use --limit to cap returned log lines, --cursor to continue from a known cursor,
--direction forward|backward for traversal, --level for severity filtering, and
--search to narrow the result set. Use --output json for cursor fields and raw
log entries.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.SandboxLogsParams{
			Direction: sandboxLogsOpts.direction,
			Level:     sandboxLogsOpts.level,
			Search:    sandboxLogsOpts.search,
		}
		if cmd.Flags().Changed("limit") {
			params.Limit = &sandboxLogsOpts.limit
		}
		if cmd.Flags().Changed("cursor") && sandboxLogsOpts.cursor > 0 {
			params.Cursor = &sandboxLogsOpts.cursor
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get sandbox logs", Method: "GET", Path: "/api/v1/sandboxes/" + args[0] + "/logs", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		resp, err := client.Control.GetSandboxLogs(cmd.Context(), args[0], params)
		if err != nil {
			return err
		}
		if sandboxOpts.output == "json" {
			return printJSON(resp)
		}
		for _, entry := range resp.Logs {
			fmt.Printf("[%s] %-5s %s\n", formatTime(entry.Timestamp), entry.Level, entry.Message)
		}
		return nil
	},
}

var sandboxPauseCmd = &cobra.Command{
	Use:   "pause <sandbox-id>",
	Short: "Pause a sandbox",
	Long:  "Pause a sandbox so it can be resumed later when auto-resume routing is enabled.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "pause sandbox", Method: "POST", Path: "/api/v1/sandboxes/" + args[0] + "/pause"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		if err := client.Control.PauseSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("Paused %s\n", args[0])
		return nil
	},
}

var sandboxTimeoutSeconds int64

var sandboxTimeoutCmd = &cobra.Command{
	Use:   "timeout <sandbox-id>",
	Short: "Set sandbox timeout in seconds",
	Long:  "Set the sandbox timeout in seconds. The default flag value is --seconds 3600.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "set sandbox timeout", Method: "POST", Path: "/api/v1/sandboxes/" + args[0] + "/timeout", Body: control.TimeoutRequest{Timeout: sandboxTimeoutSeconds}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		return client.Control.SetSandboxTimeout(cmd.Context(), args[0], &control.TimeoutRequest{Timeout: sandboxTimeoutSeconds})
	},
}

var sandboxRefreshDuration int32

var sandboxRefreshCmd = &cobra.Command{
	Use:   "refresh <sandbox-id>",
	Short: "Refresh sandbox lifetime",
	Long:  "Refresh sandbox lifetime. Pass --duration to request a specific refresh duration in seconds.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &control.RefreshSandboxRequest{}
		if cmd.Flags().Changed("duration") {
			req.Duration = &sandboxRefreshDuration
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "refresh sandbox lifetime", Method: "POST", Path: "/api/v1/sandboxes/" + args[0] + "/refreshes", Body: req})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		return client.Control.RefreshSandbox(cmd.Context(), args[0], req)
	},
}

var sandboxNetworkOpts struct {
	allowPublicTraffic  string
	allowInternetAccess string
	allowOut            []string
	denyOut             []string
}

var sandboxNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage sandbox network policy",
	Long: `Manage sandbox network policy.

Network policy can control public inbound traffic, internet egress, and explicit
egress allow/deny CIDR lists. Run update with --dry-run before applying policy
changes in automation.`,
}

var sandboxNetworkUpdateCmd = &cobra.Command{
	Use:   "update <sandbox-id>",
	Short: "Update sandbox network policy",
	Long: `Update sandbox network policy.

At least one policy flag is required. Boolean flags accept true or false.
--allow-out and --deny-out accept repeated values or comma-separated IPv4/CIDR
entries.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := buildNetworkUpdateBody()
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return cliMissingParam("--allow-public-traffic/--allow-internet-access/--allow-out/--deny-out", "Add at least one policy flag, for example: seacloud --dry-run sandbox network update sb_123 --allow-internet-access=false")
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "update sandbox network policy", Method: "PUT", Path: "/api/v1/sandboxes/" + args[0] + "/network", Body: body})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		if err := client.UpdateNetwork(cmd.Context(), args[0], body); err != nil {
			return err
		}
		fmt.Printf("Updated network policy for %s\n", args[0])
		return nil
	},
}

var sandboxVolumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Manage sandbox volumes",
	Long: `Manage persistent sandbox volumes.

Volumes are durable storage resources that can be mounted during sandbox create
with --volume-mount name:/path. Create/delete support --dry-run so callers can
preview storage changes before mutating state.`,
}

var sandboxVolumeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a volume",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "create volume", Method: "POST", Path: "/api/v1/volumes", Body: control.NewVolumeRequest{Name: args[0]}})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		volume, err := client.Control.CreateVolume(cmd.Context(), &control.NewVolumeRequest{Name: args[0]})
		if err != nil {
			return err
		}
		return printJSON(volume)
	},
}

var sandboxVolumeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list volumes", Method: "GET", Path: "/api/v1/volumes"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		volumes, err := client.Control.ListVolumes(cmd.Context())
		if err != nil {
			return err
		}
		if sandboxOpts.output == "json" {
			return printJSON(volumes)
		}
		for _, volume := range volumes {
			fmt.Printf("%-28s %s\n", volume.VolumeID, volume.Name)
		}
		return nil
	},
}

var sandboxVolumeGetCmd = &cobra.Command{
	Use:   "get <volume-id>",
	Short: "Get a volume",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get volume", Method: "GET", Path: "/api/v1/volumes/" + args[0]})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		volume, err := client.Control.GetVolume(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(volume)
	},
}

var sandboxVolumeDeleteCmd = &cobra.Command{
	Use:     "delete <volume-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a volume",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "delete volume", Method: "DELETE", Path: "/api/v1/volumes/" + args[0], IDs: args, Destructive: true})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		if err := client.Control.DeleteVolume(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", args[0])
		return nil
	},
}

var sandboxEventsOpts struct {
	limit    int
	offset   int
	orderAsc bool
	types    []string
}

var sandboxEventsCmd = &cobra.Command{
	Use:   "events [sandbox-id]",
	Short: "List sandbox lifecycle events",
	Long: `List sandbox lifecycle events.

Without an ID this lists events across the namespace. With a sandbox ID it lists
only that sandbox's events. Use --limit, --offset, --type, and --order-asc to keep
the response bounded.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.ListSandboxEventsParams{
			Limit:  sandboxEventsOpts.limit,
			Offset: sandboxEventsOpts.offset,
			Types:  splitList(sandboxEventsOpts.types),
		}
		if cmd.Flags().Changed("order-asc") {
			params.OrderAsc = &sandboxEventsOpts.orderAsc
		}
		if IsDryRun() {
			path := "/api/v1/events"
			if len(args) == 1 {
				path = "/api/v1/events/sandboxes/" + args[0]
			}
			return printDryRunPlan(dryRunPlan{Action: "list sandbox lifecycle events", Method: "GET", Path: path, Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		var events []control.SandboxLifecycleEvent
		if len(args) == 1 {
			events, err = client.Control.ListSandboxEventsBySandbox(cmd.Context(), args[0], params)
		} else {
			events, err = client.Control.ListSandboxEvents(cmd.Context(), params)
		}
		if err != nil {
			return err
		}
		return printJSON(events)
	},
}

var sandboxWebhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage lifecycle webhooks",
	Long: `Manage lifecycle webhooks under /api/v1/events.

Webhook deliveries are signed with the configured secret. Use --event to choose
event types, --max-attempts and --delay-seconds to configure retry policy, and
--dead-letter-* flags to capture exhausted deliveries. Write/delete/replay
commands support --dry-run.`,
}

var webhookCreateOpts struct {
	name              string
	url               string
	events            []string
	secret            string
	enabled           bool
	maxAttempts       int
	delaySeconds      []int
	deadLetterEnabled bool
	deadLetterURL     string
}

var webhookUpdateOpts struct {
	name              string
	url               string
	events            []string
	secret            string
	enabled           string
	maxAttempts       int
	delaySeconds      []int
	deadLetterEnabled string
	deadLetterURL     string
}

var sandboxWebhookCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a lifecycle webhook",
	Long: `Create a lifecycle webhook.

Required flags:
  --name     Human-readable webhook name.
  --url      HTTPS callback URL.
  --secret   Signature secret used for delivery verification.

Optional retry flags:
  --max-attempts controls delivery attempts.
  --delay-seconds accepts repeated or comma-separated delays.
  --dead-letter-enabled and --dead-letter-url configure exhausted delivery flow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(webhookCreateOpts.name) == "" {
			return cliMissingParam("--name", "Add --name, for example: seacloud --dry-run sandbox webhook create --name lifecycle --url https://example.com/hook --secret $WEBHOOK_SECRET")
		}
		if strings.TrimSpace(webhookCreateOpts.url) == "" {
			return cliMissingParam("--url", "Add an HTTPS callback URL with --url.")
		}
		if strings.TrimSpace(webhookCreateOpts.secret) == "" {
			return cliMissingParam("--secret", "Add --secret or inject a secret from your secret manager.")
		}
		req := &control.LifecycleWebhookCreateRequest{
			Name:            webhookCreateOpts.name,
			URL:             webhookCreateOpts.url,
			Enabled:         &webhookCreateOpts.enabled,
			Events:          splitList(webhookCreateOpts.events),
			SignatureSecret: webhookCreateOpts.secret,
			DeadLetterURL:   webhookCreateOpts.deadLetterURL,
		}
		if cmd.Flags().Changed("max-attempts") || len(webhookCreateOpts.delaySeconds) > 0 || webhookCreateOpts.deadLetterEnabled {
			req.RetryPolicy = &control.WebhookRetryPolicy{
				MaxAttempts:       webhookCreateOpts.maxAttempts,
				DelaySeconds:      webhookCreateOpts.delaySeconds,
				DeadLetterEnabled: webhookCreateOpts.deadLetterEnabled,
			}
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "create lifecycle webhook", Method: "POST", Path: "/api/v1/events/webhooks", Body: req})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		webhook, err := client.Control.CreateWebhook(cmd.Context(), req)
		if err != nil {
			return err
		}
		return printJSON(webhook)
	},
}

var sandboxWebhookUpdateCmd = &cobra.Command{
	Use:   "update <webhook-id>",
	Short: "Update a lifecycle webhook",
	Long:  "Update lifecycle webhook fields, events, signing secret, retry policy, and dead-letter URL. Run with --dry-run to inspect the PATCH body.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &control.LifecycleWebhookUpdateRequest{}
		if cmd.Flags().Changed("name") {
			req.Name = stringPtr(webhookUpdateOpts.name)
		}
		if cmd.Flags().Changed("url") {
			req.URL = stringPtr(webhookUpdateOpts.url)
		}
		if cmd.Flags().Changed("event") {
			req.Events = splitList(webhookUpdateOpts.events)
		}
		if cmd.Flags().Changed("secret") {
			req.SignatureSecret = stringPtr(webhookUpdateOpts.secret)
		}
		if cmd.Flags().Changed("enabled") {
			value, _, err := parseTriBool(webhookUpdateOpts.enabled, "enabled")
			if err != nil {
				return err
			}
			req.Enabled = &value
		}
		retryPolicy, err := buildWebhookUpdateRetryPolicy(cmd)
		if err != nil {
			return err
		}
		req.RetryPolicy = retryPolicy
		if cmd.Flags().Changed("dead-letter-url") {
			req.DeadLetterURL = stringPtr(webhookUpdateOpts.deadLetterURL)
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "update lifecycle webhook", Method: "PATCH", Path: "/api/v1/events/webhooks/" + args[0], Body: req})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		webhook, err := client.Control.UpdateWebhook(cmd.Context(), args[0], req)
		if err != nil {
			return err
		}
		return printJSON(webhook)
	},
}

var sandboxWebhookListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List lifecycle webhooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list lifecycle webhooks", Method: "GET", Path: "/api/v1/events/webhooks"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		webhooks, err := client.Control.ListWebhooks(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(webhooks)
	},
}

var sandboxWebhookGetCmd = &cobra.Command{
	Use:   "get <webhook-id>",
	Short: "Get a lifecycle webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get lifecycle webhook", Method: "GET", Path: "/api/v1/events/webhooks/" + args[0]})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		webhook, err := client.Control.GetWebhook(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(webhook)
	},
}

var sandboxWebhookDeleteCmd = &cobra.Command{
	Use:     "delete <webhook-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a lifecycle webhook",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "delete lifecycle webhook", Method: "DELETE", Path: "/api/v1/events/webhooks/" + args[0], IDs: args, Destructive: true})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		resp, err := client.Control.DeleteWebhook(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(resp)
	},
}

var webhookDeliveryOpts struct {
	webhookID string
	eventID   string
	status    string
	limit     int
	offset    int
}

var sandboxWebhookDeliveriesCmd = &cobra.Command{
	Use:   "deliveries",
	Short: "List webhook deliveries",
	Long:  "List webhook deliveries with --webhook-id, --event-id, --status, --limit, and --offset filters.",
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.ListWebhookDeliveriesParams{
			WebhookID: webhookDeliveryOpts.webhookID,
			EventID:   webhookDeliveryOpts.eventID,
			Status:    webhookDeliveryOpts.status,
			Limit:     webhookDeliveryOpts.limit,
			Offset:    webhookDeliveryOpts.offset,
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list webhook deliveries", Method: "GET", Path: "/api/v1/events/webhook-deliveries", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		deliveries, err := client.Control.ListWebhookDeliveries(cmd.Context(), params)
		if err != nil {
			return err
		}
		return printJSON(deliveries)
	},
}

var sandboxWebhookReplayCmd = &cobra.Command{
	Use:   "replay <delivery-id>",
	Short: "Replay a webhook delivery",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "replay webhook delivery", Method: "POST", Path: "/api/v1/events/webhook-deliveries/" + args[0] + "/replay"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		delivery, err := client.Control.ReplayWebhookDelivery(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(delivery)
	},
}

var sandboxTeamCmd = &cobra.Command{
	Use:   "team",
	Short: "Inspect teams and metrics",
	Long:  "Inspect teams and team-level metrics. Use --start and --end as Unix seconds to bound metric ranges.",
}

var sandboxTeamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "list teams", Method: "GET", Path: "/api/v1/teams"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		teams, err := client.Control.ListTeams(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(teams)
	},
}

var teamMetricsOpts struct {
	start  int64
	end    int64
	metric string
}

var sandboxTeamMetricsCmd = &cobra.Command{
	Use:   "metrics <team-id>",
	Short: "Show team metrics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.TeamMetricsParams{Start: teamMetricsOpts.start, End: teamMetricsOpts.end}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get team metrics", Method: "GET", Path: "/api/v1/teams/" + args[0] + "/metrics", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		metrics, err := client.Control.GetTeamMetrics(cmd.Context(), args[0], params)
		if err != nil {
			return err
		}
		return printJSON(metrics)
	},
}

var sandboxTeamMetricsMaxCmd = &cobra.Command{
	Use:   "metrics-max <team-id>",
	Short: "Show max team metric",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := &control.TeamMetricsMaxParams{
			Metric: teamMetricsOpts.metric,
			Start:  teamMetricsOpts.start,
			End:    teamMetricsOpts.end,
		}
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get max team metric", Method: "GET", Path: "/api/v1/teams/" + args[0] + "/metrics/max", Query: params})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		metric, err := client.Control.GetTeamMetricsMax(cmd.Context(), args[0], params)
		if err != nil {
			return err
		}
		return printJSON(metric)
	},
}

var sandboxObservabilityCmd = &cobra.Command{
	Use:   "observability",
	Short: "Show sandbox observability summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDryRun() {
			return printDryRunPlan(dryRunPlan{Action: "get sandbox observability summary", Method: "GET", Path: "/api/v1/observability/sandboxes"})
		}
		client, err := newSandboxClient()
		if err != nil {
			return err
		}
		summary, err := client.Control.GetObservabilitySummary(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(summary)
	},
}

func newSandboxClient() (*sandboxapi.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return nil, clierrors.ErrNotLoggedIn()
	}
	return sandboxapi.NewClientFromConfig(cfg, sandboxapi.Options{
		BaseURL:     sandboxOpts.baseURL,
		NamespaceID: firstNonEmpty(sandboxOpts.namespaceID, os.Getenv("SEACLOUD_NAMESPACE_ID")),
		UserID:      firstNonEmpty(sandboxOpts.userID, os.Getenv("SEACLOUD_USER_ID")),
		ProjectID:   firstNonEmpty(sandboxOpts.projectID, os.Getenv("SEACLOUD_PROJECT_ID")),
	})
}

func buildCreateSandboxRequest(args []string) (*control.NewSandboxRequest, error) {
	req := &control.NewSandboxRequest{
		Timeout:      int64PtrIfPositive(sandboxCreateOpts.timeout),
		AutoPause:    boolPtrIfTrue(sandboxCreateOpts.autoPause),
		AutoResume:   boolPtrIfTrue(sandboxCreateOpts.autoResume),
		Metadata:     parseKeyValues(sandboxCreateOpts.metadata),
		EnvVars:      parseKeyValues(sandboxCreateOpts.env),
		WaitReady:    boolPtrIfTrue(sandboxCreateOpts.waitReady),
		VolumeMounts: parseVolumeMounts(sandboxCreateOpts.volumeMounts),
	}
	if len(args) == 1 {
		req.TemplateID = args[0]
	}
	if value, ok, err := parseTriBool(sandboxCreateOpts.allowInternetAccess, "allow-internet-access"); err != nil {
		return nil, err
	} else if ok {
		req.AllowInternetAccess = &value
	}
	network := &control.SandboxNetworkPolicy{
		AllowOut: splitList(sandboxCreateOpts.allowOut),
		DenyOut:  splitList(sandboxCreateOpts.denyOut),
	}
	if value, ok, err := parseTriBool(sandboxCreateOpts.allowPublicTraffic, "allow-public-traffic"); err != nil {
		return nil, err
	} else if ok {
		network.AllowPublicTraffic = &value
	}
	if req.AllowInternetAccess != nil {
		network.AllowInternetAccess = req.AllowInternetAccess
	}
	if network.AllowPublicTraffic != nil || network.AllowInternetAccess != nil || len(network.AllowOut) > 0 || len(network.DenyOut) > 0 {
		req.Network = network
	}
	return req, nil
}

func shouldConnectAfterCreate(cmd *cobra.Command) bool {
	if sandboxCreateOpts.noConnect || IsDryRun() {
		return false
	}
	if sandboxOpts.output == "json" {
		return false
	}
	if sandboxCreateOpts.connect {
		return true
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func buildNetworkUpdateBody() (map[string]any, error) {
	body := map[string]any{}
	if value, ok, err := parseTriBool(sandboxNetworkOpts.allowPublicTraffic, "allow-public-traffic"); err != nil {
		return nil, err
	} else if ok {
		body["allowPublicTraffic"] = value
	}
	if value, ok, err := parseTriBool(sandboxNetworkOpts.allowInternetAccess, "allow-internet-access"); err != nil {
		return nil, err
	} else if ok {
		body["allowInternetAccess"] = value
	}
	if values := splitList(sandboxNetworkOpts.allowOut); len(values) > 0 {
		body["allowOut"] = values
	}
	if values := splitList(sandboxNetworkOpts.denyOut); len(values) > 0 {
		body["denyOut"] = values
	}
	return body, nil
}

func runSandboxCommand(ctx context.Context, runtime *runtimecmd.Service, command string, opts struct {
	background bool
	cwd        string
	user       string
	env        []string
	timeoutMS  int64
}) error {
	if opts.timeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
		defer cancel()
	}
	stdinOpen := true
	timeout := opts.timeoutMS
	process := &runtimecmd.ProcessStartRequest{
		Process: &runtimecmd.ProcessConfig{
			Cmd:  "sh",
			Args: []string{"-lc", command},
			Envs: parseKeyValues(opts.env),
			CWD:  stringPtr(opts.cwd),
		},
		Stdin: &stdinOpen,
	}
	if timeout > 0 {
		process.TimeoutMS = &timeout
	}
	requestOpts := &runtimecmd.RequestOptions{}
	if strings.TrimSpace(opts.user) != "" {
		requestOpts.Username = strings.TrimSpace(opts.user)
	}
	stream, err := runtime.Start(ctx, process, requestOpts)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("command timed out after %dms: %w", opts.timeoutMS, ctx.Err())
		}
		return err
	}
	defer stream.Close()
	start, err := waitProcessStart(stream)
	if err != nil {
		return err
	}
	if opts.background {
		fmt.Fprintf(os.Stderr, "PID: %d\n", start.PID)
		if start.CmdID != "" {
			fmt.Fprintf(os.Stderr, "CmdID: %s\n", start.CmdID)
		}
		return nil
	}
	if input, ok := readPipedStdin(); ok {
		if err := runtime.SendInput(ctx, &runtimecmd.SendInputRequest{
			Process: runtimecmd.ProcessSelector{PID: start.PID},
			Input:   runtimecmd.ProcessInput{Stdin: encodeStreamData(input)},
		}, nil); err != nil {
			return err
		}
		_ = runtime.CloseStdin(ctx, &runtimecmd.CloseStdinRequest{Process: runtimecmd.ProcessSelector{PID: start.PID}}, nil)
	}
	exitCode := 0
	for {
		frame, err := stream.Next()
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("command timed out after %dms: %w", opts.timeoutMS, ctx.Err())
			}
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if frame.Event.Data != nil {
			if frame.Event.Data.Stdout != "" {
				fmt.Fprint(os.Stdout, decodeStreamData(frame.Event.Data.Stdout))
			}
			if frame.Event.Data.Stderr != "" {
				fmt.Fprint(os.Stderr, decodeStreamData(frame.Event.Data.Stderr))
			}
		}
		if frame.Event.End != nil {
			if frame.Event.End.Status != "" {
				if parsed, err := strconv.Atoi(frame.Event.End.Status); err == nil {
					exitCode = parsed
				}
			}
			break
		}
	}
	if start.CmdID != "" {
		if result, err := runtime.GetResult(ctx, &runtimecmd.GetResultRequest{CmdID: start.CmdID}, nil); err == nil {
			exitCode = result.ExitCode
		}
	}
	if exitCode != 0 {
		return fmt.Errorf("command exited with code %d", exitCode)
	}
	return nil
}

func connectSandboxShell(ctx context.Context, client *sandboxapi.Client, sandboxID, shell string, timeout int64) error {
	if strings.TrimSpace(shell) == "" {
		shell = "sh"
	}
	_, runtime, err := client.ConnectRuntime(ctx, sandboxID, timeout)
	if err != nil {
		return err
	}
	cols, rows := terminalSize()
	stdinOpen := true
	stream, err := runtime.Start(ctx, &runtimecmd.ProcessStartRequest{
		Process: shellProcessConfig(shell),
		Stdin:   &stdinOpen,
		PTY:     &runtimecmd.PtyConfig{Size: runtimecmd.PtySize{Cols: cols, Rows: rows}},
	}, nil)
	if err != nil {
		return err
	}
	defer stream.Close()
	start, err := waitProcessStart(stream)
	if err != nil {
		return err
	}
	restore, err := makeRawTerminal()
	if err == nil {
		defer restore()
	}
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := os.Stdin.Read(buf)
			if n > 0 {
				if err := runtime.SendInput(ctx, &runtimecmd.SendInputRequest{
					Process: runtimecmd.ProcessSelector{PID: start.PID},
					Input:   runtimecmd.ProcessInput{PTY: encodeStreamData(string(buf[:n]))},
				}, nil); err != nil {
					done <- err
					return
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					done <- nil
					return
				}
				done <- readErr
				return
			}
		}
	}()
	for {
		select {
		case err := <-done:
			return err
		default:
		}
		frame, err := stream.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if frame.Event.Data != nil {
			if frame.Event.Data.PTY != "" {
				fmt.Fprint(os.Stdout, decodeStreamData(frame.Event.Data.PTY))
			} else {
				fmt.Fprint(os.Stdout, decodeStreamData(frame.Event.Data.Stdout))
				fmt.Fprint(os.Stderr, decodeStreamData(frame.Event.Data.Stderr))
			}
		}
		if frame.Event.End != nil {
			return nil
		}
	}
}

func waitProcessStart(stream *runtimecmd.ProcessStream) (*runtimecmd.ProcessStartEvent, error) {
	for {
		frame, err := stream.Next()
		if err != nil {
			return nil, err
		}
		if frame.Event.Start != nil {
			return frame.Event.Start, nil
		}
	}
}

func shellProcessConfig(shell string) *runtimecmd.ProcessConfig {
	if strings.ContainsAny(shell, " \t") {
		return &runtimecmd.ProcessConfig{Cmd: "sh", Args: []string{"-lc", shell}}
	}
	return &runtimecmd.ProcessConfig{Cmd: shell}
}

func buildWebhookUpdateRetryPolicy(cmd *cobra.Command) (*control.WebhookRetryPolicy, error) {
	if !cmd.Flags().Changed("max-attempts") && !cmd.Flags().Changed("delay-seconds") && !cmd.Flags().Changed("dead-letter-enabled") {
		return nil, nil
	}
	policy := &control.WebhookRetryPolicy{
		MaxAttempts:  webhookUpdateOpts.maxAttempts,
		DelaySeconds: webhookUpdateOpts.delaySeconds,
	}
	if cmd.Flags().Changed("dead-letter-enabled") {
		value, _, err := parseTriBool(webhookUpdateOpts.deadLetterEnabled, "dead-letter-enabled")
		if err != nil {
			return nil, err
		}
		policy.DeadLetterEnabled = value
	}
	return policy, nil
}

func printSandbox(s *control.Sandbox, output string) error {
	if output == "json" {
		return printJSON(s)
	}
	fmt.Printf("Sandbox %s\n", s.SandboxID)
	fmt.Printf("  Template: %s\n", s.TemplateID)
	fmt.Printf("  State:    %s\n", firstNonEmpty(s.State, s.Status))
	if s.EnvdURL != nil && *s.EnvdURL != "" {
		fmt.Printf("  Runtime:  %s\n", *s.EnvdURL)
	}
	return nil
}

func parseKeyValues(values []string) map[string]string {
	out := map[string]string{}
	for _, raw := range splitList(values) {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseVolumeMounts(values []string) []control.VolumeMount {
	var out []control.VolumeMount
	for _, raw := range splitList(values) {
		name, path, ok := strings.Cut(raw, ":")
		if !ok {
			name, path, ok = strings.Cut(raw, "=")
		}
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		path = strings.TrimSpace(path)
		if name != "" && path != "" {
			out = append(out, control.VolumeMount{Name: name, Path: path})
		}
	}
	return out
}

func splitList(values []string) []string {
	var out []string
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			if value := strings.TrimSpace(part); value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

func parseTriBool(raw, name string) (bool, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false, cliParamError("--"+name, "must be true or false", "Use true or false, for example: --"+name+"=false")
	}
	return value, true, nil
}

func readPipedStdin() (string, bool) {
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		return "", false
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}

func makeRawTerminal() (func(), error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return func() { _ = term.Restore(fd, oldState) }, nil
}

func terminalSize() (int, int) {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 || rows <= 0 {
		return 80, 24
	}
	return cols, rows
}

func encodeStreamData(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func decodeStreamData(data string) string {
	if strings.TrimSpace(data) == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return data
	}
	return string(decoded)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func formatFloatPtr(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(*value, 'f', 2, 64)
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func int64PtrIfPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func boolPtrIfTrue(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

func init() {
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.baseURL, "base-url", "", "Sandbox API base URL (default: https://cloud.seaart.ai/api/sandbox/v1)")
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.namespaceID, "namespace", "", "Sandbox namespace ID")
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.userID, "user-id", "", "User ID header for sandbox APIs")
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.projectID, "project-id", "", "Project/team ID header for sandbox APIs")
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.output, "output", "", "Output format: json or table (default table)")
	sandboxCmd.PersistentFlags().StringVar(&sandboxOpts.output, "format", "", "Output format alias for E2B compatibility: json, table, or pretty")

	sandboxCreateCmd.Flags().Int64Var(&sandboxCreateOpts.timeout, "timeout", 0, "Sandbox timeout in seconds")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.waitReady, "wait", false, "Wait until the sandbox is ready")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.autoPause, "auto-pause", false, "Pause instead of kill on timeout")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.autoResume, "auto-resume", false, "Allow router-triggered auto resume")
	sandboxCreateCmd.Flags().StringArrayVar(&sandboxCreateOpts.metadata, "metadata", nil, "Metadata key=value pairs, repeatable or comma-separated")
	sandboxCreateCmd.Flags().StringArrayVar(&sandboxCreateOpts.env, "env", nil, "Environment variable key=value pairs, repeatable or comma-separated")
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOpts.allowPublicTraffic, "allow-public-traffic", "", "Allow public inbound traffic: true or false")
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOpts.allowInternetAccess, "allow-internet-access", "", "Allow public internet egress: true or false")
	sandboxCreateCmd.Flags().StringArrayVar(&sandboxCreateOpts.allowOut, "allow-out", nil, "Egress allowlist IPv4/CIDR, repeatable or comma-separated")
	sandboxCreateCmd.Flags().StringArrayVar(&sandboxCreateOpts.denyOut, "deny-out", nil, "Egress denylist IPv4/CIDR, repeatable or comma-separated")
	sandboxCreateCmd.Flags().StringArrayVar(&sandboxCreateOpts.volumeMounts, "volume-mount", nil, "Volume mount name:path, repeatable")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.connect, "connect", false, "Open an interactive shell after creating")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.noConnect, "no-connect", false, "Create the sandbox without opening an interactive shell")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateOpts.killOnExit, "kill-on-exit", false, "Kill the sandbox when the connected shell exits; defaults to true for create+connect")
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOpts.shell, "shell", "sh", "Shell for --connect")
	sandboxCreateCmd.Flags().Int64Var(&sandboxCreateOpts.connectTimeout, "connect-timeout", 0, "Connect timeout in seconds")

	sandboxListCmd.Flags().StringArrayVar(&sandboxListOpts.state, "state", nil, "State filter, repeatable or comma-separated")
	sandboxListCmd.Flags().StringArrayVar(&sandboxListOpts.metadata, "metadata", nil, "Metadata filter key=value, repeatable or comma-separated")
	sandboxListCmd.Flags().IntVar(&sandboxListOpts.limit, "limit", 0, "Maximum number of sandboxes")
	sandboxListCmd.Flags().StringVar(&sandboxListOpts.nextToken, "next-token", "", "Pagination next token")

	sandboxKillCmd.Flags().BoolVar(&sandboxKillOpts.all, "all", false, "Kill all matching sandboxes")
	sandboxKillCmd.Flags().StringArrayVar(&sandboxKillOpts.state, "state", nil, "State filter for --all")
	sandboxKillCmd.Flags().StringArrayVar(&sandboxKillOpts.metadata, "metadata", nil, "Metadata filter for --all")

	sandboxExecCmd.Flags().BoolVar(&sandboxExecOpts.background, "background", false, "Run in background and print PID")
	sandboxExecCmd.Flags().StringVar(&sandboxExecOpts.cwd, "cwd", "", "Working directory")
	sandboxExecCmd.Flags().StringVar(&sandboxExecOpts.user, "user", "", "User to run as")
	sandboxExecCmd.Flags().StringArrayVar(&sandboxExecOpts.env, "env", nil, "Environment variable key=value, repeatable")
	sandboxExecCmd.Flags().Int64Var(&sandboxExecOpts.timeoutMS, "timeout-ms", 0, "Command timeout in milliseconds")

	sandboxConnectCmd.Flags().Int64Var(&sandboxConnectOpts.timeout, "timeout", 0, "Connect timeout in seconds")
	sandboxConnectCmd.Flags().StringVar(&sandboxConnectOpts.shell, "shell", "sh", "Shell to start")

	sandboxMetricsCmd.Flags().IntVar(&sandboxMetricsLimit, "limit", 0, "Maximum metric snapshots for batch metrics")

	sandboxLogsCmd.Flags().IntVar(&sandboxLogsOpts.limit, "limit", 0, "Log line limit")
	sandboxLogsCmd.Flags().Int64Var(&sandboxLogsOpts.cursor, "cursor", 0, "Log cursor")
	sandboxLogsCmd.Flags().StringVar(&sandboxLogsOpts.direction, "direction", "", "Log direction: forward or backward")
	sandboxLogsCmd.Flags().StringVar(&sandboxLogsOpts.level, "level", "", "Minimum log level")
	sandboxLogsCmd.Flags().StringVar(&sandboxLogsOpts.search, "search", "", "Search text")

	sandboxTimeoutCmd.Flags().Int64Var(&sandboxTimeoutSeconds, "seconds", 3600, "Timeout in seconds")
	sandboxRefreshCmd.Flags().Int32Var(&sandboxRefreshDuration, "duration", 0, "Refresh duration in seconds")

	sandboxNetworkUpdateCmd.Flags().StringVar(&sandboxNetworkOpts.allowPublicTraffic, "allow-public-traffic", "", "Allow public inbound traffic: true or false")
	sandboxNetworkUpdateCmd.Flags().StringVar(&sandboxNetworkOpts.allowInternetAccess, "allow-internet-access", "", "Allow public internet egress: true or false")
	sandboxNetworkUpdateCmd.Flags().StringArrayVar(&sandboxNetworkOpts.allowOut, "allow-out", nil, "Egress allowlist IPv4/CIDR")
	sandboxNetworkUpdateCmd.Flags().StringArrayVar(&sandboxNetworkOpts.denyOut, "deny-out", nil, "Egress denylist IPv4/CIDR")

	sandboxEventsCmd.Flags().IntVar(&sandboxEventsOpts.limit, "limit", 10, "Event limit")
	sandboxEventsCmd.Flags().IntVar(&sandboxEventsOpts.offset, "offset", 0, "Event offset")
	sandboxEventsCmd.Flags().BoolVar(&sandboxEventsOpts.orderAsc, "order-asc", false, "Order ascending")
	sandboxEventsCmd.Flags().StringArrayVar(&sandboxEventsOpts.types, "type", nil, "Event type, repeatable or comma-separated")

	sandboxWebhookCreateCmd.Flags().StringVar(&webhookCreateOpts.name, "name", "", "Webhook name")
	sandboxWebhookCreateCmd.Flags().StringVar(&webhookCreateOpts.url, "url", "", "Webhook URL")
	sandboxWebhookCreateCmd.Flags().StringArrayVar(&webhookCreateOpts.events, "event", nil, "Lifecycle event type, repeatable")
	sandboxWebhookCreateCmd.Flags().StringVar(&webhookCreateOpts.secret, "secret", "", "Signature secret")
	sandboxWebhookCreateCmd.Flags().BoolVar(&webhookCreateOpts.enabled, "enabled", true, "Enable webhook")
	sandboxWebhookCreateCmd.Flags().IntVar(&webhookCreateOpts.maxAttempts, "max-attempts", 0, "Maximum delivery attempts")
	sandboxWebhookCreateCmd.Flags().IntSliceVar(&webhookCreateOpts.delaySeconds, "delay-seconds", nil, "Retry delays in seconds")
	sandboxWebhookCreateCmd.Flags().BoolVar(&webhookCreateOpts.deadLetterEnabled, "dead-letter-enabled", false, "Enable dead-letter delivery")
	sandboxWebhookCreateCmd.Flags().StringVar(&webhookCreateOpts.deadLetterURL, "dead-letter-url", "", "Dead-letter URL")

	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.name, "name", "", "Webhook name")
	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.url, "url", "", "Webhook URL")
	sandboxWebhookUpdateCmd.Flags().StringArrayVar(&webhookUpdateOpts.events, "event", nil, "Lifecycle event type, repeatable")
	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.secret, "secret", "", "Signature secret")
	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.enabled, "enabled", "", "Enable webhook: true or false")
	sandboxWebhookUpdateCmd.Flags().IntVar(&webhookUpdateOpts.maxAttempts, "max-attempts", 0, "Maximum delivery attempts")
	sandboxWebhookUpdateCmd.Flags().IntSliceVar(&webhookUpdateOpts.delaySeconds, "delay-seconds", nil, "Retry delays in seconds")
	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.deadLetterEnabled, "dead-letter-enabled", "", "Enable dead-letter delivery: true or false")
	sandboxWebhookUpdateCmd.Flags().StringVar(&webhookUpdateOpts.deadLetterURL, "dead-letter-url", "", "Dead-letter URL")

	sandboxWebhookDeliveriesCmd.Flags().StringVar(&webhookDeliveryOpts.webhookID, "webhook-id", "", "Filter by webhook ID")
	sandboxWebhookDeliveriesCmd.Flags().StringVar(&webhookDeliveryOpts.eventID, "event-id", "", "Filter by event ID")
	sandboxWebhookDeliveriesCmd.Flags().StringVar(&webhookDeliveryOpts.status, "status", "", "Filter by delivery status")
	sandboxWebhookDeliveriesCmd.Flags().IntVar(&webhookDeliveryOpts.limit, "limit", 10, "Delivery limit")
	sandboxWebhookDeliveriesCmd.Flags().IntVar(&webhookDeliveryOpts.offset, "offset", 0, "Delivery offset")

	sandboxTeamMetricsCmd.Flags().Int64Var(&teamMetricsOpts.start, "start", 0, "Start Unix seconds")
	sandboxTeamMetricsCmd.Flags().Int64Var(&teamMetricsOpts.end, "end", 0, "End Unix seconds")
	sandboxTeamMetricsMaxCmd.Flags().StringVar(&teamMetricsOpts.metric, "metric", "concurrent_sandboxes", "Metric: concurrent_sandboxes or sandbox_start_rate")
	sandboxTeamMetricsMaxCmd.Flags().Int64Var(&teamMetricsOpts.start, "start", 0, "Start Unix seconds")
	sandboxTeamMetricsMaxCmd.Flags().Int64Var(&teamMetricsOpts.end, "end", 0, "End Unix seconds")

	sandboxNetworkCmd.AddCommand(sandboxNetworkUpdateCmd)
	sandboxVolumeCmd.AddCommand(sandboxVolumeListCmd, sandboxVolumeCreateCmd, sandboxVolumeGetCmd, sandboxVolumeDeleteCmd)
	sandboxWebhookCmd.AddCommand(sandboxWebhookListCmd, sandboxWebhookCreateCmd, sandboxWebhookGetCmd, sandboxWebhookUpdateCmd, sandboxWebhookDeleteCmd, sandboxWebhookDeliveriesCmd, sandboxWebhookReplayCmd)
	sandboxTeamCmd.AddCommand(sandboxTeamListCmd, sandboxTeamMetricsCmd, sandboxTeamMetricsMaxCmd)
	sandboxCmd.AddCommand(
		sandboxCreateCmd,
		sandboxListCmd,
		sandboxInfoCmd,
		sandboxKillCmd,
		sandboxExecCmd,
		sandboxConnectCmd,
		sandboxMetricsCmd,
		sandboxLogsCmd,
		sandboxPauseCmd,
		sandboxTimeoutCmd,
		sandboxRefreshCmd,
		sandboxNetworkCmd,
		sandboxVolumeCmd,
		sandboxEventsCmd,
		sandboxWebhookCmd,
		sandboxTeamCmd,
		sandboxObservabilityCmd,
	)
	rootCmd.AddCommand(sandboxCmd)
}
