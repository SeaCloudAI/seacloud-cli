package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Browse available models",
}

var (
	modelsListType     string
	modelsListKeywords string
	modelsListPage     int
	modelsListPageSize int
	modelsListOutput   string
)

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	Long: `List models available on seacloud.

Output fields (--output json):
  id                 Model identifier, use this as <model_id> in "seacloud models spec <model_id>"
  name               Human-readable model name
  type               Model type: Video | Image | Audio
  description        What the model does
  input_modalities   Accepted input types: Text | Image | Video | Audio
  output_modalities  Output types produced by the model

Pagination fields:
  total              Total number of matching models
  page               Current page
  page_size          Results per page
  total_pages        Total number of pages`,
	Example: `  seacloud models list
  seacloud models list --type video
  seacloud models list --keywords kirin
  seacloud models list --output id
  seacloud models list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := models.List(models.ListParams{
			Page:     modelsListPage,
			PageSize: modelsListPageSize,
			Type:     modelsListType,
			Keywords: modelsListKeywords,
		})
		if err != nil {
			return err
		}

		if modelsListOutput == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		if modelsListOutput == "id" {
			for _, m := range result.Models {
				fmt.Println(m.ID)
			}
			return nil
		}

		if len(result.Models) == 0 {
			fmt.Println("No models found.")
			return nil
		}

		fmt.Printf("Showing %d of %d models (page %d/%d)\n\n",
			len(result.Models), result.Total, result.Page, result.TotalPages)

		for _, m := range result.Models {
			fmt.Printf("%-30s  %-8s  %s\n", m.ID, m.Type, m.Name)
			if m.Description != "" {
				fmt.Printf("  %s\n", truncate(m.Description, 80))
			}
			fmt.Printf("  Input: %s  ->  Output: %s\n\n",
				strings.Join(m.InputModalities, ", "),
				strings.Join(m.OutputModalities, ", "),
			)
		}
		return nil
	},
}

var modelsSpecOutput string

var modelsSpecCmd = &cobra.Command{
	Use:   "spec <model_id>",
	Short: "Get full parameter spec for a model",
	Long: `Get the queue contract for a model.

Default output is a concise queue contract summary containing:
  - protocol and body mode
  - submit endpoint
  - how to pass parameters with --param

Use --output json to get the raw model-contract.v1 structure. If the model is
listed but has no published detailed contract, the CLI returns a generic queue
contract that accepts raw --param key=value fields.`,
	Example: `  seacloud models spec kling_v2_6_i2v
  seacloud models spec seedance_2_0 --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelID := args[0]

		return printModelContractSpec(modelID)
	},
}

func printModelContractSpec(modelID string) error {
	contract, err := contracts.Get(modelID, contracts.Options{})
	if errors.Is(err, contracts.ErrNotFound) {
		contract = contracts.Generic(modelID)
	} else if err != nil {
		return err
	}
	if modelsSpecOutput == "json" {
		b, _ := json.MarshalIndent(contract, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("Model: %s\n", contract.ModelID)
	fmt.Printf("Protocol: %s\n", contract.Protocol)
	fmt.Printf("Body mode: %s\n", contract.BodyMode)
	fmt.Printf("Submit: %s %s\n", contract.Endpoints.Submit.Method, contract.Endpoints.Submit.Path)
	fmt.Println("Parameters: pass raw JSON fields with --param key=value")
	return nil
}

func buildModelsQuery() string {
	q := fmt.Sprintf("?page=%d&page_size=%d", modelsListPage, modelsListPageSize)
	if modelsListType != "" {
		q += "&type=" + modelsListType
	}
	if modelsListKeywords != "" {
		q += "&keywords=" + modelsListKeywords
	}
	return q
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	modelsListCmd.Flags().StringVar(&modelsListType, "type", "", "Filter by type (video, image, audio)")
	modelsListCmd.Flags().StringVar(&modelsListKeywords, "keywords", "", "Search by keyword")
	modelsListCmd.Flags().IntVar(&modelsListPage, "page", 1, "Page number")
	modelsListCmd.Flags().IntVar(&modelsListPageSize, "page-size", 20, "Results per page")
	modelsListCmd.Flags().StringVar(&modelsListOutput, "output", "", "Output format: id (IDs only), json (full response)")

	modelsSpecCmd.Flags().StringVar(&modelsSpecOutput, "output", "", "Output format (json)")

	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsSpecCmd)
}
