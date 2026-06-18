package agentdescribe

import (
	"fmt"
	"strings"
)

func RenderMarkdown(desc Description) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# SeaCloud CLI Agent Guide\n\n")
	if desc.CLIVersion != "" {
		fmt.Fprintf(&b, "CLI version: `%s`\n\n", desc.CLIVersion)
	}
	if desc.SchemaVersion != "" {
		fmt.Fprintf(&b, "Schema version: `%s`\n\n", desc.SchemaVersion)
	}
	fmt.Fprintf(&b, "%s\n\n", desc.Summary)

	writeCommandExamples(&b, "First Steps", desc.FirstSteps)

	fmt.Fprintf(&b, "## Capabilities\n\n")
	for _, capability := range desc.Capabilities {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", capability.ID, capability.Summary)
		for _, command := range capability.Commands {
			writeCommand(&b, command)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## Recommended Workflows\n\n")
	for _, workflow := range desc.Workflows {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", workflow.Title, workflow.Summary)
		for i, step := range workflow.Steps {
			fmt.Fprintf(&b, "%d. `%s`", i+1, step.Command)
			if step.Purpose != "" {
				fmt.Fprintf(&b, " - %s", step.Purpose)
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintln(&b)
	}

	writeRules(&b, "Parameter Rules", desc.ParameterRules)
	writeRules(&b, "Output Rules", desc.OutputRules)
	writeRules(&b, "Proxy and Endpoint Rules", desc.EndpointRules)
	writeRecovery(&b, desc.Recovery)

	return b.String()
}

func writeCommandExamples(b *strings.Builder, title string, examples []CommandExample) {
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, example := range examples {
		writeCommand(b, example)
	}
	fmt.Fprintln(b)
}

func writeCommand(b *strings.Builder, example CommandExample) {
	fmt.Fprintf(b, "- `%s`", example.Command)
	if example.Purpose != "" {
		fmt.Fprintf(b, " - %s", example.Purpose)
	}
	fmt.Fprintln(b)
}

func writeRules(b *strings.Builder, title string, rules []Rule) {
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, rule := range rules {
		fmt.Fprintf(b, "### %s\n\n", rule.Title)
		for _, detail := range rule.Details {
			fmt.Fprintf(b, "- %s\n", detail)
		}
		fmt.Fprintln(b)
	}
}

func writeRecovery(b *strings.Builder, cases []RecoveryCase) {
	fmt.Fprintf(b, "## Recovery\n\n")
	for _, recovery := range cases {
		fmt.Fprintf(b, "### %s\n\n", recovery.Problem)
		for _, action := range recovery.Actions {
			fmt.Fprintf(b, "- %s\n", action)
		}
		fmt.Fprintln(b)
	}
}
