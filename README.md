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
./dist/task-cli add -p BACKLOG "Product Backlog" "Main project backlog"
./dist/task-cli list -p

# Tasks
./dist/task-cli add "Standalone Task" "Not linked to any project"
./dist/task-cli add -f BACKLOG "Project Task" "Linked to BACKLOG project"
./dist/task-cli add -f BACKLOG --priority high "Urgent Task"
# List tasks (summarized tabular view)
./dist/task-cli list
./dist/task-cli list pending
./dist/task-cli list in-progress
./dist/task-cli list -f BACKLOG
./dist/task-cli list pending -f BACKLOG

# View full task details (including multiline description paragraphs)
./dist/task-cli view 1
./dist/task-cli update 1 "Study Go static linking"
./dist/task-cli edit 1 # Opens $EDITOR (defaults to nano) to edit description
./dist/task-cli set-pending 1
./dist/task-cli set-in-progress 1
./dist/task-cli set-done 1
./dist/task-cli set-abort 1 "Out of scope"
./dist/task-cli delete 1

# Version and help
./dist/task-cli --version
./dist/task-cli --help
```
