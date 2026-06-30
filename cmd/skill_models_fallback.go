package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/auth"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
)

var errSkillModelsFallbackNotFound = errors.New("skill models fallback curl not found")

type fallbackScope string

const (
	fallbackScopeAny fallbackScope = "any"
	fallbackScopeLLM fallbackScope = "llm"
)

type skillModelsFallback struct {
	Kind           string
	ModelID        string
	DetailEndpoint string
	Curl           string
}

type fallbackSpecPayload struct {
	ContractFound                 bool     `json:"contract_found"`
	FallbackSource                string   `json:"fallback_source"`
	FallbackKind                  string   `json:"fallback_kind,omitempty"`
	ModelID                       string   `json:"model_id"`
	DetailEndpoint                string   `json:"detail_endpoint,omitempty"`
	ReferenceCurlAvailable        bool     `json:"reference_curl_available"`
	DirectCurlExecutionAllowed    bool     `json:"direct_curl_execution_allowed"`
	NextCLICommands               []string `json:"next_cli_commands,omitempty"`
	ReferenceCurl                 string   `json:"reference_curl,omitempty"`
	ReferenceOnly                 bool     `json:"reference_only,omitempty"`
	OfficialParameterDocsRequired bool     `json:"official_parameter_docs_required"`
	Message                       string   `json:"message,omitempty"`
}

func printSkillModelsFallbackSpec(modelID string, scope fallbackScope) error {
	detail, err := findSkillModelsFallback(modelID, scope)
	if err == nil {
		payload := fallbackSpecPayload{
			ContractFound:                 false,
			FallbackSource:                "skill_models",
			FallbackKind:                  detail.Kind,
			ModelID:                       detail.ModelID,
			DetailEndpoint:                detail.DetailEndpoint,
			ReferenceCurlAvailable:        true,
			DirectCurlExecutionAllowed:    false,
			NextCLICommands:               nextFallbackCLICommands(detail),
			OfficialParameterDocsRequired: true,
		}
		if modelsSpecShowReferenceCurl {
			payload.ReferenceCurl = detail.Curl
			payload.ReferenceOnly = true
		}
		if modelsSpecOutput == "json" {
			return printJSON(payload)
		}
		printFallbackDetail(detail)
		return nil
	}
	if !errors.Is(err, errSkillModelsFallbackNotFound) {
		return err
	}

	payload := missingFallbackPayload(modelID)
	if modelsSpecOutput == "json" {
		return printJSON(payload)
	}
	printMissingFallback(modelID)
	return nil
}

func skillModelsFallbackError(modelID string, scope fallbackScope) error {
	detail, err := findSkillModelsFallback(modelID, scope)
	if errors.Is(err, errSkillModelsFallbackNotFound) {
		return missingFallbackError(modelID)
	}
	if err != nil {
		return err
	}
	return fallbackCurlError(modelID, detail)
}

func findSkillModelsFallback(modelID string, scope fallbackScope) (*skillModelsFallback, error) {
	params := models.ListParams{
		PageSize:    500,
		Keywords:    modelID,
		IncludeCurl: true,
	}
	if scope == fallbackScopeLLM {
		params.Type = "llm"
	}
	result, err := models.List(params)
	if err != nil {
		return nil, err
	}
	for _, model := range result.Models {
		if !sameModelID(model, modelID) || strings.TrimSpace(model.Curl) == "" {
			continue
		}
		if scope == fallbackScopeLLM && strings.ToLower(strings.TrimSpace(model.Type)) != "llm" {
			continue
		}
		return &skillModelsFallback{
			Kind:           fallbackKindFromModel(model),
			ModelID:        preferredFallbackModelID(model, modelID),
			DetailEndpoint: skillModelsFallbackEndpoint(modelID, scope),
			Curl:           normalizeFallbackCurl(strings.TrimSpace(model.Curl)),
		}, nil
	}
	return nil, errSkillModelsFallbackNotFound
}

func normalizeFallbackCurl(curl string) string {
	base := fallbackSeaCloudBaseURL()
	if curl == "" || base == "" || base == "https://cloud.seaart.ai" {
		return curl
	}
	return strings.ReplaceAll(curl, "https://cloud.seaart.ai", base)
}

func fallbackSeaCloudBaseURL() string {
	base := strings.TrimSpace(auth.BaseURL)
	if env := strings.TrimSpace(os.Getenv("SEACLOUD_BASE_URL")); env != "" {
		base = env
	}
	return strings.TrimRight(base, "/")
}

func sameModelID(model models.Model, modelID string) bool {
	target := strings.TrimSpace(modelID)
	return strings.TrimSpace(model.ID) == target || strings.TrimSpace(model.ModelID) == target
}

func fallbackKindFromModel(model models.Model) string {
	if strings.ToLower(strings.TrimSpace(model.Type)) == "llm" {
		return "llm"
	}
	return "multimodal"
}

