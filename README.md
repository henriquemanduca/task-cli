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
./dist/task-cli lp                                                     # alias: projects
./dist/task-cli lp -d  # Shows descriptions
./dist/task-cli lp -t  # Shows creation dates

# Tasks
./dist/task-cli add "Standalone Task" "Not linked to any project"
./dist/task-cli add -f BACKLOG "Project Task" "Linked to BACKLOG project"
./dist/task-cli add -f BACKLOG --priority high "Urgent Task"
# List tasks (by default shows pending tasks with full title; use -a for all, --done for completed)
./dist/task-cli list           # Pending tasks
./dist/task-cli list -a        # All tasks (any status)
./dist/task-cli list --done    # Only completed tasks
./dist/task-cli list -d        # Include description
./dist/task-cli list -t        # Include creation dates
./dist/task-cli list -a -d -t  # All tasks with full details
./dist/task-cli list in-progress
./dist/task-cli list -f BACKLOG
./dist/task-cli list -a -f BACKLOG

# View full task details (including multiline description paragraphs; add -t to show timestamps)
./dist/task-cli view 1
./dist/task-cli view 1 -t
./dist/task-cli update 1 "Study Go static linking"
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

# Version and help
./dist/task-cli --version
./dist/task-cli --help
```
