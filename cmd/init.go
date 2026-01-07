package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/vault"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vault",
	Long: `Initialize a new coffer vault with a master password.

The vault will be created in ~/.coffer/ by default. This command will
prompt you to enter and confirm your master password.

Example:
  coffer init
  coffer init --password mypassword  # Non-interactive`,
	RunE: runInit,
}

var initPassword string

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initPassword, "password", "p", "", "Master password (non-interactive mode)")
}

func runInit(cmd *cobra.Command, args []string) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	v := vault.New(cfg)
	defer v.Close()

	// Check if already initialized
	if v.IsInitialized() {
		return fmt.Errorf("vault already initialized at %s", cfg.DataDir)
	}

	var password string

	if initPassword != "" {
		// Non-interactive mode
		if len(initPassword) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}
		password = initPassword
	} else {
		// Interactive mode - prompt for password
		fmt.Print("Enter master password: ")
		password1, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		if len(password1) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}

		// Confirm password
		fmt.Print("Confirm master password: ")
		password2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		if string(password1) != string(password2) {
			return fmt.Errorf("passwords do not match")
		}
		password = string(password1)
	}

	// Initialize vault
	if err := v.Initialize(password); err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf("Vault initialized at %s\n", cfg.DataDir)
	fmt.Println("Your vault is now unlocked. Use 'coffer lock' to lock it.")
	return nil
}
