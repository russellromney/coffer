package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/resolver"
	"github.com/russellromney/coffer/internal/store"
)

var runCmd = &cobra.Command{
	Use:   "run --env <environment> -- <command> [args...]",
	Short: "Run a command with secrets injected",
	Long: `Run a command with secrets from the specified environment
injected as environment variables.

The command and its arguments should come after "--".
Secret references (${VAR}) are resolved before injection.

Examples:
  coffer run --env prod -- npm start
  coffer run --env dev -- ./my-app --port 8080
  coffer run --env staging -- docker-compose up`,
	RunE:               runRun,
	DisableFlagParsing: false,
}

var runEnv string

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runEnv, "env", "e", "", "Environment name (required)")
	runCmd.MarkFlagRequired("env")
}

func runRun(cmd *cobra.Command, args []string) error {
	// Find the command args after --
	cmdArgs := args
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified: use 'coffer run --env <env> -- <command>'")
	}

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
	env, err := s.GetEnvironmentByName(project.ID, runEnv)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", runEnv, project.Name)
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

	// Resolve references
	resolvedSecrets, err := resolver.Resolve(decryptedSecrets)
	if err != nil {
		return fmt.Errorf("failed to resolve secret references: %w", err)
	}

	// Build environment
	environ := os.Environ()
	for key, value := range resolvedSecrets {
		environ = append(environ, fmt.Sprintf("%s=%s", key, value))
	}

	// Execute command
	execCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	execCmd.Env = environ
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// Handle signals - forward them to the child process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the command
	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Forward signals to child
	go func() {
		for sig := range sigChan {
			if execCmd.Process != nil {
				execCmd.Process.Signal(sig)
			}
		}
	}()

	// Wait for command to complete
	err = execCmd.Wait()
	signal.Stop(sigChan)
	close(sigChan)

	if err != nil {
		// If the command exited with an error, propagate the exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
