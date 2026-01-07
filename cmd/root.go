package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "coffer",
	Short: "A self-hosted secrets manager",
	Long: `Coffer is a self-hosted secrets manager that stores encrypted secrets
locally in SQLite with optional backup to S3-compatible storage via Litestream.

Use coffer to manage environment variables across projects and environments,
then inject them into your applications at runtime.

Example workflow:
  coffer init                              # Initialize vault with master password
  coffer project create myapp              # Create a project
  coffer env create dev                    # Create dev environment
  coffer set DATABASE_URL "postgres://..." --env dev
  coffer run --env dev -- npm start        # Run with secrets injected`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
}
