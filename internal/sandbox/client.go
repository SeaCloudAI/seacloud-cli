package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	sandboxgo "github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

const EnvSandboxURL = "SEACLOUD_SANDBOX_URL"

// BaseURL can be overridden at build time via ldflags.
var BaseURL = "https://sandbox-gateway.cloud.seaart.ai/api/v1"

type Options struct {
	BaseURL     string
	APIKey      string
	NamespaceID string
	UserID      string
	ProjectID   string
	Timeout     time.Duration
}

type Client struct {
	Control *control.Service
	Build   *build.Service
	options Options
}

func NewClient(opts Options) (*Client, error) {
	baseURL := resolveBaseURL(opts.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("sandbox base URL not configured: set %s", EnvSandboxURL)
	}
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("sandbox API key is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transportOpts := []core.TransportOption{
		core.WithTimeout(timeout),
		core.WithNamespaceID(opts.NamespaceID),
		core.WithUserID(opts.UserID),
		core.WithProjectID(opts.ProjectID),
	}
	controlService, err := control.NewService(baseURL, opts.APIKey, transportOpts...)
	if err != nil {
		return nil, err
	}
	buildService, err := build.NewService(baseURL, opts.APIKey, transportOpts...)
	if err != nil {
		return nil, err
	}
	opts.BaseURL = baseURL
	opts.Timeout = timeout
	return &Client{Control: controlService, Build: buildService, options: opts}, nil
}

func NewClientFromConfig(cfg *config.Config, opts Options) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	opts.APIKey = firstNonEmpty(opts.APIKey, cfg.APIKey)
	return NewClient(opts)
}

func (c *Client) RuntimeFromSandbox(s *control.Sandbox) (*sandboxgo.Runtime, error) {
	return sandboxgo.RuntimeFromSandbox(s)
}

func (c *Client) BaseURL() string {
	return c.options.BaseURL
}

func (c *Client) APIKey() string {
	return c.options.APIKey
}

func (c *Client) RuntimeFromDetail(s *control.SandboxDetail) (*sandboxgo.Runtime, error) {
	return sandboxgo.RuntimeFromDetail(s)
}

func (c *Client) ConnectRuntime(ctx context.Context, sandboxID string, timeout int64) (*control.Sandbox, *sandboxgo.Runtime, error) {
	resp, err := c.Control.ConnectSandbox(ctx, sandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, nil, err
	}
	runtime, err := c.RuntimeFromSandbox(resp.Sandbox)
	if err != nil {
		return resp.Sandbox, nil, err
	}
	return resp.Sandbox, runtime, nil
}

func (c *Client) UpdateNetwork(ctx context.Context, sandboxID string, body map[string]any) error {
	if strings.TrimSpace(sandboxID) == "" {
		return fmt.Errorf("sandbox ID is required")
	}
	path := c.Control.APIPath("/sandboxes/" + url.PathEscape(strings.TrimSpace(sandboxID)) + "/network")
	_, err := c.Control.DoRequest(ctx, http.MethodPut, path, nil, nil, body, http.StatusNoContent)
	return err
}

func resolveBaseURL(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return normalizeAPIBaseURL(value)
	}
	if value = strings.TrimSpace(os.Getenv(EnvSandboxURL)); value != "" {
		return normalizeAPIBaseURL(value)
	}
	if value = strings.TrimSpace(os.Getenv("SEACLOUD_BASE_URL")); value != "" {
		return normalizeAPIBaseURL(value)
	}
	return normalizeAPIBaseURL(BaseURL)
}

func normalizeAPIBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" || strings.HasSuffix(value, "/api/v1") || strings.Contains(value, "/api/v1/") {
		return value
	}
	return value + "/api/v1"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
