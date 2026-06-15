package contracts

import (
	"fmt"
	"strings"
)

func ValidatePrerequisites(modelID string, params map[string]any, prerequisites []Prerequisite) error {
	for _, prerequisite := range prerequisites {
		value, ok := params[prerequisite.Field]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok || !isPlaceholderPrerequisiteValue(text) {
			continue
		}
		return fmt.Errorf(
			"%s.%s requires real upstream context from %s (%s), got placeholder %q",
			modelID,
			prerequisite.Field,
			prerequisite.SourceModel,
			contextSource(prerequisite),
			text,
		)
	}
	return nil
}

func IsPlaceholderPrerequisiteValue(value string) bool {
	return isPlaceholderPrerequisiteValue(value)
}

func contextSource(prerequisite Prerequisite) string {
	if prerequisite.SourcePath != "" {
		return prerequisite.SourcePath
	}
	if prerequisite.ContextKind != "" {
		return prerequisite.ContextKind
	}
	return "provider task metadata"
}

func isPlaceholderPrerequisiteValue(value string) bool {
	switch strings.TrimSpace(value) {
	case "midjourney_provider_task_id",
		"midjourney_provider_job_id",
		"midjourney_draft_mode_job_id",
		"provider_task_id":
		return true
	default:
		return false
	}
}
