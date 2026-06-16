package cmd

import (
	"github.com/SeaCloudAI/seacloud-cli/internal/skillhub"
	"github.com/spf13/cobra"
)

var (
	listCategory string
	listSort     string
	listOutput   string
)

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	Long:  "List all available skills from SkillHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOutputFormat("--output", listOutput, "json"); err != nil {
			return err
		}
		client := skillhub.NewClient()
		return client.List(listCategory, listSort, listOutput)
	},
}

func init() {
	skillsListCmd.Flags().StringVarP(&listCategory, "category", "c", "", "Filter by category")
	skillsListCmd.Flags().StringVarP(&listSort, "sort", "s", "", "Sort by (stars, downloads, updated)")
	skillsListCmd.Flags().StringVar(&listOutput, "output", "", "Output format: json")
}
