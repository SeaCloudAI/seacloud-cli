package contracts

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
)

func saveCached(modelID string, contract *ModelContract) error {
	path, err := cachePath(modelID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadCached(modelID string) (*ModelContract, error) {
	path, err := cachePath(modelID)
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
	var contract ModelContract
	if err := json.Unmarshal(data, &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

func cachePath(modelID string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "seacloud", "contracts", url.PathEscape(modelID)+".json"), nil
}
