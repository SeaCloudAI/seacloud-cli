package cmd

import (
	"errors"
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
	"github.com/spf13/cobra"
)

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Run LLM models through LLM-only commands",
	Long: `Call LLM contract models only.

Use this command group when the selected model must be an LLM contract. For
image, video, audio, 3D, queue, or legacy generation models, use "seacloud run".`,
}

var llmRunCmd = &cobra.Command{
	Use:   "run <model_id>",
	Short: "Run an LLM model and print text, JSON, or SSE",
	Long: `Run an LLM model from a model-contract.v1 LLM contract.

Only LLM contracts are accepted:
  - llm_chat_completions with openai_chat_json
  - llm_responses with openai_responses_json

Parameters are passed as --param key=value pairs (repeatable), using the
input_schema returned by "seacloud models spec <model_id>". The model field is
controlled by the model contract and cannot be supplied as a parameter.`,
	Example: `  seacloud llm run gpt_4o_mini --param messages='[{"role":"user","content":"hello"}]'
  seacloud llm run gpt_4o_mini --stream --param messages='[{"role":"user","content":"hello"}]'
  seacloud llm run gpt_5_mini --param input=hello --output json
  seacloud --dry-run llm run gpt_5_mini --stream --param input=hello`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeLLMRun(args[0])
	},
}

func executeLLMRun(modelID string) error {
	raw, err := generation.ParseParams(runParams)
	if err != nil {
		return err
	}
	if IsDryRun() {
		return dryRunLLMRun(modelID, raw)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	contract, err := getLLMContract(modelID, cfg.AuthToken)
	if err != nil {
		if errors.Is(err, contracts.ErrNotFound) {
			return executeLLMFallback(cfg.APIKey, modelID, raw)
		}
		return err
	}
	if cfg.APIKey == "" {
		return clierrors.ErrNoAPIKey()
	}
	return runLLMContract(cfg.APIKey, modelID, contract, raw)
}

func dryRunLLMRun(modelID string, raw map[string]string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	contract, err := getLLMContract(modelID, cfg.AuthToken)
	if err != nil {
		if errors.Is(err, contracts.ErrNotFound) {
			return executeLLMFallback(cfg.APIKey, modelID, raw)
		}
		return err
	}
	return dryRunLLMContract(modelID, contract, raw)
}

func getLLMContract(modelID, authToken string) (*contracts.ModelContract, error) {
	contract, err := contracts.Get(modelID, contracts.Options{Refresh: runRefresh})
	if errors.Is(err, contracts.ErrNotFound) {
		return nil, contracts.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model contract for %q: %w", modelID, err)
	}
	if !isLLMContract(contract) {
		return nil, &clierrors.CLIError{
			Message: fmt.Sprintf("model %s is not an LLM model", modelID),
			Hint:    fmt.Sprintf("Use: seacloud run %s", modelID),
		}
	}
	return contract, nil
}

func init() {
	llmRunCmd.Flags().StringArrayVar(&runParams, "param", nil, "Parameter as key=value (repeatable)")
	llmRunCmd.Flags().StringVar(&runOutput, "output", "", "Output format: json (full response), sse (raw LLM stream)")
	llmRunCmd.Flags().IntVar(&runTimeout, "timeout", 600, "Maximum seconds to wait for result (default 10 minutes)")
	llmRunCmd.Flags().BoolVar(&runRefresh, "refresh", false, "Refresh cached model contract before running")
	llmRunCmd.Flags().BoolVar(&runStream, "stream", false, "Stream LLM output as it is generated")
	llmRunCmd.Flags().BoolVar(&runUseSkillModelFallback, "use-skill-model-fallback", false, "Use skill models reference curl as a CLI-managed fallback when no LLM contract exists")
	llmRunCmd.Flags().BoolVar(&runUseReferenceCurl, "use-reference-curl", false, "Execute the skill models reference curl internally as a last-resort fallback")

	llmCmd.AddCommand(llmRunCmd)
}
