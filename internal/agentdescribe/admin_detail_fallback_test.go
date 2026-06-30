package agentdescribe

import "testing"

func TestBuildIncludesSkillModelsFallbackGuidance(t *testing.T) {
	desc := Build("test-version")

	var found bool
	for _, rule := range desc.EndpointRules {
		if rule.Title != "Proxy and endpoint rules" {
			continue
		}
		found = true
		for _, detail := range []string{
			"When model-contracts returns 404, do not execute fallback curl directly from the shell.",
			"Use seacloud --dry-run run <model_id> --use-skill-model-fallback before any real multimodal fallback call.",
			"Use seacloud --dry-run llm run <model_id> --use-skill-model-fallback for LLM fallback checks.",
			"Use --use-reference-curl only after the CLI-managed fallback fails; the CLI must load the stored API key or managed runtime token and redact credentials.",
			"If no usable skill model fallback is found, search the official provider documentation for required parameters, enum values, media dimensions, formats, and request body shape before any paid call.",
		} {
			if !ruleHasDetail(rule, detail) {
				t.Fatalf("expected endpoint fallback detail %q in %#v", detail, rule.Details)
			}
		}
	}
	if !found {
		t.Fatalf("expected Proxy and endpoint rules in %#v", desc.EndpointRules)
	}
}
