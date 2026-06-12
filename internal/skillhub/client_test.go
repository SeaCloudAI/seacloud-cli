package skillhub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchSkillsEncodesQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("q"); got != "image generation" {
			t.Fatalf("q = %q, want image generation", got)
		}
		if got := q.Get("category"); got != "ai ml" {
			t.Fatalf("category = %q, want ai ml", got)
		}
		if got := q.Get("cursor"); got != "next page" {
			t.Fatalf("cursor = %q, want next page", got)
		}
		if got := q.Get("limit"); got != "20" {
			t.Fatalf("limit = %q, want 20", got)
		}
		_ = json.NewEncoder(w).Encode(SearchResult{
			Results: []SkillSummary{{Slug: "image-gen", DisplayName: "Image Gen", Description: "Generate images"}},
		})
	}))
	defer server.Close()

	client := &Client{apiBaseURL: server.URL, httpClient: server.Client()}
	result, err := client.SearchSkills("image generation", "ai ml", "next page")
	if err != nil {
		t.Fatalf("SearchSkills returned error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Slug != "image-gen" {
		t.Fatalf("result = %#v, want image-gen", result)
	}
}

func TestListSkillsUsesSkillsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/skills" {
			t.Fatalf("path = %q, want /skills", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("category"); got != "multimodal-generation" {
			t.Fatalf("category = %q, want multimodal-generation", got)
		}
		if got := q.Get("sort"); got != "downloads" {
			t.Fatalf("sort = %q, want downloads", got)
		}
		if got := q.Get("limit"); got != "20" {
			t.Fatalf("limit = %q, want 20", got)
		}
		_ = json.NewEncoder(w).Encode(skillsListResponse{
			Items: []SkillSummary{{Slug: "wan27", DisplayName: "Wan 2.7", Description: "Image skill"}},
		})
	}))
	defer server.Close()

	client := &Client{apiBaseURL: server.URL, httpClient: server.Client()}
	result, err := client.ListSkills("multimodal-generation", "downloads", "")
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Slug != "wan27" {
		t.Fatalf("result = %#v, want wan27", result)
	}
}

func TestSkillHubClientRequiresBaseURL(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient}
	_, err := client.SearchSkills("image", "", "")
	if err == nil {
		t.Fatal("SearchSkills returned nil error, want base URL error")
	}
	if !strings.Contains(err.Error(), "skillhub base URL not configured") {
		t.Fatalf("error = %q, want base URL configuration hint", err.Error())
	}
}