func preferredFallbackModelID(model models.Model, fallback string) string {
	if strings.TrimSpace(model.ModelID) != "" {
		return strings.TrimSpace(model.ModelID)
	}
	if strings.TrimSpace(model.ID) != "" {
		return strings.TrimSpace(model.ID)
	}
	return fallback
}

func skillModelsFallbackEndpoint(modelID string, scope fallbackScope) string {
	endpoint := fmt.Sprintf("/api/v1/skill/models?keywords=%s&page_size=500&include_curl=true", url.QueryEscape(modelID))
	if scope == fallbackScopeLLM {
		endpoint += "&type=llm"
	}
	return endpoint
}

func fallbackCurlError(modelID string, detail *skillModelsFallback) error {
	return &clierrors.CLIError{
		Message: fmt.Sprintf("model contract not found for %q; skill models reference curl is available but is reference-only.\n%s\nNext CLI step: %s",
			modelID, fallbackKindNote(detail.Kind), nextFallbackCLICommands(detail)[0]),
		Hint: officialDocsHint() + " Do not execute the reference curl directly; use seacloud with --use-skill-model-fallback first.",
	}
}

func missingFallbackError(modelID string) error {
	return &clierrors.CLIError{
		Message: fmt.Sprintf("No contract or skill model curl found for %q.", modelID),
		Hint:    officialDocsHint(),
	}
}

func missingFallbackPayload(modelID string) fallbackSpecPayload {
	return fallbackSpecPayload{
		ContractFound:                 false,
		FallbackSource:                "none",
		ModelID:                       modelID,
		ReferenceCurlAvailable:        false,
		DirectCurlExecutionAllowed:    false,
		OfficialParameterDocsRequired: true,
		Message:                       "No contract or skill model curl found. Search the official provider documentation before constructing a paid model call.",
	}
}

func printFallbackDetail(detail *skillModelsFallback) {
	fmt.Printf("Contract found: false\n")
	fmt.Printf("Fallback source: skill_models\n")
	fmt.Printf("Fallback kind: %s\n", detail.Kind)
	fmt.Printf("Model: %s\n", detail.ModelID)
	fmt.Printf("Detail endpoint: %s\n", detail.DetailEndpoint)
	fmt.Printf("%s\n", fallbackKindNote(detail.Kind))
	fmt.Println("Reference curl available: true")
	fmt.Println("Direct curl execution allowed: false")
	fmt.Println("Next CLI commands:")
	for _, command := range nextFallbackCLICommands(detail) {
		fmt.Printf("  %s\n", command)
	}
	if modelsSpecShowReferenceCurl {
		fmt.Println("Reference curl (reference only):")
		fmt.Println(detail.Curl)
	}
	fmt.Println("Official parameter docs required: true")
	fmt.Println(officialDocsHint())
}

func printMissingFallback(modelID string) {
	fmt.Printf("Contract found: false\n")
	fmt.Printf("Fallback source: none\n")
	fmt.Printf("Model: %s\n", modelID)
	fmt.Printf("Official parameter docs required: true\n")
	fmt.Println("No contract or skill model curl found.")
	fmt.Println(officialDocsHint())
}

func fallbackKindNote(kind string) string {
	switch kind {
	case "llm":
		return "LLM fallback must be tested through seacloud llm run before any reference curl fallback."
	case "multimodal":
		return "Multimodal fallback must be tested through seacloud run or run-async before any reference curl fallback."
	default:
		return "Fallback curl is reference-only and must not be executed directly by agents."
	}
}

func officialDocsHint() string {
	return "Search the official provider documentation for required parameters, enum values, media dimensions, formats, and request body shape before executing a paid model call."
}

func nextFallbackCLICommands(detail *skillModelsFallback) []string {
	modelID := shellSafeModelID(detail.ModelID)
	if detail.Kind == "llm" {
		return []string{
			fmt.Sprintf("seacloud --dry-run llm run %s --use-skill-model-fallback --param key=value", modelID),
			fmt.Sprintf("seacloud llm run %s --use-skill-model-fallback --param key=value", modelID),
			fmt.Sprintf("seacloud llm run %s --use-reference-curl --param key=value", modelID),
		}
	}
	return []string{
		fmt.Sprintf("seacloud --dry-run run %s --use-skill-model-fallback --param key=value", modelID),
		fmt.Sprintf("seacloud run %s --use-skill-model-fallback --param key=value --output json", modelID),
		fmt.Sprintf("seacloud run-async %s --use-skill-model-fallback --param key=value --output id", modelID),
		fmt.Sprintf("seacloud run %s --use-reference-curl --param key=value --output json", modelID),
	}
}

func shellSafeModelID(modelID string) string {
	return strings.ReplaceAll(modelID, "'", "")
}
