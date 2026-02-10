package cmd

import (
	"fmt"

	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Docker containers detail",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	// Main branch
	ui.Header("Main Branch (Infrastructure)")
	containers, err := docker.ComposePS(root)
	if err != nil || len(containers) == 0 {
		fmt.Println("  (not running)")
	} else {
		fmt.Printf("  %-30s %-15s %s\n", ui.Bold("NAME"), ui.Bold("STATUS"), ui.Bold("PORTS"))
		for _, c := range containers {
			fmt.Printf("  %-30s %-15s %s\n", c.Name, c.Status, c.Ports)
		}
	}

	// Worktrees
	ui.Header("Worktree App Containers")
	fmt.Println()

	for _, wt := range worktrees {
		if wt.Path == root {
			continue
		}

		fmt.Printf("  %s\n", ui.Bold(wt.Branch))
		containers, err := docker.ComposePS(wt.Path)
		if err != nil || len(containers) == 0 {
			fmt.Println("    (not running)")
		} else {
			for _, c := range containers {
				fmt.Printf("    %-30s %-15s %s\n", c.Name, c.Status, c.Ports)
			}
		}
		fmt.Println()
	}

	return nil
}
