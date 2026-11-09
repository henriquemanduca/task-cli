# AGENTS.md

## Project

Single-binary Go CLI for task management backed by SQLite. No server, frontend, or generated code. The module is named `task-cli`, so internal imports are `task-cli/internal/...`.

## Build and test

- `make build` — static build (`CGO_ENABLED=0`) to `dist/task-cli` (gitignored).
- `make test` — runs `go test ./...`.
- Single test: `go test ./internal/cmd -run TestTaskCommands` or `go test ./internal/db -run TestStoreOperations`.
- `make build-all` — cross-compiles Linux amd64/arm64/armv7 into `dist/`; run it before `make checksums` (checksums only matches `dist/task-cli-*`).
- `make run` runs from source; use it only with a throwaway DB.
- `make install-skill` — installs/updates `skill/task-cli.md` (agent skill derived from README) into `~/.agents/skills/task-cli/SKILL.md` (override with `SKILLDIR`).
- No CI, lint, or formatter config exists; `go test ./...` is the only configured verification.
- README says Go 1.22+, but `go.mod` requires 1.26.8; trust `go.mod`.

## Architecture

- `main.go` — global `--version` flag, usage text, dispatch; `--db` and `TASK_CLI_DB` are evaluated at runtime (in `cmd.ResolveContext`) for testing/internal use but hidden from public CLI help (`printUsage`).
- `internal/config/config.go` — discovers the nearest `.taskcli` (walking up to `$HOME`) and parses `.env`-style workspace defaults (currently `PROJECT_CODE`).
- `internal/cmd/` — hierarchical command routing via `Root` in `router.go`; `task.go`, `project.go`, `doc.go`, and `io.go` contain handlers.
- `internal/cmd/alias.go` — flat aliases table (`FlatAliases`) resolving legacy commands (e.g. `lp`, `np`, `add`, `done`, `attach`, `doc-open`, `doc-prune`) to hierarchical routes for backwards compatibility.
- `internal/filestore/` — platform-standard data root resolution, managed attachment copying (with CoW reflink and 100MB threshold), collision-free hash suffixing, directory opening (`xdg-open`/`open`/`explorer.exe`), and orphaned attachment pruning.
- `internal/db/db.go` — SQLite store, schema, CRUD, legacy database migration; `internal/db/transfer.go` — JSON import/export.
- Adding a command requires a handler `func(*cmd.Context, []string) error`, registration in `cmd.Root` (`internal/cmd/router.go`), and updates to `README.md`, `main.go:printUsage`, and `skill/task-cli.md` (these are separate and drift easily).

## CLI gotchas

- Workspace defaults live in `.taskcli` (`.env`-style, nearest file from cwd up to `$HOME`). `PROJECT_CODE` makes `task add` link to that project and `task list` filter by it; an explicit `-p` overrides it, and `-p ""` disables it for that invocation. `version`/`help` skip config loading.
- Global `-F/--format` is a transversal flag and can appear anywhere before, between, or after subcommands. `--db` is also transversal but hidden from public usage; use `--db` or `TASK_CLI_DB` for isolated tests. Always use a temp DB when manually exercising commands.
- Default database is `<data_root>/task-cli.db` based on platform standards (`$XDG_DATA_HOME/task-cli` or `~/.local/share/task-cli` on Linux/BSD, `~/Library/Application Support/task-cli` on macOS, `%LOCALAPPDATA%\task-cli` on Windows). On first run, legacy `~/.task-cli.db` is safely migrated via stream copy with fsync, verified with `PRAGMA quick_check;`, and renamed to `~/.task-cli.db.migrated`.
- Each command parses its own `flag.FlagSet`. `splitFlagsAndPosArgs` lets flags appear before or after positionals, but every value-taking flag must be listed in that command's `valueFlags` map or its value becomes a positional argument.
- Project filters: `add` and `update` link/move tasks using project ID (`-p/--proj/--project <id>`); `list` filters by project code (`-p/--project <code>`); `export` accepts `-f/--filter/--project/--proj` but not `-p` (README's `export -p ...` example is wrong).
- Statuses are `pending`, `in-progress`, `done`, `aborted` (with `abort` accepted as alias); `deleted` is set on soft delete; priorities are `low`, `medium`, `high`, `urgent`. `list` defaults to pending; `-a/--all/all` shows everything.
- `delete` is a soft delete (`deleted_at` set and status changed to `deleted`); normal queries and exports exclude deleted tasks unless export gets `--include-deleted`.
- Tasks with status `deleted` or `aborted` are immutable: status changes and updates (`update`, `edit`, `prio`, etc.) are prohibited and return an error.
- TSV list commands print no header at all when there are zero rows. Task TSV rows use `cleanOneLine` for title (description is viewed via `view`/`show`); keep newline/tab stripping when adding fields, and update the header when adding a column.

## Storage and import/export

- `db.NewStore` creates the schema idempotently, performs legacy database migration (`~/.task-cli.db` -> `<data_root>/task-cli.db`), and runs ad-hoc legacy migrations (for example `todo` → `pending` and `abort` → `aborted`); there is no migration framework.
- Keep the SQLite driver `modernc.org/sqlite` (pure Go). Do not switch to a CGO driver; static `CGO_ENABLED=0` builds are a project requirement.
- Centralized attachment storage: attached files are duplicated into `<data_root>/attachments/<task_id>/<stem>_<hash>.<ext>` via `internal/filestore` (attempting CoW reflink before streaming `io.Copy`). Files > 100MB link by reference by default unless `--copy` is passed; `--link` forces linking by reference without copying. If SQLite insertion fails, the newly copied file is rolled back.
- Attachment retention: deleting tasks or removing attachment records (`task doc remove`) preserves physical files on disk. `task doc prune [--dry-run]` explicitly audits and cleans orphaned files and removes empty task folders.
- Native directory opening: `task doc open <task_id>` and `task doc open --doc <doc_id>` launch the desktop file manager (`xdg-open`, `open`, `explorer.exe`), or print the path to stdout in headless or SSH environments.
- JSON transfer schema is versioned by `db.CurrentSchemaVersion` (currently 1); import rejects other versions.
- Import dedupes projects by case-insensitive code, always inserts tasks as new rows, and is transactional (dry-run rolls back). `--overwrite` only updates project metadata; invalid rows are skipped with warnings unless `--strict`.

## Tests

- `cmd` tests call handlers directly; use `setupTestContext(t)` (`project_test.go`) for a temp DB and `captureStdout` / `runWithStdin` (`io_test.go`) for output and stdin.
- Tests assert exact output substrings (some in Portuguese), so message changes can break tests.

## Release notes

- `install.sh` prefers a local `dist/task-cli-<os>-<arch>`, `dist/task-cli`, or `./task-cli`; otherwise it downloads the release asset `task-cli-<os>-<arch>` from `GITHUB_REPO="seu-usuario/task-cli"`, which is still a placeholder.
