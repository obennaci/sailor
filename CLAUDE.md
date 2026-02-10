# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Sailor is a Go CLI tool for running multiple Laravel Sail branches in parallel using git worktrees. The main branch runs the full Sail stack (MySQL, Redis, Mailpit, etc.), and each worktree runs only the app container over a shared Docker network (`sail_shared`), with its own database, ports, and dependencies.

## Build & Run

```bash
go build -o sailor ./          # build binary
go run .                        # run without building
go test ./...                   # run all tests
go test ./internal/docker/...   # run tests for a single package
go vet ./...                    # static analysis
```

No Makefile — use standard Go tooling.

## Architecture

**Entry:** `main.go` → `cmd.Execute()` → Cobra command dispatch.

**Commands** (`cmd/`): 8 subcommands registered in `cmd/root.go`:

| Command | Purpose |
|---------|---------|
| `init` | Create shared Docker network, patch main's docker-compose.yml |
| `add <branch>` | Create worktree: git worktree + copy deps + DB + .env + patch compose |
| `up` | Start app container, run deferred migrations |
| `down` | Stop app container |
| `list` / `ls` | List worktrees with status |
| `ports` | Show port allocation map |
| `status` | Show Docker container details |
| `remove` / `rm` | Stop container, drop DB, restore compose backup, remove worktree |

**Internal packages** (`internal/`):

- **git** — Worktree operations, branch management. Wraps `git` CLI commands.
- **docker** — Compose YAML parsing/patching, container lifecycle, network management, MySQL operations. Uses `yaml.Node` API to preserve comments and formatting when modifying compose files.
- **env** — `.env` file read/write/copy. Preserves comments when updating values.
- **deps** — Copies `vendor/` and `node_modules/` between worktrees, compares lock files to detect if reinstall is needed, creates Laravel storage directory structure.
- **ui** — Colored terminal output via `fatih/color`.

## Key Design Decisions

- **Zero config files:** All state is discovered at runtime from git worktrees, docker-compose.yml files, `.env` files, and running containers.
- **YAML node manipulation:** `docker/compose.go` uses `yaml.Node` directly (not struct marshal/unmarshal) to avoid destroying comments and formatting in docker-compose.yml.
- **Port allocation:** Scans `.env` files across all worktrees to find next available `APP_PORT` (base 8080) and `VITE_PORT` (base 5174).
- **Backup before modify:** Compose files are backed up to `*.sailor-backup` before patching; restored on `remove`.
- **Infra isolation:** Worktree compose files disable infra services with `profiles: ['disabled']` so only the app container runs.

## Conventions

- Commands use Cobra's `RunE` pattern (return `error`, don't `os.Exit`).
- All external tool invocations (git, docker, cp) go through `os/exec.Command`.
- Interactive prompts use `bufio.NewReader(os.Stdin)` — keep them in `cmd/` layer, not in `internal/`.
- Database names are sanitized via `docker.SanitizeDBName()` (alphanumeric + underscore, max 64 chars).
