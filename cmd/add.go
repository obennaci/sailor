package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/millancore/sailor/internal/deps"
	"github.com/millancore/sailor/internal/docker"
	"github.com/millancore/sailor/internal/env"
	"github.com/millancore/sailor/internal/git"
	"github.com/millancore/sailor/internal/ui"
	"github.com/spf13/cobra"
)

const (
	baseAppPort  = 8080
	baseVitePort = 5174
)

var addCmd = &cobra.Command{
	Use:   "add <branch> [directory]",
	Short: "Create worktree with DB, deps, and compose config",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runAdd,
}

func runAdd(cmd *cobra.Command, args []string) error {
	branch := args[0]

	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	// Verify main is initialized
	composePath := filepath.Join(root, "docker-compose.yml")
	compose, err := docker.ParseCompose(composePath)
	if err != nil {
		return err
	}
	if !compose.HasSharedNetwork(docker.SharedNetworkName) {
		return fmt.Errorf("not initialized. Run 'sailor init' first")
	}

	appService := compose.DetectAppService()
	infraServices := compose.DetectInfraServices(appService)

	// Resolve target directory
	var targetDir string
	if len(args) > 1 {
		targetDir = args[1]
	} else {
		safeBranch := strings.ReplaceAll(branch, "/", "-")
		targetDir = filepath.Join(filepath.Dir(root), fmt.Sprintf("%s-%s", filepath.Base(root), safeBranch))
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	// Duplicate check
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}
	for _, wt := range worktrees {
		if wt.Path == absTarget {
			return fmt.Errorf("worktree already exists: %s", absTarget)
		}
	}

	// Branch check
	if !git.BranchExists(root, branch) {
		ui.Warn("Branch '%s' does not exist.", branch)
		fmt.Print("  Create from current HEAD? [Y/n] ")
		answer := readLine()
		if answer != "" && !strings.HasPrefix(strings.ToLower(answer), "y") {
			return fmt.Errorf("cancelled")
		}
		if err := git.CreateBranch(root, branch); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
		ui.Success("Created branch '%s'", branch)
	}

	// ── 1. Git worktree ──
	ui.Header("1/5  Creating git worktree")
	if err := git.Add(root, absTarget, branch); err != nil {
		return fmt.Errorf("git worktree add failed: %w", err)
	}
	ui.Success("Worktree at %s", absTarget)

	// ── 2. Copy dependencies ──
	ui.Header("2/5  Copying dependencies")
	needComposer, needNpm := deps.CopyDeps(root, absTarget)

	if !needComposer {
		ui.Success("vendor/ copied — lock files match")
	} else {
		ui.Info("vendor/ needs install")
		runInstall(absTarget, "composer")
	}
	if !needNpm {
		ui.Success("node_modules/ copied — lock files match")
	} else {
		ui.Info("node_modules/ needs install")
		runInstall(absTarget, "npm")
	}

	deps.EnsureStorageDirs(absTarget)

	// ── 3. Database ──
	ui.Header("3/5  Setting up database")

	mainEnvPath := filepath.Join(root, ".env")
	sourceDB := env.Get(mainEnvPath, "DB_DATABASE", "laravel")
	dbPassword := env.Get(mainEnvPath, "DB_PASSWORD", "password")
	dbName := docker.SanitizeDBName(fmt.Sprintf("%s_%s", sourceDB, strings.ReplaceAll(branch, "/", "_")))

	runMigrateLater := false

	mysqlContainer, err := docker.FindMySQLContainer(root)
	if err == nil && docker.MySQLIsReachable(mysqlContainer) {
		ui.Info("Creating database: %s", dbName)
		if err := docker.MySQLCreateDB(mysqlContainer, dbPassword, dbName); err != nil {
			ui.Error("Failed to create database: %v", err)
		}

		sourceHasTables := docker.MySQLHasTables(mysqlContainer, dbPassword, sourceDB)

		fmt.Println()
		fmt.Printf("  %s\n", ui.Bold("How to populate the database?"))
		if sourceHasTables {
			fmt.Printf("    %s Schema only from '%s' %s\n", ui.Cyan("1)"), sourceDB, ui.Dim("(fast)"))
			fmt.Printf("    %s Schema + data from '%s' %s\n", ui.Cyan("2)"), sourceDB, ui.Dim("(snapshot)"))
		} else {
			fmt.Printf("    %s Schema only — source has no tables\n", ui.Dim("1)"))
			fmt.Printf("    %s Schema + data — source has no tables\n", ui.Dim("2)"))
		}
		fmt.Printf("    %s migrate --seed %s\n", ui.Cyan("3)"), ui.Dim("(clean, validates migrations)"))
		fmt.Printf("    %s Skip\n", ui.Cyan("4)"))
		fmt.Println()
		fmt.Print("  Choice [1]: ")
		choice := readLine()
		if choice == "" {
			choice = "1"
		}

		switch choice {
		case "1":
			if sourceHasTables {
				ui.Info("Copying schema...")
				dump, err := docker.MySQLDump(mysqlContainer, dbPassword, sourceDB, true)
				if err == nil {
					docker.MySQLImport(mysqlContainer, dbPassword, dbName, dump)
					ui.Success("Schema copied")
				} else {
					ui.Error("Failed to dump schema: %v", err)
				}
			} else {
				ui.Warn("No tables in source — will migrate --seed after start")
				runMigrateLater = true
			}
		case "2":
			if sourceHasTables {
				ui.Info("Copying schema + data...")
				dump, err := docker.MySQLDump(mysqlContainer, dbPassword, sourceDB, false)
				if err == nil {
					docker.MySQLImport(mysqlContainer, dbPassword, dbName, dump)
					ui.Success("Full copy done")
				} else {
					ui.Error("Failed to dump: %v", err)
				}
			} else {
				ui.Warn("No tables in source — will migrate --seed after start")
				runMigrateLater = true
			}
		case "3":
			runMigrateLater = true
		case "4":
			ui.Info("Skipped")
		}

		if runMigrateLater {
			os.WriteFile(filepath.Join(absTarget, ".sailor-migrate"), []byte(""), 0644)
		}
	} else {
		ui.Warn("MySQL not reachable — is the main branch running? (sail up -d)")
		ui.Warn("Skipping DB setup.")
	}

	// ── 4. Configure .env ──
	ui.Header("4/5  Configuring .env")

	appPort, vitePort := allocatePorts(root)

	// Copy .env from main
	envSrc := filepath.Join(root, ".env")
	envDst := filepath.Join(absTarget, ".env")
	if _, err := os.Stat(envSrc); err == nil {
		env.Copy(envSrc, envDst)
	} else if _, err := os.Stat(filepath.Join(absTarget, ".env.example")); err == nil {
		env.Copy(filepath.Join(absTarget, ".env.example"), envDst)
	}

	if _, err := os.Stat(envDst); err == nil {
		updates := map[string]string{
			"APP_PORT":     fmt.Sprintf("%d", appPort),
			"APP_URL":      fmt.Sprintf("http://localhost:%d", appPort),
			"APP_NAME":     fmt.Sprintf("\"%s-%s\"", filepath.Base(root), branch),
			"DB_DATABASE":  dbName,
			"REDIS_PREFIX": docker.SanitizeDBName(fmt.Sprintf("%s_%s", sourceDB, branch)) + "_",
			"VITE_PORT":    fmt.Sprintf("%d", vitePort),
		}
		if err := env.Write(envDst, updates); err != nil {
			ui.Warn("Failed to update .env: %v", err)
		} else {
			ui.Success(".env configured (port=%d, db=%s)", appPort, dbName)
		}
	} else {
		ui.Warn("No .env created — configure manually")
	}

	// ── 5. Patch worktree compose ──
	ui.Header("5/5  Patching docker-compose.yml")

	wtComposePath := filepath.Join(absTarget, "docker-compose.yml")
	wtCompose, err := docker.ParseCompose(wtComposePath)
	if err != nil {
		ui.Warn("Cannot parse worktree docker-compose.yml: %v", err)
	} else {
		if err := wtCompose.Backup(); err != nil {
			ui.Warn("Failed to backup: %v", err)
		}
		if err := wtCompose.PatchWorktreeCompose(appService, infraServices, appPort, vitePort, docker.SharedNetworkName); err != nil {
			ui.Warn("Failed to patch: %v", err)
		} else if err := wtCompose.Save(); err != nil {
			ui.Warn("Failed to save: %v", err)
		} else {
			ui.Success("Patched docker-compose.yml")
		}
	}

	// ── Done ──
	ui.Header("Ready!")
	fmt.Println()
	fmt.Printf("  %s    %s\n", ui.Dim("Branch:"), branch)
	fmt.Printf("  %s %s\n", ui.Dim("Directory:"), absTarget)
	fmt.Printf("  %s  %s\n", ui.Dim("Database:"), dbName)
	fmt.Printf("  %s   http://localhost:%d\n", ui.Dim("App URL:"), appPort)
	fmt.Printf("  %s      localhost:%d\n", ui.Dim("Vite:"), vitePort)
	fmt.Println()
	fmt.Printf("  %s cd %s && sailor up\n", ui.Bold("Start:"), absTarget)

	return nil
}

