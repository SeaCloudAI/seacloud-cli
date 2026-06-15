package cmd

import (
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/agentdescribe"
	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

var agentDescribeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Print current CLI capabilities for agents",
	Long:  "Print a deterministic Markdown guide describing the current CLI capabilities and recommended agent workflows.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		desc := agentdescribe.Build(buildinfo.Version)
		_, err := fmt.Fprint(cmd.OutOrStdout(), agentdescribe.RenderMarkdown(desc))
		return err
	},
}
