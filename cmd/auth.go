package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/SeaCloudAI/seacloud-cli/internal/auth"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage credentials for SeaCloud model and SkillHub access",
	Long:  "Manage SeaCloud credentials used for model calls",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your SeaCloud account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Managed {
			fmt.Printf("Authentication is managed by the %s runtime. Interactive login skipped.\n", managedRuntimeName(cfg))
			return nil
		}
		if cfg.AuthToken != "" {
			me, err := auth.VerifyToken(cfg.AuthToken)
			if err == nil {
				fmt.Printf("Already logged in as %s. Run seacloud auth logout to switch accounts.\n", me.Email)
				return nil
			}
		}

		token, refreshToken, apiKey, err := auth.Login(openBrowser)
		if err != nil {
			return err
		}

		me, err := auth.VerifyToken(token)
		if err != nil {
			return clierrors.ErrTokenVerification(err)
		}

		if err := config.Save(&config.Config{AuthToken: token, RefreshToken: refreshToken, APIKey: apiKey}); err != nil {
			return clierrors.ErrSaveConfig(err)
		}

		email := me.Email
		if email == "" {
			email = me.Account
		}
		fmt.Printf("\nLogged in as %s\n", email)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether SeaCloud credentials are configured and usable",
	Long:  "Show whether SeaCloud credentials are configured and usable.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}
		if cfg.Managed {
			fmt.Printf("Authenticated via managed runtime: %s\n", managedRuntimeName(cfg))
			if cfg.CredentialSource != "" {
				fmt.Printf("Credential source: %s\n", cfg.CredentialSource)
			}
			fmt.Println("Interactive login is disabled in this environment.")
			return nil
		}

		if cfg.AuthToken == "" {
			fmt.Println("Not logged in.")
			fmt.Println("  Hint: Run: seacloud auth login")
			return nil
		}

		me, err := auth.VerifyToken(cfg.AuthToken)
		if err != nil {
			fmt.Println("Token expired or invalid.")
			fmt.Println("  Hint: Run: seacloud auth login")
			return nil
		}

		email := me.Email
		if email == "" {
			email = me.Account
		}
		fmt.Printf("Logged in as %s\n", email)
		if me.Name != "" {
			fmt.Printf("Name: %s\n", me.Name)
		}
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your SeaCloud account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Managed {
			fmt.Printf("Authentication is managed by the %s runtime. No local credentials were removed.\n", managedRuntimeName(cfg))
			return nil
		}
		if err := config.Clear(); err != nil {
			return clierrors.ErrLogout(err)
		}
		fmt.Println("Logged out.")
		return nil
	},
}

var authSetKeyCmd = &cobra.Command{
	Use:   "set-key <api-key>",
	Short: "Set or replace the SeaCloud API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newKey := args[0]
		if newKey == "" {
			return fmt.Errorf("API key cannot be empty")
		}

		cfg, err := config.Load()
		if err != nil {
			cfg, err = config.LoadStored()
			if err != nil {
				cfg = &config.Config{}
			}
		}
		if cfg.Managed {
			return clierrors.ErrManagedCredentialsOverride()
		}

		cfg.APIKey = newKey
		if err := config.Save(cfg); err != nil {
			return clierrors.ErrSaveConfig(err)
		}

		fmt.Println("API key updated.")
		return nil
	},
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported operating system")
	}

	return exec.Command(cmd, args...).Start()
}

func managedRuntimeName(cfg *config.Config) string {
	if cfg != nil && cfg.Runtime != "" {
		return cfg.Runtime
	}
	return "managed"
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authSetKeyCmd)
}
