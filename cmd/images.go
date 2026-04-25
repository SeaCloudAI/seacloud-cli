package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	imageapi "github.com/SeaCloudAI/seacloud-cli/internal/images"
	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Generate images through the SeaCloud proxy",
}

var (
	imagesModel          string
	imagesPrompt         string
	imagesSize           string
	imagesResponseFormat string
	imagesOutput         string
	imagesTimeout        int
)

var imagesGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an image through the proxy-backed image API",
	Example: `  seacloud images generate --prompt="a blue cat" --output json
  seacloud images generate --model gpt-image-2 --prompt="a blue cat" --size 1024x1024
  seacloud images generate --prompt="a blue cat" --output url`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := imageapi.RequestFromValues(imagesModel, imagesPrompt, imagesSize, imagesResponseFormat)
		if err != nil {
			return err
		}

		if IsDryRun() {
			fmt.Fprintf(os.Stderr, "[dry-run] Would execute: POST <proxy>%s\n", imageapi.RouteGenerate)
			fmt.Fprintf(os.Stderr, "[dry-run] request=%+v\n", req)
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.APIKey == "" {
			return clierrors.ErrNoAPIKey()
		}

		timeout := time.Duration(imagesTimeout) * time.Second
		return executeSyncImageRequest(cfg.APIKey, req, imagesOutput, timeout)
	},
}

func executeSyncImageRequest(apiKey string, req imageapi.GenerateRequest, output string, timeout time.Duration) error {
	client := imageapi.NewClientWithTimeout(apiKey, timeout)
	resp, err := client.Generate(req)
	if err != nil {
		return clierrors.ErrSubmitFailed(err)
	}

	switch output {
	case "json":
		b, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(b))
		return nil
	case "url":
		urls, err := client.UploadResponseImages(resp)
		if err != nil {
			return clierrors.ErrSubmitFailed(err)
		}
		for _, u := range urls {
			fmt.Println(u)
		}
		return nil
	case "":
		fmt.Println(imageapi.Summary(resp))
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", output)
	}
}

func init() {
	imagesGenerateCmd.Flags().StringVar(&imagesModel, "model", imageapi.DefaultModel, "Image model ID")
	imagesGenerateCmd.Flags().StringVar(&imagesPrompt, "prompt", "", "Prompt to generate")
	imagesGenerateCmd.Flags().StringVar(&imagesSize, "size", imageapi.DefaultSize, "Image size")
	imagesGenerateCmd.Flags().StringVar(&imagesResponseFormat, "response-format", imageapi.DefaultResponseFormat, "Response format")
	imagesGenerateCmd.Flags().StringVar(&imagesOutput, "output", "", "Output format: url (assets CDN URLs), json (full response)")
	imagesGenerateCmd.Flags().IntVar(&imagesTimeout, "timeout", int((imageapi.DefaultTimeout / time.Second)), "Maximum seconds to wait for image generation")

	imagesCmd.AddCommand(imagesGenerateCmd)
}
