package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTemplateDefinitionFromImage(t *testing.T) {
	original := templateBuildOpts
	t.Cleanup(func() { templateBuildOpts = original })

	templateBuildOpts.image = "python:3.13"
	template, source, err := buildTemplateDefinition()
	if err != nil {
		t.Fatalf("buildTemplateDefinition returned error: %v", err)
	}
	if source != "python:3.13" {
		t.Fatalf("unexpected source %q", source)
	}
	jsonText, err := templateJSONForDryRun(template)
	if err != nil {
		t.Fatalf("templateJSONForDryRun returned error: %v", err)
	}
	if !strings.Contains(jsonText, `"fromImage": "python:3.13"`) {
		t.Fatalf("expected image in template json, got %s", jsonText)
	}
}

func TestBuildTemplateDefinitionFindsDockerfile(t *testing.T) {
	original := templateBuildOpts
	t.Cleanup(func() { templateBuildOpts = original })
	t.Chdir(t.TempDir())

	if err := os.WriteFile("Dockerfile", []byte("FROM ubuntu:22.04\nRUN echo ok\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	template, source, err := buildTemplateDefinition()
	if err != nil {
		t.Fatalf("buildTemplateDefinition returned error: %v", err)
	}
	if source != "Dockerfile" {
		t.Fatalf("unexpected source %q", source)
	}
	jsonText, err := templateJSONForDryRun(template)
	if err != nil {
		t.Fatalf("templateJSONForDryRun returned error: %v", err)
	}
	if !strings.Contains(jsonText, `"fromImage": "ubuntu:22.04"`) || !strings.Contains(jsonText, `"RUN"`) {
		t.Fatalf("unexpected template json: %s", jsonText)
	}
}

func TestWriteTemplateProject(t *testing.T) {
	dir := t.TempDir()
	if err := writeTemplateProject(dir, "python", "demo", false, true); err != nil {
		t.Fatalf("writeTemplateProject returned error: %v", err)
	}
	for _, name := range []string{"template.py", "build_dev.py", "build_prod.py", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	if err := writeTemplateProject(dir, "python", "demo", false, true); err == nil {
		t.Fatal("expected overwrite without force to fail")
	}
}
