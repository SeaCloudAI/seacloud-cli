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
	Long:    "SeaCloud CLI lets you manage your account, browse models, and call multimodal AI services via API Key.",
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
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print what would be executed without making any changes")
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(skillsCmd)
}
