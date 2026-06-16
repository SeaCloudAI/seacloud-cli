package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestOptimizedHelpText(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			contains: []string{
				"SeaCloud CLI is an agent-facing multimodal execution CLI.",
				"account     Check balance and billing readiness",
				"agent       Describe SeaCloud CLI capabilities for agents",
				"sandbox     Manage and interact with sandboxes",
				"template    Manage sandbox templates",
				"run         Run a model and wait for result URLs or JSON",
				"Print what would be executed without making changes",
			},
		},
		{
			name: "account",
			args: []string{"account", "--help"},
			contains: []string{
				"Manage SeaCloud account balance checks for model execution.",
				"Use this when a model call may require paid credits",
				"balance     Show current account balance and currency",
				"Print what would be executed without making changes",
			},
		},
		{
			name: "account balance",
			args: []string{"account", "balance", "--help"},
			contains: []string{
				"Show the current SeaCloud account balance.",
				"Use \"--output json\" when an agent needs structured balance data",
				"top_up_url",
				"Output format: json",
			},
		},
		{
			name: "auth",
			args: []string{"auth", "--help"},
			contains: []string{
				"Manage SeaCloud credentials used for model calls",
				"status      Show whether SeaCloud credentials are configured and usable",
			},
		},
		{
			name: "auth status",
			args: []string{"auth", "status", "--help"},
			contains: []string{
				"Show whether SeaCloud credentials are configured and usable.",
			},
		},
		{
			name: "models",
			args: []string{"models", "--help"},
			contains: []string{
				"Browse available SeaCloud models and inspect model specs before calling them.",
				"list        List available SeaCloud models",
				"spec        Get the live model-contract.v1 parameter spec for a model",
			},
		},
		{
			name: "models list",
			args: []string{"models", "list", "--help"},
			contains: []string{
				"List available SeaCloud models with model IDs, names, types",
				"input_modalities   Accepted input types: Text | Image | Video | Audio",
				"output_modalities  Output types produced by the model",
			},
		},
		{
			name: "models spec",
			args: []string{"models", "spec", "--help"},
			contains: []string{
				"Get the live model-contract.v1 parameter spec for a model before constructing \"seacloud run\".",
				"schema_version  Contract schema version",
				"model_id        Model identifier",
				"body_mode       Request body mode",
				"input_schema    JSON Schema-style parameter definition",
			},
		},
		{
			name: "skills",
			args: []string{"skills", "--help"},
			contains: []string{
				"Search, install, and manage agent skills from SeaCloud SkillHub.",
				"Use SkillHub when the user task needs a specialized workflow",
			},
		},
		{
			name: "task",
			args: []string{"task", "--help"},
			contains: []string{
				"Manage SeaCloud generation tasks.",
				"Use this after an async model run returns a task ID.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetHelpFlags(rootCmd)
			t.Cleanup(func() { resetHelpFlags(rootCmd) })
			stdout, _, err := executeRoot(t, tt.args...)
			if err != nil {
				t.Fatalf("help command returned error: %v", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(stdout, want) {
					t.Fatalf("help output missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func resetHelpFlags(cmd *cobra.Command) {
	resetHelpFlagSet(cmd.Flags())
	resetHelpFlagSet(cmd.PersistentFlags())
	for _, child := range cmd.Commands() {
		resetHelpFlags(child)
	}
}

func resetHelpFlagSet(flags *pflag.FlagSet) {
	if flags == nil {
		return
	}
	if flag := flags.Lookup("help"); flag != nil {
		_ = flag.Value.Set("false")
		flag.Changed = false
	}
}
