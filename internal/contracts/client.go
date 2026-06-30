package contracts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/modelendpoints"
)

var BaseURL = ""
var ContractBaseURL = ""
var SpecURL = ""

const defaultTimeout = 30 * time.Second

type Client struct {
	httpClient *http.Client
	baseURL    string
	specURL    string
	authToken  string
}

func NewClient() *Client {
	base := modelendpoints.ConfiguredURL(ContractBaseURL, modelendpoints.EnvContractBaseURL)
	if base == "" {
		base = BaseURL
		if env := os.Getenv(modelendpoints.EnvBaseURL); env != "" {
			base = env
		}
	}
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    config.RewriteURLThroughFolkosProxy(base),
		specURL:    config.RewriteURLThroughFolkosProxy(modelendpoints.ConfiguredURL(SpecURL, modelendpoints.EnvSpecURL)),
		authToken:  apiKeyFromConfig(),
	}
}

func apiKeyFromConfig() string {
	cfg, err := config.Load()
	if err != nil {
		return config.ExecTokenFromEnv()
	}
	return strings.TrimSpace(cfg.APIKey)
}

func (c *Client) Get(modelID string) (*ModelContract, error) {
	var contract ModelContract
	path := "/api/v1/skill/model-contracts/" + modelID
	var err error
	if c.specURL != "" {
		err = c.getURL(modelendpoints.ReplaceModelID(c.specURL, modelID), &contract)
	} else {
		err = c.get(path, &contract)
	}
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (c *Client) get(path string, out any) error {
	if c.baseURL == "" {
		return fmt.Errorf("model contracts base URL not configured: set SEACLOUD_MODEL_CONTRACTS_URL or SEACLOUD_MODELS_URL")
	}
	return c.getURL(strings.TrimRight(c.baseURL, "/")+path, out)
}

func (c *Client) getURL(rawURL string, out any) error {
	if strings.TrimSpace(c.authToken) == "" {
		return clierrors.ErrNoAPIKey()
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("unexpected response: %s", string(body))
	}
	if envelope.Status.Code == http.StatusNotFound {
		return ErrNotFound
	}
	if envelope.Status.Code != 0 && envelope.Status.Code != http.StatusOK {
		return fmt.Errorf("status %d: %s", envelope.Status.Code, envelope.Status.Message)
	}
	if envelope.Data == nil {
		return fmt.Errorf("unexpected response: %s", string(body))
	}
	return json.Unmarshal(envelope.Data, out)
}

type apiResponse struct {
	Data   json.RawMessage `json:"data"`
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}
