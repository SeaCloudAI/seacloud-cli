package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
)

var BaseURL = ""

const (
	EnvBaseURL         = "SEACLOUD_LLM_URL"
	envFallbackBaseURL = "SEACLOUD_BASE_URL"

	ProtocolChatCompletions = "llm_chat_completions"
	ProtocolResponses       = "llm_responses"
	BodyModeChatJSON        = "openai_chat_json"
	BodyModeResponsesJSON   = "openai_responses_json"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
}

type Result struct {
	ID           string         `json:"id,omitempty"`
	Model        string         `json:"model,omitempty"`
	Text         string         `json:"text"`
	Usage        map[string]any `json:"usage,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
	Raw          []byte         `json:"-"`
}

type StreamOptions struct {
	OnText func(string) error
	Raw    io.Writer
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
	}
}

func (c *Client) Complete(ctx context.Context, contract contracts.ModelContract, params map[string]any) (*Result, error) {
	resp, err := c.do(ctx, contract, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, clierrors.NewAPIError(resp.StatusCode, body)
	}
	withRaw := func(result *Result, err error) (*Result, error) {
		if result != nil {
			result.Raw = append([]byte(nil), body...)
		}
		return result, err
	}
	switch contract.Protocol {
	case ProtocolChatCompletions:
		return withRaw(parseChatCompletion(body))
	case ProtocolResponses:
		return withRaw(parseResponsesCompletion(body))
	default:
		return nil, fmt.Errorf("unsupported LLM protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
}

func (c *Client) Stream(ctx context.Context, contract contracts.ModelContract, params map[string]any, opts StreamOptions) (*Result, error) {
	resp, err := c.do(ctx, contract, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, clierrors.NewAPIError(resp.StatusCode, body)
	}
	return parseSSE(resp.Body, contract.Protocol, opts)
}

func (c *Client) do(ctx context.Context, contract contracts.ModelContract, params map[string]any) (*http.Response, error) {
	endpoint, err := endpointForContract(contract)
	if err != nil {
		return nil, err
	}
	rawURL, err := endpointURL(endpoint.Path)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	method := endpoint.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	for key, value := range config.FolkosRuntimeHeaders() {
		req.Header.Set(key, value)
	}
	return c.httpClient.Do(req)
}

func endpointForContract(contract contracts.ModelContract) (contracts.Endpoint, error) {
	switch {
	case contract.Protocol == ProtocolChatCompletions && contract.BodyMode == BodyModeChatJSON:
		return requireEndpoint(contract.Endpoints.ChatCompletions, "chat_completions")
	case contract.Protocol == ProtocolResponses && contract.BodyMode == BodyModeResponsesJSON:
		return requireEndpoint(contract.Endpoints.Responses, "responses")
	default:
		return contracts.Endpoint{}, fmt.Errorf("unsupported LLM protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
}

func requireEndpoint(endpoint contracts.Endpoint, name string) (contracts.Endpoint, error) {
	if strings.TrimSpace(endpoint.Path) == "" {
		return contracts.Endpoint{}, fmt.Errorf("contract endpoint %s path is empty", name)
	}
	return endpoint, nil
}

func endpointURL(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("contract endpoint path is empty")
	}
	if u, err := url.Parse(path); err == nil && u.Host != "" {
		return config.RewriteURLThroughFolkosProxy(path), nil
	}
	base := strings.TrimSpace(os.Getenv(EnvBaseURL))
	if base == "" {
		base = strings.TrimSpace(os.Getenv(envFallbackBaseURL))
	}
	if base == "" {
		base = BaseURL
	}
	if base == "" {
		return "", fmt.Errorf("LLM base URL not configured: set SEACLOUD_LLM_URL or use absolute contract endpoints")
	}
	return config.RewriteURLThroughFolkosProxy(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")), nil
}
