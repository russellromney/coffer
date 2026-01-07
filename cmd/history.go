package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

var historyCmd = &cobra.Command{
	Use:   "history <KEY>",
	Short: "Show secret version history",
	Long: `Show the version history of a secret.

Displays when each version was created and what action was taken.
Use --show-values to reveal the actual values (use with caution).

Examples:
  coffer history DATABASE_URL --env prod
  coffer history API_KEY --env dev --limit 5`,
	Args: cobra.ExactArgs(1),
	RunE: runHistory,
}

var (
	historyEnv        string
	historyLimit      int
	historyShowValues bool
)

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().StringVarP(&historyEnv, "env", "e", "", "Environment name (required)")
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "l", 10, "Number of versions to show")
	historyCmd.Flags().BoolVar(&historyShowValues, "show-values", false, "Show decrypted values")
	historyCmd.MarkFlagRequired("env")
}

func runHistory(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, historyEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", historyEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	key := args[0]

	// Get history
	history, err := s.GetSecretHistory(env.ID, key, historyLimit)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(history) == 0 {
		fmt.Printf("No history found for '%s' in %s/%s\n", key, project.Name, historyEnv)
		return nil
	}

	// Get encryption key if showing values
	var encKey []byte
	if historyShowValues {
		encKey, err = v.GetKey()
		if err != nil {
			return fmt.Errorf("failed to get encryption key: %w", err)
		}
	}

	fmt.Printf("History for '%s' in %s/%s:\n\n", key, project.Name, historyEnv)

	for _, h := range history {
		actionIcon := "?"
		switch h.ChangeType {
		case "create":
			actionIcon = "+"
		case "update":
			actionIcon = "~"
		case "delete":
			actionIcon = "-"
		}

		timestamp := h.CreatedAt.Format(time.RFC3339)
		fmt.Printf("  [%s] v%d  %s  %s\n", actionIcon, h.Version, timestamp, h.ChangeType)

		if historyShowValues && h.ChangeType != "delete" {
			value, err := crypto.Decrypt(encKey, h.EncryptedValue, h.Nonce, []byte(key))
			if err != nil {
				fmt.Printf("       Value: [decryption error]\n")
			} else {
				// Truncate long values
				valStr := string(value)
				if len(valStr) > 60 {
					valStr = valStr[:60] + "..."
				}
				fmt.Printf("       Value: %s\n", valStr)
			}
		}
		fmt.Println()
	}

	return nil
}
