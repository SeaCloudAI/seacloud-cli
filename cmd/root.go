package cmd

import (
	"os"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

var dryRun bool

var rootCmd = &cobra.Command{
	Use:   "seacloud",
	Short: "SeaCloud CLI - Access multimodal AI and sandbox workloads",
	Long: `SeaCloud CLI is an agent-facing multimodal execution CLI. Use it when an AI agent needs SeaCloud authentication to discover and call LLM, image, video, audio, or 3D models, manage sandbox workloads, inspect model specs, run generation tasks, track outputs, check balance, or find SkillHub skills without hand-writing provider-specific API calls.

Best-fit tasks:
  - generate or edit images, videos, audio, speech, music, or 3D assets
  - manage sandbox and template workloads
  - choose a model when the user describes a multimodal task but no model ID is known
  - inspect required parameters before calling a model
  - submit a generation task and return result URLs or JSON
  - find or install a task skill from SkillHub
  - diagnose authentication, balance, task status, or output issues

Agent path:
  seacloud auth status
  seacloud account balance --output json
  seacloud models list --output json
  seacloud models spec <model_id> --output json
  seacloud llm run <model_id> --param key=value --output json
  seacloud run <model_id> --param key=value --output json
  seacloud run-async <model_id> --param key=value
  seacloud sandbox --help
  seacloud template --help
  seacloud task status <task_id> --output json
  seacloud skills find "<task>"`,
	Version: buildinfo.Version,
}

// IsDryRun returns true if --dry-run was passed. Subcommands should check
// this before performing any network or state-mutating operations.
func IsDryRun() bool { return dryRun }

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print what would be executed without making changes")
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(accountCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(llmCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(runAsyncCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(agentCmd)
}
