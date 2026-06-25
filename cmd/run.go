package cmd

import (
	"github.com/SeaCloudAI/seacloud-cli/internal/models"
	"github.com/spf13/cobra"
)

var (
	runParams  []string
	runOutput  string
	runTimeout int
	runRefresh bool
	runStream  bool
)

var runCmd = &cobra.Command{
	Use:   "run <model_id>",
	Short: "Run a model and wait for result URLs or JSON",
	Long: `Submit a generation request and poll until the output is ready.

Parameters are passed as --param key=value pairs (repeatable).
Nested object fields use dot notation: --param camera_control.type=simple
Array fields use a JSON string: --param content='[{"type":"text","text":"hello"}]'

Values are coerced to the type declared in the model spec
(string / int / float / boolean / array). Enum and range constraints
are validated before the request is sent.

Exit codes:
  0   task succeeded
  1   error (validation, network, API, timeout)`,
	Example: `  seacloud run kling_v2_6_i2v --param image=https://example.com/cat.jpg
  seacloud run seedance_2_0 --param prompt="a cat running" --param duration=5
  seacloud run kling_v2_6_i2v --param mode=pro --output url
  seacloud run seedance_2_0 --param mode=pro --output json
  seacloud run gpt_4o_mini --stream --param messages='[{"role":"user","content":"hello"}]'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelID := args[0]
		resolvedModelID := models.ResolveModelID(modelID)
		return executeModelRun(modelID, resolvedModelID)
	},
}

func init() {
	runCmd.Flags().StringArrayVar(&runParams, "param", nil, "Parameter as key=value (repeatable)")
	runCmd.Flags().StringVar(&runOutput, "output", "", "Output format: url (URLs only), json (full response), sse (LLM stream only)")
	runCmd.Flags().IntVar(&runTimeout, "timeout", 600, "Maximum seconds to wait for result (default 10 minutes)")
	runCmd.Flags().BoolVar(&runRefresh, "refresh", false, "Refresh cached model contract before running")
	runCmd.Flags().BoolVar(&runStream, "stream", false, "Stream LLM output as it is generated")
}
