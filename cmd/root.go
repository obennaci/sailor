package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sailor",
	Short: "Run multiple Laravel Sail branches in parallel",
	Long: `sailor — Run multiple Laravel Sail branches in parallel

Architecture:
  - Your MAIN branch runs the full Sail stack (MySQL, Redis, etc.) as usual
  - Each WORKTREE runs ONLY the app container, connected to the main Sail network
  - Each worktree gets its own database (same MySQL instance)
  - Dependencies are copied (independent per worktree)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(portsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(removeCmd)
}
