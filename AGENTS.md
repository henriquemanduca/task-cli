# AGENTS.md

## Project

Single-binary Go CLI for task management backed by a **Markdown vault** (folders = projects, `.md` files = tasks). No database, server, frontend, or generated code. The module is named `task-cli`, so internal imports are `task-cli/internal/...`. The only external dependency is `go.yaml.in/yaml/v3`.

## Build and test

- `make build` — static build (`CGO_ENABLED=0`) to `dist/task-cli` (gitignored).
- `make test` — runs `go test ./...`.
- Single test: `go test ./internal/cmd -run TestTaskAdd` or `go test ./internal/vault -run TestTaskFileFormat`.
- `make build-all` — cross-compiles Linux amd64/arm64/armv7 into `dist/`; run it before `make checksums` (checksums only matches `dist/task-cli-*`).
- `make run` runs from source; use it only with a throwaway vault (`TASK_CLI_VAULT=/tmp/v`).
- `make install-skill` — installs/updates `skill/task-cli.md` into `~/.agents/skills/task-cli/SKILL.md` (override with `SKILLDIR`).
- No CI, lint, or formatter config exists; `go test ./...` is the only configured verification.
- `go.mod` requires Go 1.26.8.

## Architecture

- `main.go` — global `--version` flag, usage text (`printUsage`), dispatch.
- `internal/config/config.go` — loads `config.yaml` (`$TASK_CLI_CONFIG` or `os.UserConfigDir()/task-cli/config.yaml`); single key `vault` (default `~/.task-cli-vault`), with `~`/`$VAR` expansion; unknown keys are errors.
- `internal/vault/vault.go` — storage: projects, tasks, frontmatter parsing/writing, IDs, slugs, atomic writes.
- `internal/cmd/` — hierarchical command routing via `NewRootCommand` in `router.go`; `task.go`, `project.go` and `config.go` contain handlers; `context.go` resolves global flags and the vault path; `editor.go` has `openInEditor` (shared by `task edit` and `config`).
- `task-cli config` opens `config.yaml` in `$EDITOR` (template created if missing), validates it with `config.Load` afterwards, and — like `version`/`help` — is skipped by `ResolveContext`, so it works with a broken config.
- Adding a command requires a handler `func(*cmd.Context, []string) error`, registration in `NewRootCommand` (`internal/cmd/router.go`), and updates to `README.md`, `main.go:printUsage`, and `skill/task-cli.md` (these are separate and drift easily).

## Vault format

- Layout: `<vault>/[<project>/]<id>-<slug>.md`; tasks without project live in the vault root. Only one folder level is scanned.
- Projects have no code/ID: **project name = folder name** (any folder in the vault root not starting with `.`; spaces/accents allowed; `/`, `\` and control chars rejected by `validateName`). Names are matched case-insensitively. `project add` creates `<project>/.project.yaml` holding only an optional `description` (written via `yaml.Node`, so comments/unknown keys survive; empty description removes the key). `project update --name` renames the folder (tasks move with it).
- Task file: YAML frontmatter (`status`, `priority`, `created`, `updated`, optional `abort_reason`, `deleted`), then `# Title`, then the description. Unknown frontmatter keys are preserved on rewrite (`taskMeta.Extra`).
- IDs are global: next ID = `max(next_id, highest ID in file names + 1)`. Two files with the same ID make lookups of that ID fail with a "duplicate task ID" error.
- YAML side files (`.task-cli.yaml`, `.project.yaml`) go through `internal/vault/yamlfile.go` (`decodeYAMLFile`, `editYAMLFile`).
- Per-vault settings: `<vault>/.task-cli.yaml` (`internal/vault/settings.go`). Holds `next_id`, reserved (written) before the task file is created. It is edited through `yaml.Node` so comments and unknown keys survive; unknown keys are ignored on read (forward compatibility). Vault-level options go here; user/machine options (like the vault path) go in `config.yaml`.
- Saving a task rewrites it to its canonical path, so title changes rename the file and project changes move it.
- Writes are atomic (temp file + rename in the same folder).

## CLI gotchas

- Global `-F/--format` is transversal and can appear anywhere. `--vault <dir>` (hidden) and `TASK_CLI_VAULT` override the configured vault; always use a temp vault when manually exercising commands.
- Each command parses its own `flag.FlagSet`. `splitFlagsAndPosArgs` lets flags appear before or after positionals, but every value-taking flag must be listed in that command's `valueFlags` map or its value becomes a positional argument.
- `-p/--proj/--project` always takes a project **name** (`add`, `update`, `list`). `update -p none` moves the task back to the vault root.
- Statuses are `pending`, `in-progress`, `done`, `aborted` (with `abort` accepted as alias); `deleted` is set on soft delete; priorities are `low`, `medium`, `high`, `urgent`. `list` defaults to pending; `-a/--all/all` shows everything except deleted.
- `delete` is a soft delete: the task gets `status: deleted` + `deleted:` timestamp and is moved to `<vault>/.trash/[<project>/]` (`vault.TrashDir`). Trashed tasks are not scanned by queries, but `nextID` scans the trash so their IDs are never reused. There is no restore command (move the file back by hand).
- Tasks with status `deleted` or `aborted` are immutable: status changes and updates (`update`, `edit`, `prio`, etc.) return an error.
- `task edit` opens the task's `.md` file itself in `$EDITOR`/`$VISUAL` (default `nano`), then re-saves it through the vault (validates it, bumps `updated`, renames on title change).
- There are no flat top-level aliases (`add`, `done`, `lp`…); only hierarchical routes and their per-node aliases.
- TSV list commands print no header at all when there are zero rows. Task TSV rows use `cleanOneLine` for title; keep newline/tab stripping when adding fields, and update the header when adding a column.

## Tests

- `cmd` tests call handlers directly; use `setupTestContext(t)` (`helpers_test.go`) for a temp vault and `captureStdout` for output.
- `vault` tests override the package-level `now` for deterministic timestamps.
- Tests assert exact output strings, so message changes can break tests.

## Release notes

- `install.sh` prefers a local `dist/task-cli-<os>-<arch>`, `dist/task-cli`, or `./task-cli`; otherwise it downloads the release asset `task-cli-<os>-<arch>` from `GITHUB_REPO="seu-usuario/task-cli"`, which is still a placeholder.
