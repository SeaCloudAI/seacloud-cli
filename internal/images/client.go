package images

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

const (
	EnvProxyURL           = "SEACLOUD_FOLKOS_PROXY_URL"
	RouteGenerate         = "/seacloud-cli-proxy-api/images/generations"
	RouteUploadBase64     = "/internal/assets/upload/base64"
	DefaultModel          = "gpt-image-2"
	DefaultSize           = "1024x1024"
	DefaultResponseFormat = "b64_json"
	DefaultTimeout        = 10 * time.Minute
)

// BaseURL can be overridden at build time via ldflags:
//
//	go build -ldflags "-X github.com/SeaCloudAI/seacloud-cli/internal/images.BaseURL=http://127.0.0.1:8090"
//
// Or at runtime via the SEACLOUD_FOLKOS_PROXY_URL environment variable.
var BaseURL = ""

type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

type GenerateRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type GenerateResponse struct {
	Created      int64       `json:"created,omitempty"`
	Data         []ImageData `json:"data"`
	Background   string      `json:"background,omitempty"`
	OutputFormat string      `json:"output_format,omitempty"`
	Quality      string      `json:"quality,omitempty"`
	Size         string      `json:"size,omitempty"`
	Usage        any         `json:"usage,omitempty"`
}

type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type Base64UploadRequest struct {
	Data         string `json:"data"`
	MIMETypeHint string `json:"mime_type_hint,omitempty"`
}

type Base64UploadResponse struct {
	ObjectPath   string `json:"object_path"`
	CDNURL       string `json:"cdn_url"`
	ContentType  string `json:"content_type"`
	ResourceType string `json:"resource_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}

func NewClient(apiKey string) *Client {
	return NewClientWithTimeout(apiKey, DefaultTimeout)
}

func NewClientWithTimeout(apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(resolveBaseURL(), "/"),
	}
}

func SupportsSyncModel(modelID string) bool {
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(modelID, "gpt-image")
}

func RequestFromValues(modelID, prompt, size, responseFormat string) (GenerateRequest, error) {
	modelID = strings.TrimSpace(modelID)
	prompt = strings.TrimSpace(prompt)
	size = strings.TrimSpace(size)
	responseFormat = strings.TrimSpace(responseFormat)

	if modelID == "" {
		modelID = DefaultModel
	}
	if prompt == "" {
		return GenerateRequest{}, clierrors.ErrMissingParam(modelID, "prompt")
	}
	if size == "" {
		size = DefaultSize
	}
	if responseFormat == "" {
		responseFormat = DefaultResponseFormat
	}
	if responseFormat != DefaultResponseFormat {
		return GenerateRequest{}, clierrors.ErrInvalidParam(modelID, "response_format", fmt.Sprintf("only %q is supported", DefaultResponseFormat))
	}

	return GenerateRequest{
		Model:          modelID,
		Prompt:         prompt,
		Size:           size,
		ResponseFormat: responseFormat,
	}, nil
}

func RequestFromParams(modelID string, raw map[string]string) (GenerateRequest, error) {
	allowed := map[string]bool{
		"prompt":          true,
		"size":            true,
		"response_format": true,
	}

	for name := range raw {
		if !allowed[name] {
			return GenerateRequest{}, clierrors.ErrInvalidParam(modelID, name, "only prompt, size, and response_format are supported for sync image generation")
		}
	}

	return RequestFromValues(modelID, raw["prompt"], raw["size"], raw["response_format"])
}

func (c *Client) Generate(req GenerateRequest) (*GenerateResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("proxy base URL not configured: set %s or rebuild with -ldflags", EnvProxyURL)
	}

	var resp GenerateResponse
	if err := c.do(http.MethodPost, c.baseURL+RouteGenerate, req, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("image generation returned no data")
	}
	return &resp, nil
}

func (c *Client) UploadBase64(data, mimeTypeHint string) (*Base64UploadResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("proxy base URL not configured: set %s or rebuild with -ldflags", EnvProxyURL)
	}

	var resp Base64UploadResponse
	if err := c.do(http.MethodPost, c.baseURL+RouteUploadBase64, Base64UploadRequest{
		Data:         data,
		MIMETypeHint: strings.TrimSpace(mimeTypeHint),
	}, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.CDNURL) == "" {
		return nil, fmt.Errorf("assets upload returned no cdn_url")
	}
	return &resp, nil
}

func (c *Client) UploadResponseImages(resp *GenerateResponse) ([]string, error) {
	if resp == nil {
		return nil, fmt.Errorf("generation response is required")
	}

	mimeTypeHint := mime.TypeByExtension("." + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(resp.OutputFormat)), "."))
	urls := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		if u := strings.TrimSpace(item.URL); u != "" {
			urls = append(urls, u)
			continue
		}
		if item.B64JSON == "" {
			continue
		}
		uploaded, err := c.UploadBase64(item.B64JSON, mimeTypeHint)
		if err != nil {
			return nil, err
		}
		urls = append(urls, uploaded.CDNURL)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("image generation returned no URL or b64_json payload")
	}
	return urls, nil
}

func Summary(resp *GenerateResponse) string {
	if resp == nil {
		return "No image response."
	}

	lines := []string{fmt.Sprintf("Images: %d", len(resp.Data))}
	if resp.OutputFormat != "" {
		lines = append(lines, fmt.Sprintf("Output format: %s", resp.OutputFormat))
	}
	if resp.Size != "" {
		lines = append(lines, fmt.Sprintf("Size: %s", resp.Size))
	}
	for i, item := range resp.Data {
		index := i + 1
		switch {
		case strings.TrimSpace(item.URL) != "":
			lines = append(lines, fmt.Sprintf("Image %d URL: %s", index, item.URL))
		case item.B64JSON != "":
			lines = append(lines, fmt.Sprintf("Image %d b64_json length: %d", index, len(item.B64JSON)))
		default:
			lines = append(lines, fmt.Sprintf("Image %d: empty payload", index))
		}
		if item.RevisedPrompt != "" {
			lines = append(lines, fmt.Sprintf("Image %d revised prompt: %s", index, item.RevisedPrompt))
		}
	}
	return strings.Join(lines, "\n")
}

func resolveBaseURL() string {
	if env := normalizeBaseURL(os.Getenv(EnvProxyURL)); env != "" {
		return env
	}
	folkosBase := strings.TrimRight(strings.TrimSpace(config.FolkosProxyBaseURL()), "/")
	if folkosBase == "" {
		return normalizeBaseURL(BaseURL)
	}
	return strings.TrimSuffix(folkosBase, "/folkos-proxy")
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}

	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/")
}

func (c *Client) do(method, endpoint string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, endpoint, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")

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
		var errBody struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(respBody, &errBody) == nil {
			msg := strings.TrimSpace(errBody.Message)
			if msg == "" {
				msg = strings.TrimSpace(errBody.Error)
			}
			if msg == "" {
				msg = strings.TrimSpace(errBody.Code)
			}
			if msg != "" {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}
