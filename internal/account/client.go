package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/auth"
	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

// BaseURL can be overridden at build time via ldflags or at runtime with SEACLOUD_BASE_URL.
var BaseURL = ""

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type Balance struct {
	UserID           string `json:"user_id"`
	Balance          string `json:"balance"`
	TotalRecharged   string `json:"total_recharged"`
	Currency         string `json:"currency"`
	BillingMode      string `json:"billing_mode"`
	CreditLimit      string `json:"credit_limit"`
	Arrears          string `json:"arrears"`
	AvailableBalance string `json:"available_balance"`
}

func NewClient(token string) *Client {
	base := BaseURL
	if env := os.Getenv("SEACLOUD_BASE_URL"); env != "" {
		base = env
	}
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
		baseURL:    base,
	}
}

func (c *Client) GetBalance() (*Balance, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("account base URL not configured: set SEACLOUD_BASE_URL or rebuild with -ldflags")
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(c.baseURL, "/")+"/api/v1/user/balance", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, clierrors.ErrNetworkTimeout(err)
		}
		return nil, clierrors.ErrNetwork(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope apiResponse
	envelopeOK := json.Unmarshal(body, &envelope) == nil && envelope.Status != nil

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, clierrors.ErrTokenExpired()
	case http.StatusForbidden:
		return nil, clierrors.ErrTokenInvalid()
	}

	if envelopeOK && envelope.Status.Code != 0 && envelope.Status.Code != http.StatusOK {
		return nil, mapStatusError(envelope.Status)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if envelopeOK && envelope.Status.Message != "" {
			return nil, fmt.Errorf("%s", envelope.Status.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if !envelopeOK {
		return nil, fmt.Errorf("unexpected response: %s", string(body))
	}
	if envelope.Data == nil || string(*envelope.Data) == "null" {
		return nil, fmt.Errorf("unexpected response: %s", string(body))
	}

	var balance Balance
	if err := json.Unmarshal(*envelope.Data, &balance); err != nil {
		return nil, fmt.Errorf("unexpected response: %s", string(body))
	}
	return &balance, nil
}

func mapStatusError(status *apiStatus) error {
	switch status.Code {
	case http.StatusUnauthorized:
		return clierrors.ErrTokenExpired()
	case http.StatusForbidden:
		return clierrors.ErrTokenInvalid()
	}
	if status.Message != "" {
		return fmt.Errorf("%s", status.Message)
	}
	return fmt.Errorf("status %d", status.Code)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	req.Header.Set("X-App-Id", auth.AppID)
	req.Header.Set("X-Version", buildinfo.Version)
	req.Header.Set("X-Plat", "cli")
	req.Header.Set("X-Device-Type", "cli")
	req.Header.Set("X-Skip-Nextauth", "true")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("X-Auth-Priority", "auth_token")
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

type apiResponse struct {
	Data   *json.RawMessage `json:"data"`
	Status *apiStatus       `json:"status"`
}

type apiStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
