package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/resolver"
	"github.com/russellromney/coffer/internal/store"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export secrets",
	Long: `Export secrets from an environment to stdout.

By default, exports in .env format. Use --format json for JSON output.
Use --resolve to expand ${VAR} references.

Examples:
  coffer export --env prod > .env.prod
  coffer export --env dev --format json > secrets.json
  coffer export --env prod --resolve`,
	RunE: runExport,
}

var (
	exportEnv     string
	exportFormat  string
	exportResolve bool
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportEnv, "env", "e", "", "Environment name (required)")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "env", "Output format: env, json")
	exportCmd.Flags().BoolVar(&exportResolve, "resolve", false, "Resolve ${VAR} references")
	exportCmd.MarkFlagRequired("env")
}

func runExport(cmd *cobra.Command, args []string) error {
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
	env, err := s.GetEnvironmentByName(project.ID, exportEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", exportEnv, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Get encryption key
	encKey, err := v.GetKey()
	if err != nil {
		return fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Load and decrypt all secrets (with inheritance)
	secrets, err := s.ListSecretsWithInheritance(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	decryptedSecrets := make(map[string]string)
	for _, secret := range secrets {
		value, err := crypto.Decrypt(encKey, secret.EncryptedValue, secret.Nonce, []byte(secret.Key))
		if err != nil {
			return fmt.Errorf("failed to decrypt secret '%s': %w", secret.Key, err)
		}
		decryptedSecrets[secret.Key] = string(value)
	}

	// Resolve references if requested
	outputSecrets := decryptedSecrets
	if exportResolve {
		outputSecrets, err = resolver.Resolve(decryptedSecrets)
		if err != nil {
			return fmt.Errorf("failed to resolve references: %w", err)
		}
	}

	// Output in requested format
	switch exportFormat {
	case "env":
		outputEnvFormat(outputSecrets)
	case "json":
		if err := outputJSONFormat(outputSecrets); err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}
	default:
		return fmt.Errorf("unknown format: %s (use 'env' or 'json')", exportFormat)
	}

	return nil
}

func outputEnvFormat(secrets map[string]string) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := secrets[key]
		// Quote values that contain special characters
		if needsQuoting(value) {
			fmt.Printf("%s=\"%s\"\n", key, escapeValue(value))
		} else {
			fmt.Printf("%s=%s\n", key, value)
		}
	}
}

func outputJSONFormat(secrets map[string]string) error {
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func needsQuoting(value string) bool {
	for _, c := range value {
		if c == ' ' || c == '"' || c == '\'' || c == '\n' || c == '\t' || c == '$' || c == '`' {
			return true
		}
	}
	return false
}

func escapeValue(value string) string {
	result := ""
	for _, c := range value {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\t':
			result += "\\t"
		default:
			result += string(c)
		}
	}
	return result
}
