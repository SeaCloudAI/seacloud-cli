package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/account"
	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
	"github.com/SeaCloudAI/seacloud-cli/internal/config"
	"github.com/spf13/cobra"
)

const topUpURL = "https://cloud.seaart.ai/settings/credits"

var accountBalanceOutput string

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage account information",
}

var accountBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show current account balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOutputFormat("--output", accountBalanceOutput, "json"); err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.AuthToken) == "" {
			return clierrors.ErrNotLoggedIn()
		}

		balance, err := account.NewClient(cfg.AuthToken).GetBalance()
		if err != nil {
			return err
		}
		return printAccountBalance(balance, accountBalanceOutput)
	},
}

type accountBalanceJSON struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
	TopUpURL string  `json:"top_up_url"`
}

func init() {
	accountBalanceCmd.Flags().StringVar(&accountBalanceOutput, "output", "", "Output format (json)")
	accountCmd.AddCommand(accountBalanceCmd)
}

func printAccountBalance(balance *account.Balance, output string) error {
	amount, err := strconv.ParseFloat(strings.TrimSpace(balance.Balance), 64)
	if err != nil {
		return fmt.Errorf("invalid balance %q: %w", balance.Balance, err)
	}

	currency := strings.TrimSpace(balance.Currency)
	if currency == "" {
		currency = "USD"
	}
	if output == "json" {
		return printJSON(accountBalanceJSON{
			Balance:  amount,
			Currency: currency,
			TopUpURL: topUpURL,
		})
	}

	fmt.Printf("Balance: %s\n", formatAccountBalance(amount, currency))
	if amount > 0 {
		fmt.Printf("Top up: %s\n", topUpURL)
		return nil
	}
	fmt.Printf("  Hint: Account overdue. Top up at: %s\n", topUpURL)
	return nil
}

func formatAccountBalance(amount float64, currency string) string {
	if currency == "USD" {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%s %.2f", currency, amount)
}
