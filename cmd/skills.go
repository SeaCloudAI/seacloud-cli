package cmd

import (
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Search, install, and manage SkillHub skills",
	Long: `Search, install, and manage agent skills from SeaCloud SkillHub.

Use SkillHub when the user task needs a specialized workflow the current agent does not already have, such as creative production, media processing, document extraction, or model orchestration.`,
}

func init() {
	skillsCmd.AddCommand(skillsFindCmd)
	skillsCmd.AddCommand(skillsAddCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsConfigCmd)
}
