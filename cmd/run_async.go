package cmd

import (
	"errors"
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
	"github.com/spf13/cobra"
)

var runAsyncOutput string

type asyncSubmission struct {
	TaskID   string `json:"task_id"`
	ModelID  string `json:"model_id"`
	Status   string `json:"status"`
	Protocol string `json:"protocol"`
	Next     string `json:"next"`
}

var runAsyncCmd = &cobra.Command{
	Use:   "run-async <model_id>",
	Short: "Submit a model task asynchronously and print the task ID",
	Long: `Submit a generation request and return immediately after the task is accepted.

Use this when an agent or script should not wait for result URLs. Follow up with
seacloud task status <task_id> --output json.`,
	Example: `  seacloud run-async seedance_2_0 --param prompt="a cat running"
  seacloud run-async gpt_image_2 --param prompt="a blue cat" --output id`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if runAsyncOutput == "url" {
			return fmt.Errorf("--output url is not supported for run-async; use seacloud task status <task_id> --output url after the task completes")
		}
		if runAsyncOutput != "" && runAsyncOutput != "json" && runAsyncOutput != "id" {
			return fmt.Errorf("unsupported output format for run-async: %s", runAsyncOutput)
		}
		modelID := args[0]
		return executeModelRunAsync(modelID, models.ResolveModelID(modelID))
	},
}

func init() {
	runAsyncCmd.Flags().StringArrayVar(&runParams, "param", nil, "Parameter as key=value (repeatable)")
	runAsyncCmd.Flags().StringVar(&runAsyncOutput, "output", "", "Output format: json (default), id")
	runAsyncCmd.Flags().BoolVar(&runRefresh, "refresh", false, "Refresh cached model contract before running")
}

func executeModelRunAsync(modelID, resolvedModelID string) error {
	raw, err := generation.ParseParams(runParams)
	if err != nil {
		return err
	}
	if IsDryRun() {
		return dryRunModel(modelID, raw)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return clierrors.ErrNoAPIKey()
	}

	contract, err := contracts.Get(modelID, contracts.Options{Refresh: runRefresh})
	if err == nil {
		return runWithContractAsync(cfg.APIKey, modelID, resolvedModelID, contract, raw)
	}
	if errors.Is(err, contracts.ErrIncompatibleSchema) {
		return err
	}
	if !errors.Is(err, contracts.ErrNotFound) {
		return fmt.Errorf("failed to fetch model contract for %q: %w", modelID, err)
	}
	return runWithContractAsync(cfg.APIKey, modelID, resolvedModelID, contracts.Generic(modelID), raw)
}

func runWithContractAsync(apiKey, modelID, resolvedModelID string, contract *contracts.ModelContract, raw map[string]string) error {
	switch {
	case contract.Protocol == "queue" && contract.BodyMode == "raw_json":
		params, err := queueParamsFromContract(modelID, contract, raw)
		if err != nil {
			return err
		}
		client := queue.NewClient(apiKey)
		submitted, err := client.Submit(*contract, params)
		if err != nil {
			return clierrors.ErrSubmitFailed(err)
		}
		saveQueueSubmission(contract, submitted.ID)
		return printAsyncSubmission(asyncSubmission{
			TaskID:   submitted.ID,
			ModelID:  asyncModelID(modelID, contract.ModelID),
			Status:   "submitted",
			Protocol: "queue",
			Next:     nextTaskStatusCommand(submitted.ID),
		})
	case contract.Protocol == "generation" || contract.BodyMode == "generation_wrapper":
		resp, _, err := submitLegacyGeneration(apiKey, modelID, resolvedModelID, raw)
		if err != nil {
			return clierrors.ErrSubmitFailed(err)
		}
		return printAsyncSubmission(asyncSubmission{
			TaskID:   resp.ID,
			ModelID:  modelID,
			Status:   "submitted",
			Protocol: "generation",
			Next:     nextTaskStatusCommand(resp.ID),
		})
	default:
		return fmt.Errorf("unsupported model contract protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
}

func asyncModelID(fallback, contractModelID string) string {
	if contractModelID != "" {
		return contractModelID
	}
	return fallback
}

func nextTaskStatusCommand(taskID string) string {
	return "seacloud task status " + taskID + " --output json"
}

func printAsyncSubmission(submission asyncSubmission) error {
	if runAsyncOutput == "id" {
		fmt.Println(submission.TaskID)
		return nil
	}
	return printJSON(submission)
}
