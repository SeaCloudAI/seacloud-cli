package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/buildinfo"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/llm"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
)

func executeReferenceCurlAsync(apiKey, modelID string, detail *skillModelsFallback, raw map[string]string) error {
	parsed, body, err := referenceRequestBody(modelID, detail, raw)
	if err != nil {
		return err
	}
	if IsDryRun() {
		return dryRunReferenceCurl(parsed, body)
	}
	respBody, err := doReferenceCurl(apiKey, parsed, body)
	if err != nil {
		return err
	}
	var submitted queue.Task
	if err := json.Unmarshal(respBody, &submitted); err != nil {
		fmt.Println(string(respBody))
		return nil
	}
	taskID := submitted.ID
	if taskID == "" {
		taskID = submitted.RequestID
	}
	if taskID == "" {
		fmt.Println(string(respBody))
		return nil
	}
	return printAsyncSubmission(asyncSubmission{
		TaskID:   taskID,
		ModelID:  modelID,
		Status:   "submitted",
		Protocol: "reference_curl",
		Next:     nextTaskStatusCommand(taskID),
	})
}

func executeReferenceCurlRun(apiKey, modelID string, detail *skillModelsFallback, raw map[string]string) error {
	parsed, body, err := referenceRequestBody(modelID, detail, raw)
	if err != nil {
		return err
	}
	if IsDryRun() {
		return dryRunReferenceCurl(parsed, body)
	}
	respBody, err := doReferenceCurl(apiKey, parsed, body)
	if err != nil {
		return err
	}
	fmt.Println(string(respBody))
	return nil
}

func referenceRequestBody(modelID string, detail *skillModelsFallback, raw map[string]string) (*parsedFallbackCurl, []byte, error) {
	parsed, err := parseReferenceCurl(detail.Curl)
	if err != nil {
		return nil, nil, err
	}
	if !isAllowedSeaCloudReferenceURL(parsed.URL) {
		return nil, nil, unsupportedReferenceCurlError(modelID)
	}
	if detail.Kind == "llm" {
		contract, err := fallbackLLMContract(detail)
		if err != nil {
			return nil, nil, err
		}
		params, _, err := llmParamsFromContract(modelID, contract, raw)
		if err != nil {
			return nil, nil, err
		}
		body, _ := json.Marshal(params)
		return parsed, body, nil
	}
	allowAdditional := true
	params, err := contracts.ValidateAndCoerce(modelID, raw, freeformInputSchema(allowAdditional))
	if err != nil {
		return nil, nil, err
	}
	body, _ := json.Marshal(params)
	return parsed, body, nil
}

func doReferenceCurl(apiKey string, parsed *parsedFallbackCurl, body []byte) ([]byte, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, clierrors.ErrNoAPIKey()
	}
	rawURL, err := referenceExecutionURL(parsed.URL)
	if err != nil {
		return nil, err
	}
	method := parsed.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range parsed.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("X-Source", "cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, clierrors.NewAPIError(resp.StatusCode, respBody)
	}
	return respBody, nil
}

func referenceExecutionURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(u.Host, "cloud.seaart.ai") {
		base := firstNonEmptyReferenceBase(os.Getenv("SEACLOUD_GENERATION_URL"), os.Getenv("SEACLOUD_BASE_URL"), fallbackSeaCloudBaseURL())
		if strings.HasPrefix(u.Path, "/llm/") {
			base = firstNonEmptyReferenceBase(os.Getenv(llm.EnvBaseURL), os.Getenv("SEACLOUD_BASE_URL"), fallbackSeaCloudBaseURL())
		}
		if base != "" {
			baseURL, err := url.Parse(strings.TrimRight(base, "/"))
			if err == nil && baseURL.Scheme != "" && baseURL.Host != "" {
				u.Scheme = baseURL.Scheme
				u.Host = baseURL.Host
			}
		}
	}
	return u.String(), nil
}

func dryRunReferenceCurl(parsed *parsedFallbackCurl, body []byte) error {
	rawURL, err := referenceExecutionURL(parsed.URL)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dry-run] reference_curl=true\n")
	fmt.Fprintf(os.Stderr, "[dry-run] method=%s\n", parsed.Method)
	fmt.Fprintf(os.Stderr, "[dry-run] url=%s\n", rawURL)
	fmt.Fprintf(os.Stderr, "[dry-run] authorization=Bearer <redacted>\n")
	fmt.Fprintf(os.Stderr, "[dry-run] body=%s\n", string(body))
	return nil
}

func firstNonEmptyReferenceBase(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
