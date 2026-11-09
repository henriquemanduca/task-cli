# Task CLI Blueprint

## System Overview
Task CLI is a standalone, single-binary task management tool written in Go with embedded SQLite storage (pure-Go driver `modernc.org/sqlite`, zero CGO dependencies). It features hierarchical subcommands (`task`, `project`, `export`, `import`), transversal global flags (`-F/--format`), backwards-compatible flat aliases, and JSON backup/transfer.

## Centralized Storage & Platform Data Roots
`task-cli` uses standard operating system user data directories (`<data_root>`) to store its database and managed task attachments:
- **Linux/BSD:** `$XDG_DATA_HOME/task-cli` (defaults to `~/.local/share/task-cli` when `$XDG_DATA_HOME` is unset or empty).
- **macOS:** `~/Library/Application Support/task-cli`.
- **Windows:** `%LOCALAPPDATA%\task-cli`.

Within `<data_root>`:
- Primary SQLite database: `<data_root>/task-cli.db`
- Attachments repository: `<data_root>/attachments/<task_id>/`

### Safe Legacy Database Migration
When launching `task-cli`, if `<data_root>/task-cli.db` does not exist and a legacy database exists at `~/.task-cli.db`:
1. It streams the legacy database to `<data_root>/task-cli.db` using buffered I/O with `fsync` to avoid cross-device (`EXDEV`) filesystem errors.
2. Validates SQLite integrity on the newly created copy using `PRAGMA quick_check;`.
3. Renames the legacy database to `~/.task-cli.db.migrated` upon successful verification and emits an informational message to `stderr`.

## Document Attachments & Managed Duplication
`task-cli` manages task attachments centrally under `<data_root>/attachments/<task_id>/`, keeping attachment paths invariant to project reassignments or external directory changes:
- **Automatic Duplication:** When attaching a file via `task doc add`, the file is copied into the task's managed directory with a content-hash suffix (`<filename_stem>_<short_hash>.<ext>`, where `<short_hash>` is the first 8 hex characters of the file's SHA-256 hash). This guarantees conflict-free naming while retaining human-readable filenames.
- **100MB Heuristic:** Files exceeding 100MB trigger a warning on `stderr` and are linked by reference (storing the original absolute path without copying) to prevent accidental disk consumption.
- **Flags:**
  - `--copy`: Force physical duplication into `<data_root>/attachments/<task_id>/` even if the file exceeds 100MB.
  - `--link`: Deliberately store the original file path by reference without copying.
- **Rollback Guard:** If database insertion fails after duplicating a file, the newly created copy is deleted automatically to prevent orphaned files.
- **Retention Invariant:** Soft-deleting tasks or removing attachments retains physical files on disk to preserve audit history.

### Document Management Commands
```bash
# Attach a document (copies to <data_root>/attachments/<task_id>/<stem>_<hash>.<ext>)
task-cli task doc add <task_id> <path> [name] [--type <type>] [--copy] [--link]
# Alias: attach, doc-add

# List documents attached to a task
task-cli task doc list <task_id>
# Alias: docs, doc-list, ls

# Remove an attachment link from SQLite (retains physical file on disk)
task-cli task doc remove <doc_id>
# Alias: doc-del, doc-remove, del, rm

# Open task attachment directory in desktop file manager (or prints path in headless/SSH)
task-cli task doc open <task_id>
# Alias: doc-open

# Reveal specific document file in desktop file manager (or prints path in headless/SSH)
task-cli task doc open --doc <doc_id>

# Preview orphaned attachment files not linked to any active task
task-cli task doc prune --dry-run
# Alias: doc-prune -n

# Clean unreferenced attachment files and delete empty task directories
task-cli task doc prune
# Alias: doc-prune
```

In headless, terminal-only, or remote SSH environments without an active GUI display server (`$DISPLAY` or `$WAYLAND_DISPLAY`), `task doc open` cleanly outputs the resolved absolute directory or file path to `stdout`.

