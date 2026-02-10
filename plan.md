# sail-worktree — Go Implementation Plan

## Overview

Rewrite `sail-worktree` in Go. Two key design changes from the bash version:

1. **No config files** — Derive all state from git worktrees, docker-compose files, `.env` files, and Docker runtime.
2. **Modify existing `docker-compose.yml` directly** — Instead of generating override files, patch the real compose file to add the shared network.

---

## Architecture Recap

```
Main branch (full Sail stack):
┌──────────────────────────────────────────────┐
│  sail up -d  (main branch)                   │
│  ┌────────┐ ┌───────┐ ┌───────┐ ┌─────────┐ │
│  │ MySQL  │ │ Redis │ │Mailpit│ │  App    │ │
│  └────┬───┘ └───┬───┘ └───┬───┘ └─────────┘ │
└───────┼─────────┼─────────┼──────────────────┘
        │         │         │
════════╪═════════╪═════════╪══════  sail_shared network
        │         │         │
┌───────┼─────────┼─────────┼──────┐
│  App :8080 (worktree-a)          │
│  Only app container, shared infra│
└──────────────────────────────────┘
```

---

## State Discovery (No Config File)

Instead of `.sail-worktree.json`, the tool will **discover state at runtime**:

| What                  | Source                                                                 |
|-----------------------|------------------------------------------------------------------------|
| Git root              | `git rev-parse --git-common-dir`                                       |
| Registered worktrees  | `git worktree list --porcelain`                                        |
| Main branch directory | The worktree whose path == git root                                    |
| App service name      | Parse `docker-compose.yml` → find service with `sail-` image or `laravel.test` key |
| Infra services        | All services in main's `docker-compose.yml` except the app service     |
| DB credentials        | Read `.env` from main branch (`DB_PASSWORD`, `DB_USERNAME`, `DB_DATABASE`) |
| MySQL container       | `docker compose ps --format json` in main dir, find mysql/mariadb service |
| Worktree ports        | Read `APP_PORT` and `VITE_PORT` from each worktree's `.env`           |
| Worktree database     | Read `DB_DATABASE` from each worktree's `.env`                         |
| Running status        | `docker ps` / `docker compose ps` against each worktree directory      |
| Network existence     | `docker network inspect sail_shared`                                   |

### Port Allocation Strategy (Without Config)

Since there's no config to track slots, ports are allocated by:

1. Scan all existing worktree `.env` files for `APP_PORT` values.
2. Collect used ports into a set.
3. Starting from `BASE_APP_PORT` (8080), find the first unused port.
4. Same logic for `VITE_PORT` starting from 5174.

---

## Modifying docker-compose.yml Directly

Instead of creating override files (`docker-compose.sail-worktree.yml`), the tool will **patch the existing `docker-compose.yml`** in-place.

### For the Main Branch (`init`)

Add the `shared` external network to **all services** and to the top-level `networks` block:

```yaml
# Before (original)
services:
  laravel.test:
    networks:
      - sail
  mysql:
    networks:
      - sail

networks:
  sail:
    driver: bridge

# After (patched)
services:
  laravel.test:
    networks:
      - sail
      - shared
  mysql:
    networks:
      - sail
      - shared

networks:
  sail:
    driver: bridge
  shared:
    external: true
    name: sail_shared
```

### For Worktree Branches (`add`)

Patch the worktree's `docker-compose.yml` to:

1. Add `shared` network to the app service.
2. Remove `depends_on` from the app service (infra is external).
3. Override ports for the app service (`APP_PORT:80`, `VITE_PORT:5173`).
4. Disable all infra services via `profiles: ['disabled']`.
5. Add the `shared` external network definition.

```yaml
services:
  laravel.test:
    ports:
      - '8080:80'
      - '5174:5173'
    networks:
      - shared
    depends_on: []
  mysql:
    profiles: ['disabled']
  redis:
    profiles: ['disabled']

networks:
  shared:
    external: true
    name: sail_shared
```

### YAML Handling

Use a **comment-preserving YAML library** to avoid destroying the user's formatting:

- **Option A**: [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) — supports `yaml.Node` for structure-aware manipulation while preserving comments.
- Operate on `yaml.Node` tree, not on marshaled structs, to preserve comments and ordering.

### Backup & Safety

- Before modifying any `docker-compose.yml`, create a backup: `docker-compose.yml.sail-worktree-backup`.
- On `remove`, restore from backup if it exists.
- A `restore` command can revert all modifications.

---

## Project Structure

```
sailor/
├── go.mod
├── go.sum
├── main.go                     # CLI entrypoint (cobra)
├── cmd/
│   ├── root.go                 # Root command, global flags
│   ├── init.go                 # `sail-worktree init`
│   ├── add.go                  # `sail-worktree add <branch>`
│   ├── up.go                   # `sail-worktree up [dir]`
│   ├── down.go                 # `sail-worktree down [dir]`
│   ├── remove.go               # `sail-worktree remove <branch|dir>`
│   ├── list.go                 # `sail-worktree list`
│   ├── ports.go                # `sail-worktree ports`
│   └── status.go               # `sail-worktree status`
├── internal/
│   ├── git/
│   │   └── worktree.go         # Git operations: find root, list worktrees, add/remove
│   ├── docker/
│   │   ├── network.go          # Create/inspect shared network
│   │   ├── compose.go          # Parse & patch docker-compose.yml (yaml.Node)
│   │   ├── container.go        # Docker ps, exec, inspect
│   │   └── mysql.go            # MySQL operations: create DB, dump, check reachable
│   ├── env/
│   │   └── dotenv.go           # Read/write .env files
│   ├── deps/
│   │   └── copy.go             # Copy vendor/ and node_modules/
│   └── ui/
│       └── ui.go               # Colored output helpers (info, success, warn, error, header)
```

