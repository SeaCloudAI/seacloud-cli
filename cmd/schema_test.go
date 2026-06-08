package cmd

import "testing"

func TestSchemaRegistryIncludesAgentCriticalMethods(t *testing.T) {
	registry := schemaRegistry()
	for _, name := range []string{
		"sandboxes.create",
		"sandboxes.list",
		"webhooks.create",
		"volumes.create",
		"teams.metrics",
		"templates.build",
	} {
		item, ok := registry[name]
		if !ok {
			t.Fatalf("missing schema %s", name)
		}
		if item.Method == "" || item.Path == "" || len(item.Response) == 0 || item.NextStep == "" {
			t.Fatalf("schema %s is incomplete: %+v", name, item)
		}
	}
}

func TestDestructiveSchemasHaveDryRunExamples(t *testing.T) {
	registry := schemaRegistry()
	for _, name := range []string{"sandboxes.delete", "volumes.delete", "templates.delete"} {
		item := registry[name]
		if !item.Destructive {
			t.Fatalf("%s should be destructive", name)
		}
		if item.DryRunExample == "" {
			t.Fatalf("%s should include a dry-run example", name)
		}
	}
}
