package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/generation"
)

var BaseURL = ""

const (
	pollRequestAttempts = 3
	pollRetryBaseDelay  = 250 * time.Millisecond
)

type Client struct {
	httpClient *http.Client
	apiKey     string
}

type Task struct {
	ID            string                   `json:"id"`
	RequestID     string                   `json:"request_id"`
	Status        string                   `json:"status"`
	Model         string                   `json:"model,omitempty"`
	Metadata      map[string]any           `json:"metadata,omitempty"`
	Output        []generation.OutputGroup `json:"output,omitempty"`
	Outputs       []Output                 `json:"outputs,omitempty"`
	Error         *generation.TaskError    `json:"error,omitempty"`
	ErrorType     string                   `json:"error_type,omitempty"`
	ProviderError map[string]any           `json:"provider_error,omitempty"`
	Logs          []LogEntry               `json:"logs,omitempty"`
	Progress      float64                  `json:"progress,omitempty"`
}

type LogEntry struct {
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type Output struct {
	Type        string         `json:"type,omitempty"`
	URL         string         `json:"url,omitempty"`
	Text        string         `json:"text,omitempty"`
	JobID       string         `json:"jobId,omitempty"`
	TaskID      string         `json:"task_id,omitempty"`
	ImageNo     *int           `json:"imageNo,omitempty"`
	ImageNoAlt  *int           `json:"image_no,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	FileName    string         `json:"file_name,omitempty"`
	FileSize    int64          `json:"file_size,omitempty"`
	Width       int            `json:"width,omitempty"`
	Height      int            `json:"height,omitempty"`
	Duration    any            `json:"duration,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
	}
}

func (c *Client) Submit(contract contracts.ModelContract, params map[string]any) (*Task, error) {
	if contract.Protocol != "queue" || contract.BodyMode != "raw_json" {
		return nil, fmt.Errorf("unsupported contract protocol/body_mode: %s/%s", contract.Protocol, contract.BodyMode)
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var task Task
	if err := c.do(contract.Endpoints.Submit, body, &task); err != nil {
		return nil, err
	}
	task.normalizeID()
	if task.ID == "" {
		return nil, fmt.Errorf("queue request failed: no request_id in response")
	}
	return &task, nil
}

func (c *Client) GetStatus(contract contracts.ModelContract, requestID string) (*Task, error) {
	endpoint := replaceRequestID(contract.Endpoints.Status, requestID)
	var task Task
	if err := c.doWithRetry(endpoint, nil, &task, pollRequestAttempts); err != nil {
		return nil, err
	}
	task.normalizeID()
	return &task, nil
}

func (c *Client) GetResult(contract contracts.ModelContract, requestID string) (*Task, error) {
	endpoint := replaceRequestID(contract.Endpoints.Result, requestID)
	var task Task
	if err := c.doWithRetry(endpoint, nil, &task, pollRequestAttempts); err != nil {
		return nil, err
	}
	task.normalizeID()
	return &task, nil
}

func (c *Client) doWithRetry(endpoint contracts.Endpoint, body []byte, out any, attempts int) error {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := c.do(endpoint, body, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == attempts || !isRetryableQueueError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt) * pollRetryBaseDelay)
	}
	return lastErr
}

func (c *Client) do(endpoint contracts.Endpoint, body []byte, out any) error {
	method := endpoint.Method
	if method == "" {
		method = http.MethodGet
	}
	rawURL, err := endpointURL(endpoint.Path)
	if err != nil {
		return err
	}
	var reqBody io.Reader = bytes.NewReader([]byte{})
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	for key, value := range config.FolkosRuntimeHeaders() {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return clierrors.NewAPIError(resp.StatusCode, respBody)
	}
	return json.Unmarshal(respBody, out)
}

func isRetryableQueueError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	text := err.Error()
	return strings.Contains(text, "Client.Timeout") ||
		strings.Contains(text, "context deadline exceeded") ||
		strings.HasPrefix(text, "HTTP 5")
}

func endpointURL(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("contract endpoint path is empty")
	}
	if u, err := url.Parse(path); err == nil && u.Host != "" {
		return config.RewriteURLThroughFolkosProxy(path), nil
	}
	base := BaseURL
	if env := os.Getenv("SEACLOUD_GENERATION_URL"); env != "" {
		base = env
	}
	if base == "" {
		base = generation.BaseURL
	}
	if base == "" {
		return "", fmt.Errorf("generation base URL not configured: set SEACLOUD_GENERATION_URL or use absolute contract endpoints")
	}
	return config.RewriteURLThroughFolkosProxy(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")), nil
}

func replaceRequestID(endpoint contracts.Endpoint, requestID string) contracts.Endpoint {
	endpoint.Path = strings.ReplaceAll(endpoint.Path, "{request_id}", url.PathEscape(requestID))
	return endpoint
}

func (t *Task) normalizeID() {
	if t.ID == "" {
		t.ID = t.RequestID
	}
	if t.RequestID == "" {
		t.RequestID = t.ID
	}
	t.Status = strings.ToLower(t.Status)
	if t.Status == "" && len(t.Outputs) > 0 {
		t.Status = "completed"
	}
	if t.Status == "completed" && t.Error != nil && t.Error.Message != "" {
		t.Status = "failed"
	}
}

func (t *Task) ProviderErrorCode() string {
	if t == nil {
		return ""
	}
	if t.Error != nil && t.Error.Code != "" {
		return t.Error.Code
	}
	if t.ProviderError == nil {
		return ""
	}
	if code, ok := t.ProviderError["code"]; ok && code != nil {
		return strings.TrimSpace(fmt.Sprint(code))
	}
	return ""
}

func (t *Task) FailureReason() string {
	if t == nil {
		return "unknown error"
	}
	reason := ""
	if t.Error != nil {
		reason = strings.TrimSpace(t.Error.Message)
	}
	if reason == "" && t.ProviderError != nil {
		if message, ok := t.ProviderError["message"]; ok && message != nil {
			reason = strings.TrimSpace(fmt.Sprint(message))
		}
	}
	if reason == "" {
		reason = "unknown error"
	}
	if code := t.ProviderErrorCode(); code != "" && !strings.HasPrefix(reason, code+":") {
		return code + ": " + reason
	}
	return reason
}

func (t *Task) URLs() []string {
	var urls []string
	for _, output := range t.Outputs {
		if output.URL != "" {
			urls = append(urls, output.URL)
		}
	}
	for _, group := range t.Output {
		for _, content := range group.Content {
			if content.URL != "" {
				urls = append(urls, content.URL)
			}
		}
	}
	return urls
}
