```
    ┌──────────────────────────────────────────────┐
    │  sail up -d  (main branch)                   │
    │  ┌────────┐ ┌───────┐ ┌───────┐ ┌─────────┐  │
    │  │ MySQL  │ │       │ │       │ │         │  │
    │  │ db_main│ │ Redis │ │Mailpit│ │ App:80  │  │
    │  │ db_feat│ │       │ │       │ │         │  │
    │  │ ...    │ │       │ │       │ │         │  │
    │  └────┬───┘ └───┬───┘ └───┬───┘ └─────────┘  │
    └───────┼─────────┼─────────┼──────────────────┘
            │         │         │
    ════════╪═════════╪═════════╪══════  sail_shared network
            │         │         │
    ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
    │ App :8080    │ │ App :8081    │ │ App :8081    │
    │ feature-a    │ │ hotfix-b     │ │ feature-b    │
    │ (copy vendor)│ │ (copy vendor)│ │ (copy vendor)│
    └──────────────┘ └──────────────┘ └──────────────┘
```

# Sailor

Run multiple [Laravel Sail](https://laravel.com/docs/sail) branches in parallel using git worktrees.

Your **main branch** runs the full Sail stack (MySQL, Redis, Mailpit, etc.). Each additional branch runs only its app container over a shared Docker network, with its own database, ports, and dependencies.

## Why

Switching branches in a Laravel project means stopping containers, possibly resetting databases, and losing your working state. Sailor lets you run multiple branches side-by-side, each fully isolated, so you can develop on a feature while reviewing a PR on another port.

## Install

```bash
go install github.com/millancore/sailor@latest
```

Or build from source:

```bash
git clone https://github.com/millancore/sailor.git
cd sailor
go build -o sailor ./
```

## Quick Start

```bash
# 1. Initialize Sailor in your Laravel project (creates shared network, patches compose)
sailor init

# 2. Add a branch to work on in parallel
sailor add feature/payments

# 3. Start the branch
cd ../feature-payments   # worktree directory
sailor up

# 4. See what's running
sailor list
sailor ports
```

## Commands

| Command | Description |
|---------|-------------|
| `sailor init` | Create shared Docker network and patch the main docker-compose.yml |
| `sailor add <branch>` | Create a git worktree with its own DB, ports, .env, and patched compose |
| `sailor up` | Start the app container and run pending migrations |
| `sailor down` | Stop the app container |
| `sailor list` | List all worktrees and their status |
| `sailor ports` | Show port allocation across all worktrees |
| `sailor status` | Show Docker container details |
| `sailor remove` | Stop container, drop DB, restore compose backup, remove worktree |

## How It Works

1. **`sailor init`** creates a `sail_shared` Docker network and patches the main branch's `docker-compose.yml` to connect to it.

2. **`sailor add <branch>`** does the heavy lifting:
   - Creates a git worktree for the branch
   - Copies `vendor/` and `node_modules/` from main
   - Creates a dedicated MySQL database
   - Generates a `.env` with unique `APP_PORT` and `VITE_PORT`
   - Patches the worktree's `docker-compose.yml` to disable infra services (only the app container runs)

3. **`sailor up/down`** starts and stops the app container in the current worktree.

4. **`sailor remove`** cleans up everything: stops the container, drops the database, restores the original compose file, and removes the git worktree.

## Port Allocation

Sailor automatically assigns unique ports to avoid conflicts:

- **APP_PORT**: starts at `8080`, increments per worktree
- **VITE_PORT**: starts at `5174`, increments per worktree

Use `sailor ports` to see the full allocation map.

## Design Principles

- **Zero config files** - all state is discovered at runtime from git worktrees, compose files, `.env` files, and running containers
- **Non-destructive** - compose files are backed up before modification and restored on removal
- **Minimal overhead** - worktrees share the main branch's infra services (MySQL, Redis, etc.) over the Docker network

## Requirements

- Go 1.25+
- Git
- Docker & Docker Compose
- A Laravel Sail project

## License

MIT
