package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/millancore/sailor/internal/env"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Port allocation map",
	RunE:  runPorts,
}

func runPorts(cmd *cobra.Command, args []string) error {
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	ui.Header("Port Map")
	fmt.Println()

	// Filter worktrees (exclude main)
	var wts []git.Worktree
	for _, wt := range worktrees {
		if wt.Path != root {
			wts = append(wts, wt)
		}
	}

	if len(wts) == 0 {
		ui.Info("No worktrees registered.")
		return nil
	}

	fmt.Printf("  %-22s %6s %6s %-25s\n",
		ui.Bold("BRANCH"), ui.Bold("APP"), ui.Bold("VITE"), ui.Bold("DATABASE"))
	fmt.Printf("  %-22s %6s %6s %-25s\n",
		ui.Dim("──────"), ui.Dim("───"), ui.Dim("────"), ui.Dim("────────"))

	for _, wt := range wts {
		envPath := filepath.Join(wt.Path, ".env")
		appPort := env.Get(envPath, "APP_PORT", "?")
		vitePort := env.Get(envPath, "VITE_PORT", "?")
		dbName := env.Get(envPath, "DB_DATABASE", "?")

		fmt.Printf("  %-22s %6s %6s %-25s\n", wt.Branch, appPort, vitePort, dbName)
	}

	return nil
}
