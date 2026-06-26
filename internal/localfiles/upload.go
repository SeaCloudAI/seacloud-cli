package localfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/SeaCloudAI/seacloud-cli/internal/modelendpoints"
)

const (
	EnvUploadURL = "SEACLOUD_UPLOAD_URL"
	uploadPath   = "/api/v1/storage/files"
)

// DefaultUploadURL can be set at build time via ldflags.
var DefaultUploadURL = ""

type HTTPUploader struct {
	Endpoint   string
	AuthToken  string
	APIKey     string
	HTTPClient *http.Client
}

func NewHTTPUploader(authToken, apiKey string) *HTTPUploader {
	return &HTTPUploader{
		Endpoint:   DefaultUploadEndpoint(),
		AuthToken:  strings.TrimSpace(authToken),
		APIKey:     strings.TrimSpace(apiKey),
		HTTPClient: &http.Client{Timeout: 100 * time.Second},
	}
}

func DefaultUploadEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv(EnvUploadURL)); endpoint != "" {
		return endpoint
	}
	if base := strings.TrimSpace(os.Getenv(modelendpoints.EnvBaseURL)); base != "" {
		return strings.TrimRight(base, "/") + uploadPath
	}
	return strings.TrimSpace(DefaultUploadURL)
}

func (u *HTTPUploader) Upload(ctx context.Context, path string) (string, error) {
	endpoint := strings.TrimSpace(u.Endpoint)
	if endpoint == "" {
		return "", &clierrors.CLIError{Message: "upload_failed: upload URL is not configured"}
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := os.Open(path)
	if err != nil {
		return "", fileAccessError(path, err)
	}
	defer file.Close()
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	if err := writer.Close(); err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")
	if token := firstNonEmpty(u.APIKey, u.AuthToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range config.FolkosRuntimeHeaders() {
		req.Header.Set(key, value)
	}
	client := u.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: %v", err)}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", &clierrors.CLIError{Message: "upload_auth_required: run seacloud auth login or configure a CLI-callable upload endpoint"}
	}
	if resp.StatusCode >= 400 {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))}
	}
	url, err := uploadURLFromResponse(respBody)
	if err != nil {
		return "", err
	}
	return url, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func uploadURLFromResponse(body []byte) (string, error) {
	var direct struct {
		URL  string `json:"url"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &direct); err != nil {
		return "", &clierrors.CLIError{Message: fmt.Sprintf("upload_failed: invalid response: %v", err)}
	}
	if direct.URL != "" {
		return direct.URL, nil
	}
	if direct.Data.URL != "" {
		return direct.Data.URL, nil
	}
	return "", &clierrors.CLIError{Message: "upload_failed: upload response did not include url"}
}
