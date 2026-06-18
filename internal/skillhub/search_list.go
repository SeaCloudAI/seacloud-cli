package skillhub

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

const skillOutputJSON = "json"

type skillsOutput struct {
	Results    []skillSummaryOutput `json:"results"`
	NextCursor string               `json:"nextCursor"`
}

type skillSummaryOutput struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Downloads   int64  `json:"downloads"`
	Category    string `json:"category"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func (c *Client) Find(query, category string, interactive bool, cursor string, output string) error {
	if interactive {
		return fmt.Errorf("interactive mode not implemented yet")
	}

	if output != skillOutputJSON {
		displayQuery := query
		if displayQuery == "" {
			displayQuery = "all"
		}
		if category != "" {
			fmt.Printf("🔍 Searching for \"%s\" in category \"%s\"\n\n", displayQuery, category)
		} else {
			fmt.Printf("🔍 Searching for \"%s\"\n\n", displayQuery)
		}
	}

	var result *SearchResult
	var err error
	if query == "" {
		result, err = c.ListSkills(category, "", cursor)
	} else {
		result, err = c.SearchSkills(query, category, cursor)
	}
	if err != nil {
		return err
	}
	return renderSkills(os.Stdout, result, "result(s)", output)
}

func (c *Client) List(category, sort string, output string) error {
	result, err := c.ListSkills(category, sort, "")
	if err != nil {
		return err
	}
	return renderSkills(os.Stdout, result, "skill(s)", output)
}

func renderSkills(w io.Writer, result *SearchResult, itemLabel string, output string) error {
	if output == skillOutputJSON {
		return renderSkillsJSON(w, result)
	}
	return renderSkillsText(w, result, itemLabel)
}

func renderSkillsText(w io.Writer, result *SearchResult, itemLabel string) error {
	if len(result.Results) == 0 {
		_, err := fmt.Fprintln(w, "No skills found.")
		return err
	}

	if _, err := fmt.Fprintf(w, "Found %d %s\n\n", len(result.Results), itemLabel); err != nil {
		return err
	}
	for _, skill := range result.Results {
		summary := normalizeSkillSummary(skill)
		if _, err := fmt.Fprintf(w, "%s\n", color.CyanString(summary.DisplayName)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s\n", summary.Description); err != nil {
			return err
		}
		lines := []struct {
			label string
			value string
		}{
			{label: "slug:", value: color.YellowString(summary.Slug)},
			{label: "version:", value: valueOrUnknown(summary.Version)},
			{label: "downloads:", value: fmt.Sprintf("%d", summary.Downloads)},
			{label: "category:", value: valueOrUnknown(summary.Category)},
		}
		for _, line := range lines {
			if _, err := fmt.Fprintf(w, "  %s • %s\n",
				color.New(color.FgHiBlack).Sprint(line.label),
				line.value,
			); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if result.NextCursor != "" {
		_, err := fmt.Fprintf(w, "To view more results, use: --cursor %s\n", result.NextCursor)
		return err
	}
	return nil
}

func renderSkillsJSON(w io.Writer, result *SearchResult) error {
	out := skillsOutput{
		Results:    make([]skillSummaryOutput, 0, len(result.Results)),
		NextCursor: result.NextCursor,
	}
	for _, skill := range result.Results {
		out.Results = append(out.Results, normalizeSkillSummary(skill))
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func normalizeSkillSummary(skill SkillSummary) skillSummaryOutput {
	version := skill.Version
	if version == "" && skill.LatestVersion != nil {
		version = skill.LatestVersion.Version
	}
	downloads := skill.Downloads
	if downloads == 0 && skill.Stats != nil {
		downloads = skill.Stats.Downloads
	}
	if downloads == 0 {
		downloads = skill.DownloadCount
	}
	if downloads == 0 {
		downloads = skill.DownloadCountSnake
	}
	category := skill.Category
	if category == "" && len(skill.Categories) > 0 {
		category = strings.Join(skill.Categories, ", ")
	}
	return skillSummaryOutput{
		Slug:        skill.Slug,
		DisplayName: skill.DisplayName,
		Description: skill.Description,
		Version:     version,
		Downloads:   downloads,
		Category:    category,
		UpdatedAt:   skill.UpdatedAt,
	}
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
