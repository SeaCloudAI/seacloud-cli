package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
	"github.com/SeaCloudAI/seacloud-cli/internal/localfiles"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
)

type legacyGenerationSubmission struct {
	Response *generation.TaskStatus
	Spec     *models.ModelSpec
}

func printQueueSubmitted(id string) {
	fmt.Fprintf(os.Stderr, "Task submitted: %s\nWaiting for result...\n", id)
}

func prepareLocalFileParams(authToken, apiKey string, raw map[string]string, schema contracts.InputSchema) (*localfiles.Prepared, error) {
	uploader := localfiles.NewHTTPUploader(authToken, apiKey)
	return localfiles.Prepare(context.Background(), raw, schema, uploader.Upload)
}

func runQueueContractWithLocalFiles(apiKey, authToken, modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	prepared, err := prepareLocalFileParams(authToken, apiKey, raw, contract.InputSchema)
	if err != nil {
		return err
	}
	params, usedFallback, err := queueParamsFromPrepared(modelID, contract, prepared)
	if err != nil {
		return err
	}
	client := queue.NewClient(apiKey)
	submitted, err := submitQueuePrepared(client, contract, prepared, params, usedFallback, modelID)
	if err != nil {
		return clierrors.ErrSubmitFailed(err)
	}
	saveQueueSubmission(contract, submitted.ID)

	printQueueSubmitted(submitted.ID)
	task, err := pollQueueResult(client, contract, submitted.ID)
	if err != nil {
		return err
	}
	saveQueueProviderContext(submitted.ID, task)
	return printQueueTask(task)
}

func submitQueuePrepared(client *queue.Client, contract *contracts.ModelContract, prepared *localfiles.Prepared, params map[string]any, usedFallback bool, modelID string) (*queue.Task, error) {
	submitted, err := client.Submit(*contract, params)
	if err == nil || usedFallback || !prepared.ShouldFallback(err) {
		return submitted, err
	}
	fallbackRaw, fallbackErr := prepared.FallbackRaw(context.Background())
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	fallbackParams, fallbackErr := queueParamsFromContract(modelID, contract, fallbackRaw)
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	return client.Submit(*contract, fallbackParams)
}

func queueParamsFromPrepared(modelID string, contract *contracts.ModelContract, prepared *localfiles.Prepared) (map[string]any, bool, error) {
	params, err := queueParamsFromContract(modelID, contract, prepared.Raw)
	if err == nil || !prepared.ShouldFallback(err) {
		return params, false, err
	}
	fallbackRaw, fallbackErr := prepared.FallbackRaw(context.Background())
	if fallbackErr != nil {
		return nil, false, fallbackErr
	}
	params, err = queueParamsFromContract(modelID, contract, fallbackRaw)
	return params, true, err
}

func submitLegacyGenerationWithLocalFiles(apiKey, authToken, modelID, resolvedModelID string, contract *contracts.ModelContract, raw map[string]string) (*legacyGenerationSubmission, error) {
	schema := contracts.InputSchema{}
	if contract != nil {
		schema = contract.InputSchema
	}
	prepared, err := prepareLocalFileParams(authToken, apiKey, raw, schema)
	if err != nil {
		return nil, err
	}
	resp, spec, err := submitLegacyGeneration(apiKey, modelID, resolvedModelID, prepared.Raw)
	if err == nil || !prepared.ShouldFallback(err) {
		return &legacyGenerationSubmission{Response: resp, Spec: spec}, err
	}
	fallbackRaw, fallbackErr := prepared.FallbackRaw(context.Background())
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	resp, spec, err = submitLegacyGeneration(apiKey, modelID, resolvedModelID, fallbackRaw)
	return &legacyGenerationSubmission{Response: resp, Spec: spec}, err
}

func runQueueContractAsyncWithLocalFiles(apiKey, authToken, modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	prepared, err := prepareLocalFileParams(authToken, apiKey, raw, contract.InputSchema)
	if err != nil {
		return err
	}
	params, usedFallback, err := queueParamsFromPrepared(modelID, contract, prepared)
	if err != nil {
		return err
	}
	client := queue.NewClient(apiKey)
	submitted, err := submitQueuePrepared(client, contract, prepared, params, usedFallback, modelID)
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
}

func runLegacyAsyncWithLocalFiles(apiKey, authToken, modelID, resolvedModelID string, contract *contracts.ModelContract, raw map[string]string) error {
	submitted, err := submitLegacyGenerationWithLocalFiles(apiKey, authToken, modelID, resolvedModelID, contract, raw)
	if err != nil {
		return clierrors.ErrSubmitFailed(err)
	}
	return printAsyncSubmission(asyncSubmission{
		TaskID:   submitted.Response.ID,
		ModelID:  modelID,
		Status:   "submitted",
		Protocol: "generation",
		Next:     nextTaskStatusCommand(submitted.Response.ID),
	})
}
