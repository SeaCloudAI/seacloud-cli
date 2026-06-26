package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
	"github.com/SeaCloudAI/seacloud-cli/internal/llm"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
)

func executeModelRun(modelID, resolvedModelID string) error {
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
		return runWithContract(cfg.APIKey, cfg.AuthToken, modelID, contract, raw)
	}
	if errors.Is(err, contracts.ErrIncompatibleSchema) {
		return err
	}
	if !errors.Is(err, contracts.ErrNotFound) {
		return fmt.Errorf("failed to fetch model contract for %q: %w", modelID, err)
	}
	return runWithContract(cfg.APIKey, cfg.AuthToken, modelID, contracts.Generic(modelID), raw)
}

func dryRunModel(modelID string, raw map[string]string) error {
	contract, err := contracts.Get(modelID, contracts.Options{Refresh: runRefresh})
	if err == nil {
		return dryRunContract(modelID, contract, raw)
	}
	if errors.Is(err, contracts.ErrIncompatibleSchema) {
		return err
	}
	return dryRunContract(modelID, contracts.Generic(modelID), raw)
}

func dryRunContract(modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	if isLLMContract(contract) {
		return dryRunLLMContract(modelID, contract, raw)
	}
	raw = fillRawPrerequisitesFromCache(raw, contract.Prerequisites)
	params, err := contracts.ValidateAndCoerce(modelID, raw, contract.InputSchema)
	if err != nil {
		return err
	}
	if err := contracts.ValidatePrerequisites(modelID, params, contract.Prerequisites); err != nil {
		return err
	}
	if err := contracts.ValidateInputRules(modelID, params, contract.InputRules); err != nil {
		return err
	}
	body, _ := json.Marshal(params)
	fmt.Fprintf(os.Stderr, "[dry-run] protocol=%s\n", contract.Protocol)
	fmt.Fprintf(os.Stderr, "[dry-run] body_mode=%s\n", contract.BodyMode)
	fmt.Fprintf(os.Stderr, "[dry-run] submit=%s %s\n", contract.Endpoints.Submit.Method, contract.Endpoints.Submit.Path)
	fmt.Fprintf(os.Stderr, "[dry-run] body=%s\n", string(body))
	return nil
}

