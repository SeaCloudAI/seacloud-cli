package models

import "github.com/SeaCloudAI/seacloud-cli/internal/clierrors"

func List(params ListParams) (*ModelsListResponse, error) {
	result, err := NewClient().List(params)
	if err != nil {
		return nil, clierrors.ErrFetchModels(err)
	}
	return result, nil
}

func GetSpec(modelID string) (*ModelSpec, error) {
	resolvedModelID := ResolveModelID(modelID)

	spec, err := NewClient().GetSpec(resolvedModelID)
	if err != nil {
		if isNotFound(err) {
			return nil, clierrors.ErrModelNotFound(modelID)
		}
		return nil, clierrors.ErrFetchModelSpec(modelID, err)
	}

	backendModelID := spec.ModelID
	if backendModelID == "" {
		backendModelID = resolvedModelID
	}
	displayModelID := PreferredModelID(modelID, backendModelID)
	spec.ModelID = displayModelID
	spec.AgentPrompt = RewriteModelIDText(spec.AgentPrompt, backendModelID, displayModelID)

	return spec, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return len(s) >= 10 && s[:10] == "status 404"
}
