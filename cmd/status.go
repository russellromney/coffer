package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/models"
	"github.com/russellromney/coffer/internal/vault"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault status",
	Long: `Show the current status of the vault, including whether it's
initialized, locked/unlocked, and the active project.

Example:
  coffer status`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	fmt.Printf("Vault location: %s\n", cfg.DataDir)

	if !v.IsInitialized() {
		fmt.Println("Status: Not initialized")
		fmt.Println("\nRun 'coffer init' to create a new vault.")
		return nil
	}

	fmt.Println("Status: Initialized")

	// Show keychain status
	keychainAvailable := v.IsKeychainAvailable()
	keychainEnabled, _ := v.IsKeychainEnabled()
	if keychainAvailable {
		if keychainEnabled {
			fmt.Println("Keychain: Enabled")
		} else {
			fmt.Println("Keychain: Available (not enabled)")
		}
	} else {
		fmt.Println("Keychain: Not available")
	}

	if v.IsUnlocked() {
		fmt.Println("Lock state: Unlocked")
	} else {
		fmt.Println("Lock state: Locked")
		return nil
	}

	// Show active project if unlocked
	store, err := v.GetStore()
	if err != nil {
		return err
	}

	activeProjectID, err := store.GetConfig(models.ConfigActiveProject)
	if err == nil && activeProjectID != "" {
		project, err := store.GetProject(activeProjectID)
		if err == nil {
			fmt.Printf("Active project: %s\n", project.Name)
		}
	} else {
		fmt.Println("Active project: None (use 'coffer project use <name>')")
	}

	return nil
}