// allocatePorts scans existing worktree .env files to find unused ports.
func allocatePorts(root string) (appPort, vitePort int) {
	usedApp := make(map[int]bool)
	usedVite := make(map[int]bool)

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return baseAppPort, baseVitePort
	}

	for _, wt := range worktrees {
		if wt.Path == root {
			continue
		}
		envPath := filepath.Join(wt.Path, ".env")
		vals, err := env.Read(envPath)
		if err != nil {
			continue
		}
		if p := parsePort(vals["APP_PORT"]); p > 0 {
			usedApp[p] = true
		}
		if p := parsePort(vals["VITE_PORT"]); p > 0 {
			usedVite[p] = true
		}
	}

	appPort = baseAppPort
	for usedApp[appPort] {
		appPort++
	}
	vitePort = baseVitePort
	for usedVite[vitePort] {
		vitePort++
	}
	return
}

func parsePort(s string) int {
	var p int
	fmt.Sscanf(s, "%d", &p)
	return p
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func runInstall(dir, tool string) {
	switch tool {
	case "composer":
		if _, err := os.Stat(filepath.Join(dir, "composer.json")); err == nil {
			ui.Info("Running composer install...")
			cmd := fmt.Sprintf("cd %s && composer install --quiet 2>/dev/null", dir)
			if err := execShell(cmd); err != nil {
				ui.Warn("composer install failed — run manually")
			}
		}
	case "npm":
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			ui.Info("Running npm install...")
			cmd := fmt.Sprintf("cd %s && npm install --silent 2>/dev/null", dir)
			if err := execShell(cmd); err != nil {
				ui.Warn("npm install failed — run manually")
			}
		}
	}
}

func execShell(command string) error {
	c := exec.Command("sh", "-c", command)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
