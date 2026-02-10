package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Setup shared network and patch main docker-compose.yml",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Reinitialize even if already patched")
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	composePath := filepath.Join(root, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("no docker-compose.yml found in %s", root)
	}

	ui.Header("Initializing sailor")

	// Parse compose file
	compose, err := docker.ParseCompose(composePath)
	if err != nil {
		return err
	}

	// Check if already initialized
	if compose.HasSharedNetwork(docker.SharedNetworkName) && !forceInit {
		ui.Warn("Already initialized. Use --force to reinitialize.")
		return nil
	}

	// Detect services
	appService := compose.DetectAppService()
	infraServices := compose.DetectInfraServices(appService)

	ui.Info("App service detected: %s", appService)
	if len(infraServices) > 0 {
		ui.Info("Infra services detected: %s", strings.Join(infraServices, ", "))
	} else {
		ui.Warn("No infra services detected in docker-compose.yml")
	}

	// Create shared network
	created, err := docker.EnsureNetwork(docker.SharedNetworkName)
	if err != nil {
		return fmt.Errorf("failed to create Docker network: %w", err)
	}
	if created {
		ui.Success("Created Docker network: %s", docker.SharedNetworkName)
	} else {
		ui.Info("Docker network '%s' already exists", docker.SharedNetworkName)
	}

	// Backup and patch docker-compose.yml
	if err := compose.Backup(); err != nil {
		return fmt.Errorf("failed to backup docker-compose.yml: %w", err)
	}
	ui.Success("Backup created: docker-compose.yml%s", docker.BackupSuffix)

	if err := compose.PatchMainCompose(docker.SharedNetworkName); err != nil {
		return fmt.Errorf("failed to patch docker-compose.yml: %w", err)
	}
	if err := compose.Save(); err != nil {
		return fmt.Errorf("failed to save docker-compose.yml: %w", err)
	}
	ui.Success("Patched docker-compose.yml with shared network")

	// Add backup to .gitignore
	addToGitignore(root, "docker-compose.yml"+docker.BackupSuffix)

	ui.Success("Initialized!")
	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold("How to use:"))
	fmt.Println()
	fmt.Printf("  %s Start your main branch:\n", ui.Dim("1."))
	fmt.Printf("     %s\n", ui.Cyan("sail up -d"))
	fmt.Println()
	fmt.Printf("  %s Add worktrees:\n", ui.Dim("2."))
	fmt.Printf("     %s\n", ui.Cyan("sailor add feature/payments"))
	fmt.Println()
	fmt.Printf("  %s Main branch must be running — it provides MySQL, Redis, etc.\n", ui.Dim("Note:"))

	return nil
}

func addToGitignore(root string, entries ...string) {
	gitignorePath := filepath.Join(root, ".gitignore")

	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, entry := range entries {
		if !strings.Contains(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) > 0 {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		for _, entry := range toAdd {
			f.WriteString(entry + "\n")
		}
	}
}
