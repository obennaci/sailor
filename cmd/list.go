package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/env"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List worktrees and their status",
	RunE:    runList,
}

func runList(cmd *cobra.Command, args []string) error {
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	ui.Header("Worktrees")
	fmt.Println()

	// Main branch
	mainShort := shortenHome(root)
	mainStatus := ui.Red("stopped")

	mysqlContainer, cErr := docker.FindMySQLContainer(root)
	if cErr == nil && docker.MySQLIsReachable(mysqlContainer) {
		mainStatus = ui.Green("running (infra)")
	}
	fmt.Printf("  %s %s  %s  %s\n", ui.Green("●"), ui.Bold("main"), mainShort, mainStatus)
	fmt.Println()

	// Filter worktrees (exclude main)
	var wts []git.Worktree
	for _, wt := range worktrees {
		if wt.Path != root {
			wts = append(wts, wt)
		}
	}

	if len(wts) == 0 {
		ui.Info("No worktrees. Use 'sailor add <branch>'.")
		return nil
	}

	fmt.Printf("  %-22s %-30s %-6s %-25s %-8s\n",
		ui.Bold("BRANCH"), ui.Bold("DIRECTORY"), ui.Bold("PORT"), ui.Bold("DATABASE"), ui.Bold("STATUS"))
	fmt.Printf("  %-22s %-30s %-6s %-25s %-8s\n",
		ui.Dim("──────"), ui.Dim("─────────"), ui.Dim("────"), ui.Dim("────────"), ui.Dim("──────"))

	for _, wt := range wts {
		shortDir := shortenHome(wt.Path)
		envPath := filepath.Join(wt.Path, ".env")
		appPort := env.Get(envPath, "APP_PORT", "?")
		dbName := env.Get(envPath, "DB_DATABASE", "?")

		status := ui.Red("stopped")
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			status = ui.Yellow("missing")
		} else {
			port := 0
			fmt.Sscanf(appPort, "%d", &port)
			if port > 0 && docker.IsPortInUse(port) {
				status = ui.Green("running")
			}
		}

		fmt.Printf("  %-22s %-30s %-6s %-25s %s\n", wt.Branch, shortDir, appPort, dbName, status)
	}

	return nil
}

func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
