# cmd

Command-line routing, global flag handling and the `project`/`task` command handlers.

## Files

- `cmd.go`: `Context` (vault `Store`, output format, writers) and `Execute` (top-level dispatch).
- `context.go`: `ParseGlobalFlags` (`-F/--format`, hidden `--vault`, accepted anywhere),
  `ResolveVaultPath` (`--vault` > `TASK_CLI_VAULT` > config file > default) and `ResolveContext`.
- `router.go`: `Command` tree, help output and `NewRootCommand` (the command registry).
- `handler.go`: `Action` (Parse → Execute → Format) and `ActionHandler`.
- `project.go`, `task.go`, `config.go`: command handlers. `editor.go`: `openInEditor`. `format.go`: TSV/table output.

## Invariants

- Every value-taking flag must be listed in the command's `splitFlagsAndPosArgs` map,
  otherwise its value becomes a positional argument.
- Projects are referenced by name = folder name (case-insensitive); tasks by numeric ID.
- `deleted` tasks are immutable (`vault.ErrTaskImmutable`).
- `version`, `help` and `config` never open the vault.
