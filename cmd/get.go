package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

var getCmd = &cobra.Command{
	Use:   "get <KEY>",
	Short: "Get a secret value",
	Long: `Get the value of a secret.

The decrypted value is printed to stdout.

Examples:
  coffer get DATABASE_URL --env prod
  coffer get API_KEY --env dev`,
	Args: cobra.ExactArgs(1),
	RunE: runGet,
}

var getEnv string

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&getEnv, "env", "e", "", "Environment name (required)")
	getCmd.MarkFlagRequired("env")
}

func runGet(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	project, err := getActiveProject(s)
	if err != nil {
		return err
	}

	// Get environment
	env, err := s.GetEnvironmentByName(project.ID, getEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", getEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	key := args[0]

	// Get secret with inheritance
	mergedSecret, err := s.GetSecretWithInheritance(env.ID, key)
	if err == store.ErrNotFound {
		return fmt.Errorf("secret '%s' not found in %s/%s", key, project.Name, getEnv)
	}
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Get encryption key
	encKey, err := v.GetKey()
	if err != nil {
		return fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Decrypt value
	value, err := crypto.Decrypt(encKey, mergedSecret.EncryptedValue, mergedSecret.Nonce, []byte(key))
	if err != nil {
		return fmt.Errorf("failed to decrypt secret: %w", err)
	}

	fmt.Println(string(value))
	return nil
}
