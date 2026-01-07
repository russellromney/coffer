package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/vault"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock the vault",
	Long: `Lock the vault and clear the session.

This destroys your current session, requiring you to enter your
master password again to access secrets.

Example:
  coffer lock`,
	RunE: runLock,
}

func init() {
	rootCmd.AddCommand(lockCmd)
}

func runLock(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	// Check if initialized
	if !v.IsInitialized() {
		return fmt.Errorf("vault not initialized: run 'coffer init' first")
	}

	// Lock vault
	if err := v.Lock(); err != nil {
		return fmt.Errorf("failed to lock vault: %w", err)
	}

	fmt.Println("Vault locked")
	return nil
}
