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
- `make install-skill`: Installs/updates `SKILL.md` as a global agent skill in `~/.agents/skills/task-cli/` (or `SKILLDIR`).
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

By default, the SQLite database is located at `~/.task-cli.db` (configurable via `TASK_CLI_DB` env variable or `--db` flag).

```bash
# Projects
./dist/task-cli np BACKLOG "Product Backlog" "Main project backlog"   # alias: project-add
./dist/task-cli lp                                                     # alias: projects  (TSV output, with header)
./dist/task-cli lp -n           # Suppress header row
./dist/task-cli lp -d           # Include description column
./dist/task-cli lp -t           # Include creation date column
./dist/task-cli lp -d -t        # Description + date
./dist/task-cli lp -F table     # Formatted table view (alias: --format table)

# Tasks
./dist/task-cli add "Standalone Task" "Not linked to any project"
./dist/task-cli add -p BACKLOG "Project Task" "Linked to BACKLOG project"
./dist/task-cli add -p BACKLOG --priority high "Urgent Task"
# List tasks — TSV output by default; use -F table for human-readable formatted table
./dist/task-cli list                  # Pending tasks (TSV with header)
./dist/task-cli list -F table         # Pending tasks as formatted table
./dist/task-cli list -n               # Pending tasks, no header
./dist/task-cli list -a               # All tasks (any status)
./dist/task-cli list -a -F table      # All tasks formatted as table
./dist/task-cli list --done           # Only completed tasks
./dist/task-cli list -t               # Include creation date column
./dist/task-cli list -a -t            # All tasks with creation date
./dist/task-cli list in-progress
./dist/task-cli list -p BACKLOG
./dist/task-cli list -a -p BACKLOG

# View full task details including description (alias: show; add -t to show timestamps)
./dist/task-cli view 1
./dist/task-cli show 1
./dist/task-cli view 1 -t
./dist/task-cli update 1 "Study Go static linking"
./dist/task-cli update 1 -p BACKLOG          # Move to project
./dist/task-cli update 1 -p none             # Detach from project
./dist/task-cli edit 1 # Opens $EDITOR (defaults to nano) to edit description
./dist/task-cli set-prio 1 high
./dist/task-cli set-pending 1
./dist/task-cli set-in-progress 1
./dist/task-cli set-done 1
./dist/task-cli set-abort 1 "Out of scope"
./dist/task-cli delete 1

# Document attachments
./dist/task-cli attach 1 /path/to/spec.pdf                     # alias: doc-add
./dist/task-cli attach 1 ./notes.txt "Meeting Notes"
./dist/task-cli docs 1                                        # alias: doc-list
./dist/task-cli doc-del 1                                     # alias: doc-remove

# Export & Import (JSON)
./dist/task-cli export                                        # alias: exp (exports all to stdout)
./dist/task-cli export --pretty --out backup.json             # Formatted export to file
./dist/task-cli export -p BACKLOG --out backlog.json          # Export specific project
./dist/task-cli export --status pending                       # Export tasks with status filter
./dist/task-cli export --include-deleted                      # Include soft-deleted tasks
./dist/task-cli export --dry-run                              # Simulate export without writing
./dist/task-cli import backup.json                            # alias: imp (import from file)
./dist/task-cli import backup.json --overwrite                # Overwrite metadata for existing projects
./dist/task-cli import backup.json --dry-run                  # Validate and simulate import
./dist/task-cli import backup.json --strict                   # Abort on first validation error
./dist/task-cli import backup.json --force-docs               # Allow duplicate document attachments
cat backup.json | ./dist/task-cli imp -                       # Import via stdin pipe

# Version and help
./dist/task-cli --version
./dist/task-cli --help
```

## Output Formats (TSV & Table)

`lp` and `list` emit tab-separated values (TSV) by default with a header row, designed for unix pipes and scripts.

```
# list output columns (task descriptions are viewed via 'view' or 'show'):
id  project  status  priority  title  [created_at]

# lp output columns:
id  code  name  [description]  [created_at]
```

Use `-F table` (or `--format table`) for an aligned, human-readable view:

```bash
# human-readable aligned table view
task-cli list -F table
task-cli lp -F table

# with date timestamps
task-cli list -a -t -F table
```

Suppress the header with `-n` / `--no-header` for clean piping:

```bash
# count tasks by priority
task-cli list -a -n | cut -f4 | sort | uniq -c

# filter in-progress tasks in a project
task-cli list -a -n -p BACKLOG | awk -F'\t' '$3=="in-progress"'
```
