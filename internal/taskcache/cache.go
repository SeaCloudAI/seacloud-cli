package taskcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrNotFound = errors.New("task metadata not found")

type Metadata struct {
	RequestID        string         `json:"request_id"`
	ModelID          string         `json:"model_id"`
	Protocol         string         `json:"protocol"`
	BodyMode         string         `json:"body_mode"`
	ContractRevision string         `json:"contract_revision"`
	StatusEndpoint   string         `json:"status_endpoint"`
	ResultEndpoint   string         `json:"result_endpoint"`
	ProviderContext  map[string]any `json:"provider_context,omitempty"`
}

func Save(meta Metadata) error {
	path, err := metadataPath(meta.RequestID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Load(requestID string) (*Metadata, error) {
	path, err := metadataPath(requestID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func LatestByModel(modelID string) (*Metadata, error) {
	dir, err := metadataDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var latest *Metadata
	var latestMod time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta Metadata
		if err := json.Unmarshal(data, &meta); err != nil || meta.ModelID != modelID {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latest == nil || info.ModTime().After(latestMod) {
			copy := meta
			latest = &copy
			latestMod = info.ModTime()
		}
	}
	if latest == nil {
		return nil, ErrNotFound
	}
	return latest, nil
}

func metadataPath(requestID string) (string, error) {
	dir, err := metadataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, requestID+".json"), nil
}

func metadataDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "seacloud", "tasks"), nil
}
