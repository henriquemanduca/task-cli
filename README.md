# Task CLI

A CLI task management tool written in Go with embedded **SQLite** storage (pure Go driver via `modernc.org/sqlite`, zero CGO/glibc dependency). Built according to the static compilation and distribution practices outlined in [docs/manual_go.md](docs/manual_go.md).

## Prerequisites

- Go 1.22+ (for building)
- GNU Make (optional, for automation)

## Makefile Commands

- `make build`: Builds the static, optimized binary for the current machine (`dist/task-cli`).
- `make build-all`: Cross-compiles static binaries for Linux amd64, arm64, and armv7.
- `make checksums`: Generates SHA-256 hashes (`dist/checksums.txt`).
- `make install`: Installs binary to `~/.local/bin` (or `PREFIX/bin`).
- `make uninstall`: Removes binary from installation directory.
- `make install-skill`: Installs/updates `skill/task-cli.md` as a global agent skill in `~/.agents/skills/task-cli/SKILL.md` (or `SKILLDIR`).
- `make test`: Runs unit tests.
- `make run`: Runs the application locally via `go run .`.
- `make clean`: Removes the `dist/` directory.

## Installation & Update

### Script (`install.sh`)
The `install.sh` script detects your OS and CPU architecture automatically. It installs to `~/.local/bin` without requiring `sudo`:

```bash
# Local install / update from dist/
./install.sh

# Or install system-wide (/usr/local/bin, requires sudo):
./install.sh --system

# Uninstall:
./install.sh --uninstall
```

When hosted in a GitHub repository, users can install/update with a single command:
```bash
curl -fsSL https://raw.githubusercontent.com/seu-usuario/task-cli/main/install.sh | bash
```

## Usage

By default, the SQLite database is located at `~/.task-cli.db` (configurable via `TASK_CLI_DB` env variable or `--db` flag). Global flags such as `--db` and `-F/--format` are transversal and can appear anywhere before, between, or after subcommands.

```bash
# Projects (aggregated under 'project')
./dist/task-cli project add BACKLOG "Product Backlog" "Main project backlog"   # alias: create
./dist/task-cli project list                                                  # alias: ls (TSV output, with header)
./dist/task-cli project list -n           # Suppress header row
./dist/task-cli project list -d           # Include description column
./dist/task-cli project list -t           # Include creation date column
./dist/task-cli project list -d -t        # Description + date
./dist/task-cli project list -F table     # Formatted table view (alias: --format table)
./dist/task-cli project view BACKLOG      # View project details (alias: show)
./dist/task-cli project view BACKLOG -t   # View project details with timestamps
./dist/task-cli np BACKLOG "Product Backlog" "Main project backlog"   # legacy shorthand
./dist/task-cli lp -F table                                           # legacy shorthand

# Tasks (aggregated under 'task'; aliases: add, list, view, update, delete, status, prio)
./dist/task-cli task add "Standalone Task" "Not linked to any project"
./dist/task-cli task add -p BACKLOG "Project Task" "Linked to BACKLOG project"
./dist/task-cli task add -p BACKLOG --priority high "Urgent Task"
# List tasks — TSV output by default; use -F table for human-readable formatted table
./dist/task-cli task list                  # Pending tasks (TSV with header)
./dist/task-cli task list -F table         # Pending tasks as formatted table
./dist/task-cli task list -n               # Pending tasks, no header
./dist/task-cli task list -a               # All tasks (any status)
./dist/task-cli task list -a -F table      # All tasks formatted as table
./dist/task-cli task list --done           # Only completed tasks
./dist/task-cli task list -t               # Include creation date column
./dist/task-cli task list -a -t            # All tasks with creation date
./dist/task-cli task list in-progress
./dist/task-cli task list -p BACKLOG
./dist/task-cli task list -a -p BACKLOG

# View full task details including description (alias: show, get; add -t to show timestamps)
./dist/task-cli task view 1
./dist/task-cli task view 1 -t
./dist/task-cli task update 1 "Study Go static linking"
./dist/task-cli task update 1 -p BACKLOG          # Move to project
./dist/task-cli task update 1 -p none             # Detach from project
./dist/task-cli task edit 1                       # Opens $EDITOR (defaults to nano) to edit description
./dist/task-cli task prio 1 high                  # Set priority (alias: priority, set-prio)
./dist/task-cli task status 1 in-progress         # Set status (pending, in-progress, done, abort)
./dist/task-cli task status 1 abort "Out of scope"
./dist/task-cli task status 1                     # View current status
./dist/task-cli task done 1                       # Helper to mark done
./dist/task-cli task delete 1                     # Soft delete (alias: del, rm, remove)

# Document attachments
./dist/task-cli task doc add 1 /path/to/spec.pdf            # alias: attach, doc-add
./dist/task-cli task doc add 1 ./notes.txt "Meeting Notes"
./dist/task-cli task doc list 1                             # alias: docs, doc-list, ls
./dist/task-cli task doc remove 1                           # alias: doc-del, doc-remove, del, rm

# Export & Import (JSON)
./dist/task-cli export                                        # alias: exp (exports all to stdout)
./dist/task-cli export --pretty --out backup.json             # Formatted export to file
./dist/task-cli export -f BACKLOG --out backlog.json          # Export specific project (-f, --filter, --project)
./dist/task-cli export --status pending                       # Export tasks with status filter
./dist/task-cli export --include-deleted                      # Include soft-deleted tasks
./dist/task-cli export --dry-run                              # Simulate export without writing
./dist/task-cli import backup.json                            # alias: imp (import from file)
./dist/task-cli import backup.json --overwrite                # Overwrite metadata for existing projects
./dist/task-cli import backup.json --dry-run                  # Validate and simulate import
./dist/task-cli import backup.json --strict                   # Abort on first validation error
./dist/task-cli import backup.json --force-docs               # Allow duplicate document attachments
cat backup.json | ./dist/task-cli imp -                       # Import via stdin pipe

# Flat Aliases (Backwards Compatibility)
All traditional flat commands route directly to the hierarchical command tree:
./dist/task-cli add "Quick Task"                              # routes to 'task add'
./dist/task-cli list                                          # routes to 'task list'
./dist/task-cli done 1                                        # routes to 'task set-done'
./dist/task-cli lp                                            # routes to 'project list'
./dist/task-cli np PROJ "My Project"                          # routes to 'project add'
./dist/task-cli attach 1 /path/to/doc.pdf                     # routes to 'task doc add'
./dist/task-cli docs 1                                        # routes to 'task doc list'

# Version and help
./dist/task-cli --version
./dist/task-cli --help
```

## Output Formats (TSV & Table)

`task list` and `project list` emit tab-separated values (TSV) by default with a header row, designed for unix pipes and scripts.

```
# task list output columns (task descriptions are viewed via 'view' or 'show'):
id  project  status  priority  title  [created_at]

# project list output columns:
id  code  name  [description]  [created_at]
```

Use `-F table` (or `--format table`) for an aligned, human-readable view:

```bash
# human-readable aligned table view
task-cli task list -F table
task-cli project list -F table

# with date timestamps
task-cli task list -a -t -F table
```

Suppress the header with `-n` / `--no-header` for clean piping:

```bash
# count tasks by priority
task-cli task list -a -n | cut -f4 | sort | uniq -c

# filter in-progress tasks in a project
task-cli task list -a -n -p BACKLOG | awk -F'\t' '$3=="in-progress"'
```