func runWithContract(apiKey, authToken, modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	switch {
	case contract.Protocol == "queue" && contract.BodyMode == "raw_json":
		return runQueueContractWithLocalFiles(apiKey, authToken, modelID, contract, raw)
	case isLLMContract(contract):
		return runLLMContract(apiKey, modelID, contract, raw)
	case contract.Protocol == "generation" || contract.BodyMode == "generation_wrapper":
		return runWithLegacySpec(apiKey, authToken, modelID, models.ResolveModelID(modelID), contract, raw)
	default:
		return fmt.Errorf("unsupported model contract protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
}

func runQueueContract(apiKey string, contract *contracts.ModelContract, params map[string]any) error {
	client := queue.NewClient(apiKey)
	submitted, err := client.Submit(*contract, params)
	if err != nil {
		return clierrors.ErrSubmitFailed(err)
	}
	saveQueueSubmission(contract, submitted.ID)

	fmt.Fprintf(os.Stderr, "Task submitted: %s\nWaiting for result...\n", submitted.ID)
	task, err := pollQueueResult(client, contract, submitted.ID)
	if err != nil {
		return err
	}
	saveQueueProviderContext(submitted.ID, task)
	return printQueueTask(task)
}

func runLLMContract(apiKey, modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	params, stream, err := llmParamsFromContract(modelID, contract, raw)
	if err != nil {
		return err
	}
	if err := validateLLMOutputMode(stream); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(runTimeout)*time.Second)
	defer cancel()
	client := llm.NewClient(apiKey)
	if stream {
		options := llm.StreamOptions{}
		if runOutput == "sse" {
			options.Raw = os.Stdout
		} else if runOutput == "" {
			options.OnText = func(text string) error {
				fmt.Print(text)
				return nil
			}
		}
		result, err := client.Stream(ctx, *contract, params, options)
		if err != nil {
			return err
		}
		if runOutput == "json" {
			return printJSON(result)
		}
		if runOutput == "" {
			fmt.Println()
		}
		return nil
	}

	result, err := client.Complete(ctx, *contract, params)
	if err != nil {
		return err
	}
	if runOutput == "json" {
		fmt.Println(string(result.Raw))
		return nil
	}
	fmt.Println(result.Text)
	return nil
}

func pollQueueResult(client *queue.Client, contract *contracts.ModelContract, requestID string) (*queue.Task, error) {
	deadline := time.Now().Add(time.Duration(runTimeout) * time.Second)
	lastProgress := -1.0
	for time.Now().Before(deadline) {
		status, err := client.GetStatus(*contract, requestID)
		if err != nil {
			return nil, err
		}
		saveQueueProviderContext(requestID, status)
		printProgress(status.Progress, &lastProgress)
		switch status.Status {
		case "completed":
			return client.GetResult(*contract, requestID)
		case "failed":
			reason := "unknown error"
			if status.Error != nil && status.Error.Message != "" {
				reason = status.Error.Message
			}
			if clierrors.IsInsufficientBalance(fmt.Errorf("%s", reason)) {
				return nil, clierrors.ErrInsufficientBalance()
			}
			return nil, clierrors.ErrTaskFailed(requestID, reason)
		}
		time.Sleep(5 * time.Second)
	}
	return nil, clierrors.ErrTaskTimeout(requestID)
}

func runWithLegacySpec(apiKey, authToken, modelID, resolvedModelID string, contract *contracts.ModelContract, raw map[string]string) error {
	submitted, err := submitLegacyGenerationWithLocalFiles(apiKey, authToken, modelID, resolvedModelID, contract, raw)
	if err != nil {
		return clierrors.ErrSubmitFailed(err)
	}
	resp, spec := submitted.Response, submitted.Spec

	fmt.Fprintf(os.Stderr, "Task submitted: %s\nWaiting for result...\n", resp.ID)
	task, pollErr := generation.PollTask(apiKey, spec.API.Endpoint, resp.ID, 5*time.Second,
		time.Duration(runTimeout)*time.Second, func(progress float64) {
			printProgress(progress, nil)
		})
	if pollErr != nil {
		if task != nil && task.Status == "failed" {
			reason := "unknown error"
			if task.Error != nil {
				reason = task.Error.Message
			}
			if clierrors.IsInsufficientBalance(fmt.Errorf("%s", reason)) {
				return clierrors.ErrInsufficientBalance()
			}
			return clierrors.ErrTaskFailed(resp.ID, reason)
		}
		return clierrors.ErrTaskTimeout(resp.ID)
	}
	if task.Model == resolvedModelID {
		task.Model = modelID
	}
	return printGenerationTask(task)
}

func printProgress(progress float64, lastProgress *float64) {
	pct := int(progress * 100)
	if lastProgress != nil {
		if float64(pct)-*lastProgress < 5 && !(*lastProgress < 0 && pct == 0) {
			return
		}
		*lastProgress = float64(pct)
	}
	fmt.Fprintf(os.Stderr, "Progress: %d%%\n", pct)
}

func printGenerationTask(task *generation.TaskStatus) error {
	if runOutput == "url" {
		for _, u := range task.URLs() {
			fmt.Println(u)
		}
		return nil
	}
	if runOutput == "json" {
		return printJSON(task)
	}
	fmt.Printf("Status: %s\n", task.Status)
	for _, group := range task.Output {
		for _, content := range group.Content {
			if content.URL != "" {
				fmt.Printf("URL: %s\n", content.URL)
			}
			if content.ImgID != 0 {
				fmt.Printf("ImgID: %d\n", content.ImgID)
			}
		}
	}
	return nil
}

func printQueueTask(task *queue.Task) error {
	if runOutput == "url" {
		for _, u := range task.URLs() {
			fmt.Println(u)
		}
		return nil
	}
	if runOutput == "json" {
		return printJSON(task)
	}
	fmt.Printf("Status: %s\n", task.Status)
	for _, u := range task.URLs() {
		fmt.Printf("URL: %s\n", u)
	}
	return nil
}
