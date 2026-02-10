package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/env"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <directory|branch>",
	Aliases: []string{"rm"},
	Short:   "Stop container, drop DB, and remove worktree",
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	target := args[0]

	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	// Resolve target to a worktree
	var found *git.Worktree
	for i, wt := range worktrees {
		if wt.Path == root {
			continue
		}
		absTarget, _ := filepath.Abs(target)
		if wt.Path == absTarget || wt.Branch == target {
			found = &worktrees[i]
			break
		}
	}

	if found == nil {
		// Try matching branch with / replaced
		for i, wt := range worktrees {
			if wt.Path == root {
				continue
			}
			if strings.ReplaceAll(wt.Branch, "/", "-") == strings.ReplaceAll(target, "/", "-") {
				found = &worktrees[i]
				break
			}
		}
	}

	if found == nil {
		return fmt.Errorf("worktree not found: '%s'", target)
	}

	envPath := filepath.Join(found.Path, ".env")
	dbName := env.Get(envPath, "DB_DATABASE", "")

	ui.Header("Removing worktree")
	fmt.Printf("  %s %s\n", ui.Dim("Directory:"), found.Path)
	fmt.Printf("  %s  %s\n", ui.Dim("Database:"), orNone(dbName))
	fmt.Println()
	fmt.Print("  Continue? [y/N] ")
	answer := readLine()
	if answer == "" || (answer[0] != 'y' && answer[0] != 'Y') {
		ui.Info("Cancelled")
		return nil
	}

	// Stop container
	ui.Info("Stopping container...")
	docker.ComposeDown(found.Path)

	// Drop database
	if dbName != "" {
		mainEnvPath := filepath.Join(root, ".env")
		dbPassword := env.Get(mainEnvPath, "DB_PASSWORD", "password")
		mysqlContainer, err := docker.FindMySQLContainer(root)
		if err == nil && docker.MySQLIsReachable(mysqlContainer) {
			ui.Info("Dropping database: %s", dbName)
			if err := docker.MySQLDropDB(mysqlContainer, dbPassword, dbName); err != nil {
				ui.Warn("Could not drop database: %v", err)
			}
		}
	}

	// Restore compose backup
	composePath := filepath.Join(found.Path, "docker-compose.yml")
	docker.RestoreBackup(composePath) // best-effort

	// Remove git worktree
	ui.Info("Removing git worktree...")
	if err := git.Remove(root, found.Path); err != nil {
		ui.Warn("Failed to remove worktree: %v", err)
	}

	ui.Success("Removed: %s", target)
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
