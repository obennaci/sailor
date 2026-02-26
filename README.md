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
    ════════╪═════════╪═════════╪══════  sail network
            │         │         │
    ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
    │ App :8080    │ │ App :8081    │ │ App :8082    │
    │ feature-a    │ │ hotfix-b     │ │ feature-b    │
    │ (copy vendor)│ │ (copy vendor)│ │ (copy vendor)│
    └──────────────┘ └──────────────┘ └──────────────┘
```

# Sailor

Run multiple [Laravel Sail](https://laravel.com/docs/sail) branches in parallel using git worktrees.

Your **main branch** runs the full Sail stack (MySQL/PostgreSQL, Redis, Mailpit, etc.). Each additional branch runs only its app container connected to the main Sail network, with its own database, ports, and dependencies.

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
# 1. Start your main branch as usual
sail up -d

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
| `sailor add <branch>` | Create a git worktree with its own DB, ports, .env, and compose override |
| `sailor up` | Start the app container and run pending migrations |
| `sailor down` | Stop the app container |
| `sailor list` | List all worktrees and their status |
| `sailor ports` | Show port allocation across all worktrees |
| `sailor status` | Show Docker container details |
| `sailor remove` | Stop container, drop DB, and remove worktree |

## How It Works

1. **`sailor add <branch>`** does the heavy lifting:
   - Creates a git worktree for the branch
   - Copies `vendor/` and `node_modules/` from main
   - Creates a dedicated database (MySQL or PostgreSQL)
   - Generates a `.env` with unique `APP_PORT` and `VITE_PORT`
   - Writes a `docker-compose.override.yml` that disables infra services and connects to the main Sail network

2. **`sailor up/down`** starts and stops the app container in the current worktree.

3. **`sailor remove`** cleans up everything: stops the container, drops the database, deletes the override file, and removes the git worktree.

The original `docker-compose.yml` is **never modified**. Sailor uses Docker Compose's native [override mechanism](https://docs.docker.com/compose/how-tos/multiple-compose-files/merge/) to layer worktree-specific configuration on top.

## Port Allocation

Sailor automatically assigns unique ports to avoid conflicts:

- **APP_PORT**: starts at `8080`, increments per worktree
- **VITE_PORT**: starts at `5174`, increments per worktree

Use `sailor ports` to see the full allocation map.

## Design Principles

- **Zero config files** - all state is discovered at runtime from git worktrees, compose files, `.env` files, and running containers
- **Non-destructive** - the original `docker-compose.yml` is never touched; overrides are generated and cleaned up automatically
- **Minimal overhead** - worktrees share the main branch's infra services (MySQL, Redis, etc.) over the Sail network

## Requirements

- Go 1.25+
- Git
- Docker & Docker Compose
- A Laravel Sail project

## License

MIT
