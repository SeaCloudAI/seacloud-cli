package cmd

import (
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func queueParamsFromContract(modelID string, contract *contracts.ModelContract, raw map[string]string) (map[string]any, error) {
	raw = fillRawPrerequisitesFromCache(raw, contract.Prerequisites)
	params, err := contracts.ValidateAndCoerce(modelID, raw, contract.InputSchema)
	if err != nil {
		return nil, err
	}
	if err := contracts.ValidatePrerequisites(modelID, params, contract.Prerequisites); err != nil {
		return nil, err
	}
	if err := contracts.ValidateInputRules(modelID, params, contract.InputRules); err != nil {
		return nil, err
	}
	return params, nil
}

func saveQueueSubmission(contract *contracts.ModelContract, requestID string) {
	_ = taskcache.Save(taskcache.Metadata{
		RequestID:        requestID,
		ModelID:          contract.ModelID,
		Protocol:         contract.Protocol,
		BodyMode:         contract.BodyMode,
		ContractRevision: contract.Revision,
		StatusEndpoint:   contract.Endpoints.Status.Path,
		ResultEndpoint:   contract.Endpoints.Result.Path,
	})
}

func submitLegacyGeneration(apiKey, modelID, resolvedModelID string, raw map[string]string) (*generation.TaskStatus, *models.ModelSpec, error) {
	spec, err := models.GetSpec(modelID)
	if err != nil {
		return nil, nil, err
	}
	params, err := generation.ValidateAndCoerce(modelID, raw, spec.Parameters)
	if err != nil {
		return nil, nil, err
	}
	resp, err := generation.Submit(apiKey, spec.API.Endpoint, resolvedModelID, params)
	if err != nil {
		return nil, nil, err
	}
	return resp, spec, nil
}
