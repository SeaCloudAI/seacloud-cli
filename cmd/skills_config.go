package cmd

import (
	"github.com/SeaCloudAI/seacloud-cli/internal/skillhub"
	"github.com/spf13/cobra"
)

var (
	configSetURL string
	configShow   bool
)

var skillsConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure SkillHub CLI",
	Long:  "Configure SkillHub API URL and other settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := skillhub.NewClient()
		return client.Config(configSetURL, configShow)
	},
}

func init() {
	skillsConfigCmd.Flags().StringVar(&configSetURL, "set-url", "", "Set API base URL")
	skillsConfigCmd.Flags().BoolVar(&configShow, "show", false, "Show current configuration")
}
