package cmd

import (
	"fmt"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

func validateOutputFormat(flagName, value string, allowed ...string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return aiParamError(flagName, fmt.Sprintf("unsupported value %q; allowed values are: %s", value, strings.Join(allowed, ", ")), "Use --output json for Agent-safe structured output, or omit the flag for table output.")
}

func validateSandboxOutput() error {
	return validateOutputFormat("--output/--format", sandboxOpts.output, "json", "pretty", "table")
}

func validateTemplateOutput() error {
	if err := validateOutputFormat("--format", templateOpts.output, "json", "pretty", "table"); err != nil {
		return err
	}
	return validateOutputFormat("--output", sandboxOpts.output, "json", "pretty", "table")
}

func aiParamError(param, issue, next string) error {
	return &clierrors.CLIError{
		Message: fmt.Sprintf("invalid parameter %s: %s", param, issue),
		Hint:    next,
	}
}

func aiMissingParam(param, next string) error {
	return &clierrors.CLIError{
		Message: fmt.Sprintf("missing required parameter %s", param),
		Hint:    next,
	}
}

type dryRunPlan struct {
	DryRun       bool     `json:"dryRun"`
	Action       string   `json:"action"`
	Method       string   `json:"method,omitempty"`
	Path         string   `json:"path,omitempty"`
	Query        any      `json:"query,omitempty"`
	Body         any      `json:"body,omitempty"`
	IDs          []string `json:"ids,omitempty"`
	Destructive  bool     `json:"destructive,omitempty"`
	NoChanges    bool     `json:"noChangesMade"`
	PreviewNotes []string `json:"previewNotes,omitempty"`
	NextStep     string   `json:"nextStep,omitempty"`
}

func printDryRunPlan(plan dryRunPlan) error {
	plan.DryRun = true
	plan.NoChanges = true
	if plan.NextStep == "" {
		plan.NextStep = "Review this plan, then rerun without --dry-run to execute."
	}
	return printJSON(plan)
}
