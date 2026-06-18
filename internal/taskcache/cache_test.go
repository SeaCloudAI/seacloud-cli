package taskcache

import (
	"errors"
	"reflect"
	"testing"
)

func TestSaveAndLoadQueueMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	meta := Metadata{
		RequestID:        "req-123",
		ModelID:          "gpt_image_1",
		Protocol:         "queue",
		BodyMode:         "raw_json",
		ContractRevision: "local-1",
		StatusEndpoint:   "/model/v1/queue/gpt_image_1/requests/{request_id}/status",
		ResultEndpoint:   "/model/v1/queue/gpt_image_1/requests/{request_id}/response",
		ProviderContext:  map[string]any{"jobId": "provider-job-123", "imageNo": float64(2)},
	}
	if err := Save(meta); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := Load("req-123")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !reflect.DeepEqual(*got, meta) {
		t.Fatalf("unexpected metadata:\nwant %#v\ngot  %#v", meta, *got)
	}
}

func TestLoadMissingMetadataReturnsNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := Load("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
