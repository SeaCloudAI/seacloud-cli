package skillhub

import (
	"fmt"

	"github.com/fatih/color"
)

func (c *Client) Config(setURL string, show bool) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	if setURL != "" {
		config.APIBaseURL = setURL
		if err := SaveConfig(config); err != nil {
			return err
		}
		fmt.Printf("%s API URL updated to: %s\n",
			color.GreenString("✓"),
			color.CyanString(setURL),
		)
		return nil
	}

	if show {
		fmt.Println(color.New(color.Bold).Sprint("Current Configuration:"))
		fmt.Printf("  %s %s\n",
			color.New(color.FgHiBlack).Sprint("API URL:"),
			color.CyanString(config.APIBaseURL),
		)
		fmt.Printf("  %s %s\n",
			color.New(color.FgHiBlack).Sprint("Config file:"),
			color.CyanString(configFilePath()),
		)
		return nil
	}

	return fmt.Errorf("use --set-url or --show")
}
