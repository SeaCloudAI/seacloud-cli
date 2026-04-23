package cmd

import (
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildinfo.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
