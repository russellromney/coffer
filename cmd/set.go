package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

var setCmd = &cobra.Command{
	Use:   "set <KEY> [value]",
	Short: "Set a secret",
	Long: `Set a secret value for the given key.

If no value is provided, you'll be prompted to enter it (hidden input).
Use --stdin to read from stdin (useful for piping).

Examples:
  coffer set DATABASE_URL "postgres://..." --env prod
  coffer set API_KEY --env prod                      # Prompts for value
  echo "secret" | coffer set API_KEY --env prod --stdin`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSet,
}

var (
	setEnv   string
	setStdin bool
)

func init() {
	rootCmd.AddCommand(setCmd)
	setCmd.Flags().StringVarP(&setEnv, "env", "e", "", "Environment name (required)")
	setCmd.Flags().BoolVar(&setStdin, "stdin", false, "Read value from stdin")
	setCmd.MarkFlagRequired("env")
}

func runSet(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, setEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", setEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	key := args[0]

	// Validate key name
	if !isValidKeyName(key) {
		return fmt.Errorf("invalid key name: must contain only uppercase letters, numbers, and underscores")
	}

	// Get value
	var value string
	if len(args) == 2 {
		value = args[1]
	} else if setStdin {
		// Read from stdin
		reader := bufio.NewReader(os.Stdin)
		value, err = reader.ReadString('\n')
		if err != nil && err.Error() != "EOF" {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		value = strings.TrimSuffix(value, "\n")
	} else {
		// Prompt for value (hidden)
		fmt.Printf("Enter value for %s: ", key)
		valueBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
		value = string(valueBytes)
	}

	// Get encryption key
	encKey, err := v.GetKey()
	if err != nil {
		return fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Encrypt value with key name as AAD
	encryptedValue, nonce, err := crypto.Encrypt(encKey, []byte(value), []byte(key))
	if err != nil {
		return fmt.Errorf("failed to encrypt value: %w", err)
	}

	// Check if secret exists
	_, err = s.GetSecret(env.ID, key)
	if err == store.ErrNotFound {
		// Create new secret
		_, err = s.CreateSecret(env.ID, key, encryptedValue, nonce)
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
		fmt.Printf("Created %s in %s/%s\n", key, project.Name, setEnv)
	} else if err != nil {
		return fmt.Errorf("failed to check secret: %w", err)
	} else {
		// Update existing secret
		_, err = s.UpdateSecret(env.ID, key, encryptedValue, nonce)
		if err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
		fmt.Printf("Updated %s in %s/%s\n", key, project.Name, setEnv)
	}

	return nil
}

func isValidKeyName(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i, c := range key {
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '0' && c <= '9' && i > 0 {
			continue
		}
		if c == '_' {
			continue
		}
		return false
	}
	return true
}
