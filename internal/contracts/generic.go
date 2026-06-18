package contracts

import (
	"encoding/json"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/models"
)

func Generic(modelID string) *ModelContract {
	resolvedModelID := models.ResolveModelID(modelID)
	displayModelID := models.PreferredModelID(modelID, resolvedModelID)
	allowAdditional := true
	return &ModelContract{
		SchemaVersion:  SupportedSchemaVersion,
		ModelID:        displayModelID,
		BackendModelID: resolvedModelID,
		DisplayName:    displayModelID,
		Kind:           "multimodal",
		Protocol:       "queue",
		BodyMode:       "raw_json",
		Endpoints: ContractEndpoints{
			Submit: Endpoint{Method: "POST", Path: "/model/v1/queue/" + resolvedModelID},
			Status: Endpoint{Method: "GET", Path: "/model/v1/queue/" + resolvedModelID + "/requests/{request_id}/status"},
			Result: Endpoint{Method: "GET", Path: "/model/v1/queue/" + resolvedModelID + "/requests/{request_id}/response"},
			Cancel: Endpoint{Method: "PUT", Path: "/model/v1/queue/" + resolvedModelID + "/requests/{request_id}/cancel"},
		},
		InputSchema: InputSchema{
			Type:                 "object",
			AdditionalProperties: &allowAdditional,
			Properties:           map[string]InputSchema{},
		},
	}
}

func coerceFreeformParams(raw map[string]string) map[string]any {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		setFreeformValue(out, key, parseFreeformValue(value))
	}
	return out
}

func parseFreeformValue(value string) any {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
	}
	return value
}

func setFreeformValue(out map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		out[key] = value
		return
	}
	current := out
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
