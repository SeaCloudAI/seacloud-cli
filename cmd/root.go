package cmd

import (
	"os"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

var dryRun bool

var rootCmd = &cobra.Command{
	Use:     "seacloud",
	Short:   "SeaCloud CLI - Access multimodal AI with a single API Key",
	Long:    "SeaCloud CLI lets you manage your account, browse models, run multimodal AI services, and manage sandbox workloads via API Key.",
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
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview the operation without credentials or state changes when the command supports it")
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(agentCmd)
}
