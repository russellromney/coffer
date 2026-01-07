package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import secrets from a file",
	Long: `Import secrets from a .env or JSON file.

The file format is auto-detected based on content, or you can
specify it with --format.

Examples:
  coffer import .env --env dev
  coffer import secrets.json --env prod --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var (
	importEnv    string
	importFormat string
)

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVarP(&importEnv, "env", "e", "", "Environment name (required)")
	importCmd.Flags().StringVarP(&importFormat, "format", "f", "", "File format: env, json (auto-detected if not specified)")
	importCmd.MarkFlagRequired("env")
}

func runImport(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, importEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", importEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Read file
	filename := args[0]
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect format
	format := importFormat
	if format == "" {
		if strings.HasSuffix(filename, ".json") {
			format = "json"
		} else if isJSON(data) {
			format = "json"
		} else {
			format = "env"
		}
	}

	// Parse secrets
	var secrets map[string]string
	switch format {
	case "json":
		secrets, err = parseJSON(data)
	case "env":
		secrets, err = parseEnv(data)
	default:
		return fmt.Errorf("unknown format: %s (use 'env' or 'json')", format)
	}
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found in file")
		return nil
	}

	// Get encryption key
	encKey, err := v.GetKey()
	if err != nil {
		return fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Import secrets
	created := 0
	updated := 0

	for key, value := range secrets {
		// Validate key
		if !isValidKeyName(key) {
			fmt.Printf("Skipping invalid key: %s\n", key)
			continue
		}

		// Encrypt value
		encryptedValue, nonce, err := crypto.Encrypt(encKey, []byte(value), []byte(key))
		if err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", key, err)
		}

		// Check if exists
		_, err = s.GetSecret(env.ID, key)
		if err == store.ErrNotFound {
			_, err = s.CreateSecret(env.ID, key, encryptedValue, nonce)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", key, err)
			}
			created++
		} else if err != nil {
			return fmt.Errorf("failed to check %s: %w", key, err)
		} else {
			_, err = s.UpdateSecret(env.ID, key, encryptedValue, nonce)
			if err != nil {
				return fmt.Errorf("failed to update %s: %w", key, err)
			}
			updated++
		}
	}

	fmt.Printf("Imported to %s/%s: %d created, %d updated\n", project.Name, importEnv, created, updated)
	return nil
}

func isJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}

func parseJSON(data []byte) (map[string]string, error) {
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseEnv(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		result[key] = value
	}

	return result, scanner.Err()
}
