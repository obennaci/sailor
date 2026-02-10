package cmd

import (
	"path/filepath"

	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [directory]",
	Short: "Stop app container (default: current directory)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDown,
}

func runDown(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	docker.ComposeDown(absTarget)
	ui.Success("Stopped: %s", absTarget)
	return nil
}
