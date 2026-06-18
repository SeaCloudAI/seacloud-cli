package skillhub

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderSkillsTextIncludesVersionDownloadsAndCategory(t *testing.T) {
	var buf bytes.Buffer
	result := &SearchResult{
		Results: []SkillSummary{{
			Slug:        "image-gen",
			DisplayName: "Image Gen",
			Description: "Generate images",
			Version:     "1.2.3",
			Downloads:   42,
			Category:    "image",
			UpdatedAt:   1710000000,
		}},
		NextCursor: "next-page",
	}

	if err := renderSkillsText(&buf, result, "skill(s)"); err != nil {
		t.Fatalf("renderSkillsText returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Image Gen",
		"slug:",
		"image-gen",
		"version:",
		"1.2.3",
		"downloads:",
		"42",
		"category:",
		"image",
		"--cursor next-page",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSkillsJSONOutputsStableSummaryFields(t *testing.T) {
	var buf bytes.Buffer
	result := &SearchResult{
		Results: []SkillSummary{{
			Slug:          "prompt-kit",
			DisplayName:   "Prompt Kit",
			Description:   "Prompt helpers",
			DownloadCount: 7,
			Categories:    []string{"prompting", "agent"},
			LatestVersion: &SkillVersionSummary{Version: "0.9.0"},
			UpdatedAt:     1710000000,
		}},
		NextCursor: "cursor-2",
	}

	if err := renderSkillsJSON(&buf, result); err != nil {
		t.Fatalf("renderSkillsJSON returned error: %v", err)
	}

	var got struct {
		Results []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Downloads   int64  `json:"downloads"`
			Category    string `json:"category"`
			UpdatedAt   int64  `json:"updatedAt"`
		} `json:"results"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json output is invalid: %v\n%s", err, buf.String())
	}
	if len(got.Results) != 1 {
		t.Fatalf("results length = %d, want 1", len(got.Results))
	}
	item := got.Results[0]
	if item.Slug != "prompt-kit" || item.Version != "0.9.0" ||
		item.Downloads != 7 || item.Category != "prompting, agent" ||
		got.NextCursor != "cursor-2" || item.UpdatedAt != 1710000000 {
		t.Fatalf("unexpected json output: %#v next=%q", item, got.NextCursor)
	}
}

func TestSkillSummaryUnmarshalAcceptsFlexibleMetadataShapes(t *testing.T) {
	var skill SkillSummary
	payload := []byte(`{
		"slug":"image-gen",
		"displayName":"Image Gen",
		"description":"Generate images",
		"latestVersion":"1.2.3",
		"stats":{"downloads":42,"stars":10},
		"categories":[{"name":"image"},{"slug":"agent"}],
		"updatedAt":1710000000
	}`)

	if err := json.Unmarshal(payload, &skill); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if skill.LatestVersion == nil || skill.LatestVersion.Version != "1.2.3" {
		t.Fatalf("latestVersion = %#v, want 1.2.3", skill.LatestVersion)
	}
	if skill.Stats == nil || skill.Stats.Downloads != 42 {
		t.Fatalf("stats.downloads = %#v, want 42", skill.Stats)
	}
	if got := strings.Join(skill.Categories, ", "); got != "image, agent" {
		t.Fatalf("categories = %q, want image, agent", got)
	}

	var buf bytes.Buffer
	if err := renderSkillsJSON(&buf, &SearchResult{Results: []SkillSummary{skill}}); err != nil {
		t.Fatalf("renderSkillsJSON returned error: %v", err)
	}
	if !strings.Contains(buf.String(), `"downloads": 42`) {
		t.Fatalf("json output did not include stats downloads:\n%s", buf.String())
	}
}
