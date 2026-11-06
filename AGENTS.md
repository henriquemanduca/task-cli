# AGENTS.md

## Project

Single-binary Go CLI for task management backed by SQLite. No server, frontend, or generated code. The module is named `task-cli`, so internal imports are `task-cli/internal/...`.

## Build and test

- `make build` — static build (`CGO_ENABLED=0`) to `dist/task-cli`, then copies it to `./task-cli` (both gitignored).
- `make test` — runs `go test ./...`.
- Single test: `go test ./internal/cmd -run TestTaskCommands` or `go test ./internal/db -run TestStoreOperations`.
- `make build-all` — cross-compiles Linux amd64/arm64/armv7 into `dist/`; run it before `make checksums` (checksums only matches `dist/task-cli-*`).
- `make run` runs from source; use it only with a throwaway DB.
- `make install-skill` — installs/updates `SKILL.md` (agent skill derived from README) into `~/.agents/skills/task-cli/` (override with `SKILLDIR`).
- No CI, lint, or formatter config exists; `go test ./...` is the only configured verification.
- README says Go 1.22+, but `go.mod` requires 1.26.8; trust `go.mod`.

## Architecture

- `main.go` — global `--db`/`--version` flags, usage text, dispatch.
- `internal/cmd/` — one handler per command/alias. `cmd.go` holds the `Registry`; `task.go`, `project.go`, `doc.go`, and `io.go` contain handlers.
- `internal/db/db.go` — SQLite store, schema, CRUD; `internal/db/transfer.go` — JSON import/export.
- Adding a command requires a handler `func(*cmd.Context, []string) error`, an entry in `cmd.Registry`, and updates to `README.md`, `main.go:printUsage`, and `SKILL.md` (these are separate and drift easily).

## CLI gotchas

- Global `--db` must appear before the subcommand (`task-cli --db /tmp/x.db list`); after it, it is parsed as a command flag and fails.
- Default DB is `~/.task-cli.db`; override with `TASK_CLI_DB` or `--db`. Always use a temp DB when manually exercising commands.
- Each command parses its own `flag.FlagSet`. `splitFlagsAndPosArgs` lets flags appear before or after positionals, but every value-taking flag must be listed in that command's `valueFlags` map or its value becomes a positional argument.
- Project filters: `add`/`list`/`update` use `-p/--project`; `export` accepts `-f/--filter/--project/--proj` but not `-p` (README's `export -p ...` example is wrong).
- Statuses are `pending`, `in-progress`, `done`, `abort`; priorities are `low`, `medium`, `high`, `urgent`. `list` defaults to pending; `-a/--all/all` shows everything.
- `delete` is a soft delete (`deleted_at`); normal queries and exports exclude deleted tasks unless export gets `--include-deleted`.
- TSV list commands print no header at all when there are zero rows. Task TSV rows use `cleanOneLine` for title (description is viewed via `view`/`show`); keep newline/tab stripping when adding fields, and update the header when adding a column.

## Storage and import/export

- `db.NewStore` creates the schema idempotently and runs ad-hoc legacy migrations (for example `todo` → `pending`); there is no migration framework.
- Keep the SQLite driver `modernc.org/sqlite` (pure Go). Do not switch to a CGO driver; static `CGO_ENABLED=0` builds are a project requirement.
- `attach` requires the file to exist and stores only its absolute path; it does not copy the document into the database.
- JSON transfer schema is versioned by `db.CurrentSchemaVersion` (currently 1); import rejects other versions.
- Import dedupes projects by case-insensitive code, always inserts tasks as new rows, and is transactional (dry-run rolls back). `--overwrite` only updates project metadata; invalid rows are skipped with warnings unless `--strict`.

## Tests

- `cmd` tests call handlers directly; use `setupTestContext(t)` (`project_test.go`) for a temp DB and `captureStdout` / `runWithStdin` (`io_test.go`) for output and stdin.
- Tests assert exact output substrings (some in Portuguese), so message changes can break tests.

## Release notes

- `install.sh` prefers a local `dist/task-cli-<os>-<arch>`, `dist/task-cli`, or `./task-cli`; otherwise it downloads the release asset `task-cli-<os>-<arch>` from `GITHUB_REPO="seu-usuario/task-cli"`, which is still a placeholder.
