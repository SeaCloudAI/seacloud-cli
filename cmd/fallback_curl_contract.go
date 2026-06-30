package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/llm"
)

type parsedFallbackCurl struct {
	Method  string
	URL     string
	Path    string
	Headers map[string]string
	Body    string
}

func fallbackQueueContract(detail *skillModelsFallback) (*contracts.ModelContract, error) {
	parsed, err := parseReferenceCurl(detail.Curl)
	if err != nil {
		return nil, err
	}
	if !isAllowedSeaCloudReferenceURL(parsed.URL) {
		return nil, unsupportedReferenceCurlError(detail.ModelID)
	}
	if !strings.HasPrefix(parsed.Path, "/model/v1/queue/") {
		return nil, unsupportedReferenceCurlError(detail.ModelID)
	}
	allowAdditional := true
	submit := parsed.Path
	modelID := detail.ModelID
	if fromPath := strings.TrimPrefix(parsed.Path, "/model/v1/queue/"); fromPath != "" {
		modelID = fromPath
	}
	return &contracts.ModelContract{
		SchemaVersion: contracts.SupportedSchemaVersion,
		ModelID:       modelID,
		DisplayName:   modelID,
		Kind:          "multimodal",
		Protocol:      "queue",
		BodyMode:      "raw_json",
		Endpoints: contracts.ContractEndpoints{
			Submit: contracts.Endpoint{Method: http.MethodPost, Path: submit},
			Status: contracts.Endpoint{Method: http.MethodGet, Path: submit + "/requests/{request_id}/status"},
			Result: contracts.Endpoint{Method: http.MethodGet, Path: submit + "/requests/{request_id}/response"},
			Cancel: contracts.Endpoint{Method: http.MethodPut, Path: submit + "/requests/{request_id}/cancel"},
		},
		InputSchema: freeformInputSchema(allowAdditional),
	}, nil
}

func fallbackLLMContract(detail *skillModelsFallback) (*contracts.ModelContract, error) {
	parsed, err := parseReferenceCurl(detail.Curl)
	if err != nil {
		return nil, err
	}
	if !isAllowedSeaCloudReferenceURL(parsed.URL) {
		return nil, unsupportedReferenceCurlError(detail.ModelID)
	}
	allowAdditional := true
	contract := &contracts.ModelContract{
		SchemaVersion: contracts.SupportedSchemaVersion,
		ModelID:       detail.ModelID,
		DisplayName:   detail.ModelID,
		Kind:          "llm",
		InputSchema:   freeformInputSchema(allowAdditional),
	}
	switch parsed.Path {
	case "/llm/chat/completions":
		contract.Protocol = llm.ProtocolChatCompletions
		contract.BodyMode = llm.BodyModeChatJSON
		contract.Endpoints.ChatCompletions = contracts.Endpoint{Method: http.MethodPost, Path: parsed.Path}
	case "/llm/responses":
		contract.Protocol = llm.ProtocolResponses
		contract.BodyMode = llm.BodyModeResponsesJSON
		contract.Endpoints.Responses = contracts.Endpoint{Method: http.MethodPost, Path: parsed.Path}
	default:
		return nil, unsupportedReferenceCurlError(detail.ModelID)
	}
	return contract, nil
}

func freeformInputSchema(allowAdditional bool) contracts.InputSchema {
	return contracts.InputSchema{
		Type:                 "object",
		AdditionalProperties: &allowAdditional,
		Properties:           map[string]contracts.InputSchema{},
	}
}

func parseReferenceCurl(command string) (*parsedFallbackCurl, error) {
	tokens, err := splitCurlCommand(command)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 || tokens[0] != "curl" {
		return nil, fmt.Errorf("fallback curl is reference-only: expected a curl command")
	}
	parsed := &parsedFallbackCurl{Method: http.MethodPost, Headers: map[string]string{}}
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "-X" || token == "--request":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("fallback curl is reference-only: missing request method")
			}
			parsed.Method = strings.ToUpper(tokens[i])
		case token == "-H" || token == "--header":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("fallback curl is reference-only: missing header value")
			}
			addReferenceHeader(parsed.Headers, tokens[i])
		case strings.HasPrefix(token, "-H"):
			addReferenceHeader(parsed.Headers, strings.TrimPrefix(token, "-H"))
		case token == "-d" || token == "--data" || token == "--data-raw" || token == "--data-binary" || token == "--data-urlencode":
			i++
			if i >= len(tokens) || strings.HasPrefix(tokens[i], "@") {
				return nil, fmt.Errorf("fallback curl is reference-only: file data is not supported")
			}
			parsed.Body = tokens[i]
		case strings.HasPrefix(token, "--data=") || strings.HasPrefix(token, "--data-raw=") || strings.HasPrefix(token, "--data-binary="):
			value := token[strings.Index(token, "=")+1:]
			if strings.HasPrefix(value, "@") {
				return nil, fmt.Errorf("fallback curl is reference-only: file data is not supported")
			}
			parsed.Body = value
		case token == "-F" || token == "--form" || strings.HasPrefix(token, "-F") || strings.HasPrefix(token, "--form="):
			return nil, fmt.Errorf("fallback curl is reference-only: multipart form data is not supported")
		case token == "--url":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("fallback curl is reference-only: missing URL")
			}
			parsed.URL = tokens[i]
		case strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://"):
			parsed.URL = token
		case token == "-s" || token == "-sS" || token == "-L" || token == "--location":
			continue
		case strings.HasPrefix(token, "-"):
			return nil, fmt.Errorf("fallback curl is reference-only: unsupported curl option %s", token)
		}
	}
	if parsed.URL == "" {
		return nil, fmt.Errorf("fallback curl is reference-only: missing URL")
	}
	u, err := url.Parse(parsed.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("fallback curl is reference-only: invalid URL")
	}
	parsed.Path = u.EscapedPath()
	return parsed, nil
}

func splitCurlCommand(command string) ([]string, error) {
	if strings.Contains(command, "$(") || strings.Contains(command, "`") {
		return nil, fmt.Errorf("fallback curl is reference-only: shell substitution is not supported")
	}
	var tokens []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\n' || r == '\t' {
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("fallback curl is reference-only: invalid shell quoting")
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens, nil
}

func addReferenceHeader(headers map[string]string, raw string) {
	name, value, ok := strings.Cut(raw, ":")
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, "Authorization") || name == "" {
		return
	}
	headers[name] = strings.TrimSpace(value)
}

func isAllowedSeaCloudReferenceURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Host)
	for _, rawBase := range []string{
		"https://cloud.seaart.ai",
		fallbackSeaCloudBaseURL(),
		os.Getenv("SEACLOUD_GENERATION_URL"),
		os.Getenv("SEACLOUD_LLM_URL"),
		os.Getenv("SEACLOUD_BASE_URL"),
	} {
		if baseHost(rawBase) == host {
			return true
		}
	}
	return false
}

func baseHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

func unsupportedReferenceCurlError(modelID string) error {
	return fmt.Errorf("fallback curl is reference-only for %q; use seacloud --dry-run with --use-skill-model-fallback first, then search official provider documentation for unsupported curl shapes", modelID)
}
