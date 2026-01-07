package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/vault"
)

var keychainCmd = &cobra.Command{
	Use:   "keychain",
	Short: "Manage OS keychain integration",
	Long: `Manage OS keychain integration for passwordless unlock.

When enabled, your master key is stored in the OS keychain
(macOS Keychain, Windows Credential Manager, or Linux Secret Service).
This allows you to unlock the vault without entering your password.

Examples:
  coffer keychain status
  coffer keychain enable
  coffer keychain disable`,
}

var keychainStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show keychain status",
	Long: `Show whether keychain integration is enabled and available.

Example:
  coffer keychain status`,
	RunE: runKeychainStatus,
}

var keychainEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable keychain integration",
	Long: `Enable keychain integration for passwordless unlock.

You'll be prompted to enter your master password to verify it
before storing the key in the keychain.

Example:
  coffer keychain enable`,
	RunE: runKeychainEnable,
}

var keychainDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable keychain integration",
	Long: `Disable keychain integration and remove the key from the keychain.

Example:
  coffer keychain disable`,
	RunE: runKeychainDisable,
}

func init() {
	rootCmd.AddCommand(keychainCmd)
	keychainCmd.AddCommand(keychainStatusCmd)
	keychainCmd.AddCommand(keychainEnableCmd)
	keychainCmd.AddCommand(keychainDisableCmd)
}

func runKeychainStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	if !v.IsInitialized() {
		return fmt.Errorf("vault not initialized: run 'coffer init' first")
	}

	// Check if keychain is available
	available := v.IsKeychainAvailable()
	fmt.Printf("Keychain available: %v\n", available)

	if !available {
		fmt.Println("\nKeychain is not available on this system.")
		return nil
	}

	// Check if keychain is enabled
	enabled, err := v.IsKeychainEnabled()
	if err != nil {
		return err
	}
	fmt.Printf("Keychain enabled: %v\n", enabled)

	if enabled {
		fmt.Println("\nYou can unlock without a password using 'coffer unlock'.")
	} else {
		fmt.Println("\nEnable with 'coffer keychain enable' for passwordless unlock.")
	}

	return nil
}

func runKeychainEnable(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	if !v.IsInitialized() {
		return fmt.Errorf("vault not initialized: run 'coffer init' first")
	}

	if !v.IsKeychainAvailable() {
		return fmt.Errorf("keychain is not available on this system")
	}

	// Check if already enabled
	enabled, err := v.IsKeychainEnabled()
	if err != nil {
		return err
	}
	if enabled {
		fmt.Println("Keychain is already enabled")
		return nil
	}

	// Prompt for password
	fmt.Print("Enter master password to enable keychain: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Enable keychain
	if err := v.EnableKeychain(string(password)); err != nil {
		if err == vault.ErrInvalidPassword {
			return fmt.Errorf("invalid password")
		}
		return fmt.Errorf("failed to enable keychain: %w", err)
	}

	fmt.Println("Keychain enabled. You can now unlock without a password.")
	return nil
}

func runKeychainDisable(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	if !v.IsInitialized() {
		return fmt.Errorf("vault not initialized: run 'coffer init' first")
	}

	// Check if enabled
	enabled, err := v.IsKeychainEnabled()
	if err != nil {
		return err
	}
	if !enabled {
		fmt.Println("Keychain is not enabled")
		return nil
	}

	// Disable keychain
	if err := v.DisableKeychain(); err != nil {
		return fmt.Errorf("failed to disable keychain: %w", err)
	}

	fmt.Println("Keychain disabled. You'll need to enter your password to unlock.")
	return nil
}
