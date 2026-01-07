package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/models"
	"github.com/russellromney/coffer/internal/store"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
	Long: `Manage environments within the active project.

Environments are containers for secrets within a project.
Common environments include dev, staging, and prod.

Examples:
  coffer env create dev
  coffer env list
  coffer env delete staging`,
}

var envCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new environment",
	Long: `Create a new environment in the active project.

Example:
  coffer env create dev
  coffer env create staging
  coffer env create prod`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvCreate,
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `List all environments in the active project.

Example:
  coffer env list`,
	RunE: runEnvList,
}

var envDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an environment",
	Long: `Delete an environment and all its secrets.

This action is irreversible. Use --force to skip confirmation.
Cannot delete an environment that has child environments.

Example:
  coffer env delete staging
  coffer env delete staging --force`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvDelete,
}

var envBranchCmd = &cobra.Command{
	Use:   "branch <parent-env> <new-env-name>",
	Short: "Create an environment that inherits from a parent",
	Long: `Create a new environment that inherits secrets from a parent environment.

Child environments inherit all secrets from their parent. You can override
inherited secrets by setting them directly in the child environment.

Example:
  coffer env branch dev dev_personal
  coffer env branch prod prod_aws`,
	Args: cobra.ExactArgs(2),
	RunE: runEnvBranch,
}

var envForce bool

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envDeleteCmd)
	envCmd.AddCommand(envBranchCmd)

	envDeleteCmd.Flags().BoolVarP(&envForce, "force", "f", false, "Skip confirmation")
}

func getActiveProject(s store.Store) (*models.Project, error) {
	activeID, err := s.GetConfig(models.ConfigActiveProject)
	if err == store.ErrNotFound {
		return nil, fmt.Errorf("no active project: use 'coffer project use <name>' first")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active project: %w", err)
	}

	project, err := s.GetProject(activeID)
	if err == store.ErrNotFound {
		return nil, fmt.Errorf("active project not found: use 'coffer project use <name>'")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

func runEnvCreate(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	project, err := getActiveProject(s)
	if err != nil {
		return err
	}

	name := args[0]

	// Check if environment already exists
	_, err = s.GetEnvironmentByName(project.ID, name)
	if err == nil {
		return fmt.Errorf("environment '%s' already exists in project '%s'", name, project.Name)
	}
	if err != store.ErrNotFound {
		return err
	}

	_, err = s.CreateEnvironment(project.ID, name)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	fmt.Printf("Created environment '%s' in project '%s'\n", name, project.Name)
	return nil
}

func runEnvList(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	project, err := getActiveProject(s)
	if err != nil {
		return err
	}

	envs, err := s.ListEnvironments(project.ID)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Printf("No environments in project '%s'. Create one with 'coffer env create <name>'\n", project.Name)
		return nil
	}

	// Build a map of env ID to name for parent lookup
	envMap := make(map[string]string)
	for _, e := range envs {
		envMap[e.ID] = e.Name
	}

	fmt.Printf("Environments in '%s':\n", project.Name)
	for _, e := range envs {
		// Get merged secrets to show local vs inherited count
		mergedSecrets, _ := s.ListSecretsWithInheritance(e.ID)
		localCount := 0
		inheritedCount := 0
		for _, ms := range mergedSecrets {
			if ms.IsInherited {
				inheritedCount++
			} else {
				localCount++
			}
		}

		if e.ParentID != nil {
			parentName := envMap[*e.ParentID]
			if inheritedCount > 0 {
				fmt.Printf("  %s (%d local, %d inherited) -> inherits from '%s'\n", e.Name, localCount, inheritedCount, parentName)
			} else {
				fmt.Printf("  %s (%d secrets) -> inherits from '%s'\n", e.Name, localCount, parentName)
			}
		} else {
			fmt.Printf("  %s (%d secrets)\n", e.Name, len(mergedSecrets))
		}
	}

	return nil
}

func runEnvDelete(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	project, err := getActiveProject(s)
	if err != nil {
		return err
	}

	name := args[0]

	env, err := s.GetEnvironmentByName(project.ID, name)
	if err == store.ErrNotFound {
		return fmt.Errorf("environment '%s' not found in project '%s'", name, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Check for child environments
	children, err := s.GetEnvironmentChildren(env.ID)
	if err != nil {
		return fmt.Errorf("failed to check for child environments: %w", err)
	}
	if len(children) > 0 {
		childNames := make([]string, len(children))
		for i, c := range children {
			childNames[i] = c.Name
		}
		return fmt.Errorf("cannot delete '%s': has child environments (%s). Delete children first", name, strings.Join(childNames, ", "))
	}

	if !envForce {
		// Count secrets
		secrets, _ := s.ListSecrets(env.ID)
		fmt.Printf("Are you sure you want to delete environment '%s' (%d secrets)? [y/N] ", name, len(secrets))
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := s.DeleteEnvironment(env.ID); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	fmt.Printf("Deleted environment '%s'\n", name)
	return nil
}

func runEnvBranch(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	project, err := getActiveProject(s)
	if err != nil {
		return err
	}

	parentName := args[0]
	newName := args[1]

	// Get parent environment
	parent, err := s.GetEnvironmentByName(project.ID, parentName)
	if err == store.ErrNotFound {
		return fmt.Errorf("parent environment '%s' not found in project '%s'", parentName, project.Name)
	}
	if err != nil {
		return fmt.Errorf("failed to get parent environment: %w", err)
	}

	// Check if new environment already exists
	_, err = s.GetEnvironmentByName(project.ID, newName)
	if err == nil {
		return fmt.Errorf("environment '%s' already exists in project '%s'", newName, project.Name)
	}
	if err != store.ErrNotFound {
		return err
	}

	// Create the branch environment
	_, err = s.CreateEnvironmentWithParent(project.ID, newName, parent.ID)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	// Count inherited secrets
	parentSecrets, _ := s.ListSecrets(parent.ID)

	fmt.Printf("Created environment '%s' inheriting from '%s' (%d secrets inherited)\n", newName, parentName, len(parentSecrets))
	return nil
}
