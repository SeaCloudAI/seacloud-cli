package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func fillRawPrerequisitesFromCache(raw map[string]string, prerequisites []contracts.Prerequisite) map[string]string {
	if len(prerequisites) == 0 {
		return raw
	}
	next := map[string]string{}
	for key, value := range raw {
		next[key] = value
	}
	latestByModel := map[string]*taskcache.Metadata{}
	for _, prerequisite := range prerequisites {
		if !shouldFillPrerequisite(next[prerequisite.Field]) {
			continue
		}
		meta, ok := latestByModel[prerequisite.SourceModel]
		if !ok {
			loaded, err := taskcache.LatestByModel(prerequisite.SourceModel)
			if err != nil {
				latestByModel[prerequisite.SourceModel] = nil
				continue
			}
			meta = loaded
			latestByModel[prerequisite.SourceModel] = loaded
		}
		if meta == nil {
			continue
		}
		if value, ok := providerContextValue(meta.ProviderContext, prerequisite); ok {
			next[prerequisite.Field] = stringifyProviderContextValue(value)
		}
	}
	return next
}

func saveQueueProviderContext(requestID string, task *queue.Task) {
	context := providerContextFromQueueTask(task)
	if len(context) == 0 {
		return
	}
	meta, err := taskcache.Load(requestID)
	if errors.Is(err, taskcache.ErrNotFound) {
		meta = &taskcache.Metadata{RequestID: requestID, ModelID: task.Model, Protocol: "queue", BodyMode: "raw_json"}
	} else if err != nil {
		return
	}
	meta.ProviderContext = context
	_ = taskcache.Save(*meta)
}

func providerContextFromQueueTask(task *queue.Task) map[string]any {
	if task == nil {
		return nil
	}
	context := map[string]any{}
	mergeProviderMetadata(context, task.Metadata)
	for i, output := range task.Outputs {
		mergeProviderOutput(context, output, i)
		mergeProviderMetadata(context, output.Metadata)
	}
	for _, group := range task.Output {
		for _, content := range group.Content {
			mergeProviderMetadata(context, content.Metadata)
		}
	}
	if len(context) == 0 {
		return nil
	}
	return context
}

func mergeProviderOutput(dst map[string]any, output queue.Output, index int) {
	hasProviderID := false
	if output.JobID != "" {
		mergeProviderValue(dst, "jobId", output.JobID)
		hasProviderID = true
	}
	if output.TaskID != "" {
		mergeProviderValue(dst, "task_id", output.TaskID)
		hasProviderID = true
	}
	if output.ImageNo != nil {
		mergeProviderValue(dst, "imageNo", *output.ImageNo)
	}
	if output.ImageNoAlt != nil {
		mergeProviderValue(dst, "image_no", *output.ImageNoAlt)
	}
	if hasProviderID {
		mergeProviderValue(dst, "imageNo", index)
		mergeProviderValue(dst, "image_no", index)
	}
}

func mergeProviderMetadata(dst map[string]any, metadata map[string]any) {
	for key, value := range metadata {
		if nested, ok := value.(map[string]any); ok && isProviderContextContainer(key) {
			mergeProviderMetadata(dst, nested)
			continue
		}
		mergeProviderValue(dst, key, value)
	}
}

func mergeProviderValue(dst map[string]any, key string, value any) {
	if _, exists := dst[key]; exists || isEmptyProviderContextValue(value) {
		return
	}
	dst[key] = value
}

func providerContextValue(context map[string]any, prerequisite contracts.Prerequisite) (any, bool) {
	if len(context) == 0 {
		return nil, false
	}
	if value, ok := context[prerequisite.Field]; ok {
		return value, true
	}
	key := providerContextKey(prerequisite)
	for _, candidate := range providerContextKeyAliases(key, prerequisite.Field) {
		if value, ok := context[candidate]; ok {
			return value, true
		}
	}
	return nil, false
}

func providerContextKey(prerequisite contracts.Prerequisite) string {
	const marker = ".metadata."
	if idx := strings.LastIndex(prerequisite.SourcePath, marker); idx >= 0 {
		return prerequisite.SourcePath[idx+len(marker):]
	}
	if idx := strings.LastIndex(prerequisite.SourcePath, "."); idx >= 0 {
		return prerequisite.SourcePath[idx+1:]
	}
	return prerequisite.Field
}

func shouldFillPrerequisite(value string) bool {
	return strings.TrimSpace(value) == "" || contracts.IsPlaceholderPrerequisiteValue(value)
}

func providerContextKeyAliases(keys ...string) []string {
	seen := map[string]bool{}
	var aliases []string
	for _, key := range keys {
		for _, alias := range []string{key, snakeToLowerCamel(key), lowerCamelToSnake(key)} {
			if alias == "" || seen[alias] {
				continue
			}
			seen[alias] = true
			aliases = append(aliases, alias)
		}
	}
	return aliases
}

func snakeToLowerCamel(value string) string {
	idx := strings.Index(value, "_")
	if idx < 0 || idx == len(value)-1 {
		return value
	}
	return value[:idx] + strings.ToUpper(value[idx+1:idx+2]) + value[idx+2:]
}

func lowerCamelToSnake(value string) string {
	var b strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func stringifyProviderContextValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case float64:
		if math.Trunc(typed) == typed {
			return fmt.Sprintf("%.0f", typed)
		}
		return fmt.Sprintf("%v", typed)
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func isProviderContextContainer(key string) bool {
	switch key {
	case "metadata", "provider", "provider_context":
		return true
	default:
		return false
	}
}

func isEmptyProviderContextValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
