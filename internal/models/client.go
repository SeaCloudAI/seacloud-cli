package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/modelendpoints"
)

// BaseURL can be overridden at build time via ldflags:
//
//	go build -ldflags "-X github.com/SeaCloudAI/seacloud-cli/internal/models.BaseURL=https://cloud.vtrix.ai"
//
// Or at runtime via the SEACLOUD_MODELS_URL environment variable.
var BaseURL = ""

// ListURL and SpecURL can be overridden at build time via ldflags, or at
// runtime via SEACLOUD_MODELS_LIST_URL and SEACLOUD_MODELS_SPEC_URL.
var (
	ListURL = ""
	SpecURL = ""
)

const defaultModelsTimeout = 30 * time.Second

type Client struct {
	httpClient *http.Client
	baseURL    string
	listURL    string
	specURL    string
	authToken  string
}

func NewClient() *Client {
	base := BaseURL
	if env := os.Getenv(modelendpoints.EnvBaseURL); env != "" {
		base = env
	}
	base = config.RewriteURLThroughFolkosProxy(base)
	return &Client{
		httpClient: &http.Client{Timeout: defaultModelsTimeout},
		baseURL:    base,
		listURL:    config.RewriteURLThroughFolkosProxy(modelendpoints.ConfiguredURL(ListURL, modelendpoints.EnvListURL)),
		specURL:    config.RewriteURLThroughFolkosProxy(modelendpoints.ConfiguredURL(SpecURL, modelendpoints.EnvSpecURL)),
		authToken:  config.ExecTokenFromEnv(),
	}
}

func (c *Client) get(path string, out any) error {
	if c.baseURL == "" {
		return fmt.Errorf("models base URL not configured: set SEACLOUD_MODELS_URL or rebuild with -ldflags")
	}
	return c.getURL(strings.TrimRight(c.baseURL, "/")+path, out)
}

func (c *Client) getURL(rawURL string, out any) error {
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

	var envelope authAPIResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("unexpected response: %s", string(body))
	}

	if envelope.Status.Code != 0 && envelope.Status.Code != 200 {
		return fmt.Errorf("status %d: %s", envelope.Status.Code, envelope.Status.Message)
	}

	if envelope.Status.Code == 0 && envelope.Data == nil {
		return fmt.Errorf("unexpected response: %s", string(body))
	}

	return json.Unmarshal(envelope.Data, out)
}

// Model represents a single model in the list.
type Model struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Description      string   `json:"description"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	SourceID         string   `json:"source_id,omitempty"`
	HasSpec          bool     `json:"has_spec,omitempty"`
	SpecProtocol     string   `json:"spec_protocol,omitempty"`
}

// ModelsListResponse is the data field from /api/v1/skill/models.
type ModelsListResponse struct {
	Models     []Model `json:"models"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// ListParams holds query parameters for List.
type ListParams struct {
	Page     int
	PageSize int
	Type     string
	Keywords string
}

func (c *Client) List(params ListParams) (*ModelsListResponse, error) {
	q := buildQuery(params)
	path := "/api/v1/skill/models"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var result ModelsListResponse
	var err error
	if c.listURL != "" {
		err = c.getConfiguredList(q, &result)
	} else {
		err = c.get(path, &result)
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) getConfiguredList(query url.Values, out any) error {
	endpoint, err := modelendpoints.AppendQuery(c.listURL, query)
	if err != nil {
		return err
	}
	return c.getURL(endpoint, out)
}

// ModelSpec is the data field from /api/v1/skill/models/:id/spec.
type ModelSpec struct {
	ModelID     string       `json:"model_id"`
	Name        string       `json:"name"`
	Vendor      string       `json:"vendor"`
	Type        string       `json:"type"`
	API         ModelSpecAPI `json:"api"`
	Parameters  []ModelParam `json:"parameters"`
	AgentPrompt string       `json:"agent_prompt"`
}

type ModelSpecAPI struct {
	Endpoint string            `json:"endpoint"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
}

type ModelParam struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Type        string            `json:"type"`
	Required    bool              `json:"required"`
	Description string            `json:"description"`
	Constraints *ParamConstraints `json:"constraints,omitempty"`
	Example     any               `json:"example,omitempty"`
	Children    []ModelParam      `json:"children,omitempty"`
}

type ParamConstraints struct {
	Enum      []string `json:"enum,omitempty"`
	Default   any      `json:"default,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	MaxItems  *int     `json:"max_items,omitempty"`
}

func (c *Client) GetSpec(modelID string) (*ModelSpec, error) {
	var spec ModelSpec
	path := "/api/v1/skill/models/" + modelID + "/spec"
	var err error
	if c.specURL != "" {
		err = c.getURL(modelendpoints.ReplaceModelID(c.specURL, modelID), &spec)
	} else {
		err = c.get(path, &spec)
	}
	if err != nil {
		return nil, err
	}
	if rewritten := config.RewriteURLThroughFolkosProxy(spec.API.Endpoint); rewritten != spec.API.Endpoint {
		spec.AgentPrompt = strings.ReplaceAll(spec.AgentPrompt, spec.API.Endpoint, rewritten)
		spec.API.Endpoint = rewritten
	}
	return &spec, nil
}

type authAPIResponse struct {
	Data   json.RawMessage `json:"data"`
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}

func buildQuery(params ListParams) url.Values {
	q := url.Values{}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(params.PageSize))
	}
	if params.Type != "" {
		q.Set("type", params.Type)
	}
	if params.Keywords != "" {
		q.Set("keywords", params.Keywords)
	}
	return q
}
