package skillhub

import (
	"encoding/json"
	"strconv"
)

func (s *SkillSummary) UnmarshalJSON(data []byte) error {
	var raw struct {
		Slug               string          `json:"slug"`
		DisplayName        string          `json:"displayName"`
		Description        string          `json:"description"`
		Version            any             `json:"version"`
		Downloads          any             `json:"downloads"`
		DownloadCount      any             `json:"downloadCount"`
		DownloadCountSnake any             `json:"download_count"`
		Category           any             `json:"category"`
		Categories         json.RawMessage `json:"categories"`
		LatestVersion      json.RawMessage `json:"latestVersion"`
		Stats              *SkillStats     `json:"stats"`
		UpdatedAt          int64           `json:"updatedAt"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.Slug = raw.Slug
	s.DisplayName = raw.DisplayName
	s.Description = raw.Description
	s.Version = stringValue(raw.Version)
	s.Downloads = int64Value(raw.Downloads)
	s.DownloadCount = int64Value(raw.DownloadCount)
	s.DownloadCountSnake = int64Value(raw.DownloadCountSnake)
	s.Category = categoryValue(raw.Category)
	s.Categories = categoriesValue(raw.Categories)
	s.LatestVersion = latestVersionValue(raw.LatestVersion)
	s.Stats = raw.Stats
	s.UpdatedAt = raw.UpdatedAt
	return nil
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}

func categoryValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		for _, key := range []string{"name", "slug", "displayName"} {
			if text := stringValue(v[key]); text != "" {
				return text
			}
		}
	default:
		return ""
	}
	return ""
}

func categoriesValue(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var stringsOnly []string
	if err := json.Unmarshal(raw, &stringsOnly); err == nil {
		return stringsOnly
	}

	var objects []map[string]any
	if err := json.Unmarshal(raw, &objects); err != nil {
		return nil
	}
	out := make([]string, 0, len(objects))
	for _, item := range objects {
		if value := categoryValue(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func latestVersionValue(raw json.RawMessage) *SkillVersionSummary {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var version string
	if err := json.Unmarshal(raw, &version); err == nil {
		return &SkillVersionSummary{Version: version}
	}
	var object struct {
		Version any `json:"version"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		if version = stringValue(object.Version); version != "" {
			return &SkillVersionSummary{Version: version}
		}
	}
	return nil
}
