package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/llm"
)

func isLLMContract(contract *contracts.ModelContract) bool {
	return (contract.Protocol == llm.ProtocolChatCompletions && contract.BodyMode == llm.BodyModeChatJSON) ||
		(contract.Protocol == llm.ProtocolResponses && contract.BodyMode == llm.BodyModeResponsesJSON)
}

func llmParamsFromContract(modelID string, contract *contracts.ModelContract, raw map[string]string) (map[string]any, bool, error) {
	if _, ok := raw["model"]; ok {
		return nil, false, fmt.Errorf("model is controlled by the model contract")
	}
	stream, err := resolveLLMStream(raw)
	if err != nil {
		return nil, false, err
	}
	if runStream {
		stream = true
	}
	params, err := contracts.ValidateAndCoerce(modelID, raw, contract.InputSchema)
	if err != nil {
		return nil, false, err
	}
	if err := contracts.ValidatePrerequisites(modelID, params, contract.Prerequisites); err != nil {
		return nil, false, err
	}
	if err := contracts.ValidateInputRules(modelID, params, contract.InputRules); err != nil {
		return nil, false, err
	}
	params["model"] = llmModelID(modelID, contract.ModelID)
	if stream {
		params["stream"] = true
	}
	return params, stream, nil
}

func resolveLLMStream(raw map[string]string) (bool, error) {
	value, ok := raw["stream"]
	if !ok {
		return false, nil
	}
	stream, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}
	if runStream && !stream {
		return false, fmt.Errorf("--stream conflicts with --param stream=false")
	}
	return stream, nil
}

func llmModelID(fallback, contractModelID string) string {
	if strings.TrimSpace(contractModelID) != "" {
		return contractModelID
	}
	return fallback
}

func validateLLMOutputMode(stream bool) error {
	switch runOutput {
	case "", "json":
		return nil
	case "url":
		return fmt.Errorf("--output url is not supported for LLM models")
	case "sse":
		if !stream {
			return fmt.Errorf("--output sse requires streaming; pass --stream or --param stream=true")
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format for LLM models: %s", runOutput)
	}
}

func dryRunLLMContract(modelID string, contract *contracts.ModelContract, raw map[string]string) error {
	params, _, err := llmParamsFromContract(modelID, contract, raw)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(params)
	fmt.Fprintf(os.Stderr, "[dry-run] protocol=%s\n", contract.Protocol)
	fmt.Fprintf(os.Stderr, "[dry-run] body_mode=%s\n", contract.BodyMode)
	if contract.Endpoints.ChatCompletions.Path != "" {
		fmt.Fprintf(os.Stderr, "[dry-run] chat_completions=%s %s\n",
			defaultMethod(contract.Endpoints.ChatCompletions.Method), contract.Endpoints.ChatCompletions.Path)
	}
	if contract.Endpoints.Responses.Path != "" {
		fmt.Fprintf(os.Stderr, "[dry-run] responses=%s %s\n",
			defaultMethod(contract.Endpoints.Responses.Method), contract.Endpoints.Responses.Path)
	}
	fmt.Fprintf(os.Stderr, "[dry-run] body=%s\n", string(body))
	return nil
}

func defaultMethod(method string) string {
	if method == "" {
		return http.MethodPost
	}
	return method
}