---

## Dependencies

| Package                | Purpose                              |
|------------------------|--------------------------------------|
| `github.com/spf13/cobra` | CLI framework                     |
| `gopkg.in/yaml.v3`    | YAML parsing with Node API           |
| `github.com/fatih/color` | Terminal colors                    |
| `os/exec`              | Shell out to git, docker (stdlib)    |

---

## Command Specifications

### `init`

1. Find git root.
2. Detect app service and infra services from `docker-compose.yml`.
3. Create Docker network `sail_shared` if missing.
4. **Patch main's `docker-compose.yml`**:
   - Backup original to `docker-compose.yml.sail-worktree-backup`.
   - Add `shared` network to every service's `networks` list.
   - Add `shared` to top-level `networks` block as external.
5. Add backup file to `.gitignore`.
6. Print usage instructions.

### `add <branch> [directory]`

1. Find git root, detect state from existing worktrees and main's `.env`.
2. Verify or create the branch.
3. `git worktree add <target> <branch>`.
4. Copy `vendor/` and `node_modules/` from main (with lock file diff check).
5. Ensure Laravel storage directory structure.
6. Allocate ports by scanning existing worktree `.env` files for used ports.
7. Create the worktree database in MySQL (if reachable):
   - Prompt: schema-only / full-copy / migrate --seed / skip.
8. Write `.env` in worktree (copy from main, override `APP_PORT`, `APP_URL`, `DB_DATABASE`, `REDIS_PREFIX`, `VITE_PORT`).
9. **Patch worktree's `docker-compose.yml`**:
   - Backup original.
   - Override app service: ports, networks, clear depends_on.
   - Disable infra services with `profiles: ['disabled']`.
   - Add shared network definition.
10. Print summary (branch, directory, database, URLs).

### `up [directory]`

1. Resolve directory (default: `.`).
2. Verify shared network exists.
3. Verify MySQL reachable (warn if not).
4. `docker compose up -d <app_service>` in the target directory.
5. If `.sail-worktree-migrate` marker exists, run `php artisan migrate --seed`, then delete marker.
6. Print app URL.

### `down [directory]`

1. Resolve directory.
2. `docker compose down` in the target directory.

### `remove <branch|directory>`

1. Resolve target to directory and branch.
2. Confirm with user.
3. Stop container (`down`).
4. Drop database if MySQL reachable.
5. **Restore `docker-compose.yml` from backup** in worktree dir.
6. `git worktree remove --force`.
7. Print confirmation.

### `list`

1. Parse `git worktree list --porcelain` to get all worktrees.
2. Identify main vs worktree branches.
3. For each worktree, read `.env` for ports/db, check Docker for running status.
4. Print table: branch, directory, port, database, status.

### `ports`

1. Same discovery as `list`.
2. Print table: branch, app port, vite port, database.

### `status`

1. For main: `docker compose ps` in main directory.
2. For each worktree: `docker compose ps` in worktree directory.

---

## docker-compose.yml Patching — Implementation Detail

### Reading & Modifying with yaml.Node

```go
func PatchComposeAddSharedNetwork(filePath string, services []string, networkName string) error {
    // 1. Read file
    // 2. Decode into yaml.Node (preserves comments, order)
    // 3. Walk to "services" mapping node
    // 4. For each target service, find or create "networks" sequence
    // 5. Append "shared" if not present
    // 6. Walk to "networks" mapping node (create if missing)
    // 7. Add "shared" with external: true, name: sail_shared
    // 8. Encode back, write file
}
```

### Identifying Worktrees vs Main

A worktree managed by this tool is identified by checking:

- It appears in `git worktree list`.
- Its `docker-compose.yml` contains a `shared` network with `name: sail_shared`.
- Its `.env` contains `APP_PORT` different from the default 80.

The **main** branch is the worktree at the git root.

---

## Implementation Order

### Phase 1 — Foundation
1. `go mod init`, install dependencies.
2. Implement `internal/git/worktree.go` — find root, list worktrees.
3. Implement `internal/env/dotenv.go` — read/write `.env`.
4. Implement `internal/ui/ui.go` — colored output.
5. Implement `internal/docker/compose.go` — YAML Node parsing and patching.

### Phase 2 — Core Commands
6. Implement `cmd/init.go` — network creation + compose patching.
7. Implement `cmd/add.go` — full worktree creation flow.
8. Implement `cmd/up.go` and `cmd/down.go`.

### Phase 3 — Management Commands
9. Implement `cmd/list.go`, `cmd/ports.go`, `cmd/status.go`.
10. Implement `cmd/remove.go`.

### Phase 4 — Polish
11. Error handling, edge cases, tests.
12. Build & release (single binary, cross-compile).

---

## Key Differences from Bash Version

| Aspect               | Bash (current)                          | Go (planned)                              |
|----------------------|------------------------------------------|-------------------------------------------|
| Config storage       | `.sail-worktree.json`                    | None — discover from git + docker + .env  |
| Compose modification | Override files (`-f ... -f ...`)         | Patch `docker-compose.yml` in-place       |
| Compose startup      | Requires `-f` flags                      | Just `docker compose up -d` (no flags)    |
| Dependencies         | Hardlinks (`cp -al`) — shared inodes     | Normal copy (`cp -a`) — independent files |
| Distribution         | Script file                              | Single binary                             |
| Port tracking        | Config JSON with slot numbers            | Scan `.env` files for used ports          |
| DB name tracking     | Config JSON                              | Read from worktree `.env` `DB_DATABASE`   |
