package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List secrets",
	Long: `List all secrets in an environment.

By default, only key names are shown. Use --show-values to reveal values.

Examples:
  coffer list --env prod
  coffer list --env dev --show-values`,
	RunE: runList,
}

var (
	listEnv        string
	listShowValues bool
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listEnv, "env", "e", "", "Environment name (required)")
	listCmd.Flags().BoolVar(&listShowValues, "show-values", false, "Show secret values (use with caution)")
	listCmd.MarkFlagRequired("env")
}

func runList(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, listEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", listEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// List secrets with inheritance
	secrets, err := s.ListSecretsWithInheritance(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(secrets) == 0 {
		fmt.Printf("No secrets in %s/%s\n", project.Name, listEnv)
		return nil
	}

	// Get encryption key if showing values
	var encKey []byte
	if listShowValues {
		encKey, err = v.GetKey()
		if err != nil {
			return fmt.Errorf("failed to get encryption key: %w", err)
		}
	}

	fmt.Printf("Secrets in %s/%s:\n", project.Name, listEnv)
	for _, secret := range secrets {
		inheritedMarker := ""
		if secret.IsInherited {
			inheritedMarker = fmt.Sprintf(" [inherited from %s]", secret.SourceEnvName)
		}

		if listShowValues {
			value, err := crypto.Decrypt(encKey, secret.EncryptedValue, secret.Nonce, []byte(secret.Key))
			if err != nil {
				fmt.Printf("  %s = [decryption error]%s\n", secret.Key, inheritedMarker)
			} else {
				fmt.Printf("  %s = %s%s\n", secret.Key, string(value), inheritedMarker)
			}
		} else {
			fmt.Printf("  %s%s\n", secret.Key, inheritedMarker)
		}
	}

	return nil
}
