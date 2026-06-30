package admindetail

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/modelendpoints"
)

type Kind string

const (
	KindMultimodal Kind = "multimodal"
	KindLLM        Kind = "llm"
)

var (
	ErrNotFound     = errors.New("admin detail not found")
	ErrAuthRequired = errors.New("admin bearer token required")
)

const defaultTimeout = 30 * time.Second

type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
}

type Detail struct {
	Kind     Kind   `json:"-"`
	ModelID  string `json:"model_id"`
	Endpoint string `json:"detail_endpoint"`
	Curl     string `json:"curl"`
}

type Fallback struct {
	ContractFound                 bool   `json:"contract_found"`
	FallbackSource                string `json:"fallback_source"`
	FallbackKind                  Kind   `json:"fallback_kind"`
	ModelID                       string `json:"model_id"`
	DetailEndpoint                string `json:"detail_endpoint"`
	Curl                          string `json:"curl"`
	OfficialParameterDocsRequired bool   `json:"official_parameter_docs_required"`
}

func NewClient(authToken string) *Client {
	base := modelendpoints.ConfiguredURL(contracts.ContractBaseURL, modelendpoints.EnvContractBaseURL)
	if base == "" {
		base = contracts.BaseURL
		if env := strings.TrimSpace(os.Getenv(modelendpoints.EnvBaseURL)); env != "" {
			base = env
		}
	}
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    config.RewriteURLThroughFolkosProxy(base),
		authToken:  strings.TrimSpace(authToken),
	}
}

func (c *Client) GetMultimodal(modelID string) (*Detail, error) {
	return c.get(KindMultimodal, "/api/v1/admin/multi-models/detail", modelID)
}

func (c *Client) GetLLM(modelID string) (*Detail, error) {
	return c.get(KindLLM, "/api/v1/admin/models/detail", modelID)
}

func (d *Detail) Fallback() Fallback {
	return Fallback{
		ContractFound:                 false,
		FallbackSource:                "admin_detail",
		FallbackKind:                  d.Kind,
		ModelID:                       d.ModelID,
		DetailEndpoint:                d.Endpoint,
		Curl:                          d.Curl,
		OfficialParameterDocsRequired: true,
	}
}

func (c *Client) get(kind Kind, path, modelID string) (*Detail, error) {
	endpoint := detailEndpoint(path, modelID)
	if c.authToken == "" {
		return nil, authRequiredError(endpoint)
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("admin detail base URL not configured: set SEACLOUD_MODEL_CONTRACTS_URL or SEACLOUD_MODELS_URL")
	}
	rawURL := strings.TrimRight(c.baseURL, "/") + endpoint
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, authRequiredError(endpoint)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("unexpected admin detail response: %s", string(body))
	}
	if envelope.Status.Code == http.StatusUnauthorized || envelope.Status.Code == http.StatusForbidden {
		return nil, authRequiredError(endpoint)
	}
	if envelope.Status.Code == http.StatusNotFound || isNotFoundStatus(envelope.Status.Message) {
		return nil, ErrNotFound
	}
	if envelope.Status.Code != 0 && envelope.Status.Code != http.StatusOK {
		return nil, fmt.Errorf("admin detail status %d: %s", envelope.Status.Code, envelope.Status.Message)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("unexpected admin detail response: %s", string(body))
	}

	var payload detailResponse
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, err
	}
	curl := strings.TrimSpace(payload.Model.Curl)
	if curl == "" {
		return nil, fmt.Errorf("admin detail for %q did not include model curl", modelID)
	}
	id := strings.TrimSpace(payload.Model.ID)
	if id == "" {
		id = modelID
	}
	return &Detail{Kind: kind, ModelID: id, Endpoint: endpoint, Curl: curl}, nil
}

func detailEndpoint(path, modelID string) string {
	q := url.Values{}
	q.Set("id", modelID)
	q.Set("platform", "seacloud")
	return path + "?" + q.Encode()
}

func authRequiredError(endpoint string) error {
	return fmt.Errorf("%w: call the admin detail endpoint with an admin session token:\ncurl -sS '<base-url>%s' -H 'Authorization: Bearer <admin-token>'",
		ErrAuthRequired, endpoint)
}

func isNotFoundStatus(message string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(message, " ", ""))
	return strings.Contains(normalized, "notfound") || strings.Contains(normalized, "hasbeendeleted")
}

type apiResponse struct {
	Data   json.RawMessage `json:"data"`
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}

type detailResponse struct {
	Model struct {
		ID   string `json:"id"`
		Curl string `json:"curl"`
	} `json:"model"`
}
