package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/models"
	"github.com/russellromney/coffer/internal/store"
	"github.com/russellromney/coffer/internal/vault"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long: `Manage projects in your vault.

Projects are containers for environments and secrets. Each project
can have multiple environments (dev, staging, prod, etc.).

Examples:
  coffer project create myapp
  coffer project list
  coffer project use myapp
  coffer project delete myapp`,
}

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Long: `Create a new project with the given name.

Example:
  coffer project create myapp
  coffer project create myapp --description "My Application"`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectCreate,
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long: `List all projects in your vault.

Example:
  coffer project list`,
	RunE: runProjectList,
}

var projectUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active project",
	Long: `Set the active project for subsequent commands.

When a project is active, you don't need to specify --project
for other commands.

Example:
  coffer project use myapp`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectUse,
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a project",
	Long: `Delete a project and all its environments and secrets.

This action is irreversible. Use --force to skip confirmation.

Example:
  coffer project delete myapp
  coffer project delete myapp --force`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectDelete,
}

var (
	projectDescription string
	projectForce       bool
)

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectUseCmd)
	projectCmd.AddCommand(projectDeleteCmd)

	projectCreateCmd.Flags().StringVarP(&projectDescription, "description", "d", "", "Project description")
	projectDeleteCmd.Flags().BoolVarP(&projectForce, "force", "f", false, "Skip confirmation")
}

func getUnlockedVault() (*vault.Vault, store.Store, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, nil, err
	}

	v := vault.New(cfg)

	if !v.IsInitialized() {
		v.Close()
		return nil, nil, fmt.Errorf("vault not initialized: run 'coffer init' first")
	}

	if !v.IsUnlocked() {
		v.Close()
		return nil, nil, fmt.Errorf("vault is locked: run 'coffer unlock' first")
	}

	s, err := v.GetStore()
	if err != nil {
		v.Close()
		return nil, nil, err
	}

	return v, s, nil
}

func runProjectCreate(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	name := args[0]

	// Check if project already exists
	_, err = s.GetProjectByName(name)
	if err == nil {
		return fmt.Errorf("project '%s' already exists", name)
	}
	if err != store.ErrNotFound {
		return err
	}

	project, err := s.CreateProject(name, projectDescription)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	// Set as active project
	if err := s.SetConfig(models.ConfigActiveProject, project.ID); err != nil {
		return fmt.Errorf("failed to set active project: %w", err)
	}

	fmt.Printf("Created project '%s'\n", name)
	fmt.Printf("Project '%s' is now active\n", name)
	return nil
}

func runProjectList(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	projects, err := s.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found. Create one with 'coffer project create <name>'")
		return nil
	}

	// Get active project
	activeID, _ := s.GetConfig(models.ConfigActiveProject)

	fmt.Println("Projects:")
	for _, p := range projects {
		marker := "  "
		if p.ID == activeID {
			marker = "* "
		}
		if p.Description != "" {
			fmt.Printf("%s%s - %s\n", marker, p.Name, p.Description)
		} else {
			fmt.Printf("%s%s\n", marker, p.Name)
		}
	}

	return nil
}

func runProjectUse(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	name := args[0]

	project, err := s.GetProjectByName(name)
	if err == store.ErrNotFound {
		return fmt.Errorf("project '%s' not found", name)
	}
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if err := s.SetConfig(models.ConfigActiveProject, project.ID); err != nil {
		return fmt.Errorf("failed to set active project: %w", err)
	}

	fmt.Printf("Now using project '%s'\n", name)
	return nil
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	v, s, err := getUnlockedVault()
	if err != nil {
		return err
	}
	defer v.Close()

	name := args[0]

	project, err := s.GetProjectByName(name)
	if err == store.ErrNotFound {
		return fmt.Errorf("project '%s' not found", name)
	}
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if !projectForce {
		fmt.Printf("Are you sure you want to delete project '%s' and all its secrets? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Clear active project if this is it
	activeID, _ := s.GetConfig(models.ConfigActiveProject)
	if activeID == project.ID {
		s.DeleteConfig(models.ConfigActiveProject)
	}

	if err := s.DeleteProject(project.ID); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	fmt.Printf("Deleted project '%s'\n", name)
	return nil
}
