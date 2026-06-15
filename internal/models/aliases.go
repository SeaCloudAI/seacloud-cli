package models

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type aliasPrefixRule struct {
	DisplayPrefix string `yaml:"display_prefix"`
	BackendPrefix string `yaml:"backend_prefix"`
}

type aliasConfig struct {
	Exact    map[string]string `yaml:"exact"`
	Prefixes []aliasPrefixRule `yaml:"prefixes"`
}

//go:embed model_aliases.yaml
var embeddedAliasConfig []byte

var modelAliasConfig = mustLoadAliasConfig()

var reverseExplicitModelAliases = reverseModelAliases(modelAliasConfig.Exact)

const seaCloudSourcePrefix = "seacloud__"

func ResolveModelID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	modelID = strings.TrimPrefix(modelID, seaCloudSourcePrefix)

	if resolved, ok := modelAliasConfig.Exact[modelID]; ok {
		return resolved
	}

	for _, rule := range modelAliasConfig.Prefixes {
		if strings.HasPrefix(modelID, rule.DisplayPrefix) {
			return rule.BackendPrefix + strings.TrimPrefix(modelID, rule.DisplayPrefix)
		}
	}

	return modelID
}

func DisplayModelID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}

	if display, ok := reverseExplicitModelAliases[modelID]; ok {
		return display
	}

	for _, rule := range modelAliasConfig.Prefixes {
		if strings.HasPrefix(modelID, rule.BackendPrefix) {
			return rule.DisplayPrefix + strings.TrimPrefix(modelID, rule.BackendPrefix)
		}
	}

	return modelID
}

func PreferredModelID(requestedModelID, backendModelID string) string {
	requestedModelID = strings.TrimSpace(requestedModelID)
	backendModelID = strings.TrimSpace(backendModelID)

	if requestedModelID == "" {
		return DisplayModelID(backendModelID)
	}
	if backendModelID == "" || requestedModelID == backendModelID {
		return requestedModelID
	}
	if ResolveModelID(requestedModelID) == backendModelID {
		return requestedModelID
	}
	return DisplayModelID(backendModelID)
}

func RewriteModelIDText(text, backendModelID, displayModelID string) string {
	backendModelID = strings.TrimSpace(backendModelID)
	displayModelID = strings.TrimSpace(displayModelID)

	if text == "" || backendModelID == "" || displayModelID == "" || backendModelID == displayModelID {
		return text
	}
	return strings.ReplaceAll(text, backendModelID, displayModelID)
}

func reverseModelAliases(aliases map[string]string) map[string]string {
	reversed := make(map[string]string, len(aliases))
	for display, backend := range aliases {
		reversed[backend] = display
	}
	return reversed
}

func mustLoadAliasConfig() aliasConfig {
	var cfg aliasConfig
	if err := yaml.Unmarshal(embeddedAliasConfig, &cfg); err != nil {
		panic(fmt.Sprintf("load model alias config: %v", err))
	}
	if cfg.Exact == nil {
		cfg.Exact = map[string]string{}
	}
	for i := range cfg.Prefixes {
		cfg.Prefixes[i].DisplayPrefix = strings.TrimSpace(cfg.Prefixes[i].DisplayPrefix)
		cfg.Prefixes[i].BackendPrefix = strings.TrimSpace(cfg.Prefixes[i].BackendPrefix)
		if cfg.Prefixes[i].DisplayPrefix == "" || cfg.Prefixes[i].BackendPrefix == "" {
			panic("load model alias config: empty display_prefix/backend_prefix")
		}
	}
	return cfg
}
