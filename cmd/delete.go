package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/store"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <KEY>",
	Short: "Delete a secret",
	Long: `Delete a secret from an environment.

Use --force to skip confirmation.

Examples:
  coffer delete OLD_API_KEY --env prod
  coffer delete TEMP_KEY --env dev --force`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

var (
	deleteEnv   string
	deleteForce bool
)

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteEnv, "env", "e", "", "Environment name (required)")
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation")
	deleteCmd.MarkFlagRequired("env")
}

func runDelete(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, deleteEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", deleteEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	key := args[0]

	// Check if secret exists
	_, err = s.GetSecret(env.ID, key)
	if err == store.ErrNotFound {
		return fmt.Errorf("secret '%s' not found in %s/%s", key, project.Name, deleteEnv)
	}
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if !deleteForce {
		fmt.Printf("Are you sure you want to delete '%s' from %s/%s? [y/N] ", key, project.Name, deleteEnv)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := s.DeleteSecret(env.ID, key); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	fmt.Printf("Deleted %s from %s/%s\n", key, project.Name, deleteEnv)
	return nil
}
