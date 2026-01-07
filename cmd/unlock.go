package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/vault"
)

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock the vault",
	Long: `Unlock the vault with your master password or keychain.

If keychain is enabled, the vault will be unlocked automatically
without prompting for a password. Use --password to force password entry.

This creates a session that allows you to access secrets without
re-entering your password. The session expires after 8 hours.

Examples:
  coffer unlock                       # Uses keychain if enabled, otherwise prompts
  coffer unlock --prompt              # Always prompt for password
  coffer unlock --password secret123  # Non-interactive`,
	RunE: runUnlock,
}

var (
	unlockPrompt         bool
	unlockPasswordValue  string
)

func init() {
	rootCmd.AddCommand(unlockCmd)
	unlockCmd.Flags().BoolVar(&unlockPrompt, "prompt", false, "Force password prompt (ignore keychain)")
	unlockCmd.Flags().StringVarP(&unlockPasswordValue, "password", "p", "", "Master password (non-interactive mode)")
}

func runUnlock(cmd *cobra.Command, args []string) error {
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

	// Check if already unlocked
	if v.IsUnlocked() {
		fmt.Println("Vault is already unlocked")
		return nil
	}

	// If password provided via flag, use it directly
	if unlockPasswordValue != "" {
		if err := v.Unlock(unlockPasswordValue); err != nil {
			if err == vault.ErrInvalidPassword {
				return fmt.Errorf("invalid password")
			}
			return fmt.Errorf("failed to unlock vault: %w", err)
		}
		fmt.Println("Vault unlocked")
		return nil
	}

	// Try keychain first if not forced to prompt
	if !unlockPrompt {
		enabled, _ := v.IsKeychainEnabled()
		if enabled {
			if err := v.UnlockWithKeychain(); err == nil {
				fmt.Println("Vault unlocked (via keychain)")
				return nil
			}
			// Keychain failed, fall back to password
			fmt.Println("Keychain unlock failed, falling back to password")
		}
	}

	// Prompt for password
	fmt.Print("Enter master password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Unlock vault
	if err := v.Unlock(string(password)); err != nil {
		if err == vault.ErrInvalidPassword {
			return fmt.Errorf("invalid password")
		}
		return fmt.Errorf("failed to unlock vault: %w", err)
	}

	fmt.Println("Vault unlocked")
	return nil
}
