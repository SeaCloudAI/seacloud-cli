package contracts

import (
	"errors"
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/models"
)

func Get(modelID string, opts Options) (*ModelContract, error) {
	resolvedModelID := models.ResolveModelID(modelID)
	contract, err := NewClient().Get(resolvedModelID)
	if err != nil {
		if errors.Is(err, ErrNotFound) || opts.Refresh {
			return nil, err
		}
		if cached, cacheErr := loadCached(resolvedModelID); cacheErr == nil {
			return prepareContract(modelID, resolvedModelID, cached)
		}
		return nil, err
	}
	if err := ensureCompatible(contract); err != nil {
		return nil, err
	}
	_ = saveCached(resolvedModelID, contract)
	return prepareContract(modelID, resolvedModelID, contract)
}

func ensureCompatible(contract *ModelContract) error {
	if contract.SchemaVersion != SupportedSchemaVersion {
		return fmt.Errorf("%w: got %s, want %s",
			ErrIncompatibleSchema, contract.SchemaVersion, SupportedSchemaVersion)
	}
	return nil
}

func prepareContract(modelID, resolvedModelID string, contract *ModelContract) (*ModelContract, error) {
	if err := ensureCompatible(contract); err != nil {
		return nil, err
	}
	backendModelID := contract.ModelID
	if backendModelID == "" {
		backendModelID = resolvedModelID
	}
	contract.BackendModelID = backendModelID
	contract.ModelID = models.PreferredModelID(modelID, backendModelID)
	return contract, nil
}
