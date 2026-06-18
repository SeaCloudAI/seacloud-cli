package contracts

import (
	"fmt"
	"strings"
)

func ValidateInputRules(modelID string, params map[string]any, rules []InputRule) error {
	for _, rule := range rules {
		if !ruleConditionMatches(params, rule.When) {
			continue
		}
		switch strings.TrimSpace(rule.Kind) {
		case "one_of", "at_least_one":
			if !hasAnyParam(params, rule.Fields) {
				return fmt.Errorf("%s requires one of: %s", modelID, strings.Join(rule.Fields, ", "))
			}
		case "requires":
			for _, field := range rule.Required {
				if !hasMeaningfulParam(params, field) {
					return fmt.Errorf("%s requires parameter %s", modelID, field)
				}
			}
		case "mutually_exclusive":
			present := presentParams(params, rule.Fields)
			if len(present) > 1 {
				return fmt.Errorf("%s parameters cannot be used together: %s", modelID, strings.Join(present, ", "))
			}
		}
	}
	return nil
}

func ruleConditionMatches(params map[string]any, when map[string]any) bool {
	if len(when) == 0 {
		return true
	}
	for field, want := range when {
		got, ok := params[field]
		if !ok || fmt.Sprint(got) != fmt.Sprint(want) {
			return false
		}
	}
	return true
}

func hasAnyParam(params map[string]any, fields []string) bool {
	for _, field := range fields {
		if hasMeaningfulParam(params, field) {
			return true
		}
	}
	return false
}

func presentParams(params map[string]any, fields []string) []string {
	var present []string
	for _, field := range fields {
		if hasMeaningfulParam(params, field) {
			present = append(present, field)
		}
	}
	return present
}

func hasMeaningfulParam(params map[string]any, field string) bool {
	value, ok := params[field]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}
