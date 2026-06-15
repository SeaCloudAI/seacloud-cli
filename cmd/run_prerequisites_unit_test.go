package cmd

import (
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func TestFillRawPrerequisitesAcceptsImageNoAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := taskcache.Save(taskcache.Metadata{
		RequestID:       "v81-upstream-123",
		ModelID:         "midjourney_V8.1_t2i",
		Protocol:        "queue",
		BodyMode:        "raw_json",
		ProviderContext: map[string]any{"task_id": "provider-task-123", "imageNo": 3},
	}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	got := fillRawPrerequisitesFromCache(map[string]string{
		"task_id": "midjourney_provider_task_id",
	}, []contracts.Prerequisite{
		{Field: "task_id", SourceModel: "midjourney_V8.1_t2i", SourcePath: "outputs[].task_id"},
		{Field: "image_no", SourceModel: "midjourney_V8.1_t2i", SourcePath: "outputs[].index"},
	})

	if got["task_id"] != "provider-task-123" || got["image_no"] != "3" {
		t.Fatalf("filled params = %#v", got)
	}
}
