package skillhub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BaseURL can be overridden at build time via ldflags:
// Or at runtime via the SEACLOUD_SKILLHUB_URL environment variable.
var BaseURL = ""

const defaultTimeout = 30 * time.Second

type Client struct {
	apiBaseURL string
	httpClient *http.Client
}

func NewClient() *Client {
	apiURL := os.Getenv("SEACLOUD_SKILLHUB_URL")
	if apiURL == "" {
		config, err := LoadConfig()
		if err == nil && config.APIBaseURL != "" {
			apiURL = config.APIBaseURL
		} else {
			apiURL = BaseURL
		}
	}

	return &Client{
		apiBaseURL: apiURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) get(path string) (*http.Response, error) {
	if strings.TrimSpace(c.apiBaseURL) == "" {
		return nil, fmt.Errorf("skillhub base URL not configured: set SEACLOUD_SKILLHUB_URL or rebuild with -ldflags")
	}
	resp, err := c.httpClient.Get(strings.TrimRight(c.apiBaseURL, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func (c *Client) downloadBinary(path string) ([]byte, error) {
	if strings.TrimSpace(c.apiBaseURL) == "" {
		return nil, fmt.Errorf("skillhub base URL not configured: set SEACLOUD_SKILLHUB_URL or rebuild with -ldflags")
	}
	resp, err := c.httpClient.Get(strings.TrimRight(c.apiBaseURL, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

type SkillDetail struct {
	Skill struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
	} `json:"skill"`
	LatestVersion struct {
		Version string `json:"version"`
	} `json:"latestVersion"`
}

func (c *Client) GetSkillDetail(slug string) (*SkillDetail, error) {
	resp, err := c.get(fmt.Sprintf("/skills/%s", slug))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Skill not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Get skill failed: HTTP %d", resp.StatusCode)
	}

	var detail SkillDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &detail, nil
}

func (c *Client) DownloadSkill(slug, version string) ([]byte, error) {
	path := fmt.Sprintf("/skills/%s/download", slug)
	if version != "" {
		path += "?version=" + version
	}

	return c.downloadBinary(path)
}

type SearchResult struct {
	Results    []SkillSummary `json:"results"`
	NextCursor string         `json:"nextCursor"`
}

type SkillSummary struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type skillsListResponse struct {
	Items      []SkillSummary `json:"items"`
	NextCursor string         `json:"nextCursor"`
}

func (c *Client) SearchSkills(query, category, cursor string) (*SearchResult, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", "20")
	if category != "" {
		q.Set("category", category)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	path := "/search?" + q.Encode()

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &result, nil
}

func (c *Client) ListSkills(category, sort, cursor string) (*SearchResult, error) {
	q := url.Values{}
	q.Set("limit", "20")
	if category != "" {
		q.Set("category", category)
	}
	if sort != "" {
		q.Set("sort", sort)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	path := "/skills?" + q.Encode()

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result skillsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &SearchResult{
		Results:    result.Items,
		NextCursor: result.NextCursor,
	}, nil
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func configFilePath() string {
	return filepath.Join(homeDir(), ".claude", "seacloud-skills-config.json")
}

type Config struct {
	APIBaseURL string `json:"api_base_url"`
	InstallDir string `json:"install_dir"`
	AuthToken  string `json:"auth_token,omitempty"`
}

func LoadConfig() (*Config, error) {
	configPath := configFilePath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				APIBaseURL: BaseURL,
			}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	configPath := configFilePath()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
