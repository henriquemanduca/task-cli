# Task CLI Blueprint

## System Overview
Task CLI is a standalone, single-binary task management tool written in Go with embedded SQLite storage (pure-Go driver `modernc.org/sqlite`, zero CGO dependencies). It features hierarchical subcommands (`task`, `project`, `export`, `import`), transversal global flags (`--db`, `-F/--format`), backwards-compatible flat aliases, and JSON backup/transfer.

## Local Interface & Invariants
- `main.go:main`: Process entry point orchestrating context resolution, global flag parsing, and router execution.
- Invariants:
  - Calls database cleanup callback via `defer cleanup()` if non-nil (`main.go:cleanup`).
  - Exits with status code 0 if `remainingArgs` is empty after parsing flags (`main.go:main`).
  - Exits with status code 1 if `cmd.ResolveContext` fails or if unhandled command or execution error occurs (`main.go:main`).
- Side Effects:
  - Reads command-line arguments from `os.Args` (`main.go:rawArgs`).
  - Writes version or usage output to standard output (`main.go:printVersion`, `main.go:printUsage`).
  - Writes error messages to standard error and terminates process execution via `os.Exit` (`main.go:main`).

## Subdirectories & System Boundaries
- `internal`: [./internal/README.md](./internal/README.md) - Internal implementation packages for routing, command handlers, and SQLite storage.
  - `internal/cmd`: [./internal/cmd/README.md](./internal/cmd/README.md) - Command routing, action handlers, flag parsing, and tabular formatting.
  - `internal/db`: [./internal/db/README.md](./internal/db/README.md) - SQLite storage engine, schema migration, and JSON transfer serialization.
- `skill`: [./skill/task-cli.md](./skill/task-cli.md) - Agent skill definition for task-cli CLI operations.
- `docs`: [./docs/manual_go.md](./docs/manual_go.md) - Static Go compilation reference guide.

## Global Invariants
- **CGO-Free Pure Go**: All database and system interactions avoid CGO to guarantee static binary compilation (`CGO_ENABLED=0`).
- **Database Concurrency & Integrity**: SQLite runs with `PRAGMA journal_mode=WAL` and `foreign_keys=ON`.
- **Soft Delete Consistency**: Tasks use soft deletes (`deleted_at`), marking status as `deleted`; active queries and JSON exports exclude soft-deleted items unless explicitly requested.
- **Terminal Status Immutability**: Tasks with status `deleted` or `aborted` cannot be modified, recovered, or have their status changed.
- **Strict Format Support**: Output format is constrained to TSV (default) and table views.

## Global Side Effects
- Default SQLite database created or updated at `~/.task-cli.db` (or overridden via `TASK_CLI_DB` / `--db`).
- Temporary markdown files created when executing interactive task description editing (`$EDITOR`).
- File reads and writes during JSON import and export workflows.

## System Verification
- Run: `go test ./...`
- Build: `make build`
