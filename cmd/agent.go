package cmd

import "github.com/spf13/cobra"

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Describe SeaCloud CLI capabilities for agents",
	Long:  "Describe SeaCloud CLI capabilities and recommended workflows for AI agents.",
}

func init() {
	agentCmd.AddCommand(agentDescribeCmd)
}
