package modelendpoints

import (
	"net/url"
	"os"
	"strings"
)

const (
	EnvBaseURL         = "SEACLOUD_MODELS_URL"
	EnvContractBaseURL = "SEACLOUD_MODEL_CONTRACTS_URL"
	EnvListURL         = "SEACLOUD_MODELS_LIST_URL"
	EnvSpecURL         = "SEACLOUD_MODELS_SPEC_URL"
)

func ConfiguredURL(buildValue, envName string) string {
	if env := strings.TrimSpace(os.Getenv(envName)); env != "" {
		return env
	}
	return strings.TrimSpace(buildValue)
}

func AppendQuery(raw string, query url.Values) (string, error) {
	if len(query) == 0 {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	merged := u.Query()
	for key, values := range query {
		merged.Del(key)
		for _, value := range values {
			merged.Add(key, value)
		}
	}
	u.RawQuery = merged.Encode()
	return u.String(), nil
}

func ReplaceModelID(raw, modelID string) string {
	escaped := url.PathEscape(modelID)
	raw = strings.ReplaceAll(raw, "{model_id}", escaped)
	return strings.ReplaceAll(raw, ":model_id", escaped)
}