## Workspace Configuration (`.taskcli`)
`task-cli` looks for a `.taskcli` file in the current directory and walks up the tree until the user's home directory (inclusive). The nearest file wins; files above `$HOME` are ignored. The format is `.env`-style:

```bash
# .taskcli
PROJECT_CODE=BACKLOG
```

- `PROJECT_CODE` sets the default project **code** for `task add` and `task list`.
- `task add` links new tasks to that project; `task list` filters by it.
- Explicit flags override the file: `task list -p OTHER`.
- Pass an explicit empty project (`task list -p ""` or `task add -p ""`) to ignore the workspace default for that invocation.
- Blank lines and `#` comments are ignored; `export KEY=VALUE` and quoted values are accepted.
- Unknown keys or malformed lines make commands fail with the file path and line number. `version` and `help` do not read the file.

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
- `internal`: [./internal/README.md](./internal/README.md) - Internal implementation packages for configuration loading, routing, command handlers, file storage, and SQLite persistence.
  - `config`: [./internal/config/README.md](./internal/config/README.md) - Workspace `.taskcli` discovery and `.env`-style parsing.
  - `cmd`: [./internal/cmd/README.md](./internal/cmd/README.md) - Command routing, action handlers, flag parsing, and tabular formatting.
  - `db`: [./internal/db/README.md](./internal/db/README.md) - SQLite storage engine, schema migration, and JSON transfer serialization.
  - `filestore`: Platform standard data path resolution, file deduplication, content hashing, folder opening, and attachment pruning.
- `skill`: [./skill/task-cli.md](./skill/task-cli.md) - Agent skill definition for task-cli CLI operations.
- `docs`: [./docs/manual_go.md](./docs/manual_go.md) - Static Go compilation reference guide.

## Global Invariants
- **CGO-Free Pure Go**: All database and system interactions avoid CGO to guarantee static binary compilation (`CGO_ENABLED=0`).
- **Database Concurrency & Integrity**: SQLite runs with `PRAGMA journal_mode=WAL` and `foreign_keys=ON`.
- **Soft Delete Consistency**: Tasks use soft deletes (`deleted_at`), marking status as `deleted`; active queries and JSON exports exclude soft-deleted items unless explicitly requested.
- **Terminal Status Immutability**: Tasks with status `deleted` or `aborted` cannot be modified, recovered, or have their status changed.
- **Strict Format Support**: Output format is constrained to TSV (default) and table views.
- **Centralized Attachment Management**: Attached files are duplicated into `<data_root>/attachments/<task_id>/<stem>_<hash>.<ext>` to protect against external file moves or deletions, subject to a 100MB copy limit unless `--copy` or `--link` is specified.
- **Physical Attachment Retention**: Soft-deleting tasks or removing document attachment records retains physical files on disk; disk cleanup must be invoked explicitly via `task doc prune`.

## Global Side Effects
- Reads the nearest `.taskcli` workspace configuration from the current directory up to `$HOME`.
- Default SQLite database created or updated at the platform data directory `<data_root>/task-cli.db` (`$XDG_DATA_HOME/task-cli` or `~/.local/share/task-cli` on Linux/BSD, `~/Library/Application Support/task-cli` on macOS, `%LOCALAPPDATA%\task-cli` on Windows).
- Automatically migrates legacy database from `~/.task-cli.db` to `<data_root>/task-cli.db` on first run, verifying integrity via SQLite quick check before renaming legacy file to `~/.task-cli.db.migrated`.
- Copies attached files to `<data_root>/attachments/<task_id>/` upon `task doc add` (unless `--link` is passed or file exceeds 100MB without `--copy`).
- Opens desktop file manager or prints path to standard output upon `task doc open`.
- Deletes unreferenced files from `<data_root>/attachments/` and removes empty task folders upon `task doc prune`.
- Temporary markdown files created when executing interactive task description editing (`$EDITOR`).
- File reads and writes during JSON import and export workflows.

## System Verification
- Run: `go test ./...`
- Build: `make build`
