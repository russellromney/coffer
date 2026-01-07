package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/store"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <KEY> --version <n>",
	Short: "Restore a secret to a previous version",
	Long: `Restore a secret to a previous version from history.

Use 'coffer history <KEY>' to see available versions.

Examples:
  coffer restore DATABASE_URL --env prod --version 2
  coffer restore API_KEY --env dev --version 1`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

var (
	restoreEnv     string
	restoreVersion int
)

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVarP(&restoreEnv, "env", "e", "", "Environment name (required)")
	restoreCmd.Flags().IntVarP(&restoreVersion, "version", "v", 0, "Version to restore (required)")
	restoreCmd.MarkFlagRequired("env")
	restoreCmd.MarkFlagRequired("version")
}

func runRestore(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, restoreEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", restoreEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	key := args[0]

	// Get the version to restore
	historyEntry, err := s.GetSecretVersion(env.ID, key, restoreVersion)
	if err == store.ErrNotFound {
		return fmt.Errorf("version %d not found for '%s' in %s/%s", restoreVersion, key, project.Name, restoreEnv)
	}
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	// Check if the version was a delete
	if historyEntry.ChangeType == "delete" {
		return fmt.Errorf("cannot restore version %d: it was a deletion", restoreVersion)
	}

	// Check if secret currently exists
	_, err = s.GetSecret(env.ID, key)
	if err == store.ErrNotFound {
		// Create as new secret
		_, err = s.CreateSecret(env.ID, key, historyEntry.EncryptedValue, historyEntry.Nonce)
		if err != nil {
			return fmt.Errorf("failed to restore secret: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check secret: %w", err)
	} else {
		// Update existing secret
		_, err = s.UpdateSecret(env.ID, key, historyEntry.EncryptedValue, historyEntry.Nonce)
		if err != nil {
			return fmt.Errorf("failed to restore secret: %w", err)
		}
	}

	fmt.Printf("Restored '%s' to version %d in %s/%s\n", key, restoreVersion, project.Name, restoreEnv)
	return nil
}

// Helper for parsing version from string (in case we want to support "v2" format)
func parseVersion(s string) (int, error) {
	// Handle "v2" format
	if len(s) > 0 && s[0] == 'v' {
		s = s[1:]
	}
	return strconv.Atoi(s)
}
