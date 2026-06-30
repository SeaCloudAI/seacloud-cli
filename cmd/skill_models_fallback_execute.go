package cmd

import (
	"errors"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

func executeRunFallback(apiKey, authToken, modelID string, raw map[string]string) error {
	detail, err := fallbackDetailOrError(modelID, fallbackScopeAny)
	if err != nil {
		return err
	}
	if runUseReferenceCurl && !runUseSkillModelFallback {
		return executeReferenceCurlRun(apiKey, modelID, detail, raw)
	}
	if !runUseSkillModelFallback {
		return fallbackCurlError(modelID, detail)
	}
	contract, err := fallbackQueueContract(detail)
	if err != nil {
		if runUseReferenceCurl {
			return executeReferenceCurlRun(apiKey, modelID, detail, raw)
		}
		return err
	}
	if IsDryRun() {
		return dryRunContract(modelID, contract, raw)
	}
	if apiKey == "" {
		return clierrors.ErrNoAPIKey()
	}
	err = runQueueContractWithLocalFiles(apiKey, authToken, modelID, contract, raw)
	if err != nil && runUseReferenceCurl {
		return executeReferenceCurlRun(apiKey, modelID, detail, raw)
	}
	return err
}

func executeRunAsyncFallback(apiKey, authToken, modelID string, raw map[string]string) error {
	detail, err := fallbackDetailOrError(modelID, fallbackScopeAny)
	if err != nil {
		return err
	}
	if runUseReferenceCurl && !runUseSkillModelFallback {
		return executeReferenceCurlAsync(apiKey, modelID, detail, raw)
	}
	if !runUseSkillModelFallback {
		return fallbackCurlError(modelID, detail)
	}
	contract, err := fallbackQueueContract(detail)
	if err != nil {
		if runUseReferenceCurl {
			return executeReferenceCurlAsync(apiKey, modelID, detail, raw)
		}
		return err
	}
	if IsDryRun() {
		return dryRunContract(modelID, contract, raw)
	}
	if apiKey == "" {
		return clierrors.ErrNoAPIKey()
	}
	err = runQueueContractAsyncWithLocalFiles(apiKey, authToken, modelID, contract, raw)
	if err != nil && runUseReferenceCurl {
		return executeReferenceCurlAsync(apiKey, modelID, detail, raw)
	}
	return err
}

func executeLLMFallback(apiKey, modelID string, raw map[string]string) error {
	detail, err := fallbackDetailOrError(modelID, fallbackScopeLLM)
	if err != nil {
		return err
	}
	if runUseReferenceCurl && !runUseSkillModelFallback {
		return executeReferenceCurlRun(apiKey, modelID, detail, raw)
	}
	if !runUseSkillModelFallback {
		return fallbackCurlError(modelID, detail)
	}
	contract, err := fallbackLLMContract(detail)
	if err != nil {
		if runUseReferenceCurl {
			return executeReferenceCurlRun(apiKey, modelID, detail, raw)
		}
		return err
	}
	if IsDryRun() {
		return dryRunLLMContract(modelID, contract, raw)
	}
	if apiKey == "" {
		return clierrors.ErrNoAPIKey()
	}
	err = runLLMContract(apiKey, modelID, contract, raw)
	if err != nil && runUseReferenceCurl {
		return executeReferenceCurlRun(apiKey, modelID, detail, raw)
	}
	return err
}

func fallbackDetailOrError(modelID string, scope fallbackScope) (*skillModelsFallback, error) {
	detail, err := findSkillModelsFallback(modelID, scope)
	if errors.Is(err, errSkillModelsFallbackNotFound) {
		return nil, missingFallbackError(modelID)
	}
	if err != nil {
		return nil, err
	}
	return detail, nil
}
